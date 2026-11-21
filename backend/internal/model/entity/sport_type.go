package entity

import "time"

// SportType 运动类型字典（walking/running/cycling/swimming/yoga）。
type SportType struct {
	ID          uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id,string"`
	TypeCode    string    `gorm:"column:type_code;type:varchar(50);not null" json:"typeCode"`
	TypeName    string    `gorm:"column:type_name;type:varchar(50);not null" json:"typeName"`
	Description *string   `gorm:"column:description;type:varchar(500)" json:"description"`
	Status      uint8     `gorm:"column:status;type:tinyint unsigned;not null;default:1" json:"status"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime:milli" json:"createdAt"`
	UpdatedAt   time.Time `gorm:"column:updated_at;autoUpdateTime:milli" json:"updatedAt"`
}

func (SportType) TableName() string { return "sport_type" }
