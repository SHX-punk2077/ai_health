package service

import (
	"context"

	"ai_health/internal/dto/request"
	"ai_health/internal/dto/response"
	"ai_health/internal/pkg/errcode"
	"ai_health/internal/repository"
)

// UserService 当前登录用户的资料读写。
//
// 安全边界：UserUpdateReq 里的 Status / RoleIDs 一律忽略，
// 普通用户不能通过资料接口提权；角色分配只走管理端接口。
type UserService struct {
	auth  *AuthService
	users *repository.UserRepo
	roles *repository.RoleRepo
}

func NewUserService(auth *AuthService, users *repository.UserRepo, roles *repository.RoleRepo) *UserService {
	return &UserService{auth: auth, users: users, roles: roles}
}

// Profile 当前用户资料，与 GET /auth/me 同构（含角色与权限位）。
func (s *UserService) Profile(ctx context.Context, uid uint64) (*response.UserInfo, error) {
	return s.auth.Profile(ctx, uid)
}

// UpdateProfile 只允许改昵称 / 邮箱 / 手机号 / 头像，返回更新后的完整资料。
func (s *UserService) UpdateProfile(ctx context.Context, uid uint64, req *request.UserUpdateReq) (*response.UserInfo, error) {
	updates := map[string]any{}

	if req.Nickname != nil {
		updates["nickname"] = *req.Nickname
	}
	if req.Email != nil {
		email := *req.Email
		if email != "" {
			exists, err := s.users.ExistsExcept(ctx, "email", email, uid)
			if err != nil {
				return nil, errcode.ErrInternal.WithDetail("check email: %v", err)
			}
			if exists {
				return nil, errcode.ErrUserExists.WithDetail("email 已被占用")
			}
			updates["email"] = email
		} else {
			// 清空邮箱必须写 NULL，否则唯一索引会把空串当成一个值。
			updates["email"] = nil
		}
	}
	if req.Phone != nil {
		phone := *req.Phone
		if phone != "" {
			exists, err := s.users.ExistsExcept(ctx, "phone", phone, uid)
			if err != nil {
				return nil, errcode.ErrInternal.WithDetail("check phone: %v", err)
			}
			if exists {
				return nil, errcode.ErrUserExists.WithDetail("phone 已被占用")
			}
			updates["phone"] = phone
		} else {
			updates["phone"] = nil
		}
	}
	if req.Avatar != nil {
		updates["avatar"] = *req.Avatar
	}

	if len(updates) == 0 {
		return s.auth.Profile(ctx, uid)
	}
	if _, err := s.users.UpdateFields(ctx, uid, updates); err != nil {
		return nil, errcode.ErrInternal.WithDetail("update profile: %v", err)
	}
	return s.auth.Profile(ctx, uid)
}

// MyRoles 当前用户的角色明细，供前端展示「我的角色」。
func (s *UserService) MyRoles(ctx context.Context, uid uint64) ([]response.RoleItem, error) {
	list, err := s.roles.ListByUser(ctx, uid)
	if err != nil {
		return nil, errcode.ErrInternal.WithDetail("list roles: %v", err)
	}
	out := make([]response.RoleItem, 0, len(list))
	for _, r := range list {
		item := response.RoleItem{
			ID:       r.ID,
			RoleCode: r.RoleCode,
			RoleName: r.RoleName,
			Status:   r.Status,
		}
		if r.Description != nil {
			item.Description = *r.Description
		}
		out = append(out, item)
	}
	return out, nil
}
