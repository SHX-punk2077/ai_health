package entity

import "time"

// SysUserRole 用户-角色关联，user_id+role_id 唯一。
type SysUserRole struct {
	ID        uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id,string"`
	UserID    uint64    `gorm:"column:user_id;not null" json:"userId,string"`
	RoleID    uint64    `gorm:"column:role_id;not null" json:"roleId,string"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime:milli" json:"createdAt"`
}

func (SysUserRole) TableName() string { return "sys_user_role" }
