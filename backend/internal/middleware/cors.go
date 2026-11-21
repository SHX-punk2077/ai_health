package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ai_health/internal/config"
)

// CORS 白名单跨域，允许携带 Authorization。
func CORS(cfg *config.Config) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(cfg.CORS.AllowOrigins))
	for _, o := range cfg.CORS.AllowOrigins {
		allowed[o] = struct{}{}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if _, ok := allowed[origin]; ok || origin == "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Origin,Content-Type,Accept,Authorization,X-Request-ID")
			c.Header("Access-Control-Expose-Headers", "X-Request-ID")
			c.Header("Access-Control-Max-Age", "600")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
