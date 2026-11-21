package router

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"ai_health/internal/config"
	"ai_health/internal/consts"
	"ai_health/internal/dto/request"
	"ai_health/internal/handler"
	jwtmgr "ai_health/internal/pkg/jwt"
	"ai_health/internal/repository"
	"ai_health/internal/service"
)

type healthWarningData struct {
	Metric  string `json:"metric"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

type healthWarnRecordData struct {
	ID       string              `json:"id"`
	WeightKg *string             `json:"weightKg"`
	Warnings []healthWarningData `json:"warnings"`
}

func TestHTTPHealthValidationAndWarnings(t *testing.T) {
	db := openRouterDB(t)
	resetRouterTables(t, db)
	store := routerStore(t)
	ctx := context.Background()

	userRole := createRole(t, db, consts.RoleUser, "普通用户")
	createUser(t, db, "health_validation_http_alice", userRole)

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

	out, err := authSvc.Login(ctx, &request.LoginReq{Account: "health_validation_http_alice", Password: "Passw0rd!23"}, "10.9.0.3")
	if err != nil {
		t.Fatalf("登录失败: %v", err)
	}
	token := out.AccessToken
	now := time.Now().UTC()

	// 明显越界的数据拒绝写入，错误响应不能泄露 SQL / 表结构。
	w, env := doHealthJSON(t, r, http.MethodPost, "/api/v1/health/records", token, map[string]any{
		"measuredAt": now.Add(-2 * time.Hour).Format(time.RFC3339),
		"heightCm":   "29.99",
	})
	if env.Code != 40010 {
		t.Fatalf("身高 29.99 应返回 40010，实际 code=%d", env.Code)
	}
	raw := strings.ToLower(w.Body.String())
	for _, sensitive := range []string{"select ", "health_record", "sql", "mysql"} {
		if strings.Contains(raw, sensitive) {
			t.Fatalf("错误响应泄露内部信息 %q: %s", sensitive, w.Body.String())
		}
	}

	// 收缩压不大于舒张压。
	if _, env := doHealthJSON(t, r, http.MethodPost, "/api/v1/health/records", token, map[string]any{
		"measuredAt":        now.Add(-2 * time.Hour).Format(time.RFC3339),
		"systolicPressure":  80,
		"diastolicPressure": 90,
	}); env.Code != 40010 {
		t.Fatalf("收缩压不大于舒张压应返回 40010，实际 code=%d", env.Code)
	}

	// 可疑数据保存成功并返回提醒。
	_, env = doHealthJSON(t, r, http.MethodPost, "/api/v1/health/records", token, map[string]any{
		"measuredAt":         now.Add(-3 * time.Hour).Format(time.RFC3339),
		"systolicPressure":   150,
		"diastolicPressure":  95,
		"bloodSugarMmol":     "8.50",
		"heartRate":          110,
		"sleepDurationHours": "3.00",
	})
	if env.Code != 0 {
		t.Fatalf("可疑数据应允许保存: code=%d", env.Code)
	}
	var high healthWarnRecordData
	if err := json.Unmarshal(env.Data, &high); err != nil {
		t.Fatalf("解析可疑数据响应失败: %v", err)
	}
	seen := map[string]bool{}
	for _, warning := range high.Warnings {
		seen[warning.Metric] = true
		if warning.Level != "warning" || warning.Message == "" {
			t.Fatalf("提醒结构异常: %+v", warning)
		}
	}
	for _, metric := range []string{"systolicPressure", "diastolicPressure", "bloodSugarMmol", "heartRate", "sleepDurationHours"} {
		if !seen[metric] {
			t.Fatalf("缺少 %s 提醒: %+v", metric, high.Warnings)
		}
	}

	// 单侧血压更新必须结合已有另一侧值。
	if _, env := doHealthJSON(t, r, http.MethodPut, "/api/v1/health/records/"+high.ID, token, map[string]any{
		"systolicPressure": 70,
	}); env.Code != 40010 {
		t.Fatalf("单侧收缩压更新应结合已有舒张压返回 40010，实际 code=%d", env.Code)
	}

	// 正常数据不返回提醒。
	_, env = doHealthJSON(t, r, http.MethodPost, "/api/v1/health/records", token, map[string]any{
		"measuredAt":        now.Add(-90 * time.Minute).Format(time.RFC3339),
		"weightKg":          "70.00",
		"systolicPressure":  120,
		"diastolicPressure": 80,
		"heartRate":         72,
	})
	if env.Code != 0 {
		t.Fatalf("正常记录保存失败: code=%d", env.Code)
	}
	var normal healthWarnRecordData
	if err := json.Unmarshal(env.Data, &normal); err != nil {
		t.Fatalf("解析正常记录响应失败: %v", err)
	}
	if len(normal.Warnings) != 0 {
		t.Fatalf("正常记录不应有提醒: %+v", normal.Warnings)
	}

	// 7 天内的明显体重变化返回 weightKg 提醒。
	_, env = doHealthJSON(t, r, http.MethodPost, "/api/v1/health/records", token, map[string]any{
		"measuredAt": now.Add(-6 * 24 * time.Hour).Format(time.RFC3339),
		"weightKg":   "70.00",
	})
	if env.Code != 0 {
		t.Fatalf("创建历史体重失败: code=%d", env.Code)
	}
	_, env = doHealthJSON(t, r, http.MethodPost, "/api/v1/health/records", token, map[string]any{
		"measuredAt": now.Add(-30 * time.Minute).Format(time.RFC3339),
		"weightKg":   "72.50",
	})
	if env.Code != 0 {
		t.Fatalf("创建最新体重失败: code=%d", env.Code)
	}
	var changed healthWarnRecordData
	if err := json.Unmarshal(env.Data, &changed); err != nil {
		t.Fatalf("解析体重变化响应失败: %v", err)
	}
	foundWeight := false
	for _, warning := range changed.Warnings {
		if warning.Metric == "weightKg" {
			foundWeight = true
		}
	}
	if !foundWeight {
		t.Fatalf("明显体重变化应返回 weightKg 提醒: %+v", changed.Warnings)
	}
}
