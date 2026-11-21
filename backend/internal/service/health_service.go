package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/shopspring/decimal"

	"ai_health/internal/dto/request"
	"ai_health/internal/dto/response"
	"ai_health/internal/model/entity"
	"ai_health/internal/pkg/errcode"
	"ai_health/internal/pkg/optional"
	"ai_health/internal/repository"
)

const (
	defaultTrendDays = 90
	maxTrendDays     = 366
	maxTrendLimit    = 1000
)

type metricMeta struct {
	Name string
	Unit string
}

// healthMetricMeta 是对外指标名到展示信息的唯一映射。
var healthMetricMeta = map[string]metricMeta{
	"heightCm":             {Name: "身高", Unit: "cm"},
	"weightKg":             {Name: "体重", Unit: "kg"},
	"bloodSugarMmol":       {Name: "血糖", Unit: "mmol/L"},
	"systolicPressure":     {Name: "收缩压", Unit: "mmHg"},
	"diastolicPressure":    {Name: "舒张压", Unit: "mmHg"},
	"totalCholesterolMmol": {Name: "总胆固醇", Unit: "mmol/L"},
	"heartRate":            {Name: "心率", Unit: "bpm"},
	"visionLeft":           {Name: "左眼视力", Unit: ""},
	"visionRight":          {Name: "右眼视力", Unit: ""},
	"sleepDurationHours":   {Name: "睡眠时长", Unit: "h"},
	"dailyWaterMl":         {Name: "饮水量", Unit: "ml"},
}

// 各 DECIMAL 列的取值上限，与 migrations/000001_init.up.sql 保持一致。
var (
	maxDecimal5 = decimal.New(99999, -2)  // DECIMAL(5,2)
	maxDecimal6 = decimal.New(999999, -2) // DECIMAL(6,2)
	maxDecimal4 = decimal.New(9999, -2)   // DECIMAL(4,2)
)

// HealthService 健康档案 / 健康记录 / 趋势统计。
// 所有私有数据只使用调用方传入的 JWT 用户 ID，不接受请求体里的归属字段。
// 档案是当前可编辑快照，历史记录允许补录，二者不做隐式同步，因此本模块没有多表写入。
type HealthService struct {
	repo *repository.HealthRepo
}

func NewHealthService(repo *repository.HealthRepo) *HealthService {
	return &HealthService{repo: repo}
}

// -------- 健康档案 --------

func (s *HealthService) GetProfile(ctx context.Context, uid uint64) (*response.Profile, error) {
	if uid == 0 {
		return nil, errcode.ErrUnauthorized
	}
	p, err := s.repo.GetProfile(ctx, uid)
	if err != nil {
		return nil, internalError(err)
	}
	if p == nil {
		return nil, errcode.ErrHealthProfileNotFound
	}
	return response.NewProfile(p), nil
}

// CreateProfile 严格创建：一人一份，重复或并发重复都返回 40003。
func (s *HealthService) CreateProfile(ctx context.Context, uid uint64, req request.ProfileCreateReq) (*response.Profile, error) {
	if uid == 0 {
		return nil, errcode.ErrUnauthorized
	}
	if err := validateProfileFields(req.ProfileFields); err != nil {
		return nil, err
	}
	existing, err := s.repo.GetProfile(ctx, uid)
	if err != nil {
		return nil, internalError(err)
	}
	if existing != nil {
		return nil, errcode.ErrHealthProfileExists
	}

	vals, err := parseHealthMetrics(req.HealthMetrics)
	if err != nil {
		return nil, err
	}
	birthDate, err := parseOptionalDay(req.BirthDate, "出生日期")
	if err != nil {
		return nil, err
	}

	p := &entity.HealthProfile{
		UserID:               uid,
		RealName:             optionalText(req.RealName),
		Age:                  req.Age,
		BirthDate:            birthDate,
		Gender:               req.Gender,
		HeightCm:             vals.heightCm,
		WeightKg:             vals.weightKg,
		BloodSugarMmol:       vals.bloodSugarMmol,
		SystolicPressure:     vals.systolicPressure,
		DiastolicPressure:    vals.diastolicPressure,
		TotalCholesterolMmol: vals.totalCholesterolMmol,
		HeartRate:            vals.heartRate,
		VisionLeft:           vals.visionLeft,
		VisionRight:          vals.visionRight,
		SleepDurationHours:   vals.sleepDurationHours,
		SleepQuality:         vals.sleepQuality,
		SmokingStatus:        vals.smokingStatus,
		DrinkingStatus:       vals.drinkingStatus,
		ExerciseHabit:        vals.exerciseHabit,
		FoodPreferences:      vals.foodPreferences,
		DailyWaterMl:         vals.dailyWaterMl,
		MedicalHistory:       optionalText(req.MedicalHistory),
		AllergyHistory:       optionalText(req.AllergyHistory),
	}
	if err := s.repo.CreateProfile(ctx, p); err != nil {
		if isDuplicateKey(err) {
			return nil, errcode.ErrHealthProfileExists
		}
		return nil, internalError(err)
	}
	return response.NewProfile(p), nil
}

// UpdateProfile 部分更新：未出现的字段保持不变，显式传空串表示清空该列为 NULL。
// updates 中永远不会出现 user_id，避免请求改变档案归属。
func (s *HealthService) UpdateProfile(ctx context.Context, uid uint64, req request.ProfileUpdateReq) (*response.Profile, error) {
	if uid == 0 {
		return nil, errcode.ErrUnauthorized
	}
	if err := validateProfileFields(req.ProfileFields); err != nil {
		return nil, err
	}
	vals, err := parseHealthMetrics(req.HealthMetrics)
	if err != nil {
		return nil, err
	}
	birthDate, err := parseOptionalDay(req.BirthDate, "出生日期")
	if err != nil {
		return nil, err
	}

	var out *response.Profile
	err = s.repo.Transaction(ctx, func(tx *repository.HealthRepo) error {
		// 锁定当前用户档案，再读取血压另一侧并校验，避免并发单侧更新
		// 各自基于旧值通过校验后写入不合法组合。
		existing, err := tx.LockProfile(ctx, uid)
		if err != nil {
			return internalError(err)
		}
		if existing == nil {
			return errcode.ErrHealthProfileNotFound
		}
		if req.SystolicPressure != nil || req.DiastolicPressure != nil {
			systolic, diastolic := existing.SystolicPressure, existing.DiastolicPressure
			if req.SystolicPressure != nil {
				systolic = vals.systolicPressure
			}
			if req.DiastolicPressure != nil {
				diastolic = vals.diastolicPressure
			}
			if err := validateBloodPressureRelation(systolic, diastolic); err != nil {
				return err
			}
		}

		updates := make(map[string]any, 24)
		setDecimal(updates, "height_cm", req.HeightCm, vals.heightCm)
		setDecimal(updates, "weight_kg", req.WeightKg, vals.weightKg)
		setDecimal(updates, "blood_sugar_mmol", req.BloodSugarMmol, vals.bloodSugarMmol)
		setDecimal(updates, "total_cholesterol_mmol", req.TotalCholesterolMmol, vals.totalCholesterolMmol)
		setDecimal(updates, "vision_left", req.VisionLeft, vals.visionLeft)
		setDecimal(updates, "vision_right", req.VisionRight, vals.visionRight)
		setDecimal(updates, "sleep_duration_hours", req.SleepDurationHours, vals.sleepDurationHours)
		if req.SystolicPressure != nil {
			updates["systolic_pressure"] = *req.SystolicPressure
		}
		if req.DiastolicPressure != nil {
			updates["diastolic_pressure"] = *req.DiastolicPressure
		}
		if req.HeartRate != nil {
			updates["heart_rate"] = *req.HeartRate
		}
		if req.SleepQuality != nil {
			updates["sleep_quality"] = optionalText(req.SleepQuality)
		}
		if req.SmokingStatus != nil {
			updates["smoking_status"] = *req.SmokingStatus
		}
		if req.DrinkingStatus != nil {
			updates["drinking_status"] = *req.DrinkingStatus
		}
		if req.ExerciseHabit != nil {
			updates["exercise_habit"] = *req.ExerciseHabit
		}
		if req.FoodPreferences != nil {
			updates["food_preferences"] = optionalText(req.FoodPreferences)
		}
		if req.DailyWaterMl != nil {
			updates["daily_water_ml"] = *req.DailyWaterMl
		}
		if req.RealName != nil {
			updates["real_name"] = optionalText(req.RealName)
		}
		if req.Age != nil {
			updates["age"] = *req.Age
		}
		if req.BirthDate != nil {
			updates["birth_date"] = birthDate
		}
		if req.Gender != nil {
			updates["gender"] = *req.Gender
		}
		if req.MedicalHistory != nil {
			updates["medical_history"] = optionalText(req.MedicalHistory)
		}
		if req.AllergyHistory != nil {
			updates["allergy_history"] = optionalText(req.AllergyHistory)
		}

		if len(updates) > 0 {
			if _, err := tx.UpdateProfile(ctx, uid, updates); err != nil {
				return internalError(err)
			}
		}

		p, err := tx.GetProfile(ctx, uid)
		if err != nil {
			return internalError(err)
		}
		if p == nil {
			return errcode.ErrHealthProfileNotFound
		}
		out = response.NewProfile(p)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// -------- 健康记录 --------

func (s *HealthService) CreateRecord(ctx context.Context, uid uint64, req request.RecordCreateReq) (*response.Record, error) {
	if uid == 0 {
		return nil, errcode.ErrUnauthorized
	}
	measuredAt, err := parseMeasuredAt(req.MeasuredAt)
	if err != nil {
		return nil, err
	}
	if measuredAt.After(time.Now().UTC()) {
		return nil, errcode.ErrMeasuredAtInvalid
	}
	vals, err := parseHealthMetrics(req.HealthMetrics)
	if err != nil {
		return nil, err
	}
	source, err := normalizeRecordSource(req.Source)
	if err != nil {
		return nil, err
	}
	if err := validateTextLength(req.Remark, maxRemarkRunes, "备注"); err != nil {
		return nil, err
	}

	rec := &entity.HealthRecord{
		UserID:               uid,
		HeightCm:             vals.heightCm,
		WeightKg:             vals.weightKg,
		BloodSugarMmol:       vals.bloodSugarMmol,
		SystolicPressure:     vals.systolicPressure,
		DiastolicPressure:    vals.diastolicPressure,
		TotalCholesterolMmol: vals.totalCholesterolMmol,
		HeartRate:            vals.heartRate,
		VisionLeft:           vals.visionLeft,
		VisionRight:          vals.visionRight,
		SleepDurationHours:   vals.sleepDurationHours,
		SleepQuality:         vals.sleepQuality,
		SmokingStatus:        vals.smokingStatus,
		DrinkingStatus:       vals.drinkingStatus,
		ExerciseHabit:        vals.exerciseHabit,
		FoodPreferences:      vals.foodPreferences,
		DailyWaterMl:         vals.dailyWaterMl,
		MeasuredAt:           measuredAt,
		Source:               source,
		Remark:               optionalText(req.Remark),
	}
	if err := s.repo.CreateRecord(ctx, rec); err != nil {
		return nil, internalError(err)
	}
	out := response.NewRecord(rec)
	out.Warnings = s.recordWarnings(ctx, uid, rec)
	return out, nil
}

// ListRecords 返回当前用户记录；page/pageSize 为 Normalize 后的实际值。
func (s *HealthService) ListRecords(ctx context.Context, uid uint64, q request.RecordQuery) ([]response.Record, int64, int, int, error) {
	if uid == 0 {
		return nil, 0, 0, 0, errcode.ErrUnauthorized
	}
	q.Normalize()
	if q.Metric != "" {
		if _, ok := repository.HealthMetricColumn(q.Metric); !ok {
			return nil, 0, 0, 0, errcode.ErrHealthMetricInvalid.WithDetail("metric=%s", q.Metric)
		}
	}
	start, end, err := parseDayRange(q.StartAt, q.EndAt)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	filter := repository.RecordFilter{StartAt: start, EndAt: end, Metric: q.Metric}
	list, total, err := s.repo.ListRecords(ctx, uid, filter, q.Offset(), q.PageSize)
	if err != nil {
		return nil, 0, 0, 0, internalError(err)
	}
	out := make([]response.Record, 0, len(list))
	for i := range list {
		out = append(out, *response.NewRecord(&list[i]))
	}
	return out, total, q.Page, q.PageSize, nil
}

func (s *HealthService) GetRecord(ctx context.Context, uid, id uint64) (*response.Record, error) {
	if uid == 0 {
		return nil, errcode.ErrUnauthorized
	}
	if id == 0 {
		return nil, errcode.ErrInvalidParam.WithMsg("记录 ID 不合法")
	}
	rec, err := s.repo.GetRecord(ctx, uid, id)
	if err != nil {
		return nil, internalError(err)
	}
	if rec == nil {
		return nil, errcode.ErrHealthRecordNotFound
	}
	return response.NewRecord(rec), nil
}

func (s *HealthService) DeleteRecord(ctx context.Context, uid, id uint64) error {
	if uid == 0 {
		return errcode.ErrUnauthorized
	}
	if id == 0 {
		return errcode.ErrInvalidParam.WithMsg("记录 ID 不合法")
	}
	affected, err := s.repo.DeleteRecord(ctx, uid, id)
	if err != nil {
		return internalError(err)
	}
	if affected == 0 {
		return errcode.ErrHealthRecordNotFound
	}
	return nil
}

// UpdateRecord 部分更新当前用户的一条健康记录。
// 三态语义：未出现/显式 null 保持原值；具体值写入；DECIMAL/文本空串清空为 NULL。
// 所有查询和更新都带 user_id；受保护字段出现时明确拒绝。
func (s *HealthService) UpdateRecord(ctx context.Context, uid, id uint64, req request.RecordUpdateReq) (*response.Record, error) {
	if uid == 0 {
		return nil, errcode.ErrUnauthorized
	}
	if id == 0 {
		return nil, errcode.ErrInvalidParam.WithMsg("记录 ID 不合法")
	}
	if field := req.ProtectedField(); field != "" {
		return nil, errcode.ErrInvalidParam.WithMsg(fmt.Sprintf("不允许修改受保护字段 %s", field))
	}
	if !req.HasAnyField() {
		return nil, errcode.ErrInvalidParam.WithMsg("至少提供一个待更新字段")
	}

	var out *response.Record
	err := s.repo.Transaction(ctx, func(tx *repository.HealthRepo) error {
		// 先锁定记录并确认归属，再做字段解析与血压合并校验；锁条件含 user_id，
		// 他人记录统一返回 40004，不会读到或锁到他人数据。
		existing, err := tx.LockRecord(ctx, uid, id)
		if err != nil {
			return internalError(err)
		}
		if existing == nil {
			return errcode.ErrHealthRecordNotFound
		}

		var measuredAt *time.Time
		if req.MeasuredAt.Set && !req.MeasuredAt.Null {
			parsed, err := parseMeasuredAt(req.MeasuredAt.Value)
			if err != nil {
				return err
			}
			if parsed.After(time.Now().UTC()) {
				return errcode.ErrMeasuredAtInvalid
			}
			measuredAt = &parsed
		}

		metrics := request.HealthMetrics{
			HeightCm:             optional.Ptr(req.HeightCm),
			WeightKg:             optional.Ptr(req.WeightKg),
			BloodSugarMmol:       optional.Ptr(req.BloodSugarMmol),
			SystolicPressure:     optional.Ptr(req.SystolicPressure),
			DiastolicPressure:    optional.Ptr(req.DiastolicPressure),
			TotalCholesterolMmol: optional.Ptr(req.TotalCholesterolMmol),
			HeartRate:            optional.Ptr(req.HeartRate),
			VisionLeft:           optional.Ptr(req.VisionLeft),
			VisionRight:          optional.Ptr(req.VisionRight),
			SleepDurationHours:   optional.Ptr(req.SleepDurationHours),
			SleepQuality:         optional.Ptr(req.SleepQuality),
			SmokingStatus:        optional.Ptr(req.SmokingStatus),
			DrinkingStatus:       optional.Ptr(req.DrinkingStatus),
			ExerciseHabit:        optional.Ptr(req.ExerciseHabit),
			FoodPreferences:      optional.Ptr(req.FoodPreferences),
			DailyWaterMl:         optional.Ptr(req.DailyWaterMl),
		}
		vals, err := parseHealthMetrics(metrics)
		if err != nil {
			return err
		}
		if req.Remark.Set && !req.Remark.Null {
			if err := validateTextLength(&req.Remark.Value, maxRemarkRunes, "备注"); err != nil {
				return err
			}
		}
		if (req.SystolicPressure.Set && !req.SystolicPressure.Null) ||
			(req.DiastolicPressure.Set && !req.DiastolicPressure.Null) {
			systolic, diastolic := existing.SystolicPressure, existing.DiastolicPressure
			if req.SystolicPressure.Set && !req.SystolicPressure.Null {
				systolic = vals.systolicPressure
			}
			if req.DiastolicPressure.Set && !req.DiastolicPressure.Null {
				diastolic = vals.diastolicPressure
			}
			if err := validateBloodPressureRelation(systolic, diastolic); err != nil {
				return err
			}
		}

		updates := make(map[string]any, 18)
		if req.HeightCm.Set && !req.HeightCm.Null {
			updates["height_cm"] = vals.heightCm
		}
		if req.WeightKg.Set && !req.WeightKg.Null {
			updates["weight_kg"] = vals.weightKg
		}
		if req.BloodSugarMmol.Set && !req.BloodSugarMmol.Null {
			updates["blood_sugar_mmol"] = vals.bloodSugarMmol
		}
		if req.SystolicPressure.Set && !req.SystolicPressure.Null {
			updates["systolic_pressure"] = vals.systolicPressure
		}
		if req.DiastolicPressure.Set && !req.DiastolicPressure.Null {
			updates["diastolic_pressure"] = vals.diastolicPressure
		}
		if req.TotalCholesterolMmol.Set && !req.TotalCholesterolMmol.Null {
			updates["total_cholesterol_mmol"] = vals.totalCholesterolMmol
		}
		if req.HeartRate.Set && !req.HeartRate.Null {
			updates["heart_rate"] = vals.heartRate
		}
		if req.VisionLeft.Set && !req.VisionLeft.Null {
			updates["vision_left"] = vals.visionLeft
		}
		if req.VisionRight.Set && !req.VisionRight.Null {
			updates["vision_right"] = vals.visionRight
		}
		if req.SleepDurationHours.Set && !req.SleepDurationHours.Null {
			updates["sleep_duration_hours"] = vals.sleepDurationHours
		}
		if req.SleepQuality.Set && !req.SleepQuality.Null {
			updates["sleep_quality"] = vals.sleepQuality
		}
		if req.SmokingStatus.Set && !req.SmokingStatus.Null {
			updates["smoking_status"] = vals.smokingStatus
		}
		if req.DrinkingStatus.Set && !req.DrinkingStatus.Null {
			updates["drinking_status"] = vals.drinkingStatus
		}
		if req.ExerciseHabit.Set && !req.ExerciseHabit.Null {
			updates["exercise_habit"] = vals.exerciseHabit
		}
		if req.FoodPreferences.Set && !req.FoodPreferences.Null {
			updates["food_preferences"] = vals.foodPreferences
		}
		if req.DailyWaterMl.Set && !req.DailyWaterMl.Null {
			updates["daily_water_ml"] = vals.dailyWaterMl
		}
		if measuredAt != nil {
			updates["measured_at"] = *measuredAt
		}
		if req.Remark.Set && !req.Remark.Null {
			updates["remark"] = optionalText(&req.Remark.Value)
		}

		if len(updates) > 0 {
			// RowsAffected=0 也可能是提交的值与库中一致；不作为记录不存在的依据，
			// 统一重查记录并按归属返回。
			if _, err := tx.UpdateRecord(ctx, uid, id, updates); err != nil {
				return internalError(err)
			}
		}

		rec, err := tx.GetRecord(ctx, uid, id)
		if err != nil {
			return internalError(err)
		}
		if rec == nil {
			return errcode.ErrHealthRecordNotFound
		}
		out = response.NewRecord(rec)
		out.Warnings = s.recordWarningsWithRepo(ctx, tx, uid, rec)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// recordWarnings 读取当前记录之前的最近一条有效体重，用于短期体重变化提醒。
// 提醒查询失败不应把已经成功的写入变成失败响应，因此这里降级为不输出体重提醒。
func (s *HealthService) recordWarnings(ctx context.Context, uid uint64, rec *entity.HealthRecord) []response.HealthWarning {
	return s.recordWarningsWithRepo(ctx, s.repo, uid, rec)
}

// recordWarningsWithRepo 允许更新事务复用同一事务连接查询历史体重，
// 保证提醒读取的数据与刚写入的数据处于同一隔离视图。
func (s *HealthService) recordWarningsWithRepo(ctx context.Context, repo *repository.HealthRepo, uid uint64, rec *entity.HealthRecord) []response.HealthWarning {
	if rec == nil || rec.WeightKg == nil {
		return evaluateRecordWarnings(rec, nil, nil)
	}
	previousWeight, previousMeasuredAt, err := repo.PreviousWeight(ctx, uid, rec.MeasuredAt, rec.ID)
	if err != nil {
		return evaluateRecordWarnings(rec, nil, nil)
	}
	return evaluateRecordWarnings(rec, previousWeight, previousMeasuredAt)
}

// -------- 健康汇总 --------

// summaryMetricNames 是首页概览需要的记录指标；每项独立取最新非 NULL 值。
var summaryMetricNames = []string{
	"heightCm", "weightKg", "bloodSugarMmol", "systolicPressure",
	"diastolicPressure", "heartRate", "sleepDurationHours",
}

// summaryMetricRanges 是汇总接口采用的工程数据范围，与写入校验保持同一口径。
// 数据库中可能存在历史遗留的 0、负数等脏值，查询时必须过滤，不能当作最新有效值。
var summaryMetricRanges = map[string]repository.MetricRange{
	"heightCm":           {Min: decimal.New(3000, -2), Max: decimal.New(25000, -2)},
	"weightKg":           {Min: decimal.New(200, -2), Max: decimal.New(50000, -2)},
	"bloodSugarMmol":     {Min: decimal.New(50, -2), Max: decimal.New(5000, -2)},
	"systolicPressure":   {Min: decimal.NewFromInt(40), Max: decimal.NewFromInt(300)},
	"diastolicPressure":  {Min: decimal.NewFromInt(20), Max: decimal.NewFromInt(200)},
	"heartRate":          {Min: decimal.NewFromInt(20), Max: decimal.NewFromInt(300)},
	"sleepDurationHours": {Min: decimal.New(0, -2), Max: decimal.New(2400, -2)},
}

// Summary 返回当前用户的健康概览。
// 最新测量值只来自 health_record；BMI 在记录缺少身高或体重时才用档案补缺，
// 并通过 bmiSource 明确标记来源，不把档案快照冒充最新测量值。
func (s *HealthService) Summary(ctx context.Context, uid uint64) (*response.HealthSummary, error) {
	if uid == 0 {
		return nil, errcode.ErrUnauthorized
	}
	rows, err := s.repo.LatestMetrics(ctx, uid, summaryMetricNames, summaryMetricRanges)
	if err != nil {
		return nil, internalError(err)
	}
	latest := make(map[string]repository.LatestMetricRow, len(rows))
	for i := range rows {
		latest[rows[i].Metric] = rows[i]
	}

	out := &response.HealthSummary{
		MeasuredAt: make(map[string]time.Time),
		Sources:    make(map[string]string),
	}
	markRecord := func(metric string, row repository.LatestMetricRow) {
		out.MeasuredAt[metric] = row.MeasuredAt
		out.Sources[metric] = "record"
	}

	var height, weight *decimal.Decimal
	if row, ok := latest["heightCm"]; ok {
		d, text, err := parseLatestDecimal(row)
		if err != nil {
			return nil, internalError(err)
		}
		height = d
		out.LatestHeight = text
		markRecord("heightCm", row)
	}
	if row, ok := latest["weightKg"]; ok {
		d, text, err := parseLatestDecimal(row)
		if err != nil {
			return nil, internalError(err)
		}
		weight = d
		out.LatestWeight = text
		markRecord("weightKg", row)
	}
	if row, ok := latest["bloodSugarMmol"]; ok {
		_, text, err := parseLatestDecimal(row)
		if err != nil {
			return nil, internalError(err)
		}
		out.LatestBloodSugar = text
		markRecord("bloodSugarMmol", row)
	}
	if row, ok := latest["systolicPressure"]; ok {
		v, err := parseLatestUint16(row)
		if err != nil {
			return nil, internalError(err)
		}
		out.LatestSystolicPressure = v
		markRecord("systolicPressure", row)
	}
	if row, ok := latest["diastolicPressure"]; ok {
		v, err := parseLatestUint16(row)
		if err != nil {
			return nil, internalError(err)
		}
		out.LatestDiastolicPressure = v
		markRecord("diastolicPressure", row)
	}
	if row, ok := latest["heartRate"]; ok {
		v, err := parseLatestUint16(row)
		if err != nil {
			return nil, internalError(err)
		}
		out.LatestHeartRate = v
		markRecord("heartRate", row)
	}
	if row, ok := latest["sleepDurationHours"]; ok {
		_, text, err := parseLatestDecimal(row)
		if err != nil {
			return nil, internalError(err)
		}
		out.LatestSleepDurationHours = text
		markRecord("sleepDurationHours", row)
	}

	profile, err := s.repo.GetProfile(ctx, uid)
	if err != nil {
		return nil, internalError(err)
	}

	// BMI 优先使用记录中的最新身高/体重；只有缺失的一侧才用档案补缺。
	// 档案值同样必须落在工程范围内，避免历史脏值参与 BMI。
	heightSource, weightSource := "record", "record"
	if height == nil && metricWithinRange(profileHeight(profile), summaryMetricRanges["heightCm"]) {
		height = profileHeight(profile)
		heightSource = "profile"
	}
	if weight == nil && metricWithinRange(profileWeight(profile), summaryMetricRanges["weightKg"]) {
		weight = profileWeight(profile)
		weightSource = "profile"
	}
	if metricWithinRange(height, summaryMetricRanges["heightCm"]) &&
		metricWithinRange(weight, summaryMetricRanges["weightKg"]) &&
		height.IsPositive() && weight.IsPositive() {
		heightMeter := height.Div(decimal.NewFromInt(100))
		bmi := weight.Div(heightMeter.Pow(decimal.NewFromInt(2))).Round(2)
		text := bmi.StringFixed(2)
		out.BMI = &text
		source := "record"
		if heightSource != weightSource {
			source = "record+profile"
		} else if heightSource == "profile" {
			source = "profile"
		}
		out.BMISource = &source
	}

	lastMeasuredAt, err := s.repo.LatestMeasuredAt(ctx, uid)
	if err != nil {
		return nil, internalError(err)
	}
	out.LastMeasuredAt = lastMeasuredAt
	return out, nil
}

func profileHeight(p *entity.HealthProfile) *decimal.Decimal {
	if p == nil {
		return nil
	}
	return p.HeightCm
}

func profileWeight(p *entity.HealthProfile) *decimal.Decimal {
	if p == nil {
		return nil
	}
	return p.WeightKg
}

func metricWithinRange(v *decimal.Decimal, r repository.MetricRange) bool {
	return v != nil && !v.LessThan(r.Min) && !v.GreaterThan(r.Max)
}

func parseLatestDecimal(row repository.LatestMetricRow) (*decimal.Decimal, *string, error) {
	d, err := decimal.NewFromString(strings.TrimSpace(row.Value))
	if err != nil {
		return nil, nil, fmt.Errorf("健康指标 %s 的数据库值无法解析: %w", row.Metric, err)
	}
	text := d.StringFixed(2)
	return &d, &text, nil
}

func parseLatestUint16(row repository.LatestMetricRow) (*uint16, error) {
	v, err := strconv.ParseUint(strings.TrimSpace(row.Value), 10, 16)
	if err != nil {
		return nil, fmt.Errorf("健康指标 %s 的数据库值无法解析: %w", row.Metric, err)
	}
	out := uint16(v)
	return &out, nil
}

// -------- 趋势 --------

func (s *HealthService) Trend(ctx context.Context, uid uint64, q request.TrendQuery) (*response.Trend, error) {
	if uid == 0 {
		return nil, errcode.ErrUnauthorized
	}
	metric, ok := healthMetricMeta[q.Metric]
	if q.Metric == "" || !ok {
		return nil, errcode.ErrHealthMetricInvalid.WithDetail("metric=%q", q.Metric)
	}
	granularity := strings.TrimSpace(q.Granularity)
	if granularity == "" {
		granularity = "day"
	}
	if granularity != "day" && granularity != "week" && granularity != "month" {
		return nil, errcode.ErrHealthGranularity
	}
	if q.Limit < 0 {
		return nil, errcode.ErrInvalidParam.WithMsg("limit 不能为负数")
	}
	if q.Limit > maxTrendLimit {
		return nil, errcode.ErrInvalidParam.WithMsg(fmt.Sprintf("limit 最大 %d", maxTrendLimit))
	}

	end, err := parseDayText(q.EndDate, "结束日期")
	if err != nil {
		return nil, err
	}
	start, err := parseDayText(q.StartDate, "开始日期")
	if err != nil {
		return nil, err
	}
	if end == nil {
		now := time.Now().UTC()
		e := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		end = &e
	}
	if start == nil {
		st := end.AddDate(0, 0, -(defaultTrendDays - 1))
		start = &st
	}
	if start.After(*end) {
		return nil, errcode.ErrHealthRangeInvalid
	}
	// 含首尾最多 maxTrendDays 个自然日，因此日期差最多 maxTrendDays-1 天。
	if end.Sub(*start) >= maxTrendDays*24*time.Hour {
		return nil, errcode.ErrHealthRangeInvalid
	}

	filterEnd := end.AddDate(0, 0, 1)
	rows, err := s.repo.Trend(ctx, uid, q.Metric, repository.RecordFilter{StartAt: start, EndAt: &filterEnd}, granularity)
	if err != nil {
		return nil, internalError(err)
	}
	if q.Limit > 0 && len(rows) > q.Limit {
		rows = rows[len(rows)-q.Limit:]
	}

	points := make([]response.TrendPoint, 0, len(rows))
	for _, row := range rows {
		points = append(points, response.TrendPoint{
			Time:        row.Bucket,
			Value:       row.Value.StringFixed(2),
			SampleCount: row.SampleCount,
		})
	}
	return &response.Trend{
		Metric:      q.Metric,
		MetricName:  metric.Name,
		Unit:        metric.Unit,
		Granularity: granularity,
		StartDate:   start.Format("2006-01-02"),
		EndDate:     end.Format("2006-01-02"),
		Points:      points,
	}, nil
}

// -------- 助手 --------

type healthMetricValues struct {
	heightCm             *decimal.Decimal
	weightKg             *decimal.Decimal
	bloodSugarMmol       *decimal.Decimal
	systolicPressure     *uint16
	diastolicPressure    *uint16
	totalCholesterolMmol *decimal.Decimal
	heartRate            *uint16
	visionLeft           *decimal.Decimal
	visionRight          *decimal.Decimal
	sleepDurationHours   *decimal.Decimal
	sleepQuality         *string
	smokingStatus        *uint8
	drinkingStatus       *uint8
	exerciseHabit        *uint8
	foodPreferences      *string
	dailyWaterMl         *uint32
}

func parseHealthMetrics(m request.HealthMetrics) (healthMetricValues, error) {
	var v healthMetricValues
	var err error
	if v.heightCm, err = parseDecimalValue(m.HeightCm, maxDecimal5, "身高"); err != nil {
		return v, err
	}
	if v.weightKg, err = parseDecimalValue(m.WeightKg, maxDecimal5, "体重"); err != nil {
		return v, err
	}
	if v.bloodSugarMmol, err = parseDecimalValue(m.BloodSugarMmol, maxDecimal6, "血糖"); err != nil {
		return v, err
	}
	if v.totalCholesterolMmol, err = parseDecimalValue(m.TotalCholesterolMmol, maxDecimal6, "总胆固醇"); err != nil {
		return v, err
	}
	if v.visionLeft, err = parseDecimalValue(m.VisionLeft, maxDecimal4, "左眼视力"); err != nil {
		return v, err
	}
	if v.visionRight, err = parseDecimalValue(m.VisionRight, maxDecimal4, "右眼视力"); err != nil {
		return v, err
	}
	if v.sleepDurationHours, err = parseDecimalValue(m.SleepDurationHours, maxDecimal4, "睡眠时长"); err != nil {
		return v, err
	}
	if err := checkUint16(m.SystolicPressure, 300, "收缩压"); err != nil {
		return v, err
	}
	if err := checkUint16(m.DiastolicPressure, 200, "舒张压"); err != nil {
		return v, err
	}
	if err := checkUint16(m.HeartRate, 300, "心率"); err != nil {
		return v, err
	}
	if m.SmokingStatus != nil && *m.SmokingStatus > 2 {
		return v, errcode.ErrHealthValueInvalid.WithMsg("吸烟状态仅支持 0、1、2")
	}
	if m.DrinkingStatus != nil && *m.DrinkingStatus > 2 {
		return v, errcode.ErrHealthValueInvalid.WithMsg("饮酒状态仅支持 0、1、2")
	}
	if m.ExerciseHabit != nil && *m.ExerciseHabit > 2 {
		return v, errcode.ErrHealthValueInvalid.WithMsg("运动习惯仅支持 0、1、2")
	}
	v.systolicPressure = m.SystolicPressure
	v.diastolicPressure = m.DiastolicPressure
	v.heartRate = m.HeartRate
	v.sleepQuality = optionalText(m.SleepQuality)
	v.smokingStatus = m.SmokingStatus
	v.drinkingStatus = m.DrinkingStatus
	v.exerciseHabit = m.ExerciseHabit
	v.foodPreferences = optionalText(m.FoodPreferences)
	v.dailyWaterMl = m.DailyWaterMl
	if err := validateHealthRanges(v); err != nil {
		return v, err
	}
	if err := validateBloodPressureRelation(v.systolicPressure, v.diastolicPressure); err != nil {
		return v, err
	}
	return v, nil
}

func checkUint16(v *uint16, max uint16, field string) error {
	if v != nil && *v > max {
		return errcode.ErrHealthValueInvalid.WithMsg(fmt.Sprintf("%s不能超过 %d", field, max))
	}
	return nil
}

// validateProfileFields 是 HTTP binding 之外的服务层防线：service 也可能被内部任务直接调用。
func validateProfileFields(f request.ProfileFields) error {
	if f.Age != nil && *f.Age > 150 {
		return errcode.ErrHealthValueInvalid.WithMsg("年龄不能超过 150")
	}
	if f.Gender != nil && *f.Gender > 2 {
		return errcode.ErrHealthValueInvalid.WithMsg("性别仅支持 0、1、2")
	}
	return nil
}

// normalizeRecordSource 对应 health_record.source 的枚举；空值按 manual 处理。
func normalizeRecordSource(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return entity.SourceManual, nil
	}
	switch s {
	case entity.SourceManual, entity.SourceDevice, entity.SourceImport, entity.SourceAI:
		return s, nil
	default:
		return "", errcode.ErrHealthValueInvalid.WithMsg("来源仅支持 manual、device、import、ai")
	}
}

// parseDecimalValue 只接受非负、最多两位小数的十进制文本，并按列精度上限校验。
// nil 与空串都表示 NULL；更新场景下“空串清空”由调用方根据原始指针决定。
func parseDecimalValue(raw *string, max decimal.Decimal, field string) (*decimal.Decimal, error) {
	if raw == nil {
		return nil, nil
	}
	s := strings.TrimSpace(*raw)
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, ".")
	if len(parts) > 2 || !allDigits(parts[0]) || (len(parts) == 2 && (!allDigits(parts[1]) || len(parts[1]) > 2)) {
		return nil, errcode.ErrHealthValueInvalid.WithMsg(field + "必须是非负数字，最多两位小数")
	}
	d, err := decimal.NewFromString(s)
	if err != nil {
		return nil, errcode.ErrHealthValueInvalid.WithMsg(field + "必须是非负数字，最多两位小数")
	}
	if d.IsNegative() || d.GreaterThan(max) {
		return nil, errcode.ErrHealthValueInvalid.WithMsg(fmt.Sprintf("%s超出允许范围（最大 %s）", field, max.StringFixed(2)))
	}
	return &d, nil
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func setDecimal(updates map[string]any, column string, raw *string, value *decimal.Decimal) {
	if raw != nil {
		updates[column] = value
	}
}

// parseOptionalDay 解析请求体中的 YYYY-MM-DD；nil 或空串返回 nil。
func parseOptionalDay(raw *string, field string) (*time.Time, error) {
	if raw == nil {
		return nil, nil
	}
	return parseDayText(*raw, field)
}

// parseDayText 解析查询参数中的 YYYY-MM-DD；空串返回 nil。
func parseDayText(raw, field string) (*time.Time, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, nil
	}
	t, err := time.ParseInLocation("2006-01-02", s, time.UTC)
	if err != nil {
		return nil, errcode.ErrHealthDateInvalid.WithDetail("%s=%q", field, s)
	}
	return &t, nil
}

// parseDayRange 把 startAt/endAt 转成 [start, end) 的 UTC 区间，endAt 当天完整包含。
func parseDayRange(startRaw, endRaw string) (*time.Time, *time.Time, error) {
	var start, end *time.Time
	if s := strings.TrimSpace(startRaw); s != "" {
		t, err := time.ParseInLocation("2006-01-02", s, time.UTC)
		if err != nil {
			return nil, nil, errcode.ErrHealthDateInvalid.WithDetail("startAt=%q", s)
		}
		start = &t
	}
	if s := strings.TrimSpace(endRaw); s != "" {
		t, err := time.ParseInLocation("2006-01-02", s, time.UTC)
		if err != nil {
			return nil, nil, errcode.ErrHealthDateInvalid.WithDetail("endAt=%q", s)
		}
		if start != nil && t.Before(*start) {
			return nil, nil, errcode.ErrHealthRangeInvalid
		}
		next := t.AddDate(0, 0, 1)
		end = &next
	}
	return start, end, nil
}

func parseMeasuredAt(raw string) (time.Time, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return time.Time{}, errcode.ErrHealthMeasuredInvalid
	}
	layouts := []string{time.RFC3339Nano, "2006-01-02 15:04:05", "2006-01-02T15:04:05"}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, errcode.ErrHealthMeasuredInvalid.WithDetail("measuredAt=%q", s)
}

func optionalText(raw *string) *string {
	if raw == nil {
		return nil
	}
	s := strings.TrimSpace(*raw)
	if s == "" {
		return nil
	}
	return &s
}

func isDuplicateKey(err error) bool {
	var myErr *mysql.MySQLError
	return errors.As(err, &myErr) && myErr.Number == 1062
}

func internalError(err error) error {
	return errcode.ErrInternal.WithDetail("%v", err)
}
