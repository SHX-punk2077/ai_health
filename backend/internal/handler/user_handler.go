package handler

import (
	"github.com/gin-gonic/gin"

	"ai_health/internal/dto/request"
	"ai_health/internal/pkg/errcode"
	"ai_health/internal/pkg/response"
	"ai_health/internal/service"
)

// UserHandler 用户资料与后台用户管理。
//
// 已实现：当前用户资料读取 / 修改、我的角色。
// 待实现（RBAC 阶段）：列表、详情、增删改、启停、重置密码、分配角色，
// 对应权限位 user:create / user:update / user:delete / role:assign。
type UserHandler struct {
	svc *service.UserService
}

func NewUserHandler(svc *service.UserService) *UserHandler { return &UserHandler{svc: svc} }

// Profile 当前登录用户资料
// @Summary      获取当前用户资料
// @Tags         users
// @Security     Bearer
// @Success      200 {object} response.Body{data=response.UserInfo}
// @Router       /users/profile [get]
func (h *UserHandler) Profile(c *gin.Context) {
	info, err := h.svc.Profile(c.Request.Context(), currentUID(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, info)
}

// UpdateProfile 修改当前用户资料（昵称 / 邮箱 / 手机号 / 头像）
// @Summary      修改当前用户资料
// @Tags         users
// @Security     Bearer
// @Accept       json
// @Produce      json
// @Param        body body request.UserUpdateReq true "可改字段"
// @Success      200 {object} response.Body{data=response.UserInfo}
// @Router       /users/profile [put]
func (h *UserHandler) UpdateProfile(c *gin.Context) {
	var req request.UserUpdateReq
	if !bindJSON(c, &req) {
		return
	}
	info, err := h.svc.UpdateProfile(c.Request.Context(), currentUID(c), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, info)
}

// MyRoles 当前用户的角色列表
// @Summary      我的角色
// @Tags         users
// @Security     Bearer
// @Success      200 {object} response.Body{data=[]response.RoleItem}
// @Router       /users/roles [get]
func (h *UserHandler) MyRoles(c *gin.Context) {
	list, err := h.svc.MyRoles(c.Request.Context(), currentUID(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

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
