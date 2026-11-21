package response

import (
	"strconv"
	"time"

	"github.com/shopspring/decimal"

	"ai_health/internal/model/entity"
)

// Profile 健康档案响应。字段逐个列出，禁止把 entity 直接作为 API 响应；
// DECIMAL 一律输出两位小数字符串，避免 float64 精度损失。
type Profile struct {
	ID                   string    `json:"id"`
	UserID               string    `json:"userId"`
	RealName             *string   `json:"realName"`
	Age                  *uint8    `json:"age"`
	BirthDate            *string   `json:"birthDate"`
	Gender               *uint8    `json:"gender"`
	HeightCm             *string   `json:"heightCm"`
	WeightKg             *string   `json:"weightKg"`
	BloodSugarMmol       *string   `json:"bloodSugarMmol"`
	SystolicPressure     *uint16   `json:"systolicPressure"`
	DiastolicPressure    *uint16   `json:"diastolicPressure"`
	TotalCholesterolMmol *string   `json:"totalCholesterolMmol"`
	HeartRate            *uint16   `json:"heartRate"`
	VisionLeft           *string   `json:"visionLeft"`
	VisionRight          *string   `json:"visionRight"`
	SleepDurationHours   *string   `json:"sleepDurationHours"`
	SleepQuality         *string   `json:"sleepQuality"`
	SmokingStatus        *uint8    `json:"smokingStatus"`
	DrinkingStatus       *uint8    `json:"drinkingStatus"`
	ExerciseHabit        *uint8    `json:"exerciseHabit"`
	FoodPreferences      *string   `json:"foodPreferences"`
	DailyWaterMl         *uint32   `json:"dailyWaterMl"`
	MedicalHistory       *string   `json:"medicalHistory"`
	AllergyHistory       *string   `json:"allergyHistory"`
	BMI                  *string   `json:"bmi"`
	CreatedAt            time.Time `json:"createdAt"`
	UpdatedAt            time.Time `json:"updatedAt"`
}

// HealthWarning 健康指标异常提醒。它是工程侧的风险提示，不是医学诊断。
type HealthWarning struct {
	Metric  string `json:"metric"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

// Record 健康记录响应，只包含 health_record 表真实存在的字段。
type Record struct {
	ID                   string    `json:"id"`
	UserID               string    `json:"userId"`
	HeightCm             *string   `json:"heightCm"`
	WeightKg             *string   `json:"weightKg"`
	BloodSugarMmol       *string   `json:"bloodSugarMmol"`
	SystolicPressure     *uint16   `json:"systolicPressure"`
	DiastolicPressure    *uint16   `json:"diastolicPressure"`
	TotalCholesterolMmol *string   `json:"totalCholesterolMmol"`
	HeartRate            *uint16   `json:"heartRate"`
	VisionLeft           *string   `json:"visionLeft"`
	VisionRight          *string   `json:"visionRight"`
	SleepDurationHours   *string   `json:"sleepDurationHours"`
	SleepQuality         *string   `json:"sleepQuality"`
	SmokingStatus        *uint8    `json:"smokingStatus"`
	DrinkingStatus       *uint8    `json:"drinkingStatus"`
	ExerciseHabit        *uint8    `json:"exerciseHabit"`
	FoodPreferences      *string   `json:"foodPreferences"`
	DailyWaterMl         *uint32   `json:"dailyWaterMl"`
	MeasuredAt           time.Time `json:"measuredAt"`
	Source               string    `json:"source"`
	Remark               *string   `json:"remark"`
	CreatedAt            time.Time `json:"createdAt"`
	UpdatedAt            time.Time `json:"updatedAt"`
	// Warnings 只在创建/更新记录时按需返回；查询历史记录时不回放告警。
	Warnings []HealthWarning `json:"warnings,omitempty"`
}

// HealthSummary 首页健康概览。每个指标独立取最新非 NULL 测量值，
// 不能用档案快照冒充最新测量值；无数据时字段为 null，不补 0。
type HealthSummary struct {
	BMI                      *string    `json:"bmi"`
	LatestHeight             *string    `json:"latestHeight"`
	LatestWeight             *string    `json:"latestWeight"`
	LatestBloodSugar         *string    `json:"latestBloodSugar"`
	LatestSystolicPressure   *uint16    `json:"latestSystolicPressure"`
	LatestDiastolicPressure  *uint16    `json:"latestDiastolicPressure"`
	LatestHeartRate          *uint16    `json:"latestHeartRate"`
	LatestSleepDurationHours *string    `json:"latestSleepDurationHours"`
	LastMeasuredAt           *time.Time `json:"lastMeasuredAt"`
	// MeasuredAt 与 Sources 的 key 为指标名，分别表示该指标的测量时间和数据来源（当前只标记 record）。
	MeasuredAt map[string]time.Time `json:"measuredAt"`
	Sources    map[string]string    `json:"sources"`
	BMISource  *string              `json:"bmiSource"`
}

// TrendPoint 趋势聚合点。Time 为桶起始日（YYYY-MM-DD），Value 为两位小数字符串。
type TrendPoint struct {
	Time        string `json:"time"`
	Value       string `json:"value"`
	SampleCount int64  `json:"sampleCount"`
}

// Trend 趋势响应。
type Trend struct {
	Metric      string       `json:"metric"`
	MetricName  string       `json:"metricName"`
	Unit        string       `json:"unit"`
	Granularity string       `json:"granularity"`
	StartDate   string       `json:"startDate"`
	EndDate     string       `json:"endDate"`
	Points      []TrendPoint `json:"points"`
}

// NewProfile 把档案 entity 转成 API DTO；BMI 由服务端按身高体重推导。
func NewProfile(p *entity.HealthProfile) *Profile {
	if p == nil {
		return nil
	}
	out := &Profile{
		ID:                   strconv.FormatUint(p.ID, 10),
		UserID:               strconv.FormatUint(p.UserID, 10),
		RealName:             p.RealName,
		Age:                  p.Age,
		BirthDate:            formatDate(p.BirthDate),
		Gender:               p.Gender,
		HeightCm:             decimalString(p.HeightCm),
		WeightKg:             decimalString(p.WeightKg),
		BloodSugarMmol:       decimalString(p.BloodSugarMmol),
		SystolicPressure:     p.SystolicPressure,
		DiastolicPressure:    p.DiastolicPressure,
		TotalCholesterolMmol: decimalString(p.TotalCholesterolMmol),
		HeartRate:            p.HeartRate,
		VisionLeft:           decimalString(p.VisionLeft),
		VisionRight:          decimalString(p.VisionRight),
		SleepDurationHours:   decimalString(p.SleepDurationHours),
		SleepQuality:         p.SleepQuality,
		SmokingStatus:        p.SmokingStatus,
		DrinkingStatus:       p.DrinkingStatus,
		ExerciseHabit:        p.ExerciseHabit,
		FoodPreferences:      p.FoodPreferences,
		DailyWaterMl:         p.DailyWaterMl,
		MedicalHistory:       p.MedicalHistory,
		AllergyHistory:       p.AllergyHistory,
		BMI:                  decimalString(p.BMI()),
		CreatedAt:            p.CreatedAt,
		UpdatedAt:            p.UpdatedAt,
	}
	return out
}

// NewRecord 把健康记录 entity 转成 API DTO。
func NewRecord(r *entity.HealthRecord) *Record {
	if r == nil {
		return nil
	}
	return &Record{
		ID:                   strconv.FormatUint(r.ID, 10),
		UserID:               strconv.FormatUint(r.UserID, 10),
		HeightCm:             decimalString(r.HeightCm),
		WeightKg:             decimalString(r.WeightKg),
		BloodSugarMmol:       decimalString(r.BloodSugarMmol),
		SystolicPressure:     r.SystolicPressure,
		DiastolicPressure:    r.DiastolicPressure,
		TotalCholesterolMmol: decimalString(r.TotalCholesterolMmol),
		HeartRate:            r.HeartRate,
		VisionLeft:           decimalString(r.VisionLeft),
		VisionRight:          decimalString(r.VisionRight),
		SleepDurationHours:   decimalString(r.SleepDurationHours),
		SleepQuality:         r.SleepQuality,
		SmokingStatus:        r.SmokingStatus,
		DrinkingStatus:       r.DrinkingStatus,
		ExerciseHabit:        r.ExerciseHabit,
		FoodPreferences:      r.FoodPreferences,
		DailyWaterMl:         r.DailyWaterMl,
		MeasuredAt:           r.MeasuredAt,
		Source:               r.Source,
		Remark:               r.Remark,
		CreatedAt:            r.CreatedAt,
		UpdatedAt:            r.UpdatedAt,
	}
}

func decimalString(d *decimal.Decimal) *string {
	if d == nil {
		return nil
	}
	s := d.StringFixed(2)
	return &s
}

func formatDate(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format("2006-01-02")
	return &s
}
