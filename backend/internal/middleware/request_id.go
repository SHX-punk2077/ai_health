// Package middleware 提供 gin 中间件，顺序：RequestID → Logger → Recovery → CORS → 鉴权。
package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"ai_health/internal/consts"
)

// RequestID 透传或生成请求 ID，贯穿日志与响应体。
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader("X-Request-ID")
		if rid == "" {
			rid = uuid.NewString()
		}
		c.Set(consts.CtxRequestID, rid)
		c.Header("X-Request-ID", rid)
		c.Next()
	}
}
