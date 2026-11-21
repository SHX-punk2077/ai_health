package repository

import (
	"context"

	"gorm.io/gorm"

	"ai_health/internal/consts"
	"ai_health/internal/model/entity"
)

// RoleRepo 角色表与「角色-菜单」关联的数据访问。
type RoleRepo struct {
	db *gorm.DB
}

func NewRoleRepo(db *gorm.DB) *RoleRepo { return &RoleRepo{db: db} }

func (r *RoleRepo) DB() *gorm.DB { return r.db }

// List 角色列表；onlyEnabled 为 true 时只返回启用角色。
func (r *RoleRepo) List(ctx context.Context, onlyEnabled bool) ([]entity.SysRole, error) {
	q := r.db.WithContext(ctx).Model(&entity.SysRole{})
	if onlyEnabled {
		q = q.Where("status = ?", consts.StatusEnabled)
	}
	var list []entity.SysRole
	err := q.Order("id ASC").Find(&list).Error
	return list, err
}

// GetByID 未找到返回 (nil, nil)。
func (r *RoleRepo) GetByID(ctx context.Context, id uint64) (*entity.SysRole, error) {
	var role entity.SysRole
	err := r.db.WithContext(ctx).Where("id = ?", id).Limit(1).Find(&role).Error
	if err != nil || role.ID == 0 {
		return nil, err
	}
	return &role, nil
}

// GetByCode 按稳定编码查询，注册绑默认角色时使用。
func (r *RoleRepo) GetByCode(ctx context.Context, code string) (*entity.SysRole, error) {
	var role entity.SysRole
	err := r.db.WithContext(ctx).Where("role_code = ?", code).Limit(1).Find(&role).Error
	if err != nil || role.ID == 0 {
		return nil, err
	}
	return &role, nil
}

// ListByUser 用户拥有的全部角色（含已禁用，供管理端展示）。
func (r *RoleRepo) ListByUser(ctx context.Context, uid uint64) ([]entity.SysRole, error) {
	var list []entity.SysRole
	err := r.db.WithContext(ctx).
		Table("sys_role AS r").
		Select("r.*").
		Joins("JOIN sys_user_role AS ur ON ur.role_id = r.id").
		Where("ur.user_id = ?", uid).
		Order("r.id ASC").
		Scan(&list).Error
	return list, err
}

// CodesByUser 只取启用角色的编码，权限判定用。
func (r *RoleRepo) CodesByUser(ctx context.Context, uid uint64) ([]string, error) {
	codes := make([]string, 0, 2)
	err := r.db.WithContext(ctx).
		Table("sys_role AS r").
		Select("r.role_code").
		Joins("JOIN sys_user_role AS ur ON ur.role_id = r.id").
		Where("ur.user_id = ? AND r.status = ?", uid, consts.StatusEnabled).
		Pluck("r.role_code", &codes).Error
	return codes, err
}

// CodesByUsers 批量取多个用户的启用角色编码，列表页避免 N+1 查询。
func (r *RoleRepo) CodesByUsers(ctx context.Context, uids []uint64) (map[uint64][]string, error) {
	out := make(map[uint64][]string, len(uids))
	if len(uids) == 0 {
		return out, nil
	}
	var rows []struct {
		UserID   uint64
		RoleCode string
	}
	err := r.db.WithContext(ctx).
		Table("sys_role AS r").
		Select("ur.user_id AS user_id, r.role_code AS role_code").
		Joins("JOIN sys_user_role AS ur ON ur.role_id = r.id").
		Where("ur.user_id IN ? AND r.status = ?", uids, consts.StatusEnabled).
		Order("r.id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.UserID] = append(out[row.UserID], row.RoleCode)
	}
	return out, nil
}

func (r *RoleRepo) Create(ctx context.Context, role *entity.SysRole) error {
	return r.db.WithContext(ctx).Create(role).Error
}

// Update 按列更新，调用方负责白名单。
func (r *RoleRepo) Update(ctx context.Context, id uint64, updates map[string]any) (int64, error) {
	if len(updates) == 0 {
		return 0, nil
	}
	res := r.db.WithContext(ctx).Model(&entity.SysRole{}).Where("id = ?", id).Updates(updates)
	return res.RowsAffected, res.Error
}

// Delete 物理删除；sys_user_role / sys_role_menu 由外键级联清理。
func (r *RoleRepo) Delete(ctx context.Context, id uint64) (int64, error) {
	res := r.db.WithContext(ctx).Where("id = ?", id).Delete(&entity.SysRole{})
	return res.RowsAffected, res.Error
}

// ListMenuIDs 角色已授权的菜单 ID。
func (r *RoleRepo) ListMenuIDs(ctx context.Context, roleID uint64) ([]uint64, error) {
	ids := make([]uint64, 0, 8)
	err := r.db.WithContext(ctx).Model(&entity.SysRoleMenu{}).
		Where("role_id = ?", roleID).
		Pluck("menu_id", &ids).Error
	return ids, err
}

// ReplaceMenus 全量覆盖角色菜单授权，删除与写入在同一事务内完成。
func (r *RoleRepo) ReplaceMenus(ctx context.Context, roleID uint64, menuIDs []uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return replaceMenusTx(tx, roleID, menuIDs)
	})
}

// replaceMenusTx 供 CreateWithMenus / UpdateWithMenus 复用，保证写授权与写角色同事务。
func replaceMenusTx(tx *gorm.DB, roleID uint64, menuIDs []uint64) error {
	if err := tx.Where("role_id = ?", roleID).Delete(&entity.SysRoleMenu{}).Error; err != nil {
		return err
	}
	if len(menuIDs) == 0 {
		return nil
	}
	rows := make([]entity.SysRoleMenu, 0, len(menuIDs))
	for _, mid := range menuIDs {
		rows = append(rows, entity.SysRoleMenu{RoleID: roleID, MenuID: mid})
	}
	return tx.Create(&rows).Error
}

// Search 条件查询角色；keyword 匹配角色编码或名称，status 为 nil 表示不过滤状态。
func (r *RoleRepo) Search(ctx context.Context, keyword string, status *uint8) ([]entity.SysRole, error) {
	q := r.db.WithContext(ctx).Model(&entity.SysRole{})
	if keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where("role_code LIKE ? OR role_name LIKE ?", like, like)
	}
	if status != nil {
		q = q.Where("status = ?", *status)
	}
	var list []entity.SysRole
	err := q.Order("id ASC").Find(&list).Error
	return list, err
}

// ExistsCode 角色编码是否被占用；excludeID > 0 时忽略自身。
func (r *RoleRepo) ExistsCode(ctx context.Context, code string, excludeID uint64) (bool, error) {
	q := r.db.WithContext(ctx).Model(&entity.SysRole{}).Where("role_code = ?", code)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	var n int64
	err := q.Count(&n).Error
	return n > 0, err
}

// ListByIDs 批量取角色，用于分配前的存在性 / 状态校验。
func (r *RoleRepo) ListByIDs(ctx context.Context, ids []uint64) ([]entity.SysRole, error) {
	if len(ids) == 0 {
		return []entity.SysRole{}, nil
	}
	var list []entity.SysRole
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Order("id ASC").Find(&list).Error
	return list, err
}

// CountUsers 统计仍在使用该角色的用户数（软删用户不计）。
func (r *RoleRepo) CountUsers(ctx context.Context, roleID uint64) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).
		Table("sys_user_role AS ur").
		Joins("JOIN sys_user AS u ON u.id = ur.user_id AND u.deleted_at IS NULL").
		Where("ur.role_id = ?", roleID).
		Count(&n).Error
	return n, err
}

// UserIDs 拥有该角色的用户 ID，用于批量失效权限缓存。
func (r *RoleRepo) UserIDs(ctx context.Context, roleID uint64) ([]uint64, error) {
	ids := make([]uint64, 0, 8)
	err := r.db.WithContext(ctx).Model(&entity.SysUserRole{}).
		Where("role_id = ?", roleID).Pluck("user_id", &ids).Error
	return ids, err
}

// DeleteUnused 仅当没有活跃用户引用时删除角色，避免「先查后删」的竞态。
//
// 软删除用户不会触发 sys_user_role 的 ON DELETE CASCADE，其历史绑定行会永久
// 阻止角色删除；这些用户已不参与 CountUsers，因此先在事务内清掉他们的绑定。
func (r *RoleRepo) DeleteUnused(ctx context.Context, roleID uint64) (int64, error) {
	var affected int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`DELETE ur FROM sys_user_role ur
			JOIN sys_user u ON u.id = ur.user_id
			WHERE ur.role_id = ? AND u.deleted_at IS NOT NULL`, roleID).Error; err != nil {
			return err
		}
		res := tx.Where("id = ? AND NOT EXISTS (SELECT 1 FROM sys_user_role WHERE role_id = ?)", roleID, roleID).
			Delete(&entity.SysRole{})
		affected = res.RowsAffected
		return res.Error
	})
	return affected, err
}

// UpdateWithMenus 更新角色并全量覆盖菜单授权，两步同一事务。
func (r *RoleRepo) UpdateWithMenus(ctx context.Context, id uint64, updates map[string]any, menuIDs []uint64) (int64, error) {
	var affected int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if len(updates) > 0 {
			res := tx.Model(&entity.SysRole{}).Where("id = ?", id).Updates(updates)
			if res.Error != nil {
				return res.Error
			}
			affected = res.RowsAffected
		}
		return replaceMenusTx(tx, id, menuIDs)
	})
	return affected, err
}

// CreateWithMenus 创建角色并写入菜单授权，两步在同一事务内完成。
func (r *RoleRepo) CreateWithMenus(ctx context.Context, role *entity.SysRole, menuIDs []uint64) error {
	// status 带 default:1：GORM 会把零值字段回填成默认值，必须在 Create 前记录意图。
	disabled := role.Status == 0
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(role).Error; err != nil {
			return err
		}
		if disabled {
			if err := tx.Model(&entity.SysRole{}).Where("id = ?", role.ID).Update("status", 0).Error; err != nil {
				return err
			}
		}
		if len(menuIDs) == 0 {
			return nil
		}
		return replaceMenusTx(tx, role.ID, menuIDs)
	})
}
