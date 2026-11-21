package response

import (
	"net/http"
	"testing"

	"ai_health/internal/pkg/errcode"
)

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
