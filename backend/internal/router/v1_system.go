package router

import (
	"github.com/gin-gonic/gin"

	"ai_health/internal/handler"
	"ai_health/internal/middleware"
)

// registerSystem 系统管理：用户 / 角色 / 菜单。
//
// 注意注册顺序：/users/profile 与 /users/roles 是静态段，与 /users/:id 同级共存，
// gin 1.10 的树支持静态段优先匹配，已用 router 测试固定该行为。
func registerSystem(v1 *gin.RouterGroup, h *handler.Handlers, auth gin.HandlerFunc) {
	users := v1.Group(prefixUser, auth)
	{
		users.GET("/profile", h.User.Profile)
		users.PUT("/profile", h.User.UpdateProfile)
		users.GET("/roles", h.User.MyRoles)

		users.GET("", h.User.List)
		users.GET("/:id", h.User.Detail)
		users.POST("", middleware.RequirePerm("user:create"), h.User.Create)
		users.PUT("/:id", middleware.RequirePerm("user:update"), h.User.Update)
		users.DELETE("/:id", middleware.RequirePerm("user:delete"), h.User.Delete)
		users.PATCH("/:id/status", middleware.RequirePerm("user:update"), h.User.UpdateStatus)
		users.PATCH("/:id/password", middleware.RequirePerm("user:update"), h.User.ResetPassword)
		users.POST("/:id/roles", middleware.RequirePerm("role:assign"), h.User.AssignRoles)
	}

	roles := v1.Group(prefixRole, auth)
	{
		roles.GET("", h.Role.List)
		roles.POST("", middleware.RequirePerm("role:create"), h.Role.Create)
		roles.PUT("/:id", middleware.RequirePerm("role:update"), h.Role.Update)
		roles.DELETE("/:id", middleware.RequirePerm("role:delete"), h.Role.Delete)
		roles.POST("/:id/menus", middleware.RequirePerm("role:assign"), h.Role.AssignMenus)
	}

	menus := v1.Group(prefixMenu, auth)
	{
		menus.GET("/tree", h.Menu.Tree)
		menus.GET("/my", h.Menu.MyMenus)
		menus.POST("", middleware.RequirePerm("menu:create"), h.Menu.Create)
		menus.PUT("/:id", middleware.RequirePerm("menu:update"), h.Menu.Update)
		menus.DELETE("/:id", middleware.RequirePerm("menu:delete"), h.Menu.Delete)
	}
}
