package entity

import "time"

// SportType 运动类型字典（walking/running/cycling/swimming/yoga）。
type SportType struct {
	ID          uint64    `gorm:"column:id;type:bigint unsigned;primaryKey;autoIncrement" json:"id,string"`
	TypeCode    string    `gorm:"column:type_code;type:varchar(50);not null;uniqueIndex:uk_sport_type_code" json:"typeCode"`
	TypeName    string    `gorm:"column:type_name;type:varchar(50);not null;uniqueIndex:uk_sport_type_name" json:"typeName"`
	Description *string   `gorm:"column:description;type:varchar(500)" json:"description"`
	Status      uint8     `gorm:"column:status;type:tinyint unsigned;not null;default:1" json:"status"`
	CreatedAt   time.Time `gorm:"column:created_at;type:datetime(3);not null;autoCreateTime:milli" json:"createdAt"`
	UpdatedAt   time.Time `gorm:"column:updated_at;type:datetime(3);not null;autoUpdateTime:milli" json:"updatedAt"`

	// Infos / DiseaseDetails 一对多，外键分别是 sport_info.sport_type_id 与
	// sport_disease_detail.sport_type_id，均为 ON DELETE RESTRICT。
	Infos          []SportInfo          `gorm:"foreignKey:SportTypeID;references:ID;constraint:fk_sport_info_type,OnDelete:RESTRICT,OnUpdate:CASCADE" json:"-"`
	DiseaseDetails []SportDiseaseDetail `gorm:"foreignKey:SportTypeID;references:ID;constraint:fk_sport_disease_type,OnDelete:RESTRICT,OnUpdate:CASCADE" json:"-"`
}

func (SportType) TableName() string { return "sport_type" }
