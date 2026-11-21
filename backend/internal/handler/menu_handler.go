package handler

import (
	"github.com/gin-gonic/gin"

	"ai_health/internal/pkg/errcode"
	"ai_health/internal/pkg/response"
)

// MenuHandler 菜单树与权限位。
//
// TODO 需补 MenuService，树构建在 service 层完成，缓存 key=aihealth:cache:menu:tree。
type MenuHandler struct{}

func NewMenuHandler() *MenuHandler { return &MenuHandler{} }

// Tree 全量菜单树（管理端）
func (h *MenuHandler) Tree(c *gin.Context) {
	response.Fail(c, errcode.ErrNotImpl)
}

// MyMenus 我的菜单树（前端路由）
func (h *MenuHandler) MyMenus(c *gin.Context) {
	response.Fail(c, errcode.ErrNotImpl)
}

// Create 新建菜单/按钮
func (h *MenuHandler) Create(c *gin.Context) {
	response.Fail(c, errcode.ErrNotImpl)
}

// Update 更新菜单
func (h *MenuHandler) Update(c *gin.Context) {
	response.Fail(c, errcode.ErrNotImpl)
}

// Delete 删除菜单
func (h *MenuHandler) Delete(c *gin.Context) {
	response.Fail(c, errcode.ErrNotImpl)
}
