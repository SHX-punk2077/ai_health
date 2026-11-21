// Package repository 只做数据访问，禁止出现业务判断。
package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"ai_health/internal/model/entity"
)

// UserRepo 用户表数据访问。角色与权限查询分别在 RoleRepo、MenuRepo。
type UserRepo struct {
	db *gorm.DB
}

func NewUserRepo(db *gorm.DB) *UserRepo { return &UserRepo{db: db} }

func (r *UserRepo) DB() *gorm.DB { return r.db }

// UserFilter 用户列表筛选条件，nil 字段表示不限制。
type UserFilter struct {
	Keyword string
	Status  *uint8
	RoleID  *uint64
}

// GetByAccount 支持用户名 / 邮箱 / 手机号登录；未找到返回 (nil, nil)。
func (r *UserRepo) GetByAccount(ctx context.Context, account string) (*entity.SysUser, error) {
	var u entity.SysUser
	err := r.db.WithContext(ctx).
		Where("username = ? OR email = ? OR phone = ?", account, account, account).
		Limit(1).Find(&u).Error
	if err != nil {
		return nil, err
	}
	if u.ID == 0 {
		return nil, nil
	}
	return &u, nil
}

// GetByID 按主键查询，包含软删过滤（GORM 默认行为）。
func (r *UserRepo) GetByID(ctx context.Context, id uint64) (*entity.SysUser, error) {
	var u entity.SysUser
	err := r.db.WithContext(ctx).First(&u, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepo) Create(ctx context.Context, u *entity.SysUser) error {
	return r.db.WithContext(ctx).Create(u).Error
}

// CreateWithRole 注册事务：创建用户并绑定默认角色，任一步失败整体回滚，
// 避免出现“用户已创建但没有角色”的中间状态。
func (r *UserRepo) CreateWithRole(ctx context.Context, u *entity.SysUser, roleCode string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(u).Error; err != nil {
			return err
		}
		var role entity.SysRole
		if err := tx.Where("role_code = ? AND status = 1", roleCode).First(&role).Error; err != nil {
			return err
		}
		return tx.Create(&entity.SysUserRole{UserID: u.ID, RoleID: role.ID}).Error
	})
}

// Exists 检查唯一字段是否已被占用（NULL 值不参与唯一索引，故需排除空串）。
func (r *UserRepo) Exists(ctx context.Context, column, value string) (bool, error) {
	return r.ExistsExcept(ctx, column, value, 0)
}

// ExistsExcept 同 Exists，但忽略指定用户，供「修改资料时保留原邮箱/手机号」使用。
func (r *UserRepo) ExistsExcept(ctx context.Context, column, value string, excludeID uint64) (bool, error) {
	if value == "" {
		return false, nil
	}
	q := r.db.WithContext(ctx).Model(&entity.SysUser{}).Where(column+" = ?", value)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	var n int64
	err := q.Count(&n).Error
	return n > 0, err
}

// UpdateFields 按列更新用户；列名白名单由 service 保证。
func (r *UserRepo) UpdateFields(ctx context.Context, id uint64, updates map[string]any) (int64, error) {
	if len(updates) == 0 {
		return 0, nil
	}
	res := r.db.WithContext(ctx).Model(&entity.SysUser{}).Where("id = ?", id).Updates(updates)
	return res.RowsAffected, res.Error
}

func (r *UserRepo) UpdateLastLogin(ctx context.Context, id uint64, t time.Time) error {
	return r.db.WithContext(ctx).Model(&entity.SysUser{}).
		Where("id = ?", id).Update("last_login_at", t).Error
}

// List 分页查询，支持关键字 / 状态 / 角色筛选。
func (r *UserRepo) List(ctx context.Context, f UserFilter, offset, limit int) ([]entity.SysUser, int64, error) {
	q := r.db.WithContext(ctx).Model(&entity.SysUser{})
	if f.Keyword != "" {
		like := "%" + f.Keyword + "%"
		q = q.Where("username LIKE ? OR nickname LIKE ? OR email LIKE ? OR phone LIKE ?", like, like, like, like)
	}
	if f.Status != nil {
		q = q.Where("status = ?", *f.Status)
	}
	if f.RoleID != nil {
		q = q.Where("id IN (?)", r.db.Model(&entity.SysUserRole{}).Select("user_id").Where("role_id = ?", *f.RoleID))
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []entity.SysUser
	err := q.Order("id DESC").Offset(offset).Limit(limit).Find(&list).Error
	return list, total, err
}

// SetStatus 启用 / 禁用账号。
func (r *UserRepo) SetStatus(ctx context.Context, id uint64, status uint8) (int64, error) {
	res := r.db.WithContext(ctx).Model(&entity.SysUser{}).Where("id = ?", id).Update("status", status)
	return res.RowsAffected, res.Error
}

// SoftDelete 走 gorm.DeletedAt，保留审计轨迹。
func (r *UserRepo) SoftDelete(ctx context.Context, id uint64) (int64, error) {
	res := r.db.WithContext(ctx).Where("id = ?", id).Delete(&entity.SysUser{})
	return res.RowsAffected, res.Error
}

// RoleIDs 用户当前绑定的角色 ID，分配前做差异判断。
func (r *UserRepo) RoleIDs(ctx context.Context, uid uint64) ([]uint64, error) {
	ids := make([]uint64, 0, 2)
	err := r.db.WithContext(ctx).Model(&entity.SysUserRole{}).
		Where("user_id = ?", uid).Pluck("role_id", &ids).Error
	return ids, err
}

// CreateWithRoles 管理端建号：创建用户并写入角色，任一步失败整体回滚。
func (r *UserRepo) CreateWithRoles(ctx context.Context, u *entity.SysUser, roleIDs []uint64) error {
	// status 带 default:1：GORM 会把零值字段回填成默认值，必须在 Create 前记录意图。
	disabled := u.Status == 0
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(u).Error; err != nil {
			return err
		}
		if disabled {
			if err := tx.Model(&entity.SysUser{}).Where("id = ?", u.ID).Update("status", 0).Error; err != nil {
				return err
			}
		}
		if len(roleIDs) == 0 {
			return nil
		}
		rows := make([]entity.SysUserRole, 0, len(roleIDs))
		for _, rid := range roleIDs {
			rows = append(rows, entity.SysUserRole{UserID: u.ID, RoleID: rid})
		}
		return tx.Create(&rows).Error
	})
}

// ReplaceRoles 全量覆盖用户角色，删除与写入在同一事务内完成。
func (r *UserRepo) ReplaceRoles(ctx context.Context, uid uint64, roleIDs []uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", uid).Delete(&entity.SysUserRole{}).Error; err != nil {
			return err
		}
		if len(roleIDs) == 0 {
			return nil
		}
		rows := make([]entity.SysUserRole, 0, len(roleIDs))
		for _, rid := range roleIDs {
			rows = append(rows, entity.SysUserRole{UserID: uid, RoleID: rid})
		}
		return tx.Create(&rows).Error
	})
}
