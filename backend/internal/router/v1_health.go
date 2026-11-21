package router

import (
	"github.com/gin-gonic/gin"

	"ai_health/internal/handler"
)

// registerHealth 健康档案与健康记录，全部要求登录，数据按 uid 隔离。
func registerHealth(v1 *gin.RouterGroup, h *handler.HealthHandler, auth gin.HandlerFunc) {
	g := v1.Group(prefixHealth, auth)
	{
		g.GET("/profile", h.GetProfile)
		g.PUT("/profile", h.SaveProfile)

		g.GET("/records", h.ListRecords)
		g.POST("/records", h.CreateRecord)
		g.DELETE("/records/:id", h.DeleteRecord)
		g.GET("/trend", h.Trend)
	}
}
