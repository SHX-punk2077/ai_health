package handler

import (
	"github.com/gin-gonic/gin"

	"ai_health/internal/dto/request"
	"ai_health/internal/pkg/response"
	"ai_health/internal/service"
)

// UserHandler 用户资料与后台用户管理。
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
	var q request.UserQuery
	if !bindQuery(c, &q) {
		return
	}
	list, total, err := h.svc.List(c.Request.Context(), &q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Page(c, list, total, q.Page, q.PageSize)
}

// Detail 用户详情
func (h *UserHandler) Detail(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	item, err := h.svc.Detail(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, item)
}

// Create 新建用户
func (h *UserHandler) Create(c *gin.Context) {
	var req request.UserCreateReq
	if !bindJSON(c, &req) {
		return
	}
	item, err := h.svc.Create(c.Request.Context(), currentUID(c), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, item)
}

// Update 更新用户（不含角色）
func (h *UserHandler) Update(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req request.UserUpdateReq
	if !bindJSON(c, &req) {
		return
	}
	item, err := h.svc.Update(c.Request.Context(), currentUID(c), id, &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, item)
}

// Delete 删除用户（软删）
func (h *UserHandler) Delete(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), currentUID(c), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"id": id})
}

// UpdateStatus 启用/禁用
func (h *UserHandler) UpdateStatus(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req request.UserStatusReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.svc.UpdateStatus(c.Request.Context(), currentUID(c), id, *req.Status); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"id": id, "status": *req.Status})
}

// ResetPassword 重置密码
func (h *UserHandler) ResetPassword(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req request.ResetPasswordReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.svc.ResetPassword(c.Request.Context(), id, req.NewPassword); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"id": id})
}

// AssignRoles 分配角色
func (h *UserHandler) AssignRoles(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req request.AssignRolesReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.svc.AssignRoles(c.Request.Context(), currentUID(c), id, *req.RoleIDs); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"id": id})
}

// Roles 用户已分配角色
func (h *UserHandler) Roles(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	list, err := h.svc.Roles(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}
