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

// AuthService 注册、登录、令牌轮换与会话失效。
type AuthService struct {
	users  *repository.UserRepo
	store  *cache.Store
	tokens *jwtmgr.Manager
}

func NewAuthService(users *repository.UserRepo, store *cache.Store, tokens *jwtmgr.Manager) *AuthService {
	return &AuthService{users: users, store: store, tokens: tokens}
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

	if err := s.users.Create(ctx, u); err != nil {
		return nil, errcode.ErrInternal.WithDetail("create user: %v", err)
	}
	if err := s.users.BindRoleByCode(ctx, u.ID, consts.RoleUser); err != nil {
		return nil, errcode.ErrInternal.WithDetail("bind default role: %v", err)
	}
	return s.buildUserInfo(ctx, u, false)
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
	_ = s.users.UpdateLastLogin(ctx, u.ID, time.Now())

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
	setKey := fmt.Sprintf(consts.KeyUserJTIs, uid)
	members := s.store.SMembers(ctx, setKey)
	keys := make([]string, 0, len(members)+2)
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
	keys = append(keys, setKey, fmt.Sprintf(consts.KeyPermissions, uid))
	return s.store.Del(ctx, keys...)
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

	permKey := fmt.Sprintf(consts.KeyPermissions, u.ID)
	var perms []string
	if !s.store.GetJSON(ctx, permKey, &perms) {
		var err error
		perms, err = s.users.ListPermissions(ctx, u.ID)
		if err != nil {
			return nil, errcode.ErrInternal.WithDetail("load permissions: %v", err)
		}
		_ = s.store.SetJSON(ctx, permKey, perms, permCacheTTL)
	}
	roles, err := s.users.ListRoleCodes(ctx, u.ID)
	if err != nil {
		return nil, errcode.ErrInternal.WithDetail("load roles: %v", err)
	}
	if perms != nil {
		info.Permissions = perms
	}
	if roles != nil {
		info.Roles = roles
	}
	return info, nil
}
