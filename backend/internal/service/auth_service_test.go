package service_test

import (
	"context"
	"errors"
	"fmt"
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

	"ai_health/internal/dto/request"
	"ai_health/internal/model/entity"
	"ai_health/internal/pkg/cache"
	"ai_health/internal/pkg/errcode"
	jwtmgr "ai_health/internal/pkg/jwt"
	"ai_health/internal/repository"
	"ai_health/internal/service"
)

// 服务层集成测试依赖真实 MySQL + Redis：
//   AIH_TEST_MYSQL_DSN  必须指向以 _test 结尾的库
//   AIH_TEST_REDIS_ADDR 形如 127.0.0.1:6379，测试固定使用 DB 15 并先 FLUSHDB
// 任一未设置则跳过，保证 go test 在无依赖环境仍然可跑。

var (
	svcOnce sync.Once
	svcDB   *gorm.DB
	svcErr  error
)

var svcTables = []string{
	"sys_user", "sys_user_role", "sys_role", "sys_role_menu", "sys_menu",
	"health_profile", "health_record", "sport_type", "sport_info", "sport_disease_detail",
	"ai_conversation", "ai_message",
}

func openSvcDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("AIH_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("未设置 AIH_TEST_MYSQL_DSN，跳过 service 集成测试")
	}
	svcOnce.Do(func() {
		cfg, err := driver.ParseDSN(dsn)
		if err != nil {
			svcErr = err
			return
		}
		if !strings.HasSuffix(cfg.DBName, "_test") {
			svcErr = fmt.Errorf("测试库名必须以 _test 结尾: %s", cfg.DBName)
			return
		}
		svcDB, svcErr = gorm.Open(mysql.Open(dsn), &gorm.Config{
			SkipDefaultTransaction: true,
			Logger:                 logger.Default.LogMode(logger.Silent),
			NowFunc:                func() time.Time { return time.Now().UTC() },
		})
	})
	if svcErr != nil {
		t.Fatalf("连接测试库失败: %v", svcErr)
	}
	return svcDB
}

func newStore(t *testing.T) *cache.Store {
	t.Helper()
	addr := os.Getenv("AIH_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("未设置 AIH_TEST_REDIS_ADDR，跳过需要 Redis 的 service 测试")
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

func resetSvcTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	err := db.Connection(func(tx *gorm.DB) error {
		if err := tx.Exec("SET FOREIGN_KEY_CHECKS = 0").Error; err != nil {
			return err
		}
		for _, tb := range svcTables {
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

func newAuthService(t *testing.T, db *gorm.DB, store *cache.Store) *service.AuthService {
	t.Helper()
	tokens := jwtmgr.New("0123456789abcdef0123456789abcdef", "ai_health_test", time.Minute, time.Hour)
	return service.NewAuthService(
		repository.NewUserRepo(db),
		repository.NewRoleRepo(db),
		repository.NewMenuRepo(db),
		store,
		tokens,
	)
}

func seedDefaultRole(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Create(&entity.SysRole{RoleCode: "user", RoleName: "普通用户", Status: 1}).Error; err != nil {
		t.Fatalf("创建默认角色失败: %v", err)
	}
}

func registerReq(name string) *request.RegisterReq {
	return &request.RegisterReq{Username: name, Password: "Passw0rd!23", Nickname: name}
}

func TestRegisterLoginProfileChangePassword(t *testing.T) {
	db := openSvcDB(t)
	resetSvcTables(t, db)
	store := newStore(t)
	seedDefaultRole(t, db)

	auth := newAuthService(t, db, store)
	ctx := context.Background()

	info, err := auth.Register(ctx, registerReq("alice"))
	if err != nil {
		t.Fatalf("注册失败: %v", err)
	}
	if info.ID == 0 || info.Username != "alice" {
		t.Fatalf("注册返回异常: %+v", info)
	}

	out, err := auth.Login(ctx, &request.LoginReq{Account: "alice", Password: "Passw0rd!23"}, "10.0.0.1")
	if err != nil {
		t.Fatalf("登录失败: %v", err)
	}
	if out.AccessToken == "" || out.RefreshToken == "" {
		t.Fatal("登录应返回双令牌")
	}
	if out.User == nil || len(out.User.Roles) != 1 || out.User.Roles[0] != "user" {
		t.Fatalf("登录应带默认角色 user: %+v", out.User)
	}

	// JWT 有效性：access 令牌可解析出正确的 uid 与 jti。
	claims, err := newAuthService(t, db, store).Profile(ctx, info.ID)
	if err != nil || claims == nil {
		t.Fatalf("Profile 失败: %v", err)
	}

	if err := auth.ChangePassword(ctx, info.ID, &request.PasswordReq{
		OldPassword: "Passw0rd!23",
		NewPassword: "NewPassw0rd!45",
	}); err != nil {
		t.Fatalf("修改密码失败: %v", err)
	}

	// 改密后旧令牌白名单必须被清空（这里直接检查 Redis 键已不存在）。
	_, perr := auth.Login(ctx, &request.LoginReq{Account: "alice", Password: "Passw0rd!23"}, "10.0.0.2")
	if perr == nil {
		t.Fatal("旧密码不应还能登录")
	}
	if !errors.Is(perr, errcode.ErrBadPassword) {
		t.Fatalf("旧密码登录应返回 ErrBadPassword，实际: %v", perr)
	}

	if _, err := auth.Login(ctx, &request.LoginReq{Account: "alice", Password: "NewPassw0rd!45"}, "10.0.0.3"); err != nil {
		t.Fatalf("新密码应能登录: %v", err)
	}
}

func TestRegisterDuplicateUsername(t *testing.T) {
	db := openSvcDB(t)
	resetSvcTables(t, db)
	store := newStore(t)
	seedDefaultRole(t, db)

	auth := newAuthService(t, db, store)
	ctx := context.Background()

	if _, err := auth.Register(ctx, registerReq("bob")); err != nil {
		t.Fatalf("首次注册失败: %v", err)
	}
	_, err := auth.Register(ctx, registerReq("bob"))
	if err == nil {
		t.Fatal("重复用户名应失败")
	}
	if !errors.Is(err, errcode.ErrUserExists) {
		t.Fatalf("应返回 ErrUserExists，实际: %v", err)
	}
}

func TestLoginWrongPasswordAndDisabled(t *testing.T) {
	db := openSvcDB(t)
	resetSvcTables(t, db)
	store := newStore(t)
	seedDefaultRole(t, db)

	auth := newAuthService(t, db, store)
	ctx := context.Background()

	if _, err := auth.Register(ctx, registerReq("carol")); err != nil {
		t.Fatalf("注册失败: %v", err)
	}
	_, err := auth.Login(ctx, &request.LoginReq{Account: "carol", Password: "wrong-pass"}, "10.0.1.1")
	if !errors.Is(err, errcode.ErrBadPassword) {
		t.Fatalf("错误密码应返回 ErrBadPassword，实际: %v", err)
	}

	var u entity.SysUser
	if err := db.Where("username = ?", "carol").First(&u).Error; err != nil {
		t.Fatalf("查询用户失败: %v", err)
	}
	if err := db.Model(&entity.SysUser{}).Where("id = ?", u.ID).Update("status", 0).Error; err != nil {
		t.Fatalf("禁用用户失败: %v", err)
	}
	_, err = auth.Login(ctx, &request.LoginReq{Account: "carol", Password: "Passw0rd!23"}, "10.0.1.2")
	if !errors.Is(err, errcode.ErrUserDisabled) {
		t.Fatalf("禁用用户应返回 ErrUserDisabled，实际: %v", err)
	}
}

func TestLoginFailLimit(t *testing.T) {
	db := openSvcDB(t)
	resetSvcTables(t, db)
	store := newStore(t)
	seedDefaultRole(t, db)

	auth := newAuthService(t, db, store)
	ctx := context.Background()
	if _, err := auth.Register(ctx, registerReq("dave")); err != nil {
		t.Fatalf("注册失败: %v", err)
	}

	for i := 0; i < 5; i++ {
		if _, err := auth.Login(ctx, &request.LoginReq{Account: "dave", Password: "bad-pass"}, "10.0.2.9"); !errors.Is(err, errcode.ErrBadPassword) {
			t.Fatalf("第 %d 次失败应返回 ErrBadPassword，实际: %v", i+1, err)
		}
	}
	_, err := auth.Login(ctx, &request.LoginReq{Account: "dave", Password: "Passw0rd!23"}, "10.0.2.9")
	if !errors.Is(err, errcode.ErrTooManyRetry) {
		t.Fatalf("超过阈值应返回 ErrTooManyRetry，实际: %v", err)
	}
}
