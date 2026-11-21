package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"ai_health/internal/consts"
	"ai_health/internal/pkg/errcode"
	"ai_health/internal/pkg/response"
)

// Recovery 捕获 panic，返回统一错误体并记录堆栈。
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				zap.L().Error("panic recovered",
					zap.Any("error", rec),
					zap.String("requestId", c.GetString(consts.CtxRequestID)),
					zap.String("path", c.Request.URL.Path),
					zap.ByteString("stack", debug.Stack()),
				)
				c.AbortWithStatusJSON(http.StatusInternalServerError, response.Body{
					Code:      errcode.ErrInternal.Code,
					Message:   errcode.ErrInternal.Msg,
					RequestID: c.GetString(consts.CtxRequestID),
				})
			}
		}()
		c.Next()
	}
}
