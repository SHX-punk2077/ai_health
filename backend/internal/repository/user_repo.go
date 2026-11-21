// Package repository 只做数据访问，禁止出现业务判断。
package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"ai_health/internal/consts"
	"ai_health/internal/model/entity"
)

// UserRepo 用户、角色、权限的数据访问。
type UserRepo struct {
	db *gorm.DB
}

func NewUserRepo(db *gorm.DB) *UserRepo { return &UserRepo{db: db} }

func (r *UserRepo) DB() *gorm.DB { return r.db }

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

// Exists 检查唯一字段是否已被占用（NULL 值不参与唯一索引，故需排除空串）。
func (r *UserRepo) Exists(ctx context.Context, column, value string) (bool, error) {
	if value == "" {
		return false, nil
	}
	var n int64
	err := r.db.WithContext(ctx).Model(&entity.SysUser{}).Where(column+" = ?", value).Count(&n).Error
	return n > 0, err
}

func (r *UserRepo) UpdateLastLogin(ctx context.Context, id uint64, t time.Time) error {
	return r.db.WithContext(ctx).Model(&entity.SysUser{}).
		Where("id = ?", id).Update("last_login_at", t).Error
}

// ListRoleCodes 取用户角色编码，Redis 命中时由 service 层短路。
func (r *UserRepo) ListRoleCodes(ctx context.Context, uid uint64) ([]string, error) {
	var codes []string
	err := r.db.WithContext(ctx).
		Table("sys_role AS r").
		Select("r.role_code").
		Joins("JOIN sys_user_role AS ur ON ur.role_id = r.id").
		Where("ur.user_id = ? AND r.status = 1", uid).
		Pluck("r.role_code", &codes).Error
	return codes, err
}

// ListPermissions 角色菜单并集；super_admin 直接返回通配符。
func (r *UserRepo) ListPermissions(ctx context.Context, uid uint64) ([]string, error) {
	codes, err := r.ListRoleCodes(ctx, uid)
	if err != nil {
		return nil, err
	}
	for _, c := range codes {
		if c == consts.RoleSuperAdmin {
			return []string{"*"}, nil
		}
	}

	var perms []string
	err = r.db.WithContext(ctx).
		Table("sys_menu AS m").
		Distinct("m.permission").
		Joins("JOIN sys_role_menu AS rm ON rm.menu_id = m.id").
		Joins("JOIN sys_user_role AS ur ON ur.role_id = rm.role_id").
		Joins("JOIN sys_role AS r ON r.id = ur.role_id AND r.status = 1").
		Where("ur.user_id = ? AND m.is_enabled = 1 AND m.permission IS NOT NULL AND m.permission <> ''", uid).
		Pluck("m.permission", &perms).Error
	if perms == nil {
		perms = []string{}
	}
	return perms, err
}

// BindRoleByCode 首次注册时绑定默认角色。
func (r *UserRepo) BindRoleByCode(ctx context.Context, uid uint64, roleCode string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var role entity.SysRole
		if err := tx.Where("role_code = ?", roleCode).First(&role).Error; err != nil {
			return err
		}
		rel := entity.SysUserRole{UserID: uid, RoleID: role.ID}
		return tx.Where("user_id = ? AND role_id = ?", uid, role.ID).
			FirstOrCreate(&rel).Error
	})
}
