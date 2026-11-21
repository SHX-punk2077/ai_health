package entity

import (
	"time"

	"gorm.io/gorm"
)

// SysUser 用户主表。密码只存 bcrypt 哈希，第三方登录用户可为空。
type SysUser struct {
	ID           uint64         `gorm:"column:id;type:bigint unsigned;primaryKey;autoIncrement" json:"id,string"`
	Username     string         `gorm:"column:username;type:varchar(50);not null;uniqueIndex:uk_sys_user_username" json:"username"`
	PasswordHash *string        `gorm:"column:password_hash;type:varchar(255)" json:"-"`
	Nickname     *string        `gorm:"column:nickname;type:varchar(50)" json:"nickname"`
	Phone        *string        `gorm:"column:phone;type:varchar(20);uniqueIndex:uk_sys_user_phone" json:"phone"`
	Email        *string        `gorm:"column:email;type:varchar(100);uniqueIndex:uk_sys_user_email" json:"email"`
	Avatar       *string        `gorm:"column:avatar;type:varchar(500)" json:"avatar"`
	Status       uint8          `gorm:"column:status;type:tinyint unsigned;not null;default:1;index:idx_sys_user_status" json:"status"`
	LastLoginAt  *time.Time     `gorm:"column:last_login_at;type:datetime(3)" json:"lastLoginAt"`
	CreatedAt    time.Time      `gorm:"column:created_at;type:datetime(3);not null;autoCreateTime:milli" json:"createdAt"`
	UpdatedAt    time.Time      `gorm:"column:updated_at;type:datetime(3);not null;autoUpdateTime:milli" json:"updatedAt"`
	DeletedAt    gorm.DeletedAt `gorm:"column:deleted_at;type:datetime(3);index:idx_sys_user_deleted_at" json:"-"`

	// Roles RBAC 多对多。join 表 sys_user_role 的 id / created_at 由数据库默认值填充，
	// 关联操作只写 user_id / role_id；外键由 migration 管理，GORM 不重复建。
	Roles []SysRole `gorm:"many2many:sys_user_role;foreignKey:ID;joinForeignKey:UserID;references:ID;joinReferences:RoleID;constraint:-" json:"-"`
	// Profile 一对一健康档案（health_profile.user_id 唯一）。
	// 用 has one 表达反向关系，外键名与 migration 的 fk_health_profile_user 对齐。
	Profile *HealthProfile `gorm:"foreignKey:UserID;references:ID;constraint:fk_health_profile_user,OnDelete:CASCADE,OnUpdate:CASCADE" json:"-"`
	// 以下 has many 反向关联与 migration 外键一一对应，便于 Preload 与级联语义阅读。
	OauthIdentities []SysOauthIdentity `gorm:"foreignKey:UserID;references:ID;constraint:fk_oauth_user,OnDelete:CASCADE,OnUpdate:CASCADE" json:"-"`
	HealthRecords   []HealthRecord     `gorm:"foreignKey:UserID;references:ID;constraint:fk_health_record_user,OnDelete:CASCADE,OnUpdate:CASCADE" json:"-"`
	Conversations   []AIConversation   `gorm:"foreignKey:UserID;references:ID;constraint:fk_ai_conversation_user,OnDelete:CASCADE,OnUpdate:CASCADE" json:"-"`
}

func (SysUser) TableName() string { return "sys_user" }

// IsDisabled 账号是否被禁用。
func (u *SysUser) IsDisabled() bool { return u.Status == 0 }
