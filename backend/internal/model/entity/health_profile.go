package entity

import (
	"time"

	"github.com/shopspring/decimal"
)

// HealthProfile 健康档案，一人一份（uk_health_profile_user_id）。
// 所有指标可空，前端分步填写。
type HealthProfile struct {
	ID                   uint64           `gorm:"column:id;type:bigint unsigned;primaryKey;autoIncrement" json:"id,string"`
	UserID               uint64           `gorm:"column:user_id;type:bigint unsigned;not null;uniqueIndex:uk_health_profile_user_id" json:"userId,string"`
	RealName             *string          `gorm:"column:real_name;type:varchar(50)" json:"realName"`
	Age                  *uint8           `gorm:"column:age;type:tinyint unsigned" json:"age"`
	BirthDate            *time.Time       `gorm:"column:birth_date;type:date" json:"birthDate"`
	Gender               *uint8           `gorm:"column:gender;type:tinyint unsigned" json:"gender"`
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
	MedicalHistory       *string          `gorm:"column:medical_history;type:text" json:"medicalHistory"`
	AllergyHistory       *string          `gorm:"column:allergy_history;type:text" json:"allergyHistory"`
	CreatedAt            time.Time        `gorm:"column:created_at;type:datetime(3);not null;autoCreateTime:milli" json:"createdAt"`
	UpdatedAt            time.Time        `gorm:"column:updated_at;type:datetime(3);not null;autoUpdateTime:milli" json:"updatedAt"`

	User *SysUser `gorm:"foreignKey:UserID;references:ID;constraint:fk_health_profile_user,OnDelete:CASCADE,OnUpdate:CASCADE" json:"-"`
	// Records 该用户的全部健康记录。两者通过 user_id 关联（health_record.user_id
	// 本身已有指向 sys_user 的外键），这里不再生成额外外键。
	Records []HealthRecord `gorm:"foreignKey:UserID;references:UserID;constraint:-" json:"-"`
}

func (HealthProfile) TableName() string { return "health_profile" }

// BMI 由身高体重推导，身高缺失或为 0 返回 nil。
func (p *HealthProfile) BMI() *decimal.Decimal {
	if p.HeightCm == nil || p.WeightKg == nil {
		return nil
	}
	h := p.HeightCm.Div(decimal.NewFromInt(100))
	if h.IsZero() {
		return nil
	}
	bmi := p.WeightKg.Div(h.Mul(h)).Round(2)
	return &bmi
}
