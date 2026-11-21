package service

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/shopspring/decimal"

	"ai_health/internal/dto/response"
	"ai_health/internal/model/entity"
	"ai_health/internal/pkg/errcode"
)

// 本文件的数值范围是工程数据校验边界，用于拒绝明显不可能或超出字段业务定义的数据，
// 不是医学正常范围，也不能用于诊断。阈值集中放在这里，方便后续单独评审和调整。
var (
	minHeightCm             = decimal.RequireFromString("30.00")
	maxHeightCm             = decimal.RequireFromString("250.00")
	minWeightKg             = decimal.RequireFromString("2.00")
	maxWeightKg             = decimal.RequireFromString("500.00")
	minBloodSugarMmol       = decimal.RequireFromString("0.50")
	maxBloodSugarMmol       = decimal.RequireFromString("50.00")
	minTotalCholesterolMmol = decimal.RequireFromString("1.00")
	maxTotalCholesterolMmol = decimal.RequireFromString("30.00")
	minVision               = decimal.RequireFromString("0.00")
	maxVision               = decimal.RequireFromString("20.00")
	minSleepDurationHours   = decimal.RequireFromString("0.00")
	maxSleepDurationHours   = decimal.RequireFromString("24.00")
	minSystolicPressure     = uint16(40)
	maxSystolicPressure     = uint16(300)
	minDiastolicPressure    = uint16(20)
	maxDiastolicPressure    = uint16(200)
	minHeartRate            = uint16(20)
	maxHeartRate            = uint16(300)
	maxDailyWaterMl         = uint32(20000)
	maxSleepQualityRunes    = 30
	maxRemarkRunes          = 500
)

// 异常提醒阈值。提醒只是工程侧的风险提示，不构成医学诊断。
const (
	bpHighSystolic   = uint16(140)
	bpHighDiastolic  = uint16(90)
	bpLowSystolic    = uint16(90)
	bpLowDiastolic   = uint16(60)
	heartRateHigh    = uint16(100)
	heartRateLow     = uint16(60)
	weightWindowHour = 7 * 24 * time.Hour
)

var (
	bloodSugarHigh        = decimal.RequireFromString("7.00")
	bloodSugarLow         = decimal.RequireFromString("3.90")
	sleepDurationHigh     = decimal.RequireFromString("10.00")
	sleepDurationLow      = decimal.RequireFromString("4.00")
	weightChangeAbs       = decimal.RequireFromString("2.00")
	weightChangePercent   = decimal.RequireFromString("5.00")
	warningLevelAttention = "warning"
)

type decimalBound struct {
	field string
	value *decimal.Decimal
	min   decimal.Decimal
	max   decimal.Decimal
}

// validateHealthRanges 校验本次请求中实际提供的非 NULL 指标。
// 空串在 parseDecimalValue 中已经转成 nil，因此“清空字段”不会触发范围校验。
func validateHealthRanges(v healthMetricValues) error {
	bounds := []decimalBound{
		{"身高", v.heightCm, minHeightCm, maxHeightCm},
		{"体重", v.weightKg, minWeightKg, maxWeightKg},
		{"血糖", v.bloodSugarMmol, minBloodSugarMmol, maxBloodSugarMmol},
		{"总胆固醇", v.totalCholesterolMmol, minTotalCholesterolMmol, maxTotalCholesterolMmol},
		{"左眼视力", v.visionLeft, minVision, maxVision},
		{"右眼视力", v.visionRight, minVision, maxVision},
		{"睡眠时长", v.sleepDurationHours, minSleepDurationHours, maxSleepDurationHours},
	}
	for _, b := range bounds {
		if b.value == nil {
			continue
		}
		if b.value.LessThan(b.min) || b.value.GreaterThan(b.max) {
			return errcode.ErrHealthValueInvalid.WithMsg(fmt.Sprintf(
				"%s必须在 %s 到 %s 之间（工程数据范围，不是医学正常范围）",
				b.field, b.min.StringFixed(2), b.max.StringFixed(2),
			))
		}
	}

	if v.systolicPressure != nil && (*v.systolicPressure < minSystolicPressure || *v.systolicPressure > maxSystolicPressure) {
		return errcode.ErrHealthValueInvalid.WithMsg(fmt.Sprintf("收缩压必须在 %d 到 %d mmHg 之间", minSystolicPressure, maxSystolicPressure))
	}
	if v.diastolicPressure != nil && (*v.diastolicPressure < minDiastolicPressure || *v.diastolicPressure > maxDiastolicPressure) {
		return errcode.ErrHealthValueInvalid.WithMsg(fmt.Sprintf("舒张压必须在 %d 到 %d mmHg 之间", minDiastolicPressure, maxDiastolicPressure))
	}
	if v.heartRate != nil && (*v.heartRate < minHeartRate || *v.heartRate > maxHeartRate) {
		return errcode.ErrHealthValueInvalid.WithMsg(fmt.Sprintf("心率必须在 %d 到 %d bpm 之间", minHeartRate, maxHeartRate))
	}
	if v.dailyWaterMl != nil && *v.dailyWaterMl > maxDailyWaterMl {
		return errcode.ErrHealthValueInvalid.WithMsg(fmt.Sprintf("饮水量不能超过 %d ml", maxDailyWaterMl))
	}
	if v.sleepQuality != nil && utf8.RuneCountInString(strings.TrimSpace(*v.sleepQuality)) > maxSleepQualityRunes {
		return errcode.ErrHealthValueInvalid.WithMsg(fmt.Sprintf("睡眠质量不能超过 %d 个字符", maxSleepQualityRunes))
	}
	return nil
}

// validateBloodPressureRelation 校验收缩压必须大于舒张压。
// 只在两个值都有效时校验；部分更新场景由调用方合并数据库已有值后再调用。
func validateBloodPressureRelation(systolic, diastolic *uint16) error {
	if systolic == nil || diastolic == nil {
		return nil
	}
	if *systolic <= *diastolic {
		return errcode.ErrHealthValueInvalid.WithMsg("收缩压必须大于舒张压")
	}
	return nil
}

func validateTextLength(raw *string, max int, field string) error {
	if raw == nil {
		return nil
	}
	if utf8.RuneCountInString(strings.TrimSpace(*raw)) > max {
		return errcode.ErrHealthValueInvalid.WithMsg(fmt.Sprintf("%s不能超过 %d 个字符", field, max))
	}
	return nil
}

// evaluateRecordWarnings 生成保存成功后的异常提醒。调用方保证 rec 是已保存且归属当前用户的记录。
// previousWeight/previousMeasuredAt 只来自 measured_at 顺序上的前一条有效体重，不使用 created_at。
func evaluateRecordWarnings(rec *entity.HealthRecord, previousWeight *decimal.Decimal, previousMeasuredAt *time.Time) []response.HealthWarning {
	if rec == nil {
		return nil
	}
	warnings := make([]response.HealthWarning, 0, 6)
	add := func(metric, message string) {
		warnings = append(warnings, response.HealthWarning{
			Metric:  metric,
			Level:   warningLevelAttention,
			Message: message,
		})
	}

	if rec.SystolicPressure != nil {
		switch {
		case *rec.SystolicPressure >= bpHighSystolic:
			add("systolicPressure", "本次收缩压测量值需要关注，建议在安静状态下复测并结合其他测量结果评估；该提醒不构成医学诊断")
		case *rec.SystolicPressure < bpLowSystolic:
			add("systolicPressure", "本次收缩压测量值偏低，建议复测并关注身体感受；该提醒不构成医学诊断")
		}
	}
	if rec.DiastolicPressure != nil {
		switch {
		case *rec.DiastolicPressure >= bpHighDiastolic:
			add("diastolicPressure", "本次舒张压测量值需要关注，建议在安静状态下复测并结合其他测量结果评估；该提醒不构成医学诊断")
		case *rec.DiastolicPressure < bpLowDiastolic:
			add("diastolicPressure", "本次舒张压测量值偏低，建议复测并关注身体感受；该提醒不构成医学诊断")
		}
	}

	if rec.BloodSugarMmol != nil &&
		(rec.BloodSugarMmol.GreaterThan(bloodSugarHigh) || rec.BloodSugarMmol.LessThan(bloodSugarLow)) {
		// 当前数据模型没有空腹/餐后等测量场景，不能据此给出确定性的医学判断。
		add("bloodSugarMmol", "本次血糖值超出常见参考范围，建议结合空腹或餐后等测量场景复测并咨询专业人员；单次结果不构成医学诊断")
	}

	if rec.HeartRate != nil {
		switch {
		case *rec.HeartRate > heartRateHigh:
			add("heartRate", "本次心率测量值偏快，建议在安静状态下复测；单次结果不构成医学诊断")
		case *rec.HeartRate < heartRateLow:
			add("heartRate", "本次心率测量值偏慢，建议复测并关注身体感受；单次结果不构成医学诊断")
		}
	}

	if rec.SleepDurationHours != nil {
		switch {
		case rec.SleepDurationHours.GreaterThan(sleepDurationHigh):
			add("sleepDurationHours", "本次睡眠时长明显偏长，建议关注近期睡眠情况；单次记录不构成医学诊断")
		case rec.SleepDurationHours.LessThan(sleepDurationLow):
			add("sleepDurationHours", "本次睡眠时长明显偏短，建议关注近期睡眠情况；单次记录不构成医学诊断")
		}
	}

	if rec.WeightKg != nil && previousWeight != nil && previousMeasuredAt != nil {
		elapsed := rec.MeasuredAt.Sub(*previousMeasuredAt)
		if elapsed >= 0 && elapsed <= weightWindowHour {
			diff := rec.WeightKg.Sub(*previousWeight)
			absDiff := diff.Abs()
			percent := decimal.Zero
			if !previousWeight.IsZero() {
				percent = absDiff.Div(*previousWeight).Mul(decimal.NewFromInt(100))
			}
			if absDiff.GreaterThanOrEqual(weightChangeAbs) || percent.GreaterThanOrEqual(weightChangePercent) {
				direction := "增加"
				if diff.IsNegative() {
					direction = "减少"
				}
				add("weightKg", fmt.Sprintf(
					"与最近一次有效体重记录相比%s %s kg（%s%%），建议关注近期体重变化趋势；单次变化不构成医学诊断",
					direction, absDiff.StringFixed(2), percent.StringFixed(1),
				))
			}
		}
	}

	if len(warnings) == 0 {
		return nil
	}
	return warnings
}
