package entity

import "time"

// 风险等级：1 一般（可正常运动），2 谨慎（需控制强度），3 需专业评估。
const (
	RiskLow    uint8 = 1
	RiskMedium uint8 = 2
	RiskHigh   uint8 = 3
)

// SportDiseaseDetail 疾病-运动禁忌与建议，risk_level 决定展示告警强度。
type SportDiseaseDetail struct {
	ID                uint64    `gorm:"column:id;type:bigint unsigned;primaryKey;autoIncrement" json:"id,string"`
	SportTypeID       uint64    `gorm:"column:sport_type_id;type:bigint unsigned;not null;index:idx_sport_disease_type" json:"sportTypeId,string"`
	DiseaseName       string    `gorm:"column:disease_name;type:varchar(100);not null;index:idx_sport_disease_name" json:"diseaseName"`
	RecommendedMethod *string   `gorm:"column:recommended_method;type:varchar(1000)" json:"recommendedMethod"`
	Precautions       *string   `gorm:"column:precautions;type:varchar(1000)" json:"precautions"`
	RiskLevel         *uint8    `gorm:"column:risk_level;type:tinyint unsigned" json:"riskLevel"`
	CreatedAt         time.Time `gorm:"column:created_at;type:datetime(3);not null;autoCreateTime:milli" json:"createdAt"`
	UpdatedAt         time.Time `gorm:"column:updated_at;type:datetime(3);not null;autoUpdateTime:milli" json:"updatedAt"`

	SportType *SportType `gorm:"foreignKey:SportTypeID;references:ID;constraint:fk_sport_disease_type,OnDelete:RESTRICT,OnUpdate:CASCADE" json:"sportType,omitempty"`
}

func (SportDiseaseDetail) TableName() string { return "sport_disease_detail" }
