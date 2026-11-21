package entity

import "time"

// SysRole 角色。role_code 为权限判定的稳定标识（super_admin/admin/user）。
type SysRole struct {
	ID          uint64    `gorm:"column:id;type:bigint unsigned;primaryKey;autoIncrement" json:"id,string"`
	RoleCode    string    `gorm:"column:role_code;type:varchar(50);not null;uniqueIndex:uk_sys_role_code" json:"roleCode"`
	RoleName    string    `gorm:"column:role_name;type:varchar(50);not null" json:"roleName"`
	Description *string   `gorm:"column:description;type:varchar(255)" json:"description"`
	Status      uint8     `gorm:"column:status;type:tinyint unsigned;not null;default:1;index:idx_sys_role_status" json:"status"`
	CreatedAt   time.Time `gorm:"column:created_at;type:datetime(3);not null;autoCreateTime:milli" json:"createdAt"`
	UpdatedAt   time.Time `gorm:"column:updated_at;type:datetime(3);not null;autoUpdateTime:milli" json:"updatedAt"`

	// Users 拥有该角色的用户；与 SysUser.Roles 共用 sys_user_role join 表。
	Users []SysUser `gorm:"many2many:sys_user_role;foreignKey:ID;joinForeignKey:RoleID;references:ID;joinReferences:UserID;constraint:-" json:"-"`
	// Menus 角色被授予的菜单 / 按钮权限；与 SysMenu.Roles 共用 sys_role_menu join 表。
	Menus []SysMenu `gorm:"many2many:sys_role_menu;foreignKey:ID;joinForeignKey:RoleID;references:ID;joinReferences:MenuID;constraint:-" json:"-"`
}

func (SysRole) TableName() string { return "sys_role" }
