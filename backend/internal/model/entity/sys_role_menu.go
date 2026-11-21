package entity

import "time"

// SysRoleMenu 角色-菜单关联，role_id+menu_id 唯一。
type SysRoleMenu struct {
	ID        uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id,string"`
	RoleID    uint64    `gorm:"column:role_id;not null" json:"roleId,string"`
	MenuID    uint64    `gorm:"column:menu_id;not null" json:"menuId,string"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime:milli" json:"createdAt"`
}

func (SysRoleMenu) TableName() string { return "sys_role_menu" }
