// Package router 负责路由注册与中间件编排，是唯一了解 HTTP 对外形态的地方。
package router

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"ai_health/internal/config"
	"ai_health/internal/consts"
	"ai_health/internal/handler"
	"ai_health/internal/middleware"
	"ai_health/internal/pkg/cache"
	"ai_health/internal/pkg/jwt"
)

// New 构建 gin 引擎。中间件顺序：RequestID → Logger → Recovery → CORS。
func New(cfg *config.Config, h *handler.Handlers, tokens *jwt.Manager, store *cache.Store) *gin.Engine {
	gin.SetMode(gin.DebugMode)
	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(middleware.RequestID(), middleware.Logger(), middleware.Recovery(), middleware.CORS(cfg))

	r.GET("/healthz", func(c *gin.Context) {
		if err := store.Ping(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "degraded", "redis": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 全局限流：每 IP 每分钟 300 次
	rl := middleware.RateLimit(store, "ip", 300, time.Minute)
	auth := middleware.JWTAuth(tokens, store)

	v1 := r.Group("/api/v1", rl)
	{
		registerAuth(v1, h.Auth, auth)
		registerSystem(v1, h, auth)
		registerHealth(v1, h.Health, auth)
		registerSport(v1, h.Sport, auth)
		registerAI(v1, h.AI, auth, store)
	}

	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"code": 10002, "message": "route not found"})
	})
	return r
}

// 便于后续接入 swag；此处只声明常量避免拼写漂移。
const (
	prefixAuth   = "/auth"
	prefixUser   = "/users"
	prefixRole   = "/roles"
	prefixMenu   = "/menus"
	prefixHealth = "/health"
	prefixSport  = "/sports"
	prefixAI     = "/ai"
)

var _ = consts.RoleSuperAdmin
