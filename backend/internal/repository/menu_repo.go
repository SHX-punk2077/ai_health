package repository

import (
	"context"

	"gorm.io/gorm"

	"ai_health/internal/consts"
	"ai_health/internal/model/entity"
)

// MenuRepo 菜单 / 按钮权限的数据访问。树形组装属于业务逻辑，放在 service。
type MenuRepo struct {
	db *gorm.DB
}

func NewMenuRepo(db *gorm.DB) *MenuRepo { return &MenuRepo{db: db} }

func (r *MenuRepo) DB() *gorm.DB { return r.db }

// ListAll 全量菜单，按父级与排序号排列；onlyEnabled 为 true 时过滤禁用项。
func (r *MenuRepo) ListAll(ctx context.Context, onlyEnabled bool) ([]entity.SysMenu, error) {
	q := r.db.WithContext(ctx).Model(&entity.SysMenu{})
	if onlyEnabled {
		q = q.Where("is_enabled = ?", true)
	}
	var list []entity.SysMenu
	err := q.Order("sort_order ASC, id ASC").Find(&list).Error
	return list, err
}

// GetByID 未找到返回 (nil, nil)。
func (r *MenuRepo) GetByID(ctx context.Context, id uint64) (*entity.SysMenu, error) {
	var m entity.SysMenu
	err := r.db.WithContext(ctx).Where("id = ?", id).Limit(1).Find(&m).Error
	if err != nil || m.ID == 0 {
		return nil, err
	}
	return &m, nil
}

// ListByRoleIDs 多个角色可见菜单的去重并集（仅启用项）。
func (r *MenuRepo) ListByRoleIDs(ctx context.Context, roleIDs []uint64) ([]entity.SysMenu, error) {
	if len(roleIDs) == 0 {
		return []entity.SysMenu{}, nil
	}
	var list []entity.SysMenu
	err := r.db.WithContext(ctx).
		Table("sys_menu AS m").
		Select("DISTINCT m.*").
		Joins("JOIN sys_role_menu AS rm ON rm.menu_id = m.id").
		Where("rm.role_id IN ? AND m.is_enabled = ?", roleIDs, true).
		Order("m.sort_order ASC, m.id ASC").
		Scan(&list).Error
	return list, err
}

// ListByUser 用户通过角色可见的菜单（仅启用项），用于 GET /menus/my。
func (r *MenuRepo) ListByUser(ctx context.Context, uid uint64) ([]entity.SysMenu, error) {
	var list []entity.SysMenu
	err := r.db.WithContext(ctx).
		Table("sys_menu AS m").
		Select("DISTINCT m.*").
		Joins("JOIN sys_role_menu AS rm ON rm.menu_id = m.id").
		Joins("JOIN sys_user_role AS ur ON ur.role_id = rm.role_id").
		Joins("JOIN sys_role AS r ON r.id = ur.role_id AND r.status = ?", consts.StatusEnabled).
		Where("ur.user_id = ? AND m.is_enabled = ?", uid, true).
		Order("m.sort_order ASC, m.id ASC").
		Scan(&list).Error
	return list, err
}

// PermissionNames 用户拥有的权限位并集（sys_menu.permission，去重去空）。
func (r *MenuRepo) PermissionNames(ctx context.Context, uid uint64) ([]string, error) {
	perms := make([]string, 0, 8)
	err := r.db.WithContext(ctx).
		Table("sys_menu AS m").
		Distinct("m.permission").
		Joins("JOIN sys_role_menu AS rm ON rm.menu_id = m.id").
		Joins("JOIN sys_user_role AS ur ON ur.role_id = rm.role_id").
		Joins("JOIN sys_role AS r ON r.id = ur.role_id AND r.status = ?", consts.StatusEnabled).
		Where("ur.user_id = ? AND m.is_enabled = ? AND m.permission IS NOT NULL AND m.permission <> ''", uid, true).
		Pluck("m.permission", &perms).Error
	return perms, err
}

// Create 新建菜单；is_enabled 带 default:1，禁用节点需要在同一事务内显式回写。
func (r *MenuRepo) Create(ctx context.Context, m *entity.SysMenu) error {
	// is_enabled 带 default:1：GORM 会把零值字段回填成默认值，必须在 Create 前记录意图。
	disabled := !m.IsEnabled
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(m).Error; err != nil {
			return err
		}
		if disabled {
			return tx.Model(&entity.SysMenu{}).Where("id = ?", m.ID).Update("is_enabled", false).Error
		}
		return nil
	})
}

func (r *MenuRepo) Update(ctx context.Context, id uint64, updates map[string]any) (int64, error) {
	if len(updates) == 0 {
		return 0, nil
	}
	res := r.db.WithContext(ctx).Model(&entity.SysMenu{}).Where("id = ?", id).Updates(updates)
	return res.RowsAffected, res.Error
}

// Delete 物理删除；子节点 parent_id 由外键置空，角色关联级联删除。
func (r *MenuRepo) Delete(ctx context.Context, id uint64) (int64, error) {
	res := r.db.WithContext(ctx).Where("id = ?", id).Delete(&entity.SysMenu{})
	return res.RowsAffected, res.Error
}

// ExistsName 菜单标识是否已存在（uk_sys_menu_name），excludeID > 0 时忽略自身。
func (r *MenuRepo) ExistsName(ctx context.Context, name string, excludeID uint64) (bool, error) {
	q := r.db.WithContext(ctx).Model(&entity.SysMenu{}).Where("name = ?", name)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	var n int64
	err := q.Count(&n).Error
	return n > 0, err
}

// CountChildren 直接子节点数量，删除前校验。
func (r *MenuRepo) CountChildren(ctx context.Context, parentID uint64) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&entity.SysMenu{}).Where("parent_id = ?", parentID).Count(&n).Error
	return n, err
}

// CountByIDs 统计存在的菜单数量，与去重后的入参数量比对即可发现非法 ID。
func (r *MenuRepo) CountByIDs(ctx context.Context, ids []uint64) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	var n int64
	err := r.db.WithContext(ctx).Model(&entity.SysMenu{}).Where("id IN ?", ids).Count(&n).Error
	return n, err
}

// RoleIDs 已授予该菜单的角色 ID，用于菜单变更后的权限缓存失效。
func (r *MenuRepo) RoleIDs(ctx context.Context, menuID uint64) ([]uint64, error) {
	ids := make([]uint64, 0, 4)
	err := r.db.WithContext(ctx).Model(&entity.SysRoleMenu{}).
		Where("menu_id = ?", menuID).Pluck("role_id", &ids).Error
	return ids, err
}

// SetEnabled 批量启停菜单。
func (r *MenuRepo) SetEnabled(ctx context.Context, ids []uint64, enabled bool) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	res := r.db.WithContext(ctx).Model(&entity.SysMenu{}).Where("id IN ?", ids).Update("is_enabled", enabled)
	return res.RowsAffected, res.Error
}

// UpdateWithDescendants 更新菜单并可选级联禁用后代，保证「父级禁用 → 子树失效」的原子性。
func (r *MenuRepo) UpdateWithDescendants(ctx context.Context, id uint64, updates map[string]any, disableIDs []uint64) (int64, error) {
	var affected int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if len(updates) > 0 {
			res := tx.Model(&entity.SysMenu{}).Where("id = ?", id).Updates(updates)
			if res.Error != nil {
				return res.Error
			}
			affected = res.RowsAffected
		}
		if len(disableIDs) > 0 {
			if err := tx.Model(&entity.SysMenu{}).Where("id IN ?", disableIDs).
				Update("is_enabled", false).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return affected, err
}
