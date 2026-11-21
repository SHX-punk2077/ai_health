package handler

import (
	"github.com/gin-gonic/gin"

	"ai_health/internal/dto/request"
	"ai_health/internal/pkg/errcode"
	"ai_health/internal/pkg/response"
	"ai_health/internal/service"
)

// AuthHandler 注册 / 登录 / 刷新 / 登出 / 当前用户。
type AuthHandler struct {
	svc *service.AuthService
}

func NewAuthHandler(svc *service.AuthService) *AuthHandler { return &AuthHandler{svc: svc} }

// Register 注册
// @Summary      用户注册
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body request.RegisterReq true "注册参数"
// @Success      200 {object} response.Body{data=response.UserInfo}
// @Router       /auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req request.RegisterReq
	if !bindJSON(c, &req) {
		return
	}
	info, err := h.svc.Register(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, info)
}

// Login 登录
// @Summary      账号密码登录
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body request.LoginReq true "登录参数"
// @Success      200 {object} response.Body{data=response.Login}
// @Router       /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req request.LoginReq
	if !bindJSON(c, &req) {
		return
	}
	out, err := h.svc.Login(c.Request.Context(), &req, c.ClientIP())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, out)
}

// Refresh 刷新令牌
// @Summary      刷新访问令牌
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body request.RefreshReq true "refreshToken"
// @Success      200 {object} response.Body{data=response.Login}
// @Router       /auth/refresh [post]
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req request.RefreshReq
	if !bindJSON(c, &req) {
		return
	}
	out, err := h.svc.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, out)
}

// Logout 登出
// @Summary      退出登录（清空会话）
// @Tags         auth
// @Security     Bearer
// @Success      200 {object} response.Body
// @Router       /auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	if err := h.svc.Logout(c.Request.Context(), currentUID(c)); err != nil {
		response.Fail(c, errcode.ErrInternal.WithDetail("logout: %v", err))
		return
	}
	response.OK(c, nil)
}

// Profile 当前用户
// @Summary      获取当前登录用户信息
// @Tags         auth
// @Security     Bearer
// @Success      200 {object} response.Body{data=response.UserInfo}
// @Router       /auth/profile [get]
func (h *AuthHandler) Profile(c *gin.Context) {
	info, err := h.svc.Profile(c.Request.Context(), currentUID(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, info)
}
