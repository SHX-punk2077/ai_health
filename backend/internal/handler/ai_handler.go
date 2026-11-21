package handler

import (
	"github.com/gin-gonic/gin"

	"ai_health/internal/pkg/errcode"
	"ai_health/internal/pkg/response"
	"ai_health/internal/service"
)

// AIHandler AI 会话与问答。
//
// TODO 全部方法待接入 AIService，当前统一返回 10003。
type AIHandler struct {
	svc *service.AIService
}

func NewAIHandler(svc *service.AIService) *AIHandler {
	return &AIHandler{svc: svc}
}

// ListConversations 会话列表
func (h *AIHandler) ListConversations(c *gin.Context) {
	response.Fail(c, errcode.ErrNotImpl)
}

// CreateConversation 新建会话
func (h *AIHandler) CreateConversation(c *gin.Context) {
	response.Fail(c, errcode.ErrNotImpl)
}

// Messages 会话消息分页
func (h *AIHandler) Messages(c *gin.Context) {
	response.Fail(c, errcode.ErrNotImpl)
}

// Chat 提问（支持 SSE）
func (h *AIHandler) Chat(c *gin.Context) {
	response.Fail(c, errcode.ErrNotImpl)
}

// DeleteConversation 删除会话
func (h *AIHandler) DeleteConversation(c *gin.Context) {
	response.Fail(c, errcode.ErrNotImpl)
}
