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
//
// 权限位：
//   - 读接口（列表 / 详情）分别要求 user:list / role:list / menu:list；
//   - 写接口要求对应的 create / update / delete；
//   - 角色授权与用户角色分配都要求 role:assign；
//   - /menus/my 只要求登录，任何用户都要能拿到自己的侧边栏。
func registerSystem(v1 *gin.RouterGroup, h *handler.Handlers, auth gin.HandlerFunc) {
	users := v1.Group(prefixUser, auth)
	{
		users.GET("/profile", h.User.Profile)
		users.PUT("/profile", h.User.UpdateProfile)
		users.GET("/roles", h.User.MyRoles)

		users.GET("", middleware.RequirePerm("user:list"), h.User.List)
		users.GET("/:id", middleware.RequirePerm("user:list"), h.User.Detail)
		users.POST("", middleware.RequirePerm("user:create"), h.User.Create)
		users.PUT("/:id", middleware.RequirePerm("user:update"), h.User.Update)
		users.DELETE("/:id", middleware.RequirePerm("user:delete"), h.User.Delete)
		users.PATCH("/:id/status", middleware.RequirePerm("user:update"), h.User.UpdateStatus)
		users.PATCH("/:id/password", middleware.RequirePerm("user:update"), h.User.ResetPassword)
		users.GET("/:id/roles", middleware.RequirePerm("user:list"), h.User.Roles)
		users.PUT("/:id/roles", middleware.RequirePerm("role:assign"), h.User.AssignRoles)
	}

	roles := v1.Group(prefixRole, auth)
	{
		roles.GET("", middleware.RequirePerm("role:list"), h.Role.List)
		roles.GET("/:id", middleware.RequirePerm("role:list"), h.Role.Detail)
		roles.POST("", middleware.RequirePerm("role:create"), h.Role.Create)
		roles.PUT("/:id", middleware.RequirePerm("role:update"), h.Role.Update)
		roles.DELETE("/:id", middleware.RequirePerm("role:delete"), h.Role.Delete)
		roles.GET("/:id/menus", middleware.RequirePerm("role:list"), h.Role.MenuIDs)
		roles.PUT("/:id/menus", middleware.RequirePerm("role:assign"), h.Role.AssignMenus)
	}

	menus := v1.Group(prefixMenu, auth)
	{
		menus.GET("/tree", middleware.RequirePerm("menu:list"), h.Menu.Tree)
		menus.GET("/my", h.Menu.MyMenus)
		menus.GET("", middleware.RequirePerm("menu:list"), h.Menu.List)
		menus.GET("/:id", middleware.RequirePerm("menu:list"), h.Menu.Detail)
		menus.POST("", middleware.RequirePerm("menu:create"), h.Menu.Create)
		menus.PUT("/:id", middleware.RequirePerm("menu:update"), h.Menu.Update)
		menus.DELETE("/:id", middleware.RequirePerm("menu:delete"), h.Menu.Delete)
	}
}
