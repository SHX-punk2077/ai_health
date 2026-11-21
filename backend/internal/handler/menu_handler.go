package handler

import (
	"github.com/gin-gonic/gin"

	"ai_health/internal/dto/request"
	"ai_health/internal/pkg/response"
	"ai_health/internal/service"
)

// MenuHandler 菜单树与按钮权限管理。
type MenuHandler struct {
	svc *service.MenuService
}

func NewMenuHandler(svc *service.MenuService) *MenuHandler { return &MenuHandler{svc: svc} }

// List 平铺菜单列表
func (h *MenuHandler) List(c *gin.Context) {
	var q request.MenuQuery
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

// Tree 全量菜单树（管理端，含按钮节点）
func (h *MenuHandler) Tree(c *gin.Context) {
	var q request.MenuQuery
	if !bindQuery(c, &q) {
		return
	}
	tree, err := h.svc.Tree(c.Request.Context(), &q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, tree)
}

// MyMenus 我的菜单树（前端路由，过滤按钮）
func (h *MenuHandler) MyMenus(c *gin.Context) {
	tree, err := h.svc.MyMenus(c.Request.Context(), currentUID(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, tree)
}

// Detail 菜单详情
func (h *MenuHandler) Detail(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	node, err := h.svc.Detail(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, node)
}

// Create 新建菜单 / 按钮
func (h *MenuHandler) Create(c *gin.Context) {
	var req request.MenuCreateReq
	if !bindJSON(c, &req) {
		return
	}
	node, err := h.svc.Create(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, node)
}

// Update 更新菜单；禁用父级会级联禁用后代
func (h *MenuHandler) Update(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req request.MenuUpdateReq
	if !bindJSON(c, &req) {
		return
	}
	node, err := h.svc.Update(c.Request.Context(), id, &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, node)
}

// Delete 删除菜单（存在子节点时拒绝）
func (h *MenuHandler) Delete(c *gin.Context) {
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
