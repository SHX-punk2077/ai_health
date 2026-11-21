package service_test

import (
	"context"
	"testing"
	"time"

	"ai_health/internal/dto/request"
	"ai_health/internal/pkg/errcode"
)

func TestHealthRecordHardValidationAndWarnings(t *testing.T) {
	db := openSvcDB(t)
	resetSvcTables(t, db)
	alice := seedHealthUser(t, db, "health_rule_alice")
	svc := newHealthService(db)
	ctx := context.Background()
	measuredAt := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)

	badHeight := "29.99"
	if _, err := svc.CreateRecord(ctx, alice.ID, request.RecordCreateReq{
		HealthMetrics: request.HealthMetrics{HeightCm: &badHeight},
		MeasuredAt:    measuredAt,
	}); !errcode.ErrHealthValueInvalid.Is(err) {
		t.Fatalf("身高 29.99 应返回 40010，实际: %v", err)
	}

	badSys, badDia := uint16(80), uint16(90)
	if _, err := svc.CreateRecord(ctx, alice.ID, request.RecordCreateReq{
		HealthMetrics: request.HealthMetrics{SystolicPressure: &badSys, DiastolicPressure: &badDia},
		MeasuredAt:    measuredAt,
	}); !errcode.ErrHealthValueInvalid.Is(err) {
		t.Fatalf("收缩压不大于舒张压应返回 40010，实际: %v", err)
	}

	empty, err := svc.CreateRecord(ctx, alice.ID, request.RecordCreateReq{MeasuredAt: measuredAt})
	if err != nil {
		t.Fatalf("空记录应允许保存: %v", err)
	}
	if len(empty.Warnings) != 0 {
		t.Fatalf("空记录不应有提醒: %+v", empty.Warnings)
	}

	sys, dia, heart := uint16(150), uint16(95), uint16(110)
	sugar, sleep := "8.50", "3.00"
	out, err := svc.CreateRecord(ctx, alice.ID, request.RecordCreateReq{
		HealthMetrics: request.HealthMetrics{
			SystolicPressure:   &sys,
			DiastolicPressure:  &dia,
			BloodSugarMmol:     &sugar,
			HeartRate:          &heart,
			SleepDurationHours: &sleep,
		},
		MeasuredAt: measuredAt,
	})
	if err != nil {
		t.Fatalf("可疑数据应允许保存: %v", err)
	}
	seen := map[string]bool{}
	for _, w := range out.Warnings {
		seen[w.Metric] = true
		if w.Level != "warning" || w.Message == "" {
			t.Fatalf("提醒结构异常: %+v", w)
		}
	}
	for _, metric := range []string{"systolicPressure", "diastolicPressure", "bloodSugarMmol", "heartRate", "sleepDurationHours"} {
		if !seen[metric] {
			t.Fatalf("缺少 %s 提醒: %+v", metric, out.Warnings)
		}
	}

	recID := recordID(t, out.ID)
	if _, err := svc.UpdateRecord(ctx, alice.ID, recID, decodeUpdateReq(t, `{"systolicPressure":70}`)); !errcode.ErrHealthValueInvalid.Is(err) {
		t.Fatalf("单侧收缩压更新应结合已有舒张压返回 40010，实际: %v", err)
	}
	if _, err := svc.UpdateRecord(ctx, alice.ID, recID, decodeUpdateReq(t, `{"diastolicPressure":160}`)); !errcode.ErrHealthValueInvalid.Is(err) {
		t.Fatalf("单侧舒张压更新应结合已有收缩压返回 40010，实际: %v", err)
	}
	updated, err := svc.UpdateRecord(ctx, alice.ID, recID, decodeUpdateReq(t, `{"diastolicPressure":85}`))
	if err != nil {
		t.Fatalf("合法单侧血压更新失败: %v", err)
	}
	if len(updated.Warnings) == 0 {
		t.Fatal("收缩压仍偏高时应返回提醒")
	}

	height, weight := "175.50", "70.00"
	profileSys, profileDia := uint16(120), uint16(80)
	if _, err := svc.CreateProfile(ctx, alice.ID, request.ProfileCreateReq{ProfileFields: request.ProfileFields{
		HealthMetrics: request.HealthMetrics{
			HeightCm: &height, WeightKg: &weight,
			SystolicPressure: &profileSys, DiastolicPressure: &profileDia,
		},
	}}); err != nil {
		t.Fatalf("创建正常档案失败: %v", err)
	}
	badProfileHeight := "20.00"
	bob := seedHealthUser(t, db, "health_rule_bob")
	if _, err := svc.CreateProfile(ctx, bob.ID, request.ProfileCreateReq{ProfileFields: request.ProfileFields{
		HealthMetrics: request.HealthMetrics{HeightCm: &badProfileHeight},
	}}); !errcode.ErrHealthValueInvalid.Is(err) {
		t.Fatalf("档案身高 20 应返回 40010，实际: %v", err)
	}

	if _, err := svc.UpdateProfile(ctx, alice.ID, request.ProfileUpdateReq{ProfileFields: request.ProfileFields{
		HealthMetrics: request.HealthMetrics{SystolicPressure: func() *uint16 { v := uint16(70); return &v }()},
	}}); !errcode.ErrHealthValueInvalid.Is(err) {
		t.Fatalf("更新档案单侧收缩压应结合已有舒张压返回 40010，实际: %v", err)
	}
}
