package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"ai_health/internal/consts"
)

// Logger 结构化访问日志，慢请求单独告警。
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		cost := time.Since(start)
		fields := []zap.Field{
			zap.String("requestId", c.GetString(consts.CtxRequestID)),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("cost", cost),
			zap.String("ip", c.ClientIP()),
			zap.Uint64("uid", c.GetUint64(consts.CtxUID)),
		}
		if cost > time.Second*2 {
			zap.L().Warn("slow request", fields...)
			return
		}
		zap.L().Info("access", fields...)
	}
}
