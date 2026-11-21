package router

import (
	"time"

	"github.com/gin-gonic/gin"

	"ai_health/internal/handler"
	"ai_health/internal/middleware"
	"ai_health/internal/pkg/cache"
)

// registerAI AI 会话与问答。问答单独限流，避免刷爆上游额度。
func registerAI(v1 *gin.RouterGroup, h *handler.AIHandler, auth gin.HandlerFunc, store *cache.Store) {
	g := v1.Group(prefixAI, auth)
	{
		g.GET("/conversations", h.ListConversations)
		g.POST("/conversations", h.CreateConversation)
		g.GET("/conversations/:id/messages", h.Messages)
		g.DELETE("/conversations/:id", h.DeleteConversation)
		g.POST("/chat", middleware.RateLimit(store, "uid", 20, time.Minute), h.Chat)
	}
}
