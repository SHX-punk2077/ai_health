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

// WithMsg 返回副本并替换给用户看的文案，错误码保持不变。
func (e *Error) WithMsg(msg string) *Error {
	cp := *e
	cp.Msg = msg
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

	ErrRoleExists      = &Error{Code: 30004, Msg: "角色编码已存在"}
	ErrRoleInUse       = &Error{Code: 30005, Msg: "角色仍被用户使用，不能删除"}
	ErrSystemRole      = &Error{Code: 30006, Msg: "系统内置角色不允许该操作"}
	ErrRoleDisabled    = &Error{Code: 30007, Msg: "角色已禁用"}
	ErrMenuExists      = &Error{Code: 30008, Msg: "菜单标识已存在"}
	ErrMenuHasChildren = &Error{Code: 30009, Msg: "存在子菜单，不能删除"}
	ErrMenuParent      = &Error{Code: 30010, Msg: "父菜单不合法"}
	ErrMenuCycle       = &Error{Code: 30011, Msg: "父菜单不能是自身或其后代"}
	ErrLastSuperAdmin  = &Error{Code: 30012, Msg: "系统必须保留至少一个超级管理员"}

	ErrMeasuredAtInvalid = &Error{Code: 40001, Msg: "测量时间不能晚于当前时间"}

	ErrHealthProfileNotFound = &Error{Code: 40002, Msg: "健康档案不存在"}
	ErrHealthProfileExists   = &Error{Code: 40003, Msg: "健康档案已存在"}
	ErrHealthRecordNotFound  = &Error{Code: 40004, Msg: "健康记录不存在"}
	ErrHealthDateInvalid     = &Error{Code: 40005, Msg: "日期格式不正确，应为 YYYY-MM-DD"}
	ErrHealthMetricInvalid   = &Error{Code: 40006, Msg: "不支持的健康指标"}
	ErrHealthGranularity     = &Error{Code: 40007, Msg: "聚合粒度仅支持 day、week、month"}
	ErrHealthRangeInvalid    = &Error{Code: 40008, Msg: "时间范围不合法或超过 366 天"}
	ErrHealthMeasuredInvalid = &Error{Code: 40009, Msg: "测量时间格式不正确"}
	ErrHealthValueInvalid    = &Error{Code: 40010, Msg: "健康指标数值不合法"}

	ErrAITimeout = &Error{Code: 60001, Msg: "AI 服务响应超时，请稍后重试"}
)
