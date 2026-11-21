package router

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	driver "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"ai_health/internal/config"
	"ai_health/internal/consts"
	"ai_health/internal/dto/request"
	"ai_health/internal/handler"
	"ai_health/internal/model/entity"
	"ai_health/internal/pkg/cache"
	"ai_health/internal/pkg/hash"
	jwtmgr "ai_health/internal/pkg/jwt"
	"ai_health/internal/repository"
	"ai_health/internal/service"
)

// HTTP 层鉴权集成测试：走真实路由 + JWT 中间件 + RBAC 权限闸门。
// 需要 AIH_TEST_MYSQL_DSN 与 AIH_TEST_REDIS_ADDR，未配置时跳过。

var (
	routerOnce sync.Once
	routerDB   *gorm.DB
	routerErr  error
)

var routerTables = []string{
	"sys_user", "sys_user_role", "sys_role", "sys_role_menu", "sys_menu",
	"health_profile", "health_record", "sport_type", "sport_info", "sport_disease_detail",
	"ai_conversation", "ai_message",
}

func openRouterDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("AIH_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("未设置 AIH_TEST_MYSQL_DSN，跳过 HTTP 鉴权集成测试")
	}
	routerOnce.Do(func() {
		cfg, err := driver.ParseDSN(dsn)
		if err != nil {
			routerErr = err
			return
		}
		if !strings.HasSuffix(cfg.DBName, "_test") {
			routerErr = fmt.Errorf("测试库名必须以 _test 结尾: %s", cfg.DBName)
			return
		}
		routerDB, routerErr = gorm.Open(mysql.Open(dsn), &gorm.Config{
			SkipDefaultTransaction: true,
			Logger:                 logger.Default.LogMode(logger.Silent),
			NowFunc:                func() time.Time { return time.Now().UTC() },
		})
	})
	if routerErr != nil {
		t.Fatalf("连接测试库失败: %v", routerErr)
	}
	return routerDB
}

func resetRouterTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	err := db.Connection(func(tx *gorm.DB) error {
		if err := tx.Exec("SET FOREIGN_KEY_CHECKS = 0").Error; err != nil {
			return err
		}
		for _, tb := range routerTables {
			if err := tx.Exec("TRUNCATE TABLE " + tb).Error; err != nil {
				return err
			}
		}
		return tx.Exec("SET FOREIGN_KEY_CHECKS = 1").Error
	})
	if err != nil {
		t.Fatalf("清理测试库失败: %v", err)
	}
}

func routerStore(t *testing.T) *cache.Store {
	t.Helper()
	addr := os.Getenv("AIH_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("未设置 AIH_TEST_REDIS_ADDR，跳过 HTTP 鉴权集成测试")
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr, DB: 15})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("连接测试 Redis 失败（%s）: %v", addr, err)
	}
	if err := rdb.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("清空测试 Redis 失败: %v", err)
	}
	t.Cleanup(func() { _ = rdb.Close() })
	return cache.New(rdb)
}

func createRole(t *testing.T, db *gorm.DB, code, name string) *entity.SysRole {
	t.Helper()
	role := &entity.SysRole{RoleCode: code, RoleName: name, Status: consts.StatusEnabled}
	if err := db.Create(role).Error; err != nil {
		t.Fatalf("创建角色失败: %v", err)
	}
	return role
}

func createMenu(t *testing.T, db *gorm.DB, name, permission string) *entity.SysMenu {
	t.Helper()
	p := permission
	m := &entity.SysMenu{MenuType: entity.MenuTypeButton, Name: name, Title: name, Permission: &p, IsEnabled: true}
	if err := db.Create(m).Error; err != nil {
		t.Fatalf("创建菜单失败: %v", err)
	}
	return m
}

func createUser(t *testing.T, db *gorm.DB, username string, roles ...*entity.SysRole) *entity.SysUser {
	t.Helper()
	pwd, err := hash.Hash("Passw0rd!23")
	if err != nil {
		t.Fatalf("生成密码失败: %v", err)
	}
	u := &entity.SysUser{Username: username, PasswordHash: &pwd, Status: consts.StatusEnabled}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}
	for _, role := range roles {
		if err := db.Create(&entity.SysUserRole{UserID: u.ID, RoleID: role.ID}).Error; err != nil {
			t.Fatalf("绑定角色失败: %v", err)
		}
	}
	return u
}

type apiEnvelope struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func doRequest(t *testing.T, h http.Handler, method, path, token string) (*httptest.ResponseRecorder, apiEnvelope) {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	var body apiEnvelope
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	return w, body
}

func TestHTTPAuthorizationAndSportPath(t *testing.T) {
	db := openRouterDB(t)
	resetRouterTables(t, db)
	store := routerStore(t)
	ctx := context.Background()

	superRole := createRole(t, db, consts.RoleSuperAdmin, "超级管理员")
	adminRole := createRole(t, db, consts.RoleAdmin, "管理员")
	userRole := createRole(t, db, consts.RoleUser, "普通用户")

	// admin 拥有 role:list / menu:list，普通用户没有任何管理权限。
	roleList := createMenu(t, db, "RoleList", "role:list")
	menuList := createMenu(t, db, "MenuList", "menu:list")
	for _, m := range []*entity.SysMenu{roleList, menuList} {
		if err := db.Create(&entity.SysRoleMenu{RoleID: adminRole.ID, MenuID: m.ID}).Error; err != nil {
			t.Fatalf("授权管理员失败: %v", err)
		}
	}

	createUser(t, db, "super", superRole)
	createUser(t, db, "admin", adminRole)
	createUser(t, db, "plain", userRole)

	tokens := jwtmgr.New("0123456789abcdef0123456789abcdef", "ai_health_test", time.Minute, time.Hour)
	authSvc := service.NewAuthService(
		repository.NewUserRepo(db),
		repository.NewRoleRepo(db),
		repository.NewMenuRepo(db),
		store,
		tokens,
	)
	userSvc := service.NewUserService(authSvc, repository.NewUserRepo(db), repository.NewRoleRepo(db), store)
	roleSvc := service.NewRoleService(repository.NewRoleRepo(db), repository.NewMenuRepo(db), repository.NewUserRepo(db), store)
	menuSvc := service.NewMenuService(repository.NewMenuRepo(db), repository.NewRoleRepo(db), repository.NewUserRepo(db), store)

	h := handler.New(
		handler.NewAuthHandler(authSvc),
		handler.NewUserHandler(userSvc),
		handler.NewRoleHandler(roleSvc),
		handler.NewMenuHandler(menuSvc),
		handler.NewHealthHandler(nil),
		handler.NewSportHandler(nil),
		handler.NewAIHandler(nil),
	)
	cfg := &config.Config{}
	cfg.Server.Mode = "test"
	cfg.JWT.Secret = "0123456789abcdef0123456789abcdef"
	cfg.JWT.Issuer = "ai_health_test"
	cfg.JWT.AccessTTL = time.Minute
	cfg.JWT.RefreshTTL = time.Hour
	r := New(cfg, h, tokens, store, authSvc)

	login := func(account string) string {
		out, err := authSvc.Login(ctx, &request.LoginReq{Account: account, Password: "Passw0rd!23"}, "10.8.0.1")
		if err != nil {
			t.Fatalf("%s 登录失败: %v", account, err)
		}
		return out.AccessToken
	}
	superToken := login("super")
	adminToken := login("admin")
	plainToken := login("plain")

	// 未登录 401。
	if w, _ := doRequest(t, r, http.MethodGet, "/api/v1/roles", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("未登录访问 /roles 状态 = %d, want 401", w.Code)
	}

	// 普通用户访问管理接口 403（不是 401，也不是 200）。
	for _, path := range []string{"/api/v1/roles", "/api/v1/menus", "/api/v1/users", "/api/v1/users/1/roles"} {
		w, body := doRequest(t, r, http.MethodGet, path, plainToken)
		if w.Code != http.StatusForbidden {
			t.Fatalf("普通用户访问 %s 状态 = %d, want 403, body=%s", path, w.Code, w.Body.String())
		}
		if body.Code != 20005 {
			t.Fatalf("普通用户访问 %s code = %d, want 20005", path, body.Code)
		}
	}

	// 管理员凭 role:list / menu:list 正常访问。
	if w, _ := doRequest(t, r, http.MethodGet, "/api/v1/roles", adminToken); w.Code != http.StatusOK {
		t.Fatalf("管理员访问 /roles 状态 = %d, body=%s", w.Code, w.Body.String())
	}
	if w, _ := doRequest(t, r, http.MethodGet, "/api/v1/menus/tree", adminToken); w.Code != http.StatusOK {
		t.Fatalf("管理员访问 /menus/tree 状态 = %d, body=%s", w.Code, w.Body.String())
	}
	// 管理员没有 role:assign，不能给人分配角色。
	if w, _ := doRequest(t, r, http.MethodPut, "/api/v1/roles/1/menus", adminToken); w.Code != http.StatusForbidden {
		t.Fatalf("管理员访问 /roles/:id/menus 状态 = %d, want 403", w.Code)
	}

	// super_admin 的 * 通配符放行全部权限位。
	if w, _ := doRequest(t, r, http.MethodGet, "/api/v1/roles", superToken); w.Code != http.StatusOK {
		t.Fatalf("超级管理员访问 /roles 状态 = %d, body=%s", w.Code, w.Body.String())
	}
	if w, _ := doRequest(t, r, http.MethodGet, "/api/v1/menus", superToken); w.Code != http.StatusOK {
		t.Fatalf("超级管理员访问 /menus 状态 = %d, body=%s", w.Code, w.Body.String())
	}

	// 运动路由已改为单数；旧路径必须 404，避免前端/文档继续调用旧契约。
	if w, body := doRequest(t, r, http.MethodGet, "/api/v1/sport/types", plainToken); w.Code != http.StatusOK || body.Code != 10003 {
		t.Fatalf("新运动路径响应异常: status=%d body=%s", w.Code, w.Body.String())
	}
	if w, _ := doRequest(t, r, http.MethodGet, "/api/v1/sports/types", plainToken); w.Code != http.StatusNotFound {
		t.Fatalf("旧运动路径应 404，实际 = %d", w.Code)
	}
}
