package response

import "ai_health/internal/model/entity"

// SportInfo 运动建议（内嵌类型信息）。
type SportInfo struct {
	entity.SportInfo
}

// DiseaseDetail 疾病运动禁忌。
type DiseaseDetail struct {
	entity.SportDiseaseDetail
}
