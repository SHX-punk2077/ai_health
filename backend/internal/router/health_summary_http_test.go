package router

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	glogger "gorm.io/gorm/logger"

	"ai_health/internal/config"
	"ai_health/internal/consts"
	"ai_health/internal/dto/request"
	"ai_health/internal/handler"
	jwtmgr "ai_health/internal/pkg/jwt"
	"ai_health/internal/repository"
	"ai_health/internal/service"
)

type healthSummaryHTTPData struct {
	BMI                      *string              `json:"bmi"`
	LatestHeight             *string              `json:"latestHeight"`
	LatestWeight             *string              `json:"latestWeight"`
	LatestBloodSugar         *string              `json:"latestBloodSugar"`
	LatestSystolicPressure   *uint16              `json:"latestSystolicPressure"`
	LatestDiastolicPressure  *uint16              `json:"latestDiastolicPressure"`
	LatestHeartRate          *uint16              `json:"latestHeartRate"`
	LatestSleepDurationHours *string              `json:"latestSleepDurationHours"`
	LastMeasuredAt           *time.Time           `json:"lastMeasuredAt"`
	MeasuredAt               map[string]time.Time `json:"measuredAt"`
	Sources                  map[string]string    `json:"sources"`
	BMISource                *string              `json:"bmiSource"`
}

func TestHTTPHealthInternalErrorDoesNotLeakDatabaseDetail(t *testing.T) {
	db := openRouterDB(t)
	resetRouterTables(t, db)
	store := routerStore(t)
	ctx := context.Background()

	userRole := createRole(t, db, consts.RoleUser, "普通用户")
	createUser(t, db, "health_error_http_alice", userRole)

	tokens := jwtmgr.New("0123456789abcdef0123456789abcdef", "ai_health_test", time.Minute, time.Hour)
	authSvc := service.NewAuthService(
		repository.NewUserRepo(db),
		repository.NewRoleRepo(db),
		repository.NewMenuRepo(db),
		store,
		tokens,
	)

	// 独立连接只供健康仓储使用，随后关闭连接池制造真实的数据库错误，
	// 避免破坏同包其它用例共用的测试连接。
	dsn := os.Getenv("AIH_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("未设置 AIH_TEST_MYSQL_DSN")
	}
	brokenDB, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: glogger.Default.LogMode(glogger.Silent)})
	if err != nil {
		t.Fatalf("打开独立测试连接失败: %v", err)
	}
	brokenSQL, err := brokenDB.DB()
	if err != nil {
		t.Fatalf("获取独立连接失败: %v", err)
	}
	if err := brokenSQL.Close(); err != nil {
		t.Fatalf("关闭独立连接失败: %v", err)
	}

	healthSvc := service.NewHealthService(repository.NewHealthRepo(brokenDB))
	h := handler.New(
		handler.NewAuthHandler(authSvc),
		handler.NewUserHandler(nil),
		handler.NewRoleHandler(nil),
		handler.NewMenuHandler(nil),
		handler.NewHealthHandler(healthSvc),
		handler.NewSportHandler(nil),
		handler.NewAIHandler(nil),
	)
	cfg := &config.Config{}
	cfg.Server.Mode = "test"
	cfg.JWT.Secret = "0123456789abcdef0123456789abcdef"
	cfg.JWT.Issuer = "ai_health_test"
	cfg.JWT.AccessTTL = time.Minute
	cfg.JWT.RefreshTTL = time.Hour
	router := New(cfg, h, tokens, store, authSvc)

	out, err := authSvc.Login(ctx, &request.LoginReq{Account: "health_error_http_alice", Password: "Passw0rd!23"}, "10.9.0.5")
	if err != nil {
		t.Fatalf("登录失败: %v", err)
	}

	w, env := doHealthJSON(t, router, http.MethodGet, "/api/v1/health/summary", out.AccessToken, nil)
	if w.Code != http.StatusOK || env.Code != 10004 {
		t.Fatalf("数据库错误应返回 200/10004，实际 status=%d code=%d", w.Code, env.Code)
	}
	body := w.Body.String()
	for _, forbidden := range []string{"SELECT", "health_record", "health_profile", "database is closed", "127.0.0.1", "mysql", "sql:"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("错误响应泄露数据库细节 %q: %s", forbidden, body)
		}
	}
}

func TestHTTPHealthSummary(t *testing.T) {
	db := openRouterDB(t)
	resetRouterTables(t, db)
	store := routerStore(t)
	ctx := context.Background()

	userRole := createRole(t, db, consts.RoleUser, "普通用户")
	createUser(t, db, "health_summary_http_alice", userRole)

	tokens := jwtmgr.New("0123456789abcdef0123456789abcdef", "ai_health_test", time.Minute, time.Hour)
	authSvc := service.NewAuthService(
		repository.NewUserRepo(db),
		repository.NewRoleRepo(db),
		repository.NewMenuRepo(db),
		store,
		tokens,
	)
	healthSvc := service.NewHealthService(repository.NewHealthRepo(db))
	h := handler.New(
		handler.NewAuthHandler(authSvc),
		handler.NewUserHandler(nil),
		handler.NewRoleHandler(nil),
		handler.NewMenuHandler(nil),
		handler.NewHealthHandler(healthSvc),
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

	out, err := authSvc.Login(ctx, &request.LoginReq{Account: "health_summary_http_alice", Password: "Passw0rd!23"}, "10.9.0.4")
	if err != nil {
		t.Fatalf("登录失败: %v", err)
	}
	token := out.AccessToken

	if w, env := doHealthJSON(t, r, http.MethodGet, "/api/v1/health/summary", "", nil); w.Code != http.StatusUnauthorized || env.Code != 20001 {
		t.Fatalf("未登录汇总应 401/20001: status=%d code=%d", w.Code, env.Code)
	}
	if w, env := doHealthJSON(t, r, http.MethodGet, "/api/v1/health/summary", "bad-token", nil); w.Code != http.StatusUnauthorized || env.Code != 20002 {
		t.Fatalf("坏令牌汇总应 401/20002: status=%d code=%d", w.Code, env.Code)
	}

	// 无档案、无记录：字段为 null，不能补 0。
	_, env := doHealthJSON(t, r, http.MethodGet, "/api/v1/health/summary", token, nil)
	if env.Code != 0 {
		t.Fatalf("空用户汇总失败: code=%d", env.Code)
	}
	var empty healthSummaryHTTPData
	if err := json.Unmarshal(env.Data, &empty); err != nil {
		t.Fatalf("解析空汇总失败: %v", err)
	}
	if empty.BMI != nil || empty.LatestWeight != nil || empty.LatestHeight != nil ||
		empty.LatestSystolicPressure != nil || empty.LastMeasuredAt != nil || empty.BMISource != nil {
		t.Fatalf("空用户汇总应全部为 null: %+v", empty)
	}
	if len(empty.MeasuredAt) != 0 || len(empty.Sources) != 0 {
		t.Fatalf("空用户不应有指标时间或来源: %+v", empty)
	}

	// 不要求创建档案，直接用健康记录生成概览。
	measuredAt := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
	_, env = doHealthJSON(t, r, http.MethodPost, "/api/v1/health/records", token, map[string]any{
		"measuredAt":         measuredAt.Format(time.RFC3339),
		"heightCm":           "175.50",
		"weightKg":           "70.25",
		"bloodSugarMmol":     "5.60",
		"systolicPressure":   120,
		"diastolicPressure":  80,
		"heartRate":          72,
		"sleepDurationHours": "7.50",
	})
	if env.Code != 0 {
		t.Fatalf("创建汇总记录失败: code=%d", env.Code)
	}

	_, env = doHealthJSON(t, r, http.MethodGet, "/api/v1/health/summary", token, nil)
	if env.Code != 0 {
		t.Fatalf("汇总失败: code=%d", env.Code)
	}
	var summary healthSummaryHTTPData
	if err := json.Unmarshal(env.Data, &summary); err != nil {
		t.Fatalf("解析汇总失败: %v", err)
	}
	if summary.BMI == nil || *summary.BMI != "22.81" || summary.BMISource == nil || *summary.BMISource != "record" {
		t.Fatalf("BMI 或来源错误: bmi=%v source=%v", summary.BMI, summary.BMISource)
	}
	if summary.LatestWeight == nil || *summary.LatestWeight != "70.25" ||
		summary.LatestHeight == nil || *summary.LatestHeight != "175.50" ||
		summary.LatestBloodSugar == nil || *summary.LatestBloodSugar != "5.60" {
		t.Fatalf("汇总最新值错误: %+v", summary)
	}
	if summary.LatestSystolicPressure == nil || *summary.LatestSystolicPressure != 120 ||
		summary.LatestDiastolicPressure == nil || *summary.LatestDiastolicPressure != 80 ||
		summary.LatestHeartRate == nil || *summary.LatestHeartRate != 72 ||
		summary.LatestSleepDurationHours == nil || *summary.LatestSleepDurationHours != "7.50" {
		t.Fatalf("汇总整数/睡眠值错误: %+v", summary)
	}
	if summary.LastMeasuredAt == nil || !summary.LastMeasuredAt.Equal(measuredAt) {
		t.Fatalf("lastMeasuredAt 错误: %v", summary.LastMeasuredAt)
	}
	if got := summary.MeasuredAt["weightKg"]; !got.Equal(measuredAt) {
		t.Fatalf("weightKg measuredAt 错误: %v", got)
	}
	if summary.Sources["weightKg"] != "record" || summary.Sources["heightCm"] != "record" {
		t.Fatalf("来源标记错误: %+v", summary.Sources)
	}

	// 后续记录缺少身高时，latestHeight 仍应保留之前的有效值。
	_, env = doHealthJSON(t, r, http.MethodPost, "/api/v1/health/records", token, map[string]any{
		"measuredAt": measuredAt.Add(time.Hour).Format(time.RFC3339),
		"weightKg":   "71.00",
	})
	if env.Code != 0 {
		t.Fatalf("创建后续体重记录失败: code=%d", env.Code)
	}
	_, env = doHealthJSON(t, r, http.MethodGet, "/api/v1/health/summary", token, nil)
	if env.Code != 0 {
		t.Fatalf("二次汇总失败: code=%d", env.Code)
	}
	var second healthSummaryHTTPData
	if err := json.Unmarshal(env.Data, &second); err != nil {
		t.Fatalf("解析二次汇总失败: %v", err)
	}
	if second.LatestWeight == nil || *second.LatestWeight != "71.00" ||
		second.LatestHeight == nil || *second.LatestHeight != "175.50" {
		t.Fatalf("新记录缺身高时不能丢失历史身高: %+v", second)
	}
}
