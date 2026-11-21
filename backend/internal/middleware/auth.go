package middleware

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"

	"ai_health/internal/consts"
	"ai_health/internal/pkg/cache"
	"ai_health/internal/pkg/errcode"
	"ai_health/internal/pkg/jwt"
	"ai_health/internal/pkg/response"
)

// JWTAuth 校验签名 + Redis 白名单，并把 uid / perms 写入上下文。
// 白名单模式的价值：登出、改密、封号都能立即失效，不必等 access token 过期。
func JWTAuth(tokens *jwt.Manager, store *cache.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") && !strings.HasPrefix(header, "bearer ") {
			response.Fail(c, errcode.ErrUnauthorized)
			c.Abort()
			return
		}
		raw := strings.TrimSpace(header[len("Bearer "):])
		if raw == "" {
			response.Fail(c, errcode.ErrUnauthorized)
			c.Abort()
			return
		}

		claims, err := tokens.Parse(raw, "access")
		if err != nil {
			response.Fail(c, errcode.ErrTokenInvalid.WithDetail("parse access: %v", err))
			c.Abort()
			return
		}

		ctx := c.Request.Context()
		if !store.Exists(ctx, fmt.Sprintf(consts.KeyAccessToken, claims.UID, claims.ID)) {
			response.Fail(c, errcode.ErrTokenRevoked)
			c.Abort()
			return
		}

		var perms []string
		_ = store.GetJSON(ctx, fmt.Sprintf(consts.KeyPermissions, claims.UID), &perms)

		c.Set(consts.CtxUID, claims.UID)
		c.Set(consts.CtxPerms, perms)
		c.Next()
	}
}
