package router

import (
	"github.com/gin-gonic/gin"

	"ai_health/internal/handler"
	"ai_health/internal/middleware"
)

// registerSport 运动模块：字典与建议人人可读，维护需权限位。
func registerSport(v1 *gin.RouterGroup, h *handler.SportHandler, auth gin.HandlerFunc) {
	g := v1.Group(prefixSport, auth)
	{
		g.GET("/types", h.ListTypes)
		g.GET("/infos", h.ListInfos)
		g.GET("/diseases", h.ListDiseases)
		g.POST("/infos", middleware.RequirePerm("sport:create"), h.CreateInfo)
		g.PUT("/infos/:id", middleware.RequirePerm("sport:update"), h.UpdateInfo)
	}
}
