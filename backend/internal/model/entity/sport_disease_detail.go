package entity

import "time"

// 风险等级：1 低，2 中，3 高。
const (
	RiskLow    uint8 = 1
	RiskMedium uint8 = 2
	RiskHigh   uint8 = 3
)

// SportDiseaseDetail 疾病-运动禁忌与建议，risk_level 决定展示告警强度。
type SportDiseaseDetail struct {
	ID                uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id,string"`
	SportTypeID       uint64    `gorm:"column:sport_type_id;not null" json:"sportTypeId,string"`
	DiseaseName       string    `gorm:"column:disease_name;type:varchar(100);not null" json:"diseaseName"`
	RecommendedMethod *string   `gorm:"column:recommended_method;type:varchar(1000)" json:"recommendedMethod"`
	Precautions       *string   `gorm:"column:precautions;type:varchar(1000)" json:"precautions"`
	RiskLevel         *uint8    `gorm:"column:risk_level;type:tinyint unsigned" json:"riskLevel"`
	CreatedAt         time.Time `gorm:"column:created_at;autoCreateTime:milli" json:"createdAt"`
	UpdatedAt         time.Time `gorm:"column:updated_at;autoUpdateTime:milli" json:"updatedAt"`

	SportType *SportType `gorm:"foreignKey:SportTypeID;references:ID" json:"sportType,omitempty"`
}

func (SportDiseaseDetail) TableName() string { return "sport_disease_detail" }
