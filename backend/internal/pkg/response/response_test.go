package response

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"ai_health/internal/pkg/errcode"
)

func TestFailDoesNotLeakInternalDetail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("requestID", "req-test")
	detail := "Error 1146: Table 'ai_health_test.health_record' doesn't exist; SELECT * FROM health_record WHERE user_id=1; host=127.0.0.1"
	Fail(c, errcode.ErrInternal.WithDetail("%v", errors.New(detail)))

	body := w.Body.String()
	for _, forbidden := range []string{"Error 1146", "SELECT", "health_record", "127.0.0.1", "user_id", "mysql"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("错误响应泄露内部信息 %q: %s", forbidden, body)
		}
	}
	var got Body
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if got.Code != errcode.ErrInternal.Code || got.Message != errcode.ErrInternal.Msg {
		t.Fatalf("内部错误响应结构错误: %+v", got)
	}
}

func TestHTTPStatus(t *testing.T) {
	cases := []struct {
		name string
		code int
		want int
	}{
		{"成功", 0, http.StatusOK},
		{"参数错误", errcode.ErrInvalidParam.Code, http.StatusOK},
		{"未登录", errcode.ErrUnauthorized.Code, http.StatusUnauthorized},
		{"令牌失效", errcode.ErrTokenRevoked.Code, http.StatusUnauthorized},
		{"权限不足", errcode.ErrForbidden.Code, http.StatusForbidden},
		{"限流", errcode.ErrTooManyRetry.Code, http.StatusTooManyRequests},
		{"用户已存在", errcode.ErrUserExists.Code, http.StatusBadRequest},
		{"健康数据非法", errcode.ErrMeasuredAtInvalid.Code, http.StatusOK},
	}
	for _, tc := range cases {
		if got := httpStatus(tc.code); got != tc.want {
			t.Errorf("%s: httpStatus(%d) = %d, want %d", tc.name, tc.code, got, tc.want)
		}
	}
}
