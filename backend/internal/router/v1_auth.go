package router

import (
	"github.com/gin-gonic/gin"

	"ai_health/internal/handler"
)

// registerAuth 认证路由：登录/注册/刷新开放，其余需 Bearer。
func registerAuth(v1 *gin.RouterGroup, h *handler.AuthHandler, auth gin.HandlerFunc) {
	g := v1.Group(prefixAuth)
	{
		g.POST("/register", h.Register)
		g.POST("/login", h.Login)
		g.POST("/refresh", h.Refresh)

		authed := g.Group("", auth)
		authed.POST("/logout", h.Logout)
		authed.GET("/profile", h.Profile)
	}
}
