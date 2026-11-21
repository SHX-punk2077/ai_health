package router

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

type healthEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func doHealthRequest(t *testing.T, h http.Handler, method, path, token, body string) (*httptest.ResponseRecorder, healthEnvelope) {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	var env healthEnvelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("解析响应失败: %v, body=%s", err, w.Body.String())
	}
	return w, env
}

func doHealthJSON(t *testing.T, h http.Handler, method, path, token string, body any) (*httptest.ResponseRecorder, healthEnvelope) {
	t.Helper()
	raw := ""
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("编码请求体失败: %v", err)
		}
		raw = string(b)
	}
	return doHealthRequest(t, h, method, path, token, raw)
}

type healthProfileData struct {
	ID       string  `json:"id"`
	UserID   string  `json:"userId"`
	RealName *string `json:"realName"`
	HeightCm *string `json:"heightCm"`
	WeightKg *string `json:"weightKg"`
	BMI      *string `json:"bmi"`
}

type healthRecordData struct {
	ID         string  `json:"id"`
	WeightKg   *string `json:"weightKg"`
	MeasuredAt string  `json:"measuredAt"`
	Source     string  `json:"source"`
}

type healthPageData struct {
	List     []healthRecordData `json:"list"`
	Total    int64              `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"pageSize"`
}

type healthTrendData struct {
	MetricName  string `json:"metricName"`
	Unit        string `json:"unit"`
	Granularity string `json:"granularity"`
	Points      []struct {
		Time        string `json:"time"`
		Value       string `json:"value"`
		SampleCount int64  `json:"sampleCount"`
	} `json:"points"`
}

func TestHTTPHealthProfileRecordsAndTrends(t *testing.T) {
	db := openRouterDB(t)
	resetRouterTables(t, db)
	store := routerStore(t)
	ctx := context.Background()

	userRole := createRole(t, db, consts.RoleUser, "普通用户")
	alice := createUser(t, db, "health_http_alice", userRole)
	bob := createUser(t, db, "health_http_bob", userRole)

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
		out, err := authSvc.Login(ctx, &request.LoginReq{Account: account, Password: "Passw0rd!23"}, "10.9.0.1")
		if err != nil {
			t.Fatalf("%s 登录失败: %v", account, err)
		}
		return out.AccessToken
	}
	aliceToken := login("health_http_alice")
	bobToken := login("health_http_bob")

	// 未登录不能访问健康接口。
	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/health/profile"},
		{http.MethodPost, "/api/v1/health/profile"},
		{http.MethodGet, "/api/v1/health/records"},
		{http.MethodGet, "/api/v1/health/trends?metric=weightKg"},
	} {
		w, env := doHealthJSON(t, r, tc.method, tc.path, "", nil)
		if w.Code != http.StatusUnauthorized || env.Code != 20001 {
			t.Fatalf("未登录 %s %s: status=%d code=%d", tc.method, tc.path, w.Code, env.Code)
		}
	}

	// 未建档返回统一业务错误 40002。
	if w, env := doHealthJSON(t, r, http.MethodGet, "/api/v1/health/profile", aliceToken, nil); w.Code != http.StatusOK || env.Code != 40002 {
		t.Fatalf("未建档查询异常: status=%d code=%d", w.Code, env.Code)
	}

	// 创建档案。
	w, env := doHealthJSON(t, r, http.MethodPost, "/api/v1/health/profile", aliceToken, map[string]any{
		"realName":  "Alice",
		"heightCm":  "165.40",
		"weightKg":  "55.20",
		"birthDate": "1992-02-03",
		"gender":    2,
		"userId":    bob.ID,
	})
	if w.Code != http.StatusOK || env.Code != 0 {
		t.Fatalf("创建档案失败: status=%d body=%s", w.Code, string(env.Data))
	}
	var profile healthProfileData
	if err := json.Unmarshal(env.Data, &profile); err != nil {
		t.Fatalf("解析档案失败: %v", err)
	}
	if profile.UserID != strconv.FormatUint(alice.ID, 10) {
		t.Fatalf("档案归属被请求体篡改: %+v", profile)
	}
	if profile.HeightCm == nil || *profile.HeightCm != "165.40" {
		t.Fatalf("档案身高异常: %+v", profile)
	}
	if profile.BMI == nil {
		t.Fatalf("档案应返回 BMI: %+v", profile)
	}

	// 重复创建 40003。
	if _, env := doHealthJSON(t, r, http.MethodPost, "/api/v1/health/profile", aliceToken, map[string]any{}); env.Code != 40003 {
		t.Fatalf("重复创建应返回 40003，实际 code=%d", env.Code)
	}

	// 用户隔离：bob 查询、修改 alice 的档案都返回 40002。
	if _, env := doHealthJSON(t, r, http.MethodGet, "/api/v1/health/profile", bobToken, nil); env.Code != 40002 {
		t.Fatalf("bob 查询档案应返回 40002，实际 code=%d", env.Code)
	}
	if _, env := doHealthJSON(t, r, http.MethodPut, "/api/v1/health/profile", bobToken, map[string]any{"weightKg": "50.00"}); env.Code != 40002 {
		t.Fatalf("bob 修改档案应返回 40002，实际 code=%d", env.Code)
	}

	// 部分更新 + 请求体 userId 被忽略。
	_, env = doHealthJSON(t, r, http.MethodPut, "/api/v1/health/profile", aliceToken, map[string]any{
		"userId":   bob.ID,
		"weightKg": "56.00",
	})
	if env.Code != 0 {
		t.Fatalf("修改档案失败: code=%d", env.Code)
	}
	if err := json.Unmarshal(env.Data, &profile); err != nil {
		t.Fatalf("解析更新后档案失败: %v", err)
	}
	if profile.UserID == strconv.FormatUint(bob.ID, 10) || profile.HeightCm == nil || *profile.HeightCm != "165.40" || profile.WeightKg == nil || *profile.WeightKg != "56.00" {
		t.Fatalf("部分更新或归属校验异常: %+v", profile)
	}

	// 新增健康记录，归属由 JWT 决定。
	measuredAt := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	_, env = doHealthJSON(t, r, http.MethodPost, "/api/v1/health/records", aliceToken, map[string]any{
		"measuredAt": measuredAt,
		"weightKg":   "56.00",
		"source":     "manual",
	})
	if env.Code != 0 {
		t.Fatalf("新增记录失败: code=%d", env.Code)
	}
	var record healthRecordData
	if err := json.Unmarshal(env.Data, &record); err != nil {
		t.Fatalf("解析记录失败: %v", err)
	}
	if record.ID == "" || record.WeightKg == nil || *record.WeightKg != "56.00" {
		t.Fatalf("新增记录返回异常: %+v", record)
	}

	// 列表分页。
	_, env = doHealthJSON(t, r, http.MethodGet, "/api/v1/health/records?page=1&pageSize=999", aliceToken, nil)
	if env.Code != 0 {
		t.Fatalf("查询记录失败: code=%d", env.Code)
	}
	var page healthPageData
	if err := json.Unmarshal(env.Data, &page); err != nil {
		t.Fatalf("解析分页失败: %v", err)
	}
	if page.Total != 1 || len(page.List) != 1 || page.PageSize != 100 {
		t.Fatalf("分页参数或结果异常: %+v", page)
	}

	// 记录归属：bob 列表为空，详情/删除 40004。
	_, env = doHealthJSON(t, r, http.MethodGet, "/api/v1/health/records", bobToken, nil)
	if env.Code != 0 {
		t.Fatalf("bob 查询列表失败: code=%d", env.Code)
	}
	if err := json.Unmarshal(env.Data, &page); err != nil {
		t.Fatalf("解析 bob 分页失败: %v", err)
	}
	if page.Total != 0 {
		t.Fatalf("bob 不应看到 alice 的记录: %+v", page)
	}
	if _, env := doHealthJSON(t, r, http.MethodGet, "/api/v1/health/records/"+record.ID, bobToken, nil); env.Code != 40004 {
		t.Fatalf("bob 查询详情应返回 40004，实际 code=%d", env.Code)
	}
	if _, env := doHealthJSON(t, r, http.MethodDelete, "/api/v1/health/records/"+record.ID, bobToken, nil); env.Code != 40004 {
		t.Fatalf("bob 删除应返回 40004，实际 code=%d", env.Code)
	}
	if _, env := doHealthJSON(t, r, http.MethodGet, "/api/v1/health/records/"+record.ID, aliceToken, nil); env.Code != 0 {
		t.Fatalf("alice 查询自己的记录失败: code=%d", env.Code)
	}

	// 趋势接口：alice 1 个点；bob 空集合。
	_, env = doHealthJSON(t, r, http.MethodGet, "/api/v1/health/trends?metric=weightKg&granularity=day", aliceToken, nil)
	if env.Code != 0 {
		t.Fatalf("趋势查询失败: code=%d", env.Code)
	}
	var trend healthTrendData
	if err := json.Unmarshal(env.Data, &trend); err != nil {
		t.Fatalf("解析趋势失败: %v", err)
	}
	if trend.MetricName != "体重" || trend.Unit != "kg" || len(trend.Points) != 1 || trend.Points[0].Value != "56.00" {
		t.Fatalf("趋势返回异常: %+v", trend)
	}
	_, env = doHealthJSON(t, r, http.MethodGet, "/api/v1/health/trends?metric=weightKg", bobToken, nil)
	if env.Code != 0 {
		t.Fatalf("bob 趋势查询失败: code=%d", env.Code)
	}
	if err := json.Unmarshal(env.Data, &trend); err != nil {
		t.Fatalf("解析 bob 趋势失败: %v", err)
	}
	if trend.Points == nil || len(trend.Points) != 0 {
		t.Fatalf("bob 趋势应为空集合: %+v", trend.Points)
	}

	// 参数校验与坏令牌。
	if _, env := doHealthJSON(t, r, http.MethodGet, "/api/v1/health/trends?metric=unknown", aliceToken, nil); env.Code != 40006 {
		t.Fatalf("非法趋势指标应返回 40006，实际 code=%d", env.Code)
	}
	if _, env := doHealthJSON(t, r, http.MethodGet, "/api/v1/health/records?startAt=2026/01/01", aliceToken, nil); env.Code != 40005 {
		t.Fatalf("非法日期应返回 40005，实际 code=%d", env.Code)
	}
	if w, env := doHealthJSON(t, r, http.MethodGet, "/api/v1/health/profile", "bad-token", nil); w.Code != http.StatusUnauthorized || env.Code != 20002 {
		t.Fatalf("坏令牌应 401/20002: status=%d code=%d", w.Code, env.Code)
	}
	if w, env := doHealthRequest(t, r, http.MethodPost, "/api/v1/health/records", aliceToken, "{bad json"); w.Code != http.StatusOK || env.Code != 10001 {
		t.Fatalf("非法 JSON 应返回 10001: status=%d code=%d", w.Code, env.Code)
	}

	// 删除后详情 40004；删除响应里的 id 必须与 DTO 一样是字符串。
	_, deleteEnv := doHealthJSON(t, r, http.MethodDelete, "/api/v1/health/records/"+record.ID, aliceToken, nil)
	if deleteEnv.Code != 0 {
		t.Fatalf("删除记录失败: code=%d", deleteEnv.Code)
	}
	var deleted struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(deleteEnv.Data, &deleted); err != nil {
		t.Fatalf("解析删除响应失败: %v, data=%s", err, string(deleteEnv.Data))
	}
	if deleted.ID != record.ID {
		t.Fatalf("删除响应 id 应为字符串 %q，实际 %q", record.ID, deleted.ID)
	}
	if _, env := doHealthJSON(t, r, http.MethodGet, "/api/v1/health/records/"+record.ID, aliceToken, nil); env.Code != 40004 {
		t.Fatalf("删除后应返回 40004，实际 code=%d", env.Code)
	}
}
