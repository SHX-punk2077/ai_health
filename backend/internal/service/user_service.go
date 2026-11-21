package service

import (
	"context"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"ai_health/internal/consts"
	"ai_health/internal/dto/request"
	"ai_health/internal/dto/response"
	"ai_health/internal/model/entity"
	"ai_health/internal/pkg/cache"
	"ai_health/internal/pkg/errcode"
	"ai_health/internal/repository"
)

// UserService 当前用户资料 + 管理端用户与角色分配。
//
// 安全边界：
//   - 普通用户的资料接口只接受昵称/邮箱/手机号/头像，Status / RoleIDs 一律忽略；
//   - 只有 super_admin 能授予或撤销 super_admin；
//   - 系统必须保留至少一个可用的 super_admin，且不能删除/禁用自己。
type UserService struct {
	auth  *AuthService
	users *repository.UserRepo
	roles *repository.RoleRepo
	store *cache.Store
}

func NewUserService(auth *AuthService, users *repository.UserRepo, roles *repository.RoleRepo, store *cache.Store) *UserService {
	return &UserService{auth: auth, users: users, roles: roles, store: store}
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
	return s.rolesOf(ctx, uid)
}

// List 管理端用户分页列表。
func (s *UserService) List(ctx context.Context, q *request.UserQuery) ([]response.UserItem, int64, error) {
	q.Normalize()
	list, total, err := s.users.List(ctx, repository.UserFilter{
		Keyword: strings.TrimSpace(q.Keyword),
		Status:  q.Status,
		RoleID:  q.RoleID,
	}, q.Offset(), q.PageSize)
	if err != nil {
		return nil, 0, errcode.ErrInternal.WithDetail("list users: %v", err)
	}
	uids := make([]uint64, 0, len(list))
	for i := range list {
		uids = append(uids, list[i].ID)
	}
	codeMap, err := s.roles.CodesByUsers(ctx, uids)
	if err != nil {
		return nil, 0, errcode.ErrInternal.WithDetail("list user roles: %v", err)
	}
	out := make([]response.UserItem, 0, len(list))
	for i := range list {
		codes := codeMap[list[i].ID]
		if codes == nil {
			codes = []string{}
		}
		out = append(out, userItem(&list[i], codes))
	}
	return out, total, nil
}

// Detail 管理端用户详情。
func (s *UserService) Detail(ctx context.Context, id uint64) (*response.UserItem, error) {
	u, err := s.users.GetByID(ctx, id)
	if err != nil {
		return nil, errcode.ErrInternal.WithDetail("get user: %v", err)
	}
	if u == nil {
		return nil, errcode.ErrUserNotFound.WithDetail("用户 %d 不存在", id)
	}
	codes, err := s.roles.CodesByUser(ctx, id)
	if err != nil {
		return nil, errcode.ErrInternal.WithDetail("list user roles: %v", err)
	}
	if codes == nil {
		codes = []string{}
	}
	item := userItem(u, codes)
	return &item, nil
}

// Create 管理端建号；未指定角色时默认绑定 user 角色。
func (s *UserService) Create(ctx context.Context, actorUID uint64, req *request.UserCreateReq) (*response.UserItem, error) {
	username := strings.TrimSpace(req.Username)
	if len(username) < 3 {
		return nil, errcode.ErrInvalidParam.WithDetail("username 至少 3 个字符")
	}
	exists, err := s.users.Exists(ctx, "username", username)
	if err != nil {
		return nil, errcode.ErrInternal.WithDetail("check username: %v", err)
	}
	if exists {
		return nil, errcode.ErrUserExists.WithDetail("username=%s", username)
	}
	email := strings.TrimSpace(req.Email)
	if email != "" {
		exists, err := s.users.Exists(ctx, "email", email)
		if err != nil {
			return nil, errcode.ErrInternal.WithDetail("check email: %v", err)
		}
		if exists {
			return nil, errcode.ErrUserExists.WithDetail("email 已被占用")
		}
	}
	phone := strings.TrimSpace(req.Phone)
	if phone != "" {
		exists, err := s.users.Exists(ctx, "phone", phone)
		if err != nil {
			return nil, errcode.ErrInternal.WithDetail("check phone: %v", err)
		}
		if exists {
			return nil, errcode.ErrUserExists.WithDetail("phone 已被占用")
		}
	}

	assign, err := s.loadSelectableRoles(ctx, actorUID, req.RoleIDs)
	if err != nil {
		return nil, err
	}
	if len(assign) == 0 {
		def, err := s.roles.GetByCode(ctx, consts.RoleUser)
		if err != nil {
			return nil, errcode.ErrInternal.WithDetail("get default role: %v", err)
		}
		if def == nil {
			return nil, errcode.ErrInternal.WithDetail("默认角色 user 不存在")
		}
		if def.Status != consts.StatusEnabled {
			return nil, errcode.ErrRoleDisabled.WithDetail("默认角色 user 已禁用")
		}
		assign = append(assign, *def)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errcode.ErrInternal.WithDetail("hash password: %v", err)
	}
	hashStr := string(hash)
	u := &entity.SysUser{
		Username:     username,
		PasswordHash: &hashStr,
		Nickname:     optString(req.Nickname),
		Email:        optString(email),
		Phone:        optString(phone),
		Status:       consts.StatusEnabled,
	}
	if req.Status != nil {
		u.Status = *req.Status
	}
	roleIDs := make([]uint64, 0, len(assign))
	for _, r := range assign {
		roleIDs = append(roleIDs, r.ID)
	}
	if err := s.users.CreateWithRoles(ctx, u, roleIDs); err != nil {
		return nil, errcode.ErrInternal.WithDetail("create user: %v", err)
	}
	return s.Detail(ctx, u.ID)
}

// Update 管理端用户更新（不含角色；角色分配走 AssignRoles）。RoleIDs 字段被忽略。
func (s *UserService) Update(ctx context.Context, actorUID, id uint64, req *request.UserUpdateReq) (*response.UserItem, error) {
	u, err := s.users.GetByID(ctx, id)
	if err != nil {
		return nil, errcode.ErrInternal.WithDetail("get user: %v", err)
	}
	if u == nil {
		return nil, errcode.ErrUserNotFound.WithDetail("用户 %d 不存在", id)
	}

	updates := map[string]any{}
	if req.Nickname != nil {
		updates["nickname"] = optString(*req.Nickname)
	}
	if req.Avatar != nil {
		updates["avatar"] = optString(*req.Avatar)
	}
	if req.Email != nil {
		email := strings.TrimSpace(*req.Email)
		if email == "" {
			updates["email"] = nil
		} else {
			exists, err := s.users.ExistsExcept(ctx, "email", email, id)
			if err != nil {
				return nil, errcode.ErrInternal.WithDetail("check email: %v", err)
			}
			if exists {
				return nil, errcode.ErrUserExists.WithDetail("email 已被占用")
			}
			updates["email"] = email
		}
	}
	if req.Phone != nil {
		phone := strings.TrimSpace(*req.Phone)
		if phone == "" {
			updates["phone"] = nil
		} else {
			exists, err := s.users.ExistsExcept(ctx, "phone", phone, id)
			if err != nil {
				return nil, errcode.ErrInternal.WithDetail("check phone: %v", err)
			}
			if exists {
				return nil, errcode.ErrUserExists.WithDetail("phone 已被占用")
			}
			updates["phone"] = phone
		}
	}
	if req.Status != nil {
		if err := s.checkStatusChange(ctx, actorUID, id, *req.Status); err != nil {
			return nil, err
		}
		updates["status"] = *req.Status
	}

	if len(updates) > 0 {
		if _, err := s.users.UpdateFields(ctx, id, updates); err != nil {
			return nil, errcode.ErrInternal.WithDetail("update user: %v", err)
		}
	}
	return s.Detail(ctx, id)
}

// Delete 软删除用户并立即踢出全部会话。
func (s *UserService) Delete(ctx context.Context, actorUID, id uint64) error {
	if actorUID == id {
		return errcode.ErrForbidden.WithDetail("不能删除自己")
	}
	u, err := s.users.GetByID(ctx, id)
	if err != nil {
		return errcode.ErrInternal.WithDetail("get user: %v", err)
	}
	if u == nil {
		return errcode.ErrUserNotFound.WithDetail("用户 %d 不存在", id)
	}
	if err := s.ensureNotLastSuperAdmin(ctx, id); err != nil {
		return err
	}
	affected, err := s.users.SoftDelete(ctx, id)
	if err != nil {
		return errcode.ErrInternal.WithDetail("delete user: %v", err)
	}
	if affected == 0 {
		return errcode.ErrUserNotFound.WithDetail("用户 %d 不存在", id)
	}
	if err := s.auth.RevokeAll(ctx, id); err != nil {
		return errcode.ErrInternal.WithDetail("revoke tokens: %v", err)
	}
	invalidateRBAC(ctx, s.store, []uint64{id})
	return nil
}

// UpdateStatus 启用 / 禁用账号；禁用后立即失效其全部令牌。
func (s *UserService) UpdateStatus(ctx context.Context, actorUID, id uint64, status uint8) error {
	u, err := s.users.GetByID(ctx, id)
	if err != nil {
		return errcode.ErrInternal.WithDetail("get user: %v", err)
	}
	if u == nil {
		return errcode.ErrUserNotFound.WithDetail("用户 %d 不存在", id)
	}
	if err := s.checkStatusChange(ctx, actorUID, id, status); err != nil {
		return err
	}
	if _, err := s.users.SetStatus(ctx, id, status); err != nil {
		return errcode.ErrInternal.WithDetail("update status: %v", err)
	}
	invalidateRBAC(ctx, s.store, []uint64{id})
	if status == consts.StatusDisabled {
		if err := s.auth.RevokeAll(ctx, id); err != nil {
			return errcode.ErrInternal.WithDetail("revoke tokens: %v", err)
		}
	}
	return nil
}

// ResetPassword 管理员重置密码；成功后旧令牌全部失效。
func (s *UserService) ResetPassword(ctx context.Context, id uint64, newPassword string) error {
	u, err := s.users.GetByID(ctx, id)
	if err != nil {
		return errcode.ErrInternal.WithDetail("get user: %v", err)
	}
	if u == nil {
		return errcode.ErrUserNotFound.WithDetail("用户 %d 不存在", id)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return errcode.ErrInternal.WithDetail("hash password: %v", err)
	}
	hashStr := string(hash)
	if _, err := s.users.UpdateFields(ctx, id, map[string]any{"password_hash": hashStr}); err != nil {
		return errcode.ErrInternal.WithDetail("reset password: %v", err)
	}
	if err := s.auth.RevokeAll(ctx, id); err != nil {
		return errcode.ErrInternal.WithDetail("revoke tokens: %v", err)
	}
	return nil
}

// AssignRoles 全量覆盖用户角色。
func (s *UserService) AssignRoles(ctx context.Context, actorUID, id uint64, roleIDs []uint64) error {
	u, err := s.users.GetByID(ctx, id)
	if err != nil {
		return errcode.ErrInternal.WithDetail("get user: %v", err)
	}
	if u == nil {
		return errcode.ErrUserNotFound.WithDetail("用户 %d 不存在", id)
	}
	assign, err := s.loadSelectableRoles(ctx, actorUID, roleIDs)
	if err != nil {
		return err
	}
	curIDs, err := s.users.RoleIDs(ctx, id)
	if err != nil {
		return errcode.ErrInternal.WithDetail("list current roles: %v", err)
	}
	cur, err := s.roles.ListByIDs(ctx, curIDs)
	if err != nil {
		return errcode.ErrInternal.WithDetail("load current roles: %v", err)
	}

	actorIsSuper, err := s.isSuperAdmin(ctx, actorUID)
	if err != nil {
		return err
	}
	// 非超级管理员不能改动超级管理员的角色关系，避免把最后一个超管降级。
	if hasRole(cur, consts.RoleSuperAdmin) && !actorIsSuper {
		return errcode.ErrForbidden.WithDetail("只有 super_admin 能修改超级管理员的角色")
	}
	if hasRole(cur, consts.RoleSuperAdmin) && !hasRole(assign, consts.RoleSuperAdmin) {
		if err := s.ensureAnotherSuperAdmin(ctx); err != nil {
			return err
		}
	}

	ids := make([]uint64, 0, len(assign))
	for _, r := range assign {
		ids = append(ids, r.ID)
	}
	if err := s.users.ReplaceRoles(ctx, id, ids); err != nil {
		return errcode.ErrInternal.WithDetail("replace roles: %v", err)
	}
	invalidateRBAC(ctx, s.store, []uint64{id})
	return nil
}

// Roles 用户已分配角色明细（含禁用角色，供管理端修正）。
func (s *UserService) Roles(ctx context.Context, id uint64) ([]response.RoleItem, error) {
	u, err := s.users.GetByID(ctx, id)
	if err != nil {
		return nil, errcode.ErrInternal.WithDetail("get user: %v", err)
	}
	if u == nil {
		return nil, errcode.ErrUserNotFound.WithDetail("用户 %d 不存在", id)
	}
	return s.rolesOf(ctx, id)
}

func (s *UserService) rolesOf(ctx context.Context, uid uint64) ([]response.RoleItem, error) {
	list, err := s.roles.ListByUser(ctx, uid)
	if err != nil {
		return nil, errcode.ErrInternal.WithDetail("list roles: %v", err)
	}
	out := make([]response.RoleItem, 0, len(list))
	for i := range list {
		out = append(out, roleItem(&list[i]))
	}
	return out, nil
}

// loadSelectableRoles 校验角色存在且启用，并限制 super_admin 只能由 super_admin 授予。
func (s *UserService) loadSelectableRoles(ctx context.Context, actorUID uint64, roleIDs []uint64) ([]entity.SysRole, error) {
	ids := uniqueIDs(roleIDs)
	if len(ids) == 0 {
		return []entity.SysRole{}, nil
	}
	list, err := s.roles.ListByIDs(ctx, ids)
	if err != nil {
		return nil, errcode.ErrInternal.WithDetail("load roles: %v", err)
	}
	if len(list) != len(ids) {
		return nil, errcode.ErrInvalidParam.WithDetail("存在无效的 roleId")
	}
	for i := range list {
		if list[i].Status != consts.StatusEnabled {
			return nil, errcode.ErrRoleDisabled.WithDetail("角色 %s 已禁用，不能分配", list[i].RoleCode)
		}
	}
	if hasRole(list, consts.RoleSuperAdmin) {
		ok, err := s.isSuperAdmin(ctx, actorUID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, errcode.ErrForbidden.WithDetail("只有 super_admin 能授予 super_admin")
		}
	}
	return list, nil
}

func (s *UserService) checkStatusChange(ctx context.Context, actorUID, id uint64, status uint8) error {
	if status == consts.StatusDisabled {
		if actorUID == id {
			return errcode.ErrForbidden.WithDetail("不能禁用自己")
		}
		if err := s.ensureNotLastSuperAdmin(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// ensureNotLastSuperAdmin 目标若是最后一个启用的超管则拒绝降级 / 禁用 / 删除。
func (s *UserService) ensureNotLastSuperAdmin(ctx context.Context, id uint64) error {
	curIDs, err := s.users.RoleIDs(ctx, id)
	if err != nil {
		return errcode.ErrInternal.WithDetail("list user roles: %v", err)
	}
	cur, err := s.roles.ListByIDs(ctx, curIDs)
	if err != nil {
		return errcode.ErrInternal.WithDetail("load user roles: %v", err)
	}
	if !hasRole(cur, consts.RoleSuperAdmin) {
		return nil
	}
	return s.ensureAnotherSuperAdmin(ctx)
}

func (s *UserService) ensureAnotherSuperAdmin(ctx context.Context) error {
	role, err := s.roles.GetByCode(ctx, consts.RoleSuperAdmin)
	if err != nil {
		return errcode.ErrInternal.WithDetail("get super_admin role: %v", err)
	}
	if role == nil {
		return errcode.ErrInternal.WithDetail("super_admin 角色缺失")
	}
	n, err := s.roles.CountUsers(ctx, role.ID)
	if err != nil {
		return errcode.ErrInternal.WithDetail("count super admins: %v", err)
	}
	if n <= 1 {
		return errcode.ErrLastSuperAdmin
	}
	return nil
}

func (s *UserService) isSuperAdmin(ctx context.Context, uid uint64) (bool, error) {
	codes, err := s.auth.RoleCodes(ctx, uid)
	if err != nil {
		return false, errcode.ErrInternal.WithDetail("load actor roles: %v", err)
	}
	for _, code := range codes {
		if code == consts.RoleSuperAdmin {
			return true, nil
		}
	}
	return false, nil
}

func hasRole(roles []entity.SysRole, code string) bool {
	for i := range roles {
		if roles[i].RoleCode == code {
			return true
		}
	}
	return false
}

func userItem(u *entity.SysUser, roleCodes []string) response.UserItem {
	item := response.UserItem{
		ID:          u.ID,
		Username:    u.Username,
		Status:      u.Status,
		LastLoginAt: u.LastLoginAt,
		CreatedAt:   u.CreatedAt,
		Roles:       roleCodes,
	}
	if u.Nickname != nil {
		item.Nickname = *u.Nickname
	}
	if u.Email != nil {
		item.Email = *u.Email
	}
	if u.Phone != nil {
		item.Phone = *u.Phone
	}
	if u.Avatar != nil {
		item.Avatar = *u.Avatar
	}
	return item
}
