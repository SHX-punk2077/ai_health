package entity

import "time"

// SysRole 角色。role_code 为权限判定的稳定标识（super_admin/admin/user）。
type SysRole struct {
	ID          uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id,string"`
	RoleCode    string    `gorm:"column:role_code;type:varchar(50);not null" json:"roleCode"`
	RoleName    string    `gorm:"column:role_name;type:varchar(50);not null" json:"roleName"`
	Description *string   `gorm:"column:description;type:varchar(255)" json:"description"`
	Status      uint8     `gorm:"column:status;type:tinyint unsigned;not null;default:1" json:"status"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime:milli" json:"createdAt"`
	UpdatedAt   time.Time `gorm:"column:updated_at;autoUpdateTime:milli" json:"updatedAt"`
}

func (SysRole) TableName() string { return "sys_role" }
