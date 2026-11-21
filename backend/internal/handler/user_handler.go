package handler

import (
	"github.com/gin-gonic/gin"

	"ai_health/internal/pkg/errcode"
	"ai_health/internal/pkg/response"
)

// UserHandler 后台用户管理。
//
// TODO 需补 UserService（建议拆 service/user_service.go），权限位 user:create / user:update / user:delete。
type UserHandler struct{}

func NewUserHandler() *UserHandler { return &UserHandler{} }

// List 用户分页列表
func (h *UserHandler) List(c *gin.Context) {
	response.Fail(c, errcode.ErrNotImpl)
}

// Detail 用户详情
func (h *UserHandler) Detail(c *gin.Context) {
	response.Fail(c, errcode.ErrNotImpl)
}

// Create 新建用户
func (h *UserHandler) Create(c *gin.Context) {
	response.Fail(c, errcode.ErrNotImpl)
}

// Update 更新用户
func (h *UserHandler) Update(c *gin.Context) {
	response.Fail(c, errcode.ErrNotImpl)
}

// Delete 删除用户（软删）
func (h *UserHandler) Delete(c *gin.Context) {
	response.Fail(c, errcode.ErrNotImpl)
}

// UpdateStatus 启用/禁用
func (h *UserHandler) UpdateStatus(c *gin.Context) {
	response.Fail(c, errcode.ErrNotImpl)
}

// ResetPassword 重置密码
func (h *UserHandler) ResetPassword(c *gin.Context) {
	response.Fail(c, errcode.ErrNotImpl)
}

// AssignRoles 分配角色
func (h *UserHandler) AssignRoles(c *gin.Context) {
	response.Fail(c, errcode.ErrNotImpl)
}
