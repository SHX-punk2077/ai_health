package service

import (
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"ai_health/internal/dto/request"
	"ai_health/internal/model/entity"
	"ai_health/internal/pkg/errcode"
)

func testStr(v string) *string { return &v }
func testU16(v uint16) *uint16 { return &v }

func TestParseHealthMetricsEngineeringBounds(t *testing.T) {
	cases := []struct {
		name    string
		metrics request.HealthMetrics
		wantErr bool
	}{
		{name: "正常值", metrics: request.HealthMetrics{HeightCm: testStr("175.50"), WeightKg: testStr("70.25"), BloodSugarMmol: testStr("5.60"), SystolicPressure: testU16(120), DiastolicPressure: testU16(80), HeartRate: testU16(72), SleepDurationHours: testStr("7.50")}},
		{name: "全部下边界", metrics: request.HealthMetrics{HeightCm: testStr("30.00"), WeightKg: testStr("2.00"), BloodSugarMmol: testStr("0.50"), TotalCholesterolMmol: testStr("1.00"), VisionLeft: testStr("0.00"), VisionRight: testStr("0.00"), SleepDurationHours: testStr("0.00"), SystolicPressure: testU16(40), DiastolicPressure: testU16(20), HeartRate: testU16(20), DailyWaterMl: func() *uint32 { v := uint32(0); return &v }()}},
		{name: "全部上边界", metrics: request.HealthMetrics{HeightCm: testStr("250.00"), WeightKg: testStr("500.00"), BloodSugarMmol: testStr("50.00"), TotalCholesterolMmol: testStr("30.00"), VisionLeft: testStr("20.00"), VisionRight: testStr("20.00"), SleepDurationHours: testStr("24.00"), SystolicPressure: testU16(300), DiastolicPressure: testU16(200), HeartRate: testU16(300), DailyWaterMl: func() *uint32 { v := uint32(20000); return &v }()}},
		{name: "身高过低", metrics: request.HealthMetrics{HeightCm: testStr("29.99")}, wantErr: true},
		{name: "身高过高", metrics: request.HealthMetrics{HeightCm: testStr("250.01")}, wantErr: true},
		{name: "体重过低", metrics: request.HealthMetrics{WeightKg: testStr("1.99")}, wantErr: true},
		{name: "体重过高", metrics: request.HealthMetrics{WeightKg: testStr("500.01")}, wantErr: true},
		{name: "血糖过低", metrics: request.HealthMetrics{BloodSugarMmol: testStr("0.49")}, wantErr: true},
		{name: "血糖过高", metrics: request.HealthMetrics{BloodSugarMmol: testStr("50.01")}, wantErr: true},
		{name: "胆固醇过高", metrics: request.HealthMetrics{TotalCholesterolMmol: testStr("30.01")}, wantErr: true},
		{name: "视力过高", metrics: request.HealthMetrics{VisionLeft: testStr("20.01")}, wantErr: true},
		{name: "睡眠超过24小时", metrics: request.HealthMetrics{SleepDurationHours: testStr("24.01")}, wantErr: true},
		{name: "收缩压过低", metrics: request.HealthMetrics{SystolicPressure: testU16(39), DiastolicPressure: testU16(20)}, wantErr: true},
		{name: "舒张压过低", metrics: request.HealthMetrics{SystolicPressure: testU16(100), DiastolicPressure: testU16(19)}, wantErr: true},
		{name: "心率过低", metrics: request.HealthMetrics{HeartRate: testU16(19)}, wantErr: true},
		{name: "饮水量过高", metrics: request.HealthMetrics{DailyWaterMl: func() *uint32 { v := uint32(20001); return &v }()}, wantErr: true},
		{name: "收缩压不大于舒张压", metrics: request.HealthMetrics{SystolicPressure: testU16(80), DiastolicPressure: testU16(80)}, wantErr: true},
		{name: "收缩压小于舒张压", metrics: request.HealthMetrics{SystolicPressure: testU16(70), DiastolicPressure: testU16(90)}, wantErr: true},
		{name: "睡眠质量过长", metrics: request.HealthMetrics{SleepQuality: testStr(strings.Repeat("睡", 31))}, wantErr: true},
		{name: "空串表示清空", metrics: request.HealthMetrics{HeightCm: testStr(""), WeightKg: testStr(""), BloodSugarMmol: testStr("")}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseHealthMetrics(tc.metrics)
			if tc.wantErr {
				if err == nil {
					t.Fatal("期望 40010，实际 nil")
				}
				if !errcode.ErrHealthValueInvalid.Is(err) {
					t.Fatalf("期望 40010，实际: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("期望成功，实际: %v", err)
			}
		})
	}
}

func TestEvaluateRecordWarnings(t *testing.T) {
	now := time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC)
	dec := func(s string) *decimal.Decimal { d := decimal.RequireFromString(s); return &d }

	t.Run("正常记录无提醒", func(t *testing.T) {
		rec := &entity.HealthRecord{MeasuredAt: now, WeightKg: dec("70.00"), SystolicPressure: testU16(120), DiastolicPressure: testU16(80), BloodSugarMmol: dec("5.60"), HeartRate: testU16(72), SleepDurationHours: dec("7.50")}
		if got := evaluateRecordWarnings(rec, nil, nil); len(got) != 0 {
			t.Fatalf("正常记录不应有提醒: %+v", got)
		}
	})

	t.Run("血压血糖心率睡眠偏高或偏低", func(t *testing.T) {
		rec := &entity.HealthRecord{MeasuredAt: now, SystolicPressure: testU16(150), DiastolicPressure: testU16(95), BloodSugarMmol: dec("8.50"), HeartRate: testU16(110), SleepDurationHours: dec("3.00")}
		got := evaluateRecordWarnings(rec, nil, nil)
		metrics := map[string]bool{}
		for _, w := range got {
			metrics[w.Metric] = true
			if w.Level != "warning" {
				t.Fatalf("提醒级别异常: %+v", w)
			}
			if w.Message == "" {
				t.Fatalf("提醒文案不能为空: %+v", w)
			}
		}
		for _, m := range []string{"systolicPressure", "diastolicPressure", "bloodSugarMmol", "heartRate", "sleepDurationHours"} {
			if !metrics[m] {
				t.Fatalf("缺少 %s 提醒: %+v", m, got)
			}
		}
	})

	t.Run("低血压和低血糖低心率", func(t *testing.T) {
		rec := &entity.HealthRecord{MeasuredAt: now, SystolicPressure: testU16(85), DiastolicPressure: testU16(55), BloodSugarMmol: dec("3.50"), HeartRate: testU16(50)}
		got := evaluateRecordWarnings(rec, nil, nil)
		if len(got) != 4 {
			t.Fatalf("期望 4 条提醒，实际 %+v", got)
		}
	})

	t.Run("体重按 measured_at 窗口比较", func(t *testing.T) {
		prevAt := now.Add(-6 * 24 * time.Hour)
		prev := decimal.RequireFromString("70.00")
		rec := &entity.HealthRecord{MeasuredAt: now, WeightKg: dec("72.50")}
		got := evaluateRecordWarnings(rec, &prev, &prevAt)
		if len(got) != 1 || got[0].Metric != "weightKg" {
			t.Fatalf("期望体重提醒，实际 %+v", got)
		}

		small := decimal.RequireFromString("71.00")
		if got := evaluateRecordWarnings(&entity.HealthRecord{MeasuredAt: now, WeightKg: dec("71.00")}, &small, &prevAt); len(got) != 0 {
			t.Fatalf("小幅体重变化不应提醒: %+v", got)
		}
		oldAt := now.Add(-14 * 24 * time.Hour)
		if got := evaluateRecordWarnings(&entity.HealthRecord{MeasuredAt: now, WeightKg: dec("80.00")}, &prev, &oldAt); len(got) != 0 {
			t.Fatalf("超过 7 天的历史体重不应触发短期提醒: %+v", got)
		}
		if got := evaluateRecordWarnings(&entity.HealthRecord{MeasuredAt: now, WeightKg: dec("80.00")}, nil, nil); len(got) != 0 {
			t.Fatalf("没有历史体重不应提醒: %+v", got)
		}
	})
}
