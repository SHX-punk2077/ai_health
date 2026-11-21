package entity

import (
	"fmt"

	"gorm.io/gorm"
)

// SetupJoinTables 用显式 join 实体注册 RBAC 多对多关联。
// 生产环境不执行 AutoMigrate，因此只需注册关联，不能创建中间表。
func SetupJoinTables(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("nil db")
	}

	relations := []struct {
		model any
		field string
		join  any
	}{
		{&SysUser{}, "Roles", &SysUserRole{}},
		{&SysRole{}, "Users", &SysUserRole{}},
		{&SysRole{}, "Menus", &SysRoleMenu{}},
		{&SysMenu{}, "Roles", &SysRoleMenu{}},
	}

	for _, rel := range relations {
		if err := db.SetupJoinTable(rel.model, rel.field, rel.join); err != nil {
			return fmt.Errorf("setup join table %T.%s: %w", rel.model, rel.field, err)
		}
	}
	return nil
}
