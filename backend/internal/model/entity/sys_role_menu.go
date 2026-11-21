package entity

import "time"

// SysRoleMenu 角色-菜单关联，role_id+menu_id 唯一。
// 该表由 migration 维护，id / created_at 有数据库默认值；
// 业务上优先通过 SysRole.Menus / SysMenu.Roles 操作。
type SysRoleMenu struct {
	ID        uint64    `gorm:"column:id;type:bigint unsigned;primaryKey;autoIncrement" json:"id,string"`
	RoleID    uint64    `gorm:"column:role_id;type:bigint unsigned;not null;uniqueIndex:uk_sys_role_menu,priority:1" json:"roleId,string"`
	MenuID    uint64    `gorm:"column:menu_id;type:bigint unsigned;not null;uniqueIndex:uk_sys_role_menu,priority:2;index:idx_sys_role_menu_menu_id" json:"menuId,string"`
	CreatedAt time.Time `gorm:"column:created_at;type:datetime(3);not null;autoCreateTime:milli" json:"createdAt"`

	Role *SysRole `gorm:"foreignKey:RoleID;references:ID;constraint:fk_sys_role_menu_role,OnDelete:CASCADE,OnUpdate:CASCADE" json:"-"`
	Menu *SysMenu `gorm:"foreignKey:MenuID;references:ID;constraint:fk_sys_role_menu_menu,OnDelete:CASCADE,OnUpdate:CASCADE" json:"-"`
}

func (SysRoleMenu) TableName() string { return "sys_role_menu" }
