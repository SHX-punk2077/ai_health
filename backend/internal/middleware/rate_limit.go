package middleware

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"ai_health/internal/consts"
	"ai_health/internal/pkg/cache"
	"ai_health/internal/pkg/errcode"
	"ai_health/internal/pkg/response"
)

// RateLimit 固定窗口限流（Redis INCR + EXPIRE）。
// scope=ip 时按客户端 IP 计数，scope=uid 时按用户计数；Redis 不可用时放行，避免拖垮业务。
func RateLimit(store *cache.Store, scope string, limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		subject := c.ClientIP()
		if scope == "uid" {
			if uid := c.GetUint64(consts.CtxUID); uid > 0 {
				subject = fmt.Sprintf("u%d", uid)
			}
		}
		key := fmt.Sprintf(consts.KeyRateLimit, scope, subject)
		if n := store.Incr(c.Request.Context(), key, window); n > int64(limit) {
			response.Fail(c, errcode.ErrTooManyRetry.WithDetail("rate limit %s/%s", scope, subject))
			c.Abort()
			return
		}
		c.Next()
	}
}
