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
	})
}
