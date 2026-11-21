package handler

import (
	"github.com/gin-gonic/gin"

	"ai_health/internal/pkg/errcode"
	"ai_health/internal/pkg/response"
)

// RoleHandler 角色与菜单授权。
//
// TODO 需补 RoleService，注意 super_admin 角色禁止删除。
type RoleHandler struct{}

func NewRoleHandler() *RoleHandler { return &RoleHandler{} }

// List 角色列表
func (h *RoleHandler) List(c *gin.Context) {
	response.Fail(c, errcode.ErrNotImpl)
}

// Create 新建角色
func (h *RoleHandler) Create(c *gin.Context) {
	response.Fail(c, errcode.ErrNotImpl)
}

// Update 更新角色
func (h *RoleHandler) Update(c *gin.Context) {
	response.Fail(c, errcode.ErrNotImpl)
}

// Delete 删除角色
func (h *RoleHandler) Delete(c *gin.Context) {
	response.Fail(c, errcode.ErrNotImpl)
}

// AssignMenus 分配菜单权限
func (h *RoleHandler) AssignMenus(c *gin.Context) {
	response.Fail(c, errcode.ErrNotImpl)
}
