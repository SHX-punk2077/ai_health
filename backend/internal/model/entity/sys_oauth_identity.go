package entity

import "time"

// SysOauthIdentity 第三方登录绑定，provider+open_id 唯一。
type SysOauthIdentity struct {
	ID        uint64    `gorm:"column:id;type:bigint unsigned;primaryKey;autoIncrement" json:"id,string"`
	UserID    uint64    `gorm:"column:user_id;type:bigint unsigned;not null;index:idx_oauth_user_id" json:"userId,string"`
	Provider  string    `gorm:"column:provider;type:varchar(30);not null;uniqueIndex:uk_oauth_provider_open_id,priority:1" json:"provider"`
	OpenID    string    `gorm:"column:open_id;type:varchar(128);not null;uniqueIndex:uk_oauth_provider_open_id,priority:2" json:"openId"`
	UnionID   *string   `gorm:"column:union_id;type:varchar(128);index:idx_oauth_union_id" json:"unionId"`
	CreatedAt time.Time `gorm:"column:created_at;type:datetime(3);not null;autoCreateTime:milli" json:"createdAt"`
	UpdatedAt time.Time `gorm:"column:updated_at;type:datetime(3);not null;autoUpdateTime:milli" json:"updatedAt"`

	User *SysUser `gorm:"foreignKey:UserID;references:ID;constraint:fk_oauth_user,OnDelete:CASCADE,OnUpdate:CASCADE" json:"-"`
}

func (SysOauthIdentity) TableName() string { return "sys_oauth_identity" }
