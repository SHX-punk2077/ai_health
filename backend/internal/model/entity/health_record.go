package entity

import (
	"time"

	"github.com/shopspring/decimal"
)

// 健康记录来源，对应 source 列；只允许写入枚举内的值。
const (
	SourceManual = "manual"
	SourceDevice = "device"
	SourceImport = "import"
	SourceAI     = "ai"
)

// HealthRecord 健康记录，按 measured_at 存历史快照，不可软删（数据留痕）。
type HealthRecord struct {
	ID                   uint64           `gorm:"column:id;type:bigint unsigned;primaryKey;autoIncrement" json:"id,string"`
	UserID               uint64           `gorm:"column:user_id;type:bigint unsigned;not null;index:idx_health_record_user_measured,priority:1" json:"userId,string"`
	HeightCm             *decimal.Decimal `gorm:"column:height_cm;type:decimal(5,2)" json:"heightCm"`
	WeightKg             *decimal.Decimal `gorm:"column:weight_kg;type:decimal(5,2)" json:"weightKg"`
	BloodSugarMmol       *decimal.Decimal `gorm:"column:blood_sugar_mmol;type:decimal(6,2)" json:"bloodSugarMmol"`
	SystolicPressure     *uint16          `gorm:"column:systolic_pressure;type:smallint unsigned" json:"systolicPressure"`
	DiastolicPressure    *uint16          `gorm:"column:diastolic_pressure;type:smallint unsigned" json:"diastolicPressure"`
	TotalCholesterolMmol *decimal.Decimal `gorm:"column:total_cholesterol_mmol;type:decimal(6,2)" json:"totalCholesterolMmol"`
	HeartRate            *uint16          `gorm:"column:heart_rate;type:smallint unsigned" json:"heartRate"`
	VisionLeft           *decimal.Decimal `gorm:"column:vision_left;type:decimal(4,2)" json:"visionLeft"`
	VisionRight          *decimal.Decimal `gorm:"column:vision_right;type:decimal(4,2)" json:"visionRight"`
	SleepDurationHours   *decimal.Decimal `gorm:"column:sleep_duration_hours;type:decimal(4,2)" json:"sleepDurationHours"`
	SleepQuality         *string          `gorm:"column:sleep_quality;type:varchar(30)" json:"sleepQuality"`
	SmokingStatus        *uint8           `gorm:"column:smoking_status;type:tinyint unsigned" json:"smokingStatus"`
	DrinkingStatus       *uint8           `gorm:"column:drinking_status;type:tinyint unsigned" json:"drinkingStatus"`
	ExerciseHabit        *uint8           `gorm:"column:exercise_habit;type:tinyint unsigned" json:"exerciseHabit"`
	FoodPreferences      *string          `gorm:"column:food_preferences;type:text" json:"foodPreferences"`
	DailyWaterMl         *uint32          `gorm:"column:daily_water_ml;type:int unsigned" json:"dailyWaterMl"`
	MeasuredAt           time.Time        `gorm:"column:measured_at;type:datetime(3);not null;index:idx_health_record_user_measured,priority:2,sort:desc;index:idx_health_record_measured" json:"measuredAt"`
	Source               string           `gorm:"column:source;type:varchar(30);not null;default:manual" json:"source"`
	Remark               *string          `gorm:"column:remark;type:varchar(500)" json:"remark"`
	CreatedAt            time.Time        `gorm:"column:created_at;type:datetime(3);not null;autoCreateTime:milli" json:"createdAt"`
	UpdatedAt            time.Time        `gorm:"column:updated_at;type:datetime(3);not null;autoUpdateTime:milli" json:"updatedAt"`

	User *SysUser `gorm:"foreignKey:UserID;references:ID;constraint:fk_health_record_user,OnDelete:CASCADE,OnUpdate:CASCADE" json:"-"`
}

func (HealthRecord) TableName() string { return "health_record" }
