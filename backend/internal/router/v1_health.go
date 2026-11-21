package router

import (
	"github.com/gin-gonic/gin"

	"ai_health/internal/handler"
)

// registerHealth 健康档案、健康记录与趋势，全部要求登录，数据按 uid 隔离。
func registerHealth(v1 *gin.RouterGroup, h *handler.HealthHandler, auth gin.HandlerFunc) {
	g := v1.Group(prefixHealth, auth)
	{
		g.GET("/profile", h.GetProfile)
		g.POST("/profile", h.CreateProfile)
		g.PUT("/profile", h.UpdateProfile)

		g.GET("/records", h.ListRecords)
		g.POST("/records", h.CreateRecord)
		g.GET("/records/:id", h.GetRecord)
		g.PUT("/records/:id", h.UpdateRecord)
		g.DELETE("/records/:id", h.DeleteRecord)

		g.GET("/summary", h.Summary)
		g.GET("/trends", h.Trend)
		// 旧路径保留为别名，避免已接入的调用方 404。
		g.GET("/trend", h.Trend)
	}
}
