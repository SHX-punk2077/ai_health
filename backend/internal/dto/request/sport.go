package request

// SportInfoReq 运动建议维护入参。
type SportInfoReq struct {
	SportTypeID       uint64  `json:"sportTypeId" binding:"required"`
	SuitableTime      string  `json:"suitableTime" binding:"max=100"`
	SuitableHeartRate string  `json:"suitableHeartRate" binding:"max=100"`
	SuitableFrequency string  `json:"suitableFrequency" binding:"max=100"`
	RecommendedSpeed  string  `json:"recommendedSpeed" binding:"max=100"`
	DurationMinutes   *uint16 `json:"durationMinutes"`
	Intensity         string  `json:"intensity" binding:"max=30"`
	Description       string  `json:"description"`
	Status            *uint8  `json:"status" binding:"omitempty,oneof=0 1"`
}

// SportQuery 运动建议查询。
type SportQuery struct {
	PageQuery
	TypeCode string `form:"typeCode" binding:"max=50"`
	Status   *uint8 `form:"status"`
}

// DiseaseQuery 疾病运动禁忌查询。
type DiseaseQuery struct {
	PageQuery
	DiseaseName string `form:"diseaseName" binding:"max=100"`
	TypeCode    string `form:"typeCode" binding:"max=50"`
	RiskLevel   *uint8 `form:"riskLevel" binding:"omitempty,oneof=1 2 3"`
}
