package errcode

import (
	"errors"
	"fmt"
)

// Error 业务错误：Code 给前端判分支，Msg 给用户看，Detail 只进日志。
type Error struct {
	Code   int    `json:"code"`
	Msg    string `json:"message"`
	Detail string `json:"-"`
}

func (e *Error) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("[%d] %s: %s", e.Code, e.Msg, e.Detail)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Msg)
}

// WithDetail 返回副本，避免共享变量被并发改写。
func (e *Error) WithDetail(format string, args ...any) *Error {
	cp := *e
	cp.Detail = fmt.Sprintf(format, args...)
	return &cp
}

// Is 便于 errors.Is 判定。
func (e *Error) Is(target error) bool {
	var t *Error
	if errors.As(target, &t) {
		return t.Code == e.Code
	}
	return false
}

// 错误码分段：1xxxx 通用 / 2xxxx 认证 / 3xxxx 用户 / 4xxxx 健康 / 5xxxx 运动 / 6xxxx AI
var (
	ErrOK           = &Error{Code: 0, Msg: "ok"}
	ErrInvalidParam = &Error{Code: 10001, Msg: "参数错误"}
	ErrNotFound     = &Error{Code: 10002, Msg: "资源不存在"}
	ErrNotImpl      = &Error{Code: 10003, Msg: "接口实现中"}
	ErrInternal     = &Error{Code: 10004, Msg: "服务器内部错误"}

	ErrUnauthorized = &Error{Code: 20001, Msg: "未登录或登录已过期"}
	ErrTokenInvalid = &Error{Code: 20002, Msg: "令牌无效"}
	ErrTokenRevoked = &Error{Code: 20003, Msg: "登录状态已失效，请重新登录"}
	ErrBadPassword  = &Error{Code: 20004, Msg: "用户名或密码错误"}
	ErrForbidden    = &Error{Code: 20005, Msg: "没有权限执行该操作"}
	ErrTooManyRetry = &Error{Code: 20006, Msg: "尝试次数过多，请稍后再试"}

	ErrUserExists   = &Error{Code: 30001, Msg: "用户名/邮箱/手机号已存在"}
	ErrUserNotFound = &Error{Code: 30002, Msg: "用户不存在"}
	ErrUserDisabled = &Error{Code: 30003, Msg: "账号已被禁用"}

	ErrMeasuredAtInvalid = &Error{Code: 40001, Msg: "测量时间不能晚于当前时间"}

	ErrAITimeout = &Error{Code: 60001, Msg: "AI 服务响应超时，请稍后重试"}
)
