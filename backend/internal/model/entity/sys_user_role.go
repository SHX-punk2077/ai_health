package entity

import "time"

// SysUserRole 用户-角色关联，user_id+role_id 唯一。
// 该表由 migration 维护，id / created_at 有数据库默认值；
// 业务上优先通过 SysUser.Roles 操作，需要审计明细时再直接使用本实体。
type SysUserRole struct {
	ID        uint64    `gorm:"column:id;type:bigint unsigned;primaryKey;autoIncrement" json:"id,string"`
	UserID    uint64    `gorm:"column:user_id;type:bigint unsigned;not null;uniqueIndex:uk_sys_user_role,priority:1" json:"userId,string"`
	RoleID    uint64    `gorm:"column:role_id;type:bigint unsigned;not null;uniqueIndex:uk_sys_user_role,priority:2;index:idx_sys_user_role_role_id" json:"roleId,string"`
	CreatedAt time.Time `gorm:"column:created_at;type:datetime(3);not null;autoCreateTime:milli" json:"createdAt"`

	User *SysUser `gorm:"foreignKey:UserID;references:ID;constraint:fk_sys_user_role_user,OnDelete:CASCADE,OnUpdate:CASCADE" json:"-"`
	Role *SysRole `gorm:"foreignKey:RoleID;references:ID;constraint:fk_sys_user_role_role,OnDelete:CASCADE,OnUpdate:CASCADE" json:"-"`
}

func (SysUserRole) TableName() string { return "sys_user_role" }
