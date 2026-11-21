package service

import (
	"context"
	"strings"

	"ai_health/internal/dto/request"
	"ai_health/internal/dto/response"
	"ai_health/internal/model/entity"
	"ai_health/internal/pkg/cache"
	"ai_health/internal/pkg/errcode"
	"ai_health/internal/repository"
)

// MenuService 菜单树与按钮权限管理。
//
// 不变量：
//   - 按钮节点必须带 permission，否则授权后不产生任何权限位；
//   - 父节点必须是目录或菜单，不能是按钮；
//   - 禁用父菜单时级联禁用全部后代，重新启用父级不会自动恢复后代（避免误放权）。
type MenuService struct {
	menus *repository.MenuRepo
	roles *repository.RoleRepo
	users *repository.UserRepo
	store *cache.Store
}

func NewMenuService(menus *repository.MenuRepo, roles *repository.RoleRepo, users *repository.UserRepo, store *cache.Store) *MenuService {
	return &MenuService{menus: menus, roles: roles, users: users, store: store}
}

// List 平铺菜单列表，管理端使用。
func (s *MenuService) List(ctx context.Context, q *request.MenuQuery) ([]*response.MenuNode, error) {
	list, err := s.menus.ListAll(ctx, q.OnlyEnabled)
	if err != nil {
		return nil, errcode.ErrInternal.WithDetail("list menus: %v", err)
	}
	out := make([]*response.MenuNode, 0, len(list))
	for i := range list {
		out = append(out, toMenuNode(&list[i]))
	}
	return out, nil
}

// Tree 管理端菜单树，包含按钮节点。
func (s *MenuService) Tree(ctx context.Context, q *request.MenuQuery) ([]*response.MenuNode, error) {
	list, err := s.menus.ListAll(ctx, q.OnlyEnabled)
	if err != nil {
		return nil, errcode.ErrInternal.WithDetail("list menus: %v", err)
	}
	return buildMenuTree(list), nil
}

// MyMenus 当前用户可见的侧边栏菜单（启用角色 + 启用菜单，过滤按钮）。
func (s *MenuService) MyMenus(ctx context.Context, uid uint64) ([]*response.MenuNode, error) {
	list, err := s.menus.ListByUser(ctx, uid)
	if err != nil {
		return nil, errcode.ErrInternal.WithDetail("list user menus: %v", err)
	}
	visible := make([]entity.SysMenu, 0, len(list))
	for i := range list {
		if list[i].IsButton() {
			continue
		}
		visible = append(visible, list[i])
	}
	return buildMenuTree(visible), nil
}

// Detail 菜单详情。
func (s *MenuService) Detail(ctx context.Context, id uint64) (*response.MenuNode, error) {
	m, err := s.menus.GetByID(ctx, id)
	if err != nil {
		return nil, errcode.ErrInternal.WithDetail("get menu: %v", err)
	}
	if m == nil {
		return nil, errcode.ErrNotFound.WithDetail("菜单 %d 不存在", id)
	}
	return toMenuNode(m), nil
}

// Create 新建菜单 / 按钮。父节点必须存在、启用且不是按钮。
func (s *MenuService) Create(ctx context.Context, req *request.MenuCreateReq) (*response.MenuNode, error) {
	name := strings.TrimSpace(req.Name)
	title := strings.TrimSpace(req.Title)
	if name == "" || title == "" {
		return nil, errcode.ErrInvalidParam.WithDetail("name / title 不能为空")
	}
	permission := strings.TrimSpace(req.Permission)
	if req.MenuType == entity.MenuTypeButton && permission == "" {
		return nil, errcode.ErrInvalidParam.WithDetail("按钮节点必须设置 permission")
	}

	parentID, err := s.resolveParent(ctx, 0, req.ParentID)
	if err != nil {
		return nil, err
	}
	exists, err := s.menus.ExistsName(ctx, name, 0)
	if err != nil {
		return nil, errcode.ErrInternal.WithDetail("check menu name: %v", err)
	}
	if exists {
		return nil, errcode.ErrMenuExists.WithDetail("name=%s", name)
	}

	enabled := true
	if req.IsEnabled != nil {
		enabled = *req.IsEnabled
	}
	hidden := false
	if req.IsHidden != nil {
		hidden = *req.IsHidden
	}
	m := &entity.SysMenu{
		ParentID:   parentID,
		MenuType:   req.MenuType,
		Name:       name,
		Title:      title,
		Path:       optString(req.Path),
		Component:  optString(req.Component),
		Redirect:   optString(req.Redirect),
		Icon:       optString(req.Icon),
		Permission: optString(permission),
		SortOrder:  req.SortOrder,
		IsHidden:   hidden,
		IsEnabled:  enabled,
	}
	if err := s.menus.Create(ctx, m); err != nil {
		return nil, errcode.ErrInternal.WithDetail("create menu: %v", err)
	}
	return s.Detail(ctx, m.ID)
}

// Update 部分更新菜单；禁用时级联禁用后代，父级变更做环检测。
func (s *MenuService) Update(ctx context.Context, id uint64, req *request.MenuUpdateReq) (*response.MenuNode, error) {
	m, err := s.menus.GetByID(ctx, id)
	if err != nil {
		return nil, errcode.ErrInternal.WithDetail("get menu: %v", err)
	}
	if m == nil {
		return nil, errcode.ErrNotFound.WithDetail("菜单 %d 不存在", id)
	}
	all, err := s.menus.ListAll(ctx, false)
	if err != nil {
		return nil, errcode.ErrInternal.WithDetail("list menus: %v", err)
	}
	byID := make(map[uint64]*entity.SysMenu, len(all))
	for i := range all {
		byID[all[i].ID] = &all[i]
	}

	updates := map[string]any{}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, errcode.ErrInvalidParam.WithDetail("name 不能为空")
		}
		exists, err := s.menus.ExistsName(ctx, name, id)
		if err != nil {
			return nil, errcode.ErrInternal.WithDetail("check menu name: %v", err)
		}
		if exists {
			return nil, errcode.ErrMenuExists.WithDetail("name=%s", name)
		}
		updates["name"] = name
	}
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			return nil, errcode.ErrInvalidParam.WithDetail("title 不能为空")
		}
		updates["title"] = title
	}
	if req.MenuType != nil {
		if *req.MenuType == entity.MenuTypeButton {
			n, err := s.menus.CountChildren(ctx, id)
			if err != nil {
				return nil, errcode.ErrInternal.WithDetail("count children: %v", err)
			}
			if n > 0 {
				return nil, errcode.ErrInvalidParam.WithDetail("存在子节点，不能改为按钮")
			}
		}
		updates["menu_type"] = *req.MenuType
	}
	newParentID := m.ParentID
	if req.ParentID != nil {
		parentID, err := s.resolveParent(ctx, id, req.ParentID)
		if err != nil {
			return nil, err
		}
		newParentID = parentID
		if parentID == nil {
			updates["parent_id"] = nil
		} else {
			if err := checkMenuCycle(byID, id, *parentID); err != nil {
				return nil, err
			}
			updates["parent_id"] = *parentID
		}
	}
	if req.Path != nil {
		updates["path"] = optString(*req.Path)
	}
	if req.Component != nil {
		updates["component"] = optString(*req.Component)
	}
	if req.Redirect != nil {
		updates["redirect"] = optString(*req.Redirect)
	}
	if req.Icon != nil {
		updates["icon"] = optString(*req.Icon)
	}
	if req.Permission != nil {
		updates["permission"] = optString(*req.Permission)
	}
	if req.SortOrder != nil {
		updates["sort_order"] = *req.SortOrder
	}
	if req.IsHidden != nil {
		updates["is_hidden"] = *req.IsHidden
	}

	// 类型或权限位变化后仍要满足「按钮必须有 permission」。
	newType := m.MenuType
	if req.MenuType != nil {
		newType = *req.MenuType
	}
	newPerm := m.Permission
	if req.Permission != nil {
		newPerm = optString(*req.Permission)
	}
	if newType == entity.MenuTypeButton && (newPerm == nil || *newPerm == "") {
		return nil, errcode.ErrInvalidParam.WithDetail("按钮节点必须设置 permission")
	}

	disableIDs := []uint64{}
	if req.IsEnabled != nil {
		// 启用节点时祖先必须全部启用，避免绕过「禁用父级 → 子树失效」的约束。
		if *req.IsEnabled {
			if err := ensureAncestorsEnabled(byID, newParentID); err != nil {
				return nil, err
			}
		}
		updates["is_enabled"] = *req.IsEnabled
		if !*req.IsEnabled {
			disableIDs = collectDescendants(all, id)
		}
	}
	if len(updates) == 0 {
		return toMenuNode(m), nil
	}

	if len(disableIDs) > 0 {
		_, err = s.menus.UpdateWithDescendants(ctx, id, updates, disableIDs)
	} else {
		_, err = s.menus.Update(ctx, id, updates)
	}
	if err != nil {
		return nil, errcode.ErrInternal.WithDetail("update menu: %v", err)
	}
	// 级联禁用的后代也可能被单独授权，不能只失效根菜单关联的角色。
	s.invalidateMenuUsers(ctx, append([]uint64{id}, disableIDs...)...)
	return s.Detail(ctx, id)
}

// Delete 删除菜单。存在子节点时拒绝，避免整棵子树被外键 SET NULL 拍平成根节点。
func (s *MenuService) Delete(ctx context.Context, id uint64) error {
	m, err := s.menus.GetByID(ctx, id)
	if err != nil {
		return errcode.ErrInternal.WithDetail("get menu: %v", err)
	}
	if m == nil {
		return errcode.ErrNotFound.WithDetail("菜单 %d 不存在", id)
	}
	n, err := s.menus.CountChildren(ctx, id)
	if err != nil {
		return errcode.ErrInternal.WithDetail("count children: %v", err)
	}
	if n > 0 {
		return errcode.ErrMenuHasChildren.WithDetail("菜单 %d 有 %d 个子节点", id, n)
	}
	roleIDs, err := s.menus.RoleIDs(ctx, id)
	if err != nil {
		return errcode.ErrInternal.WithDetail("list menu roles: %v", err)
	}
	affected, err := s.menus.Delete(ctx, id)
	if err != nil {
		return errcode.ErrInternal.WithDetail("delete menu: %v", err)
	}
	if affected == 0 {
		return errcode.ErrNotFound.WithDetail("菜单 %d 不存在", id)
	}
	s.invalidateRoles(ctx, roleIDs)
	return nil
}

// resolveParent 校验父菜单：0 / nil 表示根节点；父级必须存在、启用且不是按钮。
func (s *MenuService) resolveParent(ctx context.Context, selfID uint64, raw *uint64) (*uint64, error) {
	if raw == nil || *raw == 0 {
		return nil, nil
	}
	pid := *raw
	if pid == selfID {
		return nil, errcode.ErrMenuCycle.WithDetail("父菜单不能是自身")
	}
	parent, err := s.menus.GetByID(ctx, pid)
	if err != nil {
		return nil, errcode.ErrInternal.WithDetail("get parent menu: %v", err)
	}
	if parent == nil {
		return nil, errcode.ErrMenuParent.WithDetail("父菜单 %d 不存在", pid)
	}
	if parent.IsButton() {
		return nil, errcode.ErrMenuParent.WithDetail("按钮节点不能作为父菜单")
	}
	if !parent.IsEnabled {
		return nil, errcode.ErrMenuParent.WithDetail("父菜单 %d 已禁用", pid)
	}
	return &pid, nil
}

func (s *MenuService) invalidateMenuUsers(ctx context.Context, menuIDs ...uint64) {
	roleIDs := make([]uint64, 0, len(menuIDs))
	for _, menuID := range menuIDs {
		ids, err := s.menus.RoleIDs(ctx, menuID)
		if err != nil {
			continue
		}
		roleIDs = append(roleIDs, ids...)
	}
	s.invalidateRoles(ctx, roleIDs)
}

// ensureAncestorsEnabled 从直接父级向上检查，祖先必须存在且处于启用状态。
func ensureAncestorsEnabled(byID map[uint64]*entity.SysMenu, parentID *uint64) error {
	cur := parentID
	for steps := 0; steps <= len(byID); steps++ {
		if cur == nil || *cur == 0 {
			return nil
		}
		p, ok := byID[*cur]
		if !ok {
			return errcode.ErrMenuParent.WithDetail("父菜单 %d 不存在", *cur)
		}
		if p.IsButton() {
			return errcode.ErrMenuParent.WithDetail("按钮节点不能作为父菜单")
		}
		if !p.IsEnabled {
			return errcode.ErrMenuParent.WithDetail("父菜单 %d 已禁用", p.ID)
		}
		cur = p.ParentID
	}
	return errcode.ErrMenuCycle.WithDetail("菜单层级存在异常")
}

func (s *MenuService) invalidateRoles(ctx context.Context, roleIDs []uint64) {
	uids := make([]uint64, 0, len(roleIDs))
	for _, rid := range roleIDs {
		ids, err := s.roles.UserIDs(ctx, rid)
		if err != nil {
			continue
		}
		uids = append(uids, ids...)
	}
	invalidateRBAC(ctx, s.store, uids)
}

// checkMenuCycle 新父级不能是自身或其后代：从新父级向上走到根，遇到自身即非法。
func checkMenuCycle(byID map[uint64]*entity.SysMenu, selfID, parentID uint64) error {
	cur := parentID
	for steps := 0; steps <= len(byID); steps++ {
		if cur == selfID {
			return errcode.ErrMenuCycle.WithDetail("父菜单不能是自身或其后代")
		}
		p, ok := byID[cur]
		if !ok || p.ParentID == nil || *p.ParentID == 0 {
			return nil
		}
		cur = *p.ParentID
	}
	return errcode.ErrMenuCycle.WithDetail("菜单层级存在异常")
}

// collectDescendants 收集全部后代 ID（不含自身），用于父级禁用时级联。
func collectDescendants(all []entity.SysMenu, rootID uint64) []uint64 {
	children := make(map[uint64][]uint64, len(all))
	for i := range all {
		if all[i].ParentID != nil {
			children[*all[i].ParentID] = append(children[*all[i].ParentID], all[i].ID)
		}
	}
	out := make([]uint64, 0, 8)
	queue := []uint64{rootID}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, child := range children[cur] {
			out = append(out, child)
			queue = append(queue, child)
		}
	}
	return out
}

func buildMenuTree(list []entity.SysMenu) []*response.MenuNode {
	nodes := make(map[uint64]*response.MenuNode, len(list))
	for i := range list {
		nodes[list[i].ID] = toMenuNode(&list[i])
	}
	roots := make([]*response.MenuNode, 0, 8)
	for i := range list {
		node := nodes[list[i].ID]
		if list[i].ParentID != nil {
			if parent, ok := nodes[*list[i].ParentID]; ok {
				parent.Children = append(parent.Children, node)
				continue
			}
		}
		roots = append(roots, node)
	}
	return roots
}

func toMenuNode(m *entity.SysMenu) *response.MenuNode {
	node := &response.MenuNode{
		ID:        m.ID,
		ParentID:  m.ParentID,
		MenuType:  m.MenuType,
		Name:      m.Name,
		Title:     m.Title,
		SortOrder: m.SortOrder,
		IsHidden:  m.IsHidden,
		IsEnabled: m.IsEnabled,
	}
	if m.Path != nil {
		node.Path = *m.Path
	}
	if m.Component != nil {
		node.Component = *m.Component
	}
	if m.Redirect != nil {
		node.Redirect = *m.Redirect
	}
	if m.Icon != nil {
		node.Icon = *m.Icon
	}
	if m.Permission != nil {
		node.Permission = *m.Permission
	}
	return node
}

func optString(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}
