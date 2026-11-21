package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"ai_health/internal/model/entity"
)

func summaryDec(raw string) *decimal.Decimal {
	d := decimal.RequireFromString(raw)
	return &d
}

func TestHealthSummaryIgnoresOutOfRangeValues(t *testing.T) {
	db := openSvcDB(t)
	resetSvcTables(t, db)
	alice := seedHealthUser(t, db, "summary_dirty_alice")
	bob := seedHealthUser(t, db, "summary_dirty_bob")
	charlie := seedHealthUser(t, db, "summary_dirty_charlie")
	svc := newHealthService(db)
	ctx := context.Background()

	// 直接写库绕过写入校验，模拟历史遗留的零值/负值脏数据。
	valid := entity.HealthRecord{
		UserID:             alice.ID,
		HeightCm:           summaryDec("170.00"),
		WeightKg:           summaryDec("60.00"),
		BloodSugarMmol:     summaryDec("5.00"),
		SystolicPressure:   u16Ptr(120),
		DiastolicPressure:  u16Ptr(80),
		HeartRate:          u16Ptr(70),
		SleepDurationHours: summaryDec("7.00"),
		MeasuredAt:         time.Date(2026, 11, 1, 8, 0, 0, 0, time.UTC),
		Source:             entity.SourceManual,
	}
	if err := db.Create(&valid).Error; err != nil {
		t.Fatalf("写入有效记录失败: %v", err)
	}
	invalid := entity.HealthRecord{
		UserID:             alice.ID,
		HeightCm:           summaryDec("0.00"),
		WeightKg:           summaryDec("-5.00"),
		BloodSugarMmol:     summaryDec("0.00"),
		SystolicPressure:   u16Ptr(10),
		DiastolicPressure:  u16Ptr(10),
		HeartRate:          u16Ptr(0),
		SleepDurationHours: summaryDec("30.00"),
		MeasuredAt:         time.Date(2026, 11, 20, 8, 0, 0, 0, time.UTC),
		Source:             entity.SourceManual,
	}
	if err := db.Create(&invalid).Error; err != nil {
		t.Fatalf("写入脏数据记录失败: %v", err)
	}

	summary, err := svc.Summary(ctx, alice.ID)
	if err != nil {
		t.Fatalf("汇总失败: %v", err)
	}
	checks := []struct {
		name string
		got  *string
		want string
	}{
		{"latestHeight", summary.LatestHeight, "170.00"},
		{"latestWeight", summary.LatestWeight, "60.00"},
		{"latestBloodSugar", summary.LatestBloodSugar, "5.00"},
		{"latestSleepDurationHours", summary.LatestSleepDurationHours, "7.00"},
	}
	for _, c := range checks {
		if c.got == nil || *c.got != c.want {
			t.Fatalf("%s 应跳过越界新值并回退到历史有效值 %s，实际: %v", c.name, c.want, c.got)
		}
	}
	if summary.LatestSystolicPressure == nil || *summary.LatestSystolicPressure != 120 ||
		summary.LatestDiastolicPressure == nil || *summary.LatestDiastolicPressure != 80 ||
		summary.LatestHeartRate == nil || *summary.LatestHeartRate != 70 {
		t.Fatalf("血压/心率应回退到历史有效值: %+v", summary)
	}
	if summary.BMI == nil || *summary.BMI != "20.76" || summary.BMISource == nil || *summary.BMISource != "record" {
		t.Fatalf("BMI 不应使用零值/负值: bmi=%v source=%v", summary.BMI, summary.BMISource)
	}
	if got := summary.MeasuredAt["weightKg"]; !got.Equal(valid.MeasuredAt) {
		t.Fatalf("体重时间应为历史有效记录时间，实际: %v", got)
	}
	if summary.LastMeasuredAt == nil || !summary.LastMeasuredAt.Equal(invalid.MeasuredAt) {
		t.Fatalf("lastMeasuredAt 仍是最近一次测量时间（即使指标无效）: %+v", summary.LastMeasuredAt)
	}

	// 最新记录身高有效但体重为 0：BMI 用档案体重补缺，latestWeight 仍为 null。
	profile := entity.HealthProfile{UserID: bob.ID, HeightCm: summaryDec("180.00"), WeightKg: summaryDec("80.00")}
	if err := db.Create(&profile).Error; err != nil {
		t.Fatalf("写入 bob 档案失败: %v", err)
	}
	if err := db.Create(&entity.HealthRecord{
		UserID:     bob.ID,
		HeightCm:   summaryDec("180.00"),
		WeightKg:   summaryDec("0.00"),
		MeasuredAt: time.Date(2026, 11, 20, 8, 0, 0, 0, time.UTC),
		Source:     entity.SourceManual,
	}).Error; err != nil {
		t.Fatalf("写入 bob 记录失败: %v", err)
	}
	bobSummary, err := svc.Summary(ctx, bob.ID)
	if err != nil {
		t.Fatalf("bob 汇总失败: %v", err)
	}
	if bobSummary.LatestWeight != nil {
		t.Fatalf("零体重不应成为 latestWeight: %+v", bobSummary.LatestWeight)
	}
	if bobSummary.LatestHeight == nil || *bobSummary.LatestHeight != "180.00" {
		t.Fatalf("有效身高应保留: %+v", bobSummary.LatestHeight)
	}
	if bobSummary.BMI == nil || *bobSummary.BMI != "24.69" || bobSummary.BMISource == nil || *bobSummary.BMISource != "record+profile" {
		t.Fatalf("零体重应回退档案体重计算 BMI: bmi=%v source=%v", bobSummary.BMI, bobSummary.BMISource)
	}

	// 记录和档案都只有无效值时，BMI 和最新值必须为 null，不能补 0。
	if err := db.Create(&entity.HealthProfile{UserID: charlie.ID, HeightCm: summaryDec("0.00"), WeightKg: summaryDec("-1.00")}).Error; err != nil {
		t.Fatalf("写入 charlie 档案失败: %v", err)
	}
	if err := db.Create(&entity.HealthRecord{
		UserID:     charlie.ID,
		HeightCm:   summaryDec("0.00"),
		WeightKg:   summaryDec("0.00"),
		MeasuredAt: time.Date(2026, 11, 20, 8, 0, 0, 0, time.UTC),
		Source:     entity.SourceManual,
	}).Error; err != nil {
		t.Fatalf("写入 charlie 记录失败: %v", err)
	}
	charlieSummary, err := svc.Summary(ctx, charlie.ID)
	if err != nil {
		t.Fatalf("charlie 汇总失败: %v", err)
	}
	if charlieSummary.BMI != nil || charlieSummary.LatestWeight != nil || charlieSummary.LatestHeight != nil {
		t.Fatalf("全部无效时不应产生 BMI 或最新值: %+v", charlieSummary)
	}

	// 最新记录完全没有体重时，仍应回退到更早的有效体重，并返回对应测量时间。
	dave := seedHealthUser(t, db, "summary_dirty_dave")
	oldWeight := entity.HealthRecord{
		UserID:     dave.ID,
		WeightKg:   summaryDec("65.00"),
		MeasuredAt: time.Date(2026, 11, 2, 8, 0, 0, 0, time.UTC),
		Source:     entity.SourceManual,
	}
	if err := db.Create(&oldWeight).Error; err != nil {
		t.Fatalf("写入 dave 历史体重失败: %v", err)
	}
	if err := db.Create(&entity.HealthRecord{
		UserID:     dave.ID,
		HeartRate:  u16Ptr(72),
		MeasuredAt: time.Date(2026, 11, 21, 8, 0, 0, 0, time.UTC),
		Source:     entity.SourceManual,
	}).Error; err != nil {
		t.Fatalf("写入 dave 最新记录失败: %v", err)
	}
	daveSummary, err := svc.Summary(ctx, dave.ID)
	if err != nil {
		t.Fatalf("dave 汇总失败: %v", err)
	}
	if daveSummary.LatestWeight == nil || *daveSummary.LatestWeight != "65.00" {
		t.Fatalf("最新记录缺体重时应回退历史有效值: %+v", daveSummary.LatestWeight)
	}
	if got := daveSummary.MeasuredAt["weightKg"]; !got.Equal(oldWeight.MeasuredAt) {
		t.Fatalf("回退体重的测量时间应为历史记录时间: %v", got)
	}
	if daveSummary.LatestHeartRate == nil || *daveSummary.LatestHeartRate != 72 {
		t.Fatalf("最新心率应来自最新记录: %+v", daveSummary.LatestHeartRate)
	}
}
