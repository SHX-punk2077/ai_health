package request

// ProfileSaveReq 健康档案整体保存（一人一份，upsert 语义）。
// 数值用字符串接收，避免前端 float 精度问题，服务层再转 decimal。
type ProfileSaveReq struct {
	RealName             *string `json:"realName" binding:"omitempty,max=50"`
	Age                  *uint8  `json:"age" binding:"omitempty,max=150"`
	BirthDate            *string `json:"birthDate" binding:"omitempty,datetime=2006-01-02"`
	Gender               *uint8  `json:"gender" binding:"omitempty,oneof=0 1 2"`
	HeightCm             *string `json:"heightCm" binding:"omitempty,numeric"`
	WeightKg             *string `json:"weightKg" binding:"omitempty,numeric"`
	BloodSugarMmol       *string `json:"bloodSugarMmol" binding:"omitempty,numeric"`
	SystolicPressure     *uint16 `json:"systolicPressure" binding:"omitempty,max=300"`
	DiastolicPressure    *uint16 `json:"diastolicPressure" binding:"omitempty,max=200"`
	TotalCholesterolMmol *string `json:"totalCholesterolMmol" binding:"omitempty,numeric"`
	HeartRate            *uint16 `json:"heartRate" binding:"omitempty,max=300"`
	VisionLeft           *string `json:"visionLeft" binding:"omitempty,numeric"`
	VisionRight          *string `json:"visionRight" binding:"omitempty,numeric"`
	SleepDurationHours   *string `json:"sleepDurationHours" binding:"omitempty,numeric"`
	SleepQuality         *string `json:"sleepQuality" binding:"omitempty,max=30"`
	SmokingStatus        *uint8  `json:"smokingStatus"`
	DrinkingStatus       *uint8  `json:"drinkingStatus"`
	ExerciseHabit        *uint8  `json:"exerciseHabit"`
	FoodPreferences      *string `json:"foodPreferences"`
	DailyWaterMl         *uint32 `json:"dailyWaterMl"`
	MedicalHistory       *string `json:"medicalHistory"`
	AllergyHistory       *string `json:"allergyHistory"`
}

// RecordCreateReq 一条健康记录。
type RecordCreateReq struct {
	ProfileSaveReq
	MeasuredAt string `json:"measuredAt" binding:"required,datetime=2006-01-02 15:04:05"`
	Source     string `json:"source" binding:"omitempty,oneof=manual device ai"`
	Remark     string `json:"remark" binding:"max=500"`
}

// RecordQuery 记录列表筛选。
type RecordQuery struct {
	PageQuery
	StartAt string `form:"startAt" binding:"omitempty,datetime=2006-01-02"`
	EndAt   string `form:"endAt" binding:"omitempty,datetime=2006-01-02"`
}

// TrendQuery 趋势曲线：按指标名取近 N 条。
type TrendQuery struct {
	Metric string `form:"metric" binding:"required,oneof=weightKg bloodSugarMmol systolicPressure"`
	Limit  int    `form:"limit"`
}
