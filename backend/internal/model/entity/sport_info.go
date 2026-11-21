package entity

import "time"

// SportInfo 运动建议：适宜时间/心率/频率等，一类型可多条。
type SportInfo struct {
	ID                uint64    `gorm:"column:id;type:bigint unsigned;primaryKey;autoIncrement" json:"id,string"`
	SportTypeID       uint64    `gorm:"column:sport_type_id;type:bigint unsigned;not null;index:idx_sport_info_type_status,priority:1" json:"sportTypeId,string"`
	SuitableTime      *string   `gorm:"column:suitable_time;type:varchar(100)" json:"suitableTime"`
	SuitableHeartRate *string   `gorm:"column:suitable_heart_rate;type:varchar(100)" json:"suitableHeartRate"`
	SuitableFrequency *string   `gorm:"column:suitable_frequency;type:varchar(100)" json:"suitableFrequency"`
	RecommendedSpeed  *string   `gorm:"column:recommended_speed;type:varchar(100)" json:"recommendedSpeed"`
	DurationMinutes   *uint16   `gorm:"column:duration_minutes;type:smallint unsigned" json:"durationMinutes"`
	Intensity         *string   `gorm:"column:intensity;type:varchar(30)" json:"intensity"`
	Description       *string   `gorm:"column:description;type:text" json:"description"`
	Status            uint8     `gorm:"column:status;type:tinyint unsigned;not null;default:1;index:idx_sport_info_type_status,priority:2" json:"status"`
	CreatedAt         time.Time `gorm:"column:created_at;type:datetime(3);not null;autoCreateTime:milli" json:"createdAt"`
	UpdatedAt         time.Time `gorm:"column:updated_at;type:datetime(3);not null;autoUpdateTime:milli" json:"updatedAt"`

	SportType *SportType `gorm:"foreignKey:SportTypeID;references:ID;constraint:fk_sport_info_type,OnDelete:RESTRICT,OnUpdate:CASCADE" json:"sportType,omitempty"`
}

func (SportInfo) TableName() string { return "sport_info" }
