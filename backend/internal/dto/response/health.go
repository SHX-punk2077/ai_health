package response

import (
	"time"

	"ai_health/internal/model/entity"
)

// Profile 健康档案，附带服务端推导的 BMI。
type Profile struct {
	entity.HealthProfile
	BMI *float64 `json:"bmi,omitempty"`
}

// Record 健康记录。
type Record struct {
	entity.HealthRecord
}

// TrendPoint 趋势曲线点。
type TrendPoint struct {
	MeasuredAt time.Time `json:"measuredAt"`
	Value      float64   `json:"value"`
}

// Trend 趋势响应。
type Trend struct {
	Metric string       `json:"metric"`
	Unit   string       `json:"unit"`
	Points []TrendPoint `json:"points"`
}
