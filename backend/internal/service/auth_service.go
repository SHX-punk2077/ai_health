// Package service 承载业务规则与事务边界，只依赖 repository 与 pkg。
package service

import (
	"context"
	"fmt"
	"time"

	"ai_health/internal/consts"
	"ai_health/internal/dto/request"
	"ai_health/internal/dto/response"
	"ai_health/internal/model/entity"
	"ai_health/internal/pkg/cache"
	"ai_health/internal/pkg/errcode"
	"ai_health/internal/pkg/hash"
	jwtmgr "ai_health/internal/pkg/jwt"
	"ai_health/internal/repository"
)

const (
	maxLoginFail = 5
	permCacheTTL = 10 * time.Minute
)

// AuthService 注册、登录、令牌轮换、改密与会话失效。
//
// 角色 / 权限查询从 RoleRepo、MenuRepo 取；super_admin 的通配符属于业务规则，留在本层。
type AuthService struct {
	users  *repository.UserRepo
	roles  *repository.RoleRepo
	menus  *repository.MenuRepo
	store  *cache.Store
	tokens *jwtmgr.Manager
}

func NewAuthService(
	users *repository.UserRepo,
	roles *repository.RoleRepo,
	menus *repository.MenuRepo,
	store *cache.Store,
	tokens *jwtmgr.Manager,
) *AuthService {
	return &AuthService{users: users, roles: roles, menus: menus, store: store, tokens: tokens}
}

// Register 自助注册，默认绑定 user 角色。
func (s *AuthService) Register(ctx context.Context, req *request.RegisterReq) (*response.UserInfo, error) {
	for _, item := range []struct{ col, val string }{
		{"username", req.Username},
		{"email", req.Email},
		{"phone", req.Phone},
	} {
		exists, err := s.users.Exists(ctx, item.col, item.val)
		if err != nil {
			return nil, errcode.ErrInternal.WithDetail("check %s: %v", item.col, err)
		}
		if exists {
			return nil, errcode.ErrUserExists
		}
	}

	pwd, err := hash.Hash(req.Password)
	if err != nil {
		return nil, errcode.ErrInternal.WithDetail("hash password: %v", err)
	}

	u := &entity.SysUser{
		Username:     req.Username,
		PasswordHash: &pwd,
		Status:       consts.StatusEnabled,
	}
	if req.Nickname != "" {
		u.Nickname = &req.Nickname
	} else {
		u.Nickname = &req.Username
	}
	if req.Email != "" {
		u.Email = &req.Email
	}
	if req.Phone != "" {
		u.Phone = &req.Phone
	}

	// 创建用户与绑定默认角色在同一事务内完成，避免“无角色用户”的半成品数据。
	if err := s.users.CreateWithRole(ctx, u, consts.RoleUser); err != nil {
		return nil, errcode.ErrInternal.WithDetail("create user with default role: %v", err)
	}
	// 注册响应即带上默认角色，前端无需再请求一次 /users/roles。
	return s.buildUserInfo(ctx, u, true)
}

// Login 账号密码登录；连续失败按 IP 计数，超过阈值锁定 15 分钟。
func (s *AuthService) Login(ctx context.Context, req *request.LoginReq, ip string) (*response.Login, error) {
	failKey := fmt.Sprintf(consts.KeyLoginFail, ip)
	if n := s.store.Incr(ctx, failKey, 15*time.Minute); n > maxLoginFail {
		return nil, errcode.ErrTooManyRetry
	}

	u, err := s.users.GetByAccount(ctx, req.Account)
	if err != nil {
		return nil, errcode.ErrInternal.WithDetail("query account: %v", err)
	}
	if u == nil || u.PasswordHash == nil || !hash.Verify(*u.PasswordHash, req.Password) {
		return nil, errcode.ErrBadPassword
	}
	if u.IsDisabled() {
		return nil, errcode.ErrUserDisabled
	}

	_ = s.store.Del(ctx, failKey)
	_ = s.users.UpdateLastLogin(ctx, u.ID, time.Now().UTC())

	token, err := s.issue(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	info, err := s.buildUserInfo(ctx, u, true)
	if err != nil {
		return nil, err
	}
	return &response.Login{Token: *token, User: info}, nil
}

// Refresh 刷新令牌：一次性使用，旧 refresh 立即作废（轮换）。
func (s *AuthService) Refresh(ctx context.Context, raw string) (*response.Login, error) {
	claims, err := s.tokens.Parse(raw, "refresh")
	if err != nil {
		return nil, errcode.ErrTokenInvalid.WithDetail("parse refresh: %v", err)
	}
	key := fmt.Sprintf(consts.KeyRefreshToken, claims.UID, claims.ID)
	if !s.store.Exists(ctx, key) {
		return nil, errcode.ErrTokenRevoked
	}
	_ = s.store.Del(ctx, key)

	u, err := s.users.GetByID(ctx, claims.UID)
	if err != nil || u == nil {
		return nil, errcode.ErrUserNotFound
	}
	if u.IsDisabled() {
		return nil, errcode.ErrUserDisabled
	}

	token, err := s.issue(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	info, err := s.buildUserInfo(ctx, u, true)
	if err != nil {
		return nil, err
	}
	return &response.Login{Token: *token, User: info}, nil
}

// Logout 清空该用户全部会话（单设备下线可改为只删当前 jti）。
func (s *AuthService) Logout(ctx context.Context, uid uint64) error {
	return s.revokeAll(ctx, uid)
}

// ChangePassword 校验旧密码后更新为新哈希，并让该用户所有既有令牌立即失效。
func (s *AuthService) ChangePassword(ctx context.Context, uid uint64, req *request.PasswordReq) error {
	if req.OldPassword == req.NewPassword {
		return errcode.ErrInvalidParam.WithDetail("新密码不能与旧密码相同")
	}

	u, err := s.users.GetByID(ctx, uid)
	if err != nil {
		return errcode.ErrInternal.WithDetail("get user: %v", err)
	}
	if u == nil {
		return errcode.ErrUserNotFound
	}
	if u.PasswordHash == nil || !hash.Verify(*u.PasswordHash, req.OldPassword) {
		return errcode.ErrBadPassword
	}

	pwd, err := hash.Hash(req.NewPassword)
	if err != nil {
		return errcode.ErrInternal.WithDetail("hash password: %v", err)
	}
	if _, err := s.users.UpdateFields(ctx, uid, map[string]any{"password_hash": pwd}); err != nil {
		return errcode.ErrInternal.WithDetail("update password: %v", err)
	}
	return s.revokeAll(ctx, uid)
}

// Profile 当前用户信息，带权限缓存。
func (s *AuthService) Profile(ctx context.Context, uid uint64) (*response.UserInfo, error) {
	u, err := s.users.GetByID(ctx, uid)
	if err != nil {
		return nil, errcode.ErrInternal.WithDetail("get user: %v", err)
	}
	if u == nil {
		return nil, errcode.ErrUserNotFound
	}
	return s.buildUserInfo(ctx, u, true)
}

// RoleCodes 用户角色编码，带缓存；供中间件与鉴权链路复用。
func (s *AuthService) RoleCodes(ctx context.Context, uid uint64) ([]string, error) {
	key := fmt.Sprintf(consts.KeyRoles, uid)
	codes := make([]string, 0, 2)
	if s.store.GetJSON(ctx, key, &codes) {
		return codes, nil
	}
	codes, err := s.roles.CodesByUser(ctx, uid)
	if err != nil {
		return nil, errcode.ErrInternal.WithDetail("load roles: %v", err)
	}
	if codes == nil {
		codes = []string{}
	}
	_ = s.store.SetJSON(ctx, key, codes, permCacheTTL)
	return codes, nil
}

// Permissions 用户权限位，带缓存。super_admin 返回通配符 "*"。
func (s *AuthService) Permissions(ctx context.Context, uid uint64) ([]string, error) {
	key := fmt.Sprintf(consts.KeyPermissions, uid)
	perms := make([]string, 0, 8)
	if s.store.GetJSON(ctx, key, &perms) {
		return perms, nil
	}

	codes, err := s.roles.CodesByUser(ctx, uid)
	if err != nil {
		return nil, errcode.ErrInternal.WithDetail("load roles: %v", err)
	}
	perms = []string{}
	for _, c := range codes {
		if c == consts.RoleSuperAdmin {
			perms = []string{"*"}
			break
		}
	}
	if len(perms) == 0 {
		perms, err = s.menus.PermissionNames(ctx, uid)
		if err != nil {
			return nil, errcode.ErrInternal.WithDetail("load permissions: %v", err)
		}
		if perms == nil {
			perms = []string{}
		}
	}
	_ = s.store.SetJSON(ctx, key, perms, permCacheTTL)
	return perms, nil
}

// issue 签发双令牌并写入 Redis 会话（JWT 白名单模式）。
func (s *AuthService) issue(ctx context.Context, uid uint64) (*response.Token, error) {
	pair, err := s.tokens.Generate(uid)
	if err != nil {
		return nil, errcode.ErrInternal.WithDetail("sign token: %v", err)
	}

	setKey := fmt.Sprintf(consts.KeyUserJTIs, uid)
	_ = s.store.Set(ctx, fmt.Sprintf(consts.KeyAccessToken, uid, pair.AccessJTI), "1", s.tokens.AccessTTL())
	_ = s.store.Set(ctx, fmt.Sprintf(consts.KeyRefreshToken, uid, pair.RefreshJTI), "1", s.tokens.RefreshTTL())
	_ = s.store.SAdd(ctx, setKey, "a:"+pair.AccessJTI, "r:"+pair.RefreshJTI)
	_ = s.store.Set(ctx, setKey+":ttl", "1", s.tokens.RefreshTTL())

	return &response.Token{
		AccessToken:      pair.AccessToken,
		RefreshToken:     pair.RefreshToken,
		TokenType:        "Bearer",
		ExpiresIn:        pair.AccessExpiresIn,
		RefreshExpiresIn: pair.RefreshExpiresIn,
	}, nil
}

// revokeAll 删除该用户全部 access / refresh 白名单与权限缓存。
func (s *AuthService) revokeAll(ctx context.Context, uid uint64) error {
	setKey := fmt.Sprintf(consts.KeyUserJTIs, uid)
	members := s.store.SMembers(ctx, setKey)
	keys := make([]string, 0, len(members)+3)
	for _, m := range members {
		if len(m) < 3 {
			continue
		}
		switch m[:2] {
		case "a:":
			keys = append(keys, fmt.Sprintf(consts.KeyAccessToken, uid, m[2:]))
		case "r:":
			keys = append(keys, fmt.Sprintf(consts.KeyRefreshToken, uid, m[2:]))
		}
	}
	keys = append(keys,
		setKey,
		setKey+":ttl",
		fmt.Sprintf(consts.KeyPermissions, uid),
		fmt.Sprintf(consts.KeyRoles, uid),
	)
	return s.store.Del(ctx, keys...)
}

// buildUserInfo 组装用户信息；withPerm 为 true 时附带角色与权限。
func (s *AuthService) buildUserInfo(ctx context.Context, u *entity.SysUser, withPerm bool) (*response.UserInfo, error) {
	info := &response.UserInfo{
		ID:          u.ID,
		Username:    u.Username,
		Status:      u.Status,
		Roles:       []string{},
		Permissions: []string{},
	}
	if u.Nickname != nil {
		info.Nickname = *u.Nickname
	}
	if u.Avatar != nil {
		info.Avatar = *u.Avatar
	}
	if u.Email != nil {
		info.Email = *u.Email
	}
	if u.Phone != nil {
		info.Phone = *u.Phone
	}
	if !withPerm {
		return info, nil
	}

	perms, err := s.Permissions(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	roles, err := s.RoleCodes(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	if perms != nil {
		info.Permissions = perms
	}
	if roles != nil {
		info.Roles = roles
	}
	return info, nil
}
