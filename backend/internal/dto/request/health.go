package request

import (
	"encoding/json"

	"ai_health/internal/pkg/optional"
)

// HealthMetrics 健康档案与健康记录共有的测量字段。
// DECIMAL 列统一用字符串接收，服务层解析为 decimal.Decimal，避免 float64 精度损失。
// 这里刻意不包含档案独有的姓名、年龄、生日、性别、病史等字段。
type HealthMetrics struct {
	HeightCm             *string `json:"heightCm"`
	WeightKg             *string `json:"weightKg"`
	BloodSugarMmol       *string `json:"bloodSugarMmol"`
	SystolicPressure     *uint16 `json:"systolicPressure" binding:"omitempty,max=300"`
	DiastolicPressure    *uint16 `json:"diastolicPressure" binding:"omitempty,max=200"`
	TotalCholesterolMmol *string `json:"totalCholesterolMmol"`
	HeartRate            *uint16 `json:"heartRate" binding:"omitempty,max=300"`
	VisionLeft           *string `json:"visionLeft"`
	VisionRight          *string `json:"visionRight"`
	SleepDurationHours   *string `json:"sleepDurationHours"`
	SleepQuality         *string `json:"sleepQuality" binding:"omitempty,max=30"`
	SmokingStatus        *uint8  `json:"smokingStatus" binding:"omitempty,oneof=0 1 2"`
	DrinkingStatus       *uint8  `json:"drinkingStatus" binding:"omitempty,oneof=0 1 2"`
	ExerciseHabit        *uint8  `json:"exerciseHabit" binding:"omitempty,oneof=0 1 2"`
	FoodPreferences      *string `json:"foodPreferences"`
	DailyWaterMl         *uint32 `json:"dailyWaterMl"`
}

// ProfileFields 健康档案字段。健康记录请求不得嵌入此结构。
type ProfileFields struct {
	HealthMetrics
	RealName       *string `json:"realName" binding:"omitempty,max=50"`
	Age            *uint8  `json:"age" binding:"omitempty,max=150"`
	BirthDate      *string `json:"birthDate"`
	Gender         *uint8  `json:"gender" binding:"omitempty,oneof=0 1 2"`
	MedicalHistory *string `json:"medicalHistory"`
	AllergyHistory *string `json:"allergyHistory"`
}

// ProfileCreateReq 创建当前登录用户的健康档案。
// 不包含 userId：归属只允许来自 JWT，避免前端篡改。
type ProfileCreateReq struct {
	ProfileFields
}

// ProfileUpdateReq 部分更新当前登录用户的健康档案：
// 未出现的字段保持不变；文本/日期/数值字符串传空串表示清空为 NULL。
type ProfileUpdateReq struct {
	ProfileFields
}

// RecordCreateReq 一条健康记录；只包含 health_record 真实存在的列。
type RecordCreateReq struct {
	HealthMetrics
	MeasuredAt string  `json:"measuredAt" binding:"required"`
	Source     string  `json:"source" binding:"omitempty,oneof=manual device import ai"`
	Remark     *string `json:"remark" binding:"omitempty,max=500"`
}

// RecordQuery 健康记录列表筛选。
// startAt/endAt 为 YYYY-MM-DD，endAt 当天完整包含；metric 过滤该指标非 NULL 的记录。
type RecordQuery struct {
	PageQuery
	StartAt string `form:"startAt"`
	EndAt   string `form:"endAt"`
	Metric  string `form:"metric"`
}

// TrendQuery 健康趋势查询。
// granularity 默认 day；limit 只保留最近 N 个聚合点，0 表示不限制。
type TrendQuery struct {
	Metric      string `form:"metric" binding:"required"`
	StartDate   string `form:"startDate"`
	EndDate     string `form:"endDate"`
	Granularity string `form:"granularity"`
	Limit       int    `form:"limit"`
}

// RecordUpdateReq 部分更新一条健康记录。
// 三态语义：字段未出现=保持原值；显式 null=保持原值；具体值才写入。
// DECIMAL/文本字段传空串表示清空为 NULL；整数/枚举字段显式 null 也保持原值。
// user_id、记录 id、created_at、source 属于受保护字段，出现即拒绝，不能借更新请求篡改归属或来源。
type RecordUpdateReq struct {
	HeightCm             optional.Optional[string] `json:"heightCm"`
	WeightKg             optional.Optional[string] `json:"weightKg"`
	BloodSugarMmol       optional.Optional[string] `json:"bloodSugarMmol"`
	SystolicPressure     optional.Optional[uint16] `json:"systolicPressure"`
	DiastolicPressure    optional.Optional[uint16] `json:"diastolicPressure"`
	TotalCholesterolMmol optional.Optional[string] `json:"totalCholesterolMmol"`
	HeartRate            optional.Optional[uint16] `json:"heartRate"`
	VisionLeft           optional.Optional[string] `json:"visionLeft"`
	VisionRight          optional.Optional[string] `json:"visionRight"`
	SleepDurationHours   optional.Optional[string] `json:"sleepDurationHours"`
	SleepQuality         optional.Optional[string] `json:"sleepQuality"`
	SmokingStatus        optional.Optional[uint8]  `json:"smokingStatus"`
	DrinkingStatus       optional.Optional[uint8]  `json:"drinkingStatus"`
	ExerciseHabit        optional.Optional[uint8]  `json:"exerciseHabit"`
	FoodPreferences      optional.Optional[string] `json:"foodPreferences"`
	DailyWaterMl         optional.Optional[uint32] `json:"dailyWaterMl"`
	MeasuredAt           optional.Optional[string] `json:"measuredAt"`
	Remark               optional.Optional[string] `json:"remark"`

	// 以下字段只用于识别并拒绝受保护字段，业务逻辑绝不会读取其值。
	ForbiddenID             json.RawMessage `json:"id"`
	ForbiddenUserID         json.RawMessage `json:"userId"`
	ForbiddenUserIDSnake    json.RawMessage `json:"user_id"`
	ForbiddenCreatedAt      json.RawMessage `json:"createdAt"`
	ForbiddenCreatedAtSnake json.RawMessage `json:"created_at"`
	ForbiddenSource         json.RawMessage `json:"source"`
}

// HasAnyField 表示请求体是否出现至少一个可更新字段（含显式 null）。
func (r RecordUpdateReq) HasAnyField() bool {
	return r.HeightCm.Set || r.WeightKg.Set || r.BloodSugarMmol.Set ||
		r.SystolicPressure.Set || r.DiastolicPressure.Set || r.TotalCholesterolMmol.Set ||
		r.HeartRate.Set || r.VisionLeft.Set || r.VisionRight.Set ||
		r.SleepDurationHours.Set || r.SleepQuality.Set || r.SmokingStatus.Set ||
		r.DrinkingStatus.Set || r.ExerciseHabit.Set || r.FoodPreferences.Set ||
		r.DailyWaterMl.Set || r.MeasuredAt.Set || r.Remark.Set
}

// ProtectedField 返回请求体中出现的第一个受保护字段名；没有则返回空串。
func (r RecordUpdateReq) ProtectedField() string {
	switch {
	case len(r.ForbiddenID) > 0:
		return "id"
	case len(r.ForbiddenUserID) > 0:
		return "userId"
	case len(r.ForbiddenUserIDSnake) > 0:
		return "user_id"
	case len(r.ForbiddenCreatedAt) > 0:
		return "createdAt"
	case len(r.ForbiddenCreatedAtSnake) > 0:
		return "created_at"
	case len(r.ForbiddenSource) > 0:
		return "source"
	default:
		return ""
	}
}
