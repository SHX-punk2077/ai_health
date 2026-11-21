package middleware

import (
	"context"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"ai_health/internal/consts"
	"ai_health/internal/pkg/cache"
	"ai_health/internal/pkg/errcode"
	"ai_health/internal/pkg/jwt"
	"ai_health/internal/pkg/response"
)

// IdentityResolver 认证通过后加载角色与权限，由 service 层（AuthService）实现。
// 中间件只声明接口，避免反向依赖 service 包。
type IdentityResolver interface {
	RoleCodes(ctx context.Context, uid uint64) ([]string, error)
	Permissions(ctx context.Context, uid uint64) ([]string, error)
}

// JWTAuth 校验签名 + Redis 白名单，并把 uid / roles / perms 写入上下文。
//
// 白名单模式的价值：登出、改密、封号都能立即失效，不必等 access token 过期。
// Redis 不可用时降级为「只验签名」并打告警日志，避免缓存故障放大成全站 401。
func JWTAuth(tokens *jwt.Manager, store *cache.Store, ids IdentityResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(strings.ToLower(header), "bearer ") {
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
		key := fmt.Sprintf(consts.KeyAccessToken, claims.UID, claims.ID)
		switch ok, err := store.TryExists(ctx, key); {
		case err != nil:
			zap.L().Warn("redis unavailable, degrade to stateless auth",
				zap.Uint64("uid", claims.UID), zap.Error(err))
		case !ok:
			response.Fail(c, errcode.ErrTokenRevoked)
			c.Abort()
			return
		}

		roles := []string{}
		perms := []string{}
		if ids != nil {
			if v, err := ids.RoleCodes(ctx, claims.UID); err != nil {
				zap.L().Warn("load roles failed", zap.Uint64("uid", claims.UID), zap.Error(err))
			} else if v != nil {
				roles = v
			}
			// 权限位必须在这里准备好，RequirePerm 只读上下文，不再回源。
			if v, err := ids.Permissions(ctx, claims.UID); err != nil {
				zap.L().Warn("load permissions failed", zap.Uint64("uid", claims.UID), zap.Error(err))
			} else if v != nil {
				perms = v
			}
		}

		c.Set(consts.CtxUID, claims.UID)
		c.Set(consts.CtxRoles, roles)
		c.Set(consts.CtxPerms, perms)
		c.Next()
	}
}
