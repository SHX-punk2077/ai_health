package middleware

import (
	"github.com/gin-gonic/gin"

	"ai_health/internal/consts"
	"ai_health/internal/pkg/errcode"
	"ai_health/internal/pkg/response"
)

// RequirePerm 路由级权限校验，权限位来自 sys_menu.permission。
func RequirePerm(perm string) gin.HandlerFunc {
	return func(c *gin.Context) {
		v, ok := c.Get(consts.CtxPerms)
		if !ok {
			response.Fail(c, errcode.ErrForbidden)
			c.Abort()
			return
		}
		perms, _ := v.([]string)
		for _, p := range perms {
			if p == "*" || p == perm {
				c.Next()
				return
			}
		}
		response.Fail(c, errcode.ErrForbidden.WithDetail("need permission %s", perm))
		c.Abort()
	}
}
