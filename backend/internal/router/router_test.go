package router

import (
	"testing"
	"time"

	"gorm.io/gorm"

	"ai_health/internal/config"
	"ai_health/internal/handler"
	"ai_health/internal/pkg/cache"
	jwtmgr "ai_health/internal/pkg/jwt"
)

// 路由是前后端契约的一部分：这里用表驱动锁住本次阶段要求的全部路径。
func TestRequiredRoutesRegistered(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.Mode = "test"
	cfg.JWT.Secret = "0123456789abcdef0123456789abcdef"
	cfg.JWT.Issuer = "test"
	cfg.JWT.AccessTTL = time.Minute
	cfg.JWT.RefreshTTL = time.Hour

	tokens := jwtmgr.New(cfg.JWT.Secret, cfg.JWT.Issuer, cfg.JWT.AccessTTL, cfg.JWT.RefreshTTL)
	store := cache.New(nil)
	var db *gorm.DB
	h := handler.New(
		handler.NewAuthHandler(nil),
		handler.NewUserHandler(nil),
		handler.NewRoleHandler(nil),
		handler.NewMenuHandler(nil),
		handler.NewHealthHandler(nil),
		handler.NewSportHandler(nil),
		handler.NewAIHandler(nil),
	)
	_ = db

	r := New(cfg, h, tokens, store, nil)

	got := make(map[string]bool, len(r.Routes()))
	for _, rt := range r.Routes() {
		got[rt.Method+" "+rt.Path] = true
	}

	// 当前契约：认证、用户、角色、菜单、健康；运动路由统一为单数 /sport。
	// AI 的剩余接口在后续阶段补齐。
	want := []string{
		"POST /api/v1/auth/register",
		"POST /api/v1/auth/login",
		"GET /api/v1/auth/me",
		"PUT /api/v1/auth/password",
		"GET /api/v1/users/profile",
		"PUT /api/v1/users/profile",
		"GET /api/v1/users/roles",
		"GET /api/v1/users",
		"GET /api/v1/users/:id",
		"POST /api/v1/users",
		"PUT /api/v1/users/:id",
		"DELETE /api/v1/users/:id",
		"PATCH /api/v1/users/:id/status",
		"PATCH /api/v1/users/:id/password",
		"GET /api/v1/users/:id/roles",
		"PUT /api/v1/users/:id/roles",

		"GET /api/v1/roles",
		"GET /api/v1/roles/:id",
		"POST /api/v1/roles",
		"PUT /api/v1/roles/:id",
		"DELETE /api/v1/roles/:id",
		"GET /api/v1/roles/:id/menus",
		"PUT /api/v1/roles/:id/menus",

		"GET /api/v1/menus",
		"GET /api/v1/menus/tree",
		"GET /api/v1/menus/:id",
		"GET /api/v1/menus/my",
		"POST /api/v1/menus",
		"PUT /api/v1/menus/:id",
		"DELETE /api/v1/menus/:id",

		"GET /api/v1/health/profile",
		"POST /api/v1/health/profile",
		"PUT /api/v1/health/profile",
		"GET /api/v1/health/records",
		"POST /api/v1/health/records",
		"GET /api/v1/health/records/:id",
		"PUT /api/v1/health/records/:id",
		"DELETE /api/v1/health/records/:id",
		"GET /api/v1/health/summary",
		"GET /api/v1/health/trends",
		"GET /api/v1/sport/types",
		"GET /api/v1/sport/infos",
		"GET /api/v1/sport/diseases",
		"POST /api/v1/sport/infos",
		"PUT /api/v1/sport/infos/:id",
		"POST /api/v1/ai/conversations",
		"GET /api/v1/ai/conversations",
		"GET /api/v1/ai/conversations/:id/messages",
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("缺少路由: %s", w)
		}
	}
}

// /users/profile、/users/roles 与 /users/:id 同级共存，必须保证静态段优先。
func TestStaticSegmentWinsOverParam(t *testing.T) {
	cfg := &config.Config{}
	cfg.JWT.Secret = "0123456789abcdef0123456789abcdef"
	cfg.JWT.AccessTTL = time.Minute
	cfg.JWT.RefreshTTL = time.Hour
	tokens := jwtmgr.New(cfg.JWT.Secret, "test", cfg.JWT.AccessTTL, cfg.JWT.RefreshTTL)

	h := handler.New(
		handler.NewAuthHandler(nil),
		handler.NewUserHandler(nil),
		handler.NewRoleHandler(nil),
		handler.NewMenuHandler(nil),
		handler.NewHealthHandler(nil),
		handler.NewSportHandler(nil),
		handler.NewAIHandler(nil),
	)
	r := New(cfg, h, tokens, cache.New(nil), nil)

	var hasProfile, hasParam bool
	for _, rt := range r.Routes() {
		switch rt.Path {
		case "/api/v1/users/profile":
			hasProfile = true
		case "/api/v1/users/:id":
			hasParam = true
		}
	}
	if !hasProfile || !hasParam {
		t.Fatalf("静态段与参数段应共存: profile=%v param=%v", hasProfile, hasParam)
	}
}
