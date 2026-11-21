package service

import (
	"context"
	"regexp"
	"strings"

	"ai_health/internal/consts"
	"ai_health/internal/dto/request"
	"ai_health/internal/dto/response"
	"ai_health/internal/model/entity"
	"ai_health/internal/pkg/cache"
	"ai_health/internal/pkg/errcode"
	"ai_health/internal/repository"
)

// roleCodePattern 角色编码会出现在 JWT 之外的权限判定与前端判断中，限制为稳定格式。
var roleCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{2,49}$`)

// RoleService 角色管理与角色菜单授权。
type RoleService struct {
	roles *repository.RoleRepo
	menus *repository.MenuRepo
	users *repository.UserRepo
	store *cache.Store
}

func NewRoleService(roles *repository.RoleRepo, menus *repository.MenuRepo, users *repository.UserRepo, store *cache.Store) *RoleService {
	return &RoleService{roles: roles, menus: menus, users: users, store: store}
}

// List 角色列表，支持关键字与状态筛选。
func (s *RoleService) List(ctx context.Context, q *request.RoleQuery) ([]response.RoleItem, error) {
	list, err := s.roles.Search(ctx, strings.TrimSpace(q.Keyword), q.Status)
	if err != nil {
		return nil, errcode.ErrInternal.WithDetail("list roles: %v", err)
	}
	out := make([]response.RoleItem, 0, len(list))
	for i := range list {
		out = append(out, roleItem(&list[i]))
	}
	return out, nil
}

// Detail 角色详情。
func (s *RoleService) Detail(ctx context.Context, id uint64) (*response.RoleItem, error) {
	role, err := s.roles.GetByID(ctx, id)
	if err != nil {
		return nil, errcode.ErrInternal.WithDetail("get role: %v", err)
	}
	if role == nil {
		return nil, errcode.ErrNotFound.WithDetail("角色 %d 不存在", id)
	}
	item := roleItem(role)
	return &item, nil
}

// Create 创建角色并可同时授权菜单。
func (s *RoleService) Create(ctx context.Context, req *request.RoleCreateReq) (*response.RoleItem, error) {
	code := strings.TrimSpace(req.RoleCode)
	name := strings.TrimSpace(req.RoleName)
	if !roleCodePattern.MatchString(code) {
		return nil, errcode.ErrInvalidParam.WithDetail("roleCode 需以小写字母开头，仅含小写字母、数字、下划线，长度 3-50")
	}
	if name == "" {
		return nil, errcode.ErrInvalidParam.WithDetail("roleName 不能为空")
	}
	if code == consts.RoleSuperAdmin {
		return nil, errcode.ErrSystemRole.WithDetail("super_admin 为内置角色，不能重复创建")
	}
	exists, err := s.roles.ExistsCode(ctx, code, 0)
	if err != nil {
		return nil, errcode.ErrInternal.WithDetail("check role code: %v", err)
	}
	if exists {
		return nil, errcode.ErrRoleExists.WithDetail("roleCode=%s", code)
	}
	menuIDs, err := s.normalizeMenuIDs(ctx, req.MenuIDs)
	if err != nil {
		return nil, err
	}

	role := &entity.SysRole{RoleCode: code, RoleName: name, Status: consts.StatusEnabled}
	if desc := strings.TrimSpace(req.Description); desc != "" {
		role.Description = &desc
	}
	if req.Status != nil {
		role.Status = *req.Status
	}
	if err := s.roles.CreateWithMenus(ctx, role, menuIDs); err != nil {
		return nil, errcode.ErrInternal.WithDetail("create role: %v", err)
	}
	return s.Detail(ctx, role.ID)
}

// Update 部分更新角色；MenuIDs 为 nil 不动授权，为 [] 清空授权。
func (s *RoleService) Update(ctx context.Context, id uint64, req *request.RoleUpdateReq) (*response.RoleItem, error) {
	role, err := s.roles.GetByID(ctx, id)
	if err != nil {
		return nil, errcode.ErrInternal.WithDetail("get role: %v", err)
	}
	if role == nil {
		return nil, errcode.ErrNotFound.WithDetail("角色 %d 不存在", id)
	}

	updates := map[string]any{}
	if req.RoleName != nil {
		name := strings.TrimSpace(*req.RoleName)
		if name == "" {
			return nil, errcode.ErrInvalidParam.WithDetail("roleName 不能为空")
		}
		updates["role_name"] = name
	}
	if req.Description != nil {
		if desc := strings.TrimSpace(*req.Description); desc != "" {
			updates["description"] = desc
		} else {
			updates["description"] = nil
		}
	}
	if req.Status != nil {
		if role.RoleCode == consts.RoleSuperAdmin && *req.Status == consts.StatusDisabled {
			return nil, errcode.ErrSystemRole.WithDetail("super_admin 不能禁用")
		}
		updates["status"] = *req.Status
	}

	menuChanged := req.MenuIDs != nil
	var menuIDs []uint64
	if menuChanged {
		menuIDs, err = s.normalizeMenuIDs(ctx, *req.MenuIDs)
		if err != nil {
			return nil, err
		}
	}
	if len(updates) == 0 && !menuChanged {
		return s.Detail(ctx, id)
	}

	if menuChanged {
		_, err = s.roles.UpdateWithMenus(ctx, id, updates, menuIDs)
	} else {
		_, err = s.roles.Update(ctx, id, updates)
	}
	if err != nil {
		return nil, errcode.ErrInternal.WithDetail("update role: %v", err)
	}
	// 角色状态或菜单授权变化，持有该角色的用户权限缓存全部失效。
	s.invalidateRoleUsers(ctx, id)
	return s.Detail(ctx, id)
}

// Delete 删除未被使用的角色。已分配给用户的角色拒绝删除，避免用户突然失去权限。
func (s *RoleService) Delete(ctx context.Context, id uint64) error {
	role, err := s.roles.GetByID(ctx, id)
	if err != nil {
		return errcode.ErrInternal.WithDetail("get role: %v", err)
	}
	if role == nil {
		return errcode.ErrNotFound.WithDetail("角色 %d 不存在", id)
	}
	if role.RoleCode == consts.RoleSuperAdmin || role.RoleCode == consts.RoleUser {
		return errcode.ErrSystemRole.WithDetail("%s 为系统内置角色，不能删除", role.RoleCode)
	}

	uids, err := s.roles.UserIDs(ctx, id)
	if err != nil {
		return errcode.ErrInternal.WithDetail("list role users: %v", err)
	}
	affected, err := s.roles.DeleteUnused(ctx, id)
	if err != nil {
		return errcode.ErrInternal.WithDetail("delete role: %v", err)
	}
	if affected == 0 {
		// 删除条件未命中：可能是并发分配，也可能已被删除。
		n, err := s.roles.CountUsers(ctx, id)
		if err != nil {
			return errcode.ErrInternal.WithDetail("count role users: %v", err)
		}
		if n > 0 {
			return errcode.ErrRoleInUse.WithDetail("角色 %d 仍有 %d 个用户", id, n)
		}
		return errcode.ErrNotFound.WithDetail("角色 %d 不存在", id)
	}
	invalidateRBAC(ctx, s.store, uids)
	return nil
}

// MenuIDs 角色已授权的菜单 ID，用于前端回显授权树。
func (s *RoleService) MenuIDs(ctx context.Context, id uint64) ([]uint64, error) {
	role, err := s.roles.GetByID(ctx, id)
	if err != nil {
		return nil, errcode.ErrInternal.WithDetail("get role: %v", err)
	}
	if role == nil {
		return nil, errcode.ErrNotFound.WithDetail("角色 %d 不存在", id)
	}
	ids, err := s.roles.ListMenuIDs(ctx, id)
	if err != nil {
		return nil, errcode.ErrInternal.WithDetail("list role menus: %v", err)
	}
	return ids, nil
}

// AssignMenus 全量覆盖角色菜单授权。禁用角色不允许继续授权。
func (s *RoleService) AssignMenus(ctx context.Context, id uint64, menuIDs []uint64) error {
	role, err := s.roles.GetByID(ctx, id)
	if err != nil {
		return errcode.ErrInternal.WithDetail("get role: %v", err)
	}
	if role == nil {
		return errcode.ErrNotFound.WithDetail("角色 %d 不存在", id)
	}
	if role.Status != consts.StatusEnabled {
		return errcode.ErrRoleDisabled.WithDetail("角色 %d 已禁用，不能授权", id)
	}
	ids, err := s.normalizeMenuIDs(ctx, menuIDs)
	if err != nil {
		return err
	}
	uids, err := s.roles.UserIDs(ctx, id)
	if err != nil {
		return errcode.ErrInternal.WithDetail("list role users: %v", err)
	}
	if err := s.roles.ReplaceMenus(ctx, id, ids); err != nil {
		return errcode.ErrInternal.WithDetail("replace role menus: %v", err)
	}
	invalidateRBAC(ctx, s.store, uids)
	return nil
}

// normalizeMenuIDs 去重并确认菜单全部存在，防止前端伪造菜单 ID。
func (s *RoleService) normalizeMenuIDs(ctx context.Context, ids []uint64) ([]uint64, error) {
	ids = uniqueIDs(ids)
	if len(ids) == 0 {
		return []uint64{}, nil
	}
	n, err := s.menus.CountByIDs(ctx, ids)
	if err != nil {
		return nil, errcode.ErrInternal.WithDetail("count menus: %v", err)
	}
	if n != int64(len(ids)) {
		return nil, errcode.ErrInvalidParam.WithDetail("存在无效的 menuId")
	}
	return ids, nil
}

func (s *RoleService) invalidateRoleUsers(ctx context.Context, roleID uint64) {
	uids, err := s.roles.UserIDs(ctx, roleID)
	if err != nil {
		return
	}
	invalidateRBAC(ctx, s.store, uids)
}

func roleItem(r *entity.SysRole) response.RoleItem {
	item := response.RoleItem{
		ID:       r.ID,
		RoleCode: r.RoleCode,
		RoleName: r.RoleName,
		Status:   r.Status,
	}
	if r.Description != nil {
		item.Description = *r.Description
	}
	return item
}
