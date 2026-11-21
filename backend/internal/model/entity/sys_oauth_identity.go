package entity

import "time"

// SysOauthIdentity 第三方登录绑定，provider+open_id 唯一。
type SysOauthIdentity struct {
	ID        uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id,string"`
	UserID    uint64    `gorm:"column:user_id;not null" json:"userId,string"`
	Provider  string    `gorm:"column:provider;type:varchar(30);not null" json:"provider"`
	OpenID    string    `gorm:"column:open_id;type:varchar(128);not null" json:"openId"`
	UnionID   *string   `gorm:"column:union_id;type:varchar(128)" json:"unionId"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime:milli" json:"createdAt"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime:milli" json:"updatedAt"`
}

func (SysOauthIdentity) TableName() string { return "sys_oauth_identity" }
