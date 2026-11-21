package handler

import (
	"github.com/gin-gonic/gin"

	"ai_health/internal/consts"
	"ai_health/internal/pkg/errcode"
	"ai_health/internal/pkg/response"
)

// bindJSON 绑定并校验请求体，失败时已写好响应，调用方直接 return。
func bindJSON(c *gin.Context, dst any) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		response.Fail(c, errcode.ErrInvalidParam.WithDetail("bind json: %v", err))
		return false
	}
	return true
}

// bindQuery 绑定 URL 查询参数。
func bindQuery(c *gin.Context, dst any) bool {
	if err := c.ShouldBindQuery(dst); err != nil {
		response.Fail(c, errcode.ErrInvalidParam.WithDetail("bind query: %v", err))
		return false
	}
	return true
}

// currentUID 从 JWT 中间件写入的上下文取用户 ID。
func currentUID(c *gin.Context) uint64 {
	if v, ok := c.Get(consts.CtxUID); ok {
		if uid, ok := v.(uint64); ok {
			return uid
		}
	}
	return 0
}
