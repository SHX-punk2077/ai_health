package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"ai_health/internal/consts"
)

// RequirePerm 是后端真正的权限闸门：前端隐藏菜单只是体验，这里才是边界。
func TestRequirePerm(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name  string
		perms []string
		set   bool
		want  int
	}{
		{"没有权限上下文", nil, false, http.StatusForbidden},
		{"空权限", []string{}, true, http.StatusForbidden},
		{"权限不足", []string{"user:read"}, true, http.StatusForbidden},
		{"命中权限位", []string{"user:read", "user:create"}, true, http.StatusOK},
		{"管理员通配符", []string{"*"}, true, http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			r.GET("/probe", func(c *gin.Context) {
				if tc.set {
					c.Set(consts.CtxPerms, tc.perms)
				}
			}, RequirePerm("user:create"), func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/probe", nil))
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d, body=%s", w.Code, tc.want, w.Body.String())
			}
		})
	}
}
