package handler

import (
	"github.com/gin-gonic/gin"

	"ai_health/internal/dto/request"
	"ai_health/internal/pkg/response"
	"ai_health/internal/service"
)

// RoleHandler 角色管理与角色菜单授权。
type RoleHandler struct {
	svc *service.RoleService
}

func NewRoleHandler(svc *service.RoleService) *RoleHandler { return &RoleHandler{svc: svc} }

// List 角色列表（支持 keyword / status）
func (h *RoleHandler) List(c *gin.Context) {
	var q request.RoleQuery
	if !bindQuery(c, &q) {
		return
	}
	list, err := h.svc.List(c.Request.Context(), &q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

// Detail 角色详情
func (h *RoleHandler) Detail(c *gin.Context) {
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

// Create 新建角色（可同时授权菜单）
func (h *RoleHandler) Create(c *gin.Context) {
	var req request.RoleCreateReq
	if !bindJSON(c, &req) {
		return
	}
	item, err := h.svc.Create(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, item)
}

// Update 更新角色；menuIds 为 nil 不改授权，[] 清空
func (h *RoleHandler) Update(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req request.RoleUpdateReq
	if !bindJSON(c, &req) {
		return
	}
	item, err := h.svc.Update(c.Request.Context(), id, &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, item)
}

// Delete 删除角色（仍被用户使用时拒绝）
func (h *RoleHandler) Delete(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"id": id})
}

// MenuIDs 角色已授权菜单 ID
func (h *RoleHandler) MenuIDs(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	ids, err := h.svc.MenuIDs(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"roleId": id, "menuIds": ids})
}

// AssignMenus 全量覆盖角色菜单授权
func (h *RoleHandler) AssignMenus(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req request.AssignMenusReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.svc.AssignMenus(c.Request.Context(), id, *req.MenuIDs); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"roleId": id})
}
