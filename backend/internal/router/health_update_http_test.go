package router

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
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

type healthUpdateRecordData struct {
	ID         string  `json:"id"`
	UserID     string  `json:"userId"`
	HeightCm   *string `json:"heightCm"`
	WeightKg   *string `json:"weightKg"`
	Systolic   *uint16 `json:"systolicPressure"`
	Remark     *string `json:"remark"`
	MeasuredAt string  `json:"measuredAt"`
	Source     string  `json:"source"`
}

func TestHTTPHealthRecordUpdate(t *testing.T) {
	db := openRouterDB(t)
	resetRouterTables(t, db)
	store := routerStore(t)
	ctx := context.Background()

	userRole := createRole(t, db, consts.RoleUser, "普通用户")
	alice := createUser(t, db, "health_update_http_alice", userRole)
	bob := createUser(t, db, "health_update_http_bob", userRole)

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

	login := func(account string) string {
		out, err := authSvc.Login(ctx, &request.LoginReq{Account: account, Password: "Passw0rd!23"}, "10.9.0.2")
		if err != nil {
			t.Fatalf("%s 登录失败: %v", account, err)
		}
		return out.AccessToken
	}
	aliceToken := login("health_update_http_alice")
	bobToken := login("health_update_http_bob")

	measuredAt := time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339)
	_, env := doHealthJSON(t, r, http.MethodPost, "/api/v1/health/records", aliceToken, map[string]any{
		"measuredAt":        measuredAt,
		"heightCm":          "175.50",
		"weightKg":          "56.00",
		"systolicPressure":  120,
		"diastolicPressure": 80,
		"source":            "manual",
		"remark":            "初始",
	})
	if env.Code != 0 {
		t.Fatalf("创建记录失败: code=%d", env.Code)
	}
	var created healthUpdateRecordData
	if err := json.Unmarshal(env.Data, &created); err != nil {
		t.Fatalf("解析创建记录失败: %v", err)
	}
	path := "/api/v1/health/records/" + created.ID

	// 未登录、坏令牌都不能更新。
	if w, env := doHealthJSON(t, r, http.MethodPut, path, "", map[string]any{"weightKg": "57.00"}); w.Code != http.StatusUnauthorized || env.Code != 20001 {
		t.Fatalf("未登录更新应 401/20001: status=%d code=%d", w.Code, env.Code)
	}
	if w, env := doHealthJSON(t, r, http.MethodPut, path, "bad-token", map[string]any{"weightKg": "57.00"}); w.Code != http.StatusUnauthorized || env.Code != 20002 {
		t.Fatalf("坏令牌更新应 401/20002: status=%d code=%d", w.Code, env.Code)
	}

	// 正常部分更新。
	_, env = doHealthJSON(t, r, http.MethodPut, path, aliceToken, map[string]any{"weightKg": "57.25"})
	if env.Code != 0 {
		t.Fatalf("更新记录失败: code=%d", env.Code)
	}
	var updated healthUpdateRecordData
	if err := json.Unmarshal(env.Data, &updated); err != nil {
		t.Fatalf("解析更新记录失败: %v", err)
	}
	if updated.ID != created.ID || updated.UserID != strconv.FormatUint(alice.ID, 10) ||
		updated.WeightKg == nil || *updated.WeightKg != "57.25" ||
		updated.HeightCm == nil || *updated.HeightCm != "175.50" ||
		updated.Systolic == nil || *updated.Systolic != 120 {
		t.Fatalf("更新结果不符合部分更新语义: %+v", updated)
	}

	// 受保护字段明确拒绝，不能静默忽略，也不能改变数据。
	for _, body := range []map[string]any{
		{"weightKg": "58.00", "userId": bob.ID},
		{"weightKg": "58.00", "id": created.ID},
		{"weightKg": "58.00", "createdAt": time.Now().UTC().Format(time.RFC3339)},
		{"weightKg": "58.00", "source": "device"},
	} {
		if _, env := doHealthJSON(t, r, http.MethodPut, path, aliceToken, body); env.Code != 10001 {
			t.Fatalf("受保护字段应返回 10001: body=%v code=%d", body, env.Code)
		}
	}
	_, env = doHealthJSON(t, r, http.MethodGet, path, aliceToken, nil)
	if env.Code != 0 {
		t.Fatalf("读取记录失败: code=%d", env.Code)
	}
	if err := json.Unmarshal(env.Data, &updated); err != nil {
		t.Fatalf("解析记录失败: %v", err)
	}
	if updated.WeightKg == nil || *updated.WeightKg != "57.25" {
		t.Fatalf("受保护字段请求不应改变体重: %+v", updated.WeightKg)
	}

	// 空对象、非法数值、非法时间和不存在记录。
	if _, env := doHealthJSON(t, r, http.MethodPut, path, aliceToken, map[string]any{}); env.Code != 10001 {
		t.Fatalf("空更新对象应返回 10001，实际 code=%d", env.Code)
	}
	if _, env := doHealthJSON(t, r, http.MethodPut, path, aliceToken, map[string]any{"weightKg": "12.345"}); env.Code != 40010 {
		t.Fatalf("非法 DECIMAL 应返回 40010，实际 code=%d", env.Code)
	}
	if _, env := doHealthJSON(t, r, http.MethodPut, path, aliceToken, map[string]any{"measuredAt": "not-a-time"}); env.Code != 40009 {
		t.Fatalf("非法测量时间应返回 40009，实际 code=%d", env.Code)
	}
	if _, env := doHealthJSON(t, r, http.MethodPut, "/api/v1/health/records/99999999", aliceToken, map[string]any{"weightKg": "58.00"}); env.Code != 40004 {
		t.Fatalf("不存在记录应返回 40004，实际 code=%d", env.Code)
	}

	// 跨用户更新统一 40004。
	if _, env := doHealthJSON(t, r, http.MethodPut, path, bobToken, map[string]any{"weightKg": "1.00"}); env.Code != 40004 {
		t.Fatalf("跨用户更新应返回 40004，实际 code=%d", env.Code)
	}

	// 合法更新测量时间和备注。
	newAt := time.Now().UTC().Add(-3 * time.Hour).Format(time.RFC3339)
	_, env = doHealthJSON(t, r, http.MethodPut, path, aliceToken, map[string]any{
		"measuredAt": newAt,
		"remark":     "重新测量",
	})
	if env.Code != 0 {
		t.Fatalf("更新时间和备注失败: code=%d", env.Code)
	}
	if err := json.Unmarshal(env.Data, &updated); err != nil {
		t.Fatalf("解析更新结果失败: %v", err)
	}
	if updated.Remark == nil || *updated.Remark != "重新测量" || updated.Source != "manual" {
		t.Fatalf("更新时间/备注结果异常: %+v", updated)
	}
}
