// Package handler 只做参数绑定、校验与响应封装，业务规则一律下沉到 service。
package handler

// Handlers 聚合所有 HTTP 处理器，供 router 注入。
type Handlers struct {
	Auth   *AuthHandler
	User   *UserHandler
	Role   *RoleHandler
	Menu   *MenuHandler
	Health *HealthHandler
	Sport  *SportHandler
	AI     *AIHandler
}

func New(
	auth *AuthHandler,
	user *UserHandler,
	role *RoleHandler,
	menu *MenuHandler,
	health *HealthHandler,
	sport *SportHandler,
	ai *AIHandler,
) *Handlers {
	return &Handlers{Auth: auth, User: user, Role: role, Menu: menu, Health: health, Sport: sport, AI: ai}
}
