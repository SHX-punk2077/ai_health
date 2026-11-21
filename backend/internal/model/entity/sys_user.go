package entity

import (
	"time"

	"gorm.io/gorm"
)

// SysUser 用户主表。密码只存 bcrypt 哈希，第三方登录用户可为空。
type SysUser struct {
	ID           uint64         `gorm:"column:id;primaryKey;autoIncrement" json:"id,string"`
	Username     string         `gorm:"column:username;type:varchar(50);not null" json:"username"`
	PasswordHash *string        `gorm:"column:password_hash;type:varchar(255)" json:"-"`
	Nickname     *string        `gorm:"column:nickname;type:varchar(50)" json:"nickname"`
	Phone        *string        `gorm:"column:phone;type:varchar(20)" json:"phone"`
	Email        *string        `gorm:"column:email;type:varchar(100)" json:"email"`
	Avatar       *string        `gorm:"column:avatar;type:varchar(500)" json:"avatar"`
	Status       uint8          `gorm:"column:status;type:tinyint unsigned;not null;default:1" json:"status"`
	LastLoginAt  *time.Time     `gorm:"column:last_login_at;type:datetime(3)" json:"lastLoginAt"`
	CreatedAt    time.Time      `gorm:"column:created_at;autoCreateTime:milli" json:"createdAt"`
	UpdatedAt    time.Time      `gorm:"column:updated_at;autoUpdateTime:milli" json:"updatedAt"`
	DeletedAt    gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
}

func (SysUser) TableName() string { return "sys_user" }

// IsDisabled 账号是否被禁用。
func (u *SysUser) IsDisabled() bool { return u.Status == 0 }
