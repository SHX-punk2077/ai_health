package service_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"

	"ai_health/internal/consts"
	"ai_health/internal/dto/request"
	"ai_health/internal/model/entity"
	"ai_health/internal/pkg/errcode"
	"ai_health/internal/pkg/hash"
	"ai_health/internal/repository"
	"ai_health/internal/service"
)

func seedHealthUser(t *testing.T, db *gorm.DB, name string) *entity.SysUser {
	t.Helper()
	pwd, err := hash.Hash("Passw0rd!23")
	if err != nil {
		t.Fatalf("生成密码失败: %v", err)
	}
	u := &entity.SysUser{Username: name, PasswordHash: &pwd, Status: consts.StatusEnabled}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("创建用户 %s 失败: %v", name, err)
	}
	return u
}

func newHealthService(db *gorm.DB) *service.HealthService {
	return service.NewHealthService(repository.NewHealthRepo(db))
}

func assertHealthCode(t *testing.T, err error, want *errcode.Error) {
	t.Helper()
	if err == nil {
		t.Fatalf("期望错误 %v，实际 nil", want)
	}
	if !errors.Is(err, want) {
		t.Fatalf("期望错误码 %d，实际: %v", want.Code, err)
	}
}

func healthRecordReq(measuredAt string, weight *string) request.RecordCreateReq {
	req := request.RecordCreateReq{MeasuredAt: measuredAt, Source: entity.SourceManual}
	req.WeightKg = weight
	return req
}

func TestHealthProfileCRUDAndIsolation(t *testing.T) {
	db := openSvcDB(t)
	resetSvcTables(t, db)
	alice := seedHealthUser(t, db, "health_alice")
	bob := seedHealthUser(t, db, "health_bob")
	svc := newHealthService(db)
	ctx := context.Background()

	if _, err := svc.GetProfile(ctx, alice.ID); !errors.Is(err, errcode.ErrHealthProfileNotFound) {
		t.Fatalf("未建档应返回 40002，实际: %v", err)
	}

	birth := "1990-05-06"
	height := "175.50"
	weight := "70.25"
	realName := "张三"
	create := request.ProfileCreateReq{ProfileFields: request.ProfileFields{
		HealthMetrics: request.HealthMetrics{HeightCm: &height, WeightKg: &weight},
		RealName:      &realName,
		BirthDate:     &birth,
		Gender:        func() *uint8 { v := uint8(1); return &v }(),
	}}
	p, err := svc.CreateProfile(ctx, alice.ID, create)
	if err != nil {
		t.Fatalf("创建档案失败: %v", err)
	}
	if p.ID == "" || p.UserID != fmt.Sprint(alice.ID) {
		t.Fatalf("档案归属异常: %+v", p)
	}
	if p.HeightCm == nil || *p.HeightCm != "175.50" {
		t.Fatalf("身高 DECIMAL 精度异常: %+v", p.HeightCm)
	}
	if p.WeightKg == nil || *p.WeightKg != "70.25" {
		t.Fatalf("体重 DECIMAL 精度异常: %+v", p.WeightKg)
	}
	if p.BMI == nil || *p.BMI == "" {
		t.Fatalf("身高体重齐全时应有 BMI: %+v", p)
	}
	if p.BirthDate == nil || *p.BirthDate != "1990-05-06" {
		t.Fatalf("出生日期格式异常: %+v", p.BirthDate)
	}

	if _, err := svc.CreateProfile(ctx, alice.ID, create); !errors.Is(err, errcode.ErrHealthProfileExists) {
		t.Fatalf("重复创建应返回 40003，实际: %v", err)
	}

	// 用户隔离：bob 既查不到也改不了 alice 的档案。
	if _, err := svc.GetProfile(ctx, bob.ID); !errors.Is(err, errcode.ErrHealthProfileNotFound) {
		t.Fatalf("bob 查询应返回 40002，实际: %v", err)
	}
	if _, err := svc.UpdateProfile(ctx, bob.ID, request.ProfileUpdateReq{}); !errors.Is(err, errcode.ErrHealthProfileNotFound) {
		t.Fatalf("bob 更新应返回 40002，实际: %v", err)
	}

	// 部分更新：未传身高保持不变，传空串的姓名被清空为 NULL。
	newWeight := "72.00"
	empty := ""
	updated, err := svc.UpdateProfile(ctx, alice.ID, request.ProfileUpdateReq{ProfileFields: request.ProfileFields{
		HealthMetrics: request.HealthMetrics{WeightKg: &newWeight},
		RealName:      &empty,
	}})
	if err != nil {
		t.Fatalf("更新档案失败: %v", err)
	}
	if updated.HeightCm == nil || *updated.HeightCm != "175.50" {
		t.Fatalf("未传字段不应被修改: %+v", updated.HeightCm)
	}
	if updated.WeightKg == nil || *updated.WeightKg != "72.00" {
		t.Fatalf("体重更新失败: %+v", updated.WeightKg)
	}
	if updated.RealName != nil {
		t.Fatalf("显式空串应清空为 NULL，实际: %v", *updated.RealName)
	}

	// DECIMAL 列同样支持显式空串清空，且不能被写成 0。
	emptyHeight := ""
	cleared, err := svc.UpdateProfile(ctx, alice.ID, request.ProfileUpdateReq{ProfileFields: request.ProfileFields{
		HealthMetrics: request.HealthMetrics{HeightCm: &emptyHeight},
	}})
	if err != nil {
		t.Fatalf("清空身高失败: %v", err)
	}
	if cleared.HeightCm != nil {
		t.Fatalf("空串应清空 DECIMAL 列为 NULL，实际: %v", *cleared.HeightCm)
	}

	badAge := uint8(151)
	if _, err := svc.UpdateProfile(ctx, alice.ID, request.ProfileUpdateReq{ProfileFields: request.ProfileFields{
		Age: &badAge,
	}}); !errors.Is(err, errcode.ErrHealthValueInvalid) {
		t.Fatalf("年龄 151 应返回 40010，实际: %v", err)
	}
	badGender := uint8(3)
	if _, err := svc.UpdateProfile(ctx, alice.ID, request.ProfileUpdateReq{ProfileFields: request.ProfileFields{
		Gender: &badGender,
	}}); !errors.Is(err, errcode.ErrHealthValueInvalid) {
		t.Fatalf("性别 3 应返回 40010，实际: %v", err)
	}

	bad := "12.345"
	if _, err := svc.UpdateProfile(ctx, alice.ID, request.ProfileUpdateReq{ProfileFields: request.ProfileFields{
		HealthMetrics: request.HealthMetrics{HeightCm: &bad},
	}}); !errors.Is(err, errcode.ErrHealthValueInvalid) {
		t.Fatalf("三位小数应返回 40010，实际: %v", err)
	}

	// Repository 层也不接受把 user_id 放进更新列，防止未来调用方绕过 Service 改归属。
	if _, err := repository.NewHealthRepo(db).UpdateProfile(ctx, alice.ID, map[string]any{"user_id": bob.ID}); err == nil {
		t.Fatal("repository 应拒绝更新 user_id")
	}
	after, err := svc.GetProfile(ctx, alice.ID)
	if err != nil || after.UserID != fmt.Sprint(alice.ID) {
		t.Fatalf("user_id 不应被改变: userID=%v err=%v", after, err)
	}
}

func TestHealthProfileConcurrentCreate(t *testing.T) {
	db := openSvcDB(t)
	resetSvcTables(t, db)
	alice := seedHealthUser(t, db, "health_race")
	svc := newHealthService(db)
	ctx := context.Background()

	const workers = 6
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	start := make(chan struct{})
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := svc.CreateProfile(ctx, alice.ID, request.ProfileCreateReq{})
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errs)

	success, duplicated := 0, 0
	for err := range errs {
		switch {
		case err == nil:
			success++
		case errors.Is(err, errcode.ErrHealthProfileExists):
			duplicated++
		default:
			t.Fatalf("并发创建返回未知错误: %v", err)
		}
	}
	if success != 1 || duplicated != workers-1 {
		t.Fatalf("并发创建结果异常: success=%d duplicated=%d", success, duplicated)
	}

	var count int64
	if err := db.Model(&entity.HealthProfile{}).Where("user_id = ?", alice.ID).Count(&count).Error; err != nil {
		t.Fatalf("统计档案失败: %v", err)
	}
	if count != 1 {
		t.Fatalf("并发创建产生了 %d 份档案", count)
	}
}

func TestHealthRecordsCRUDAndIsolation(t *testing.T) {
	db := openSvcDB(t)
	resetSvcTables(t, db)
	alice := seedHealthUser(t, db, "record_alice")
	bob := seedHealthUser(t, db, "record_bob")
	svc := newHealthService(db)
	ctx := context.Background()

	now := time.Now().UTC()
	day1 := time.Date(now.Year(), now.Month(), now.Day(), 10, 0, 0, 0, time.UTC).AddDate(0, 0, -5)
	day1Late := day1.Add(10 * time.Hour)
	day3 := day1.AddDate(0, 0, 2).Add(2 * time.Hour)
	day5 := day1.AddDate(0, 0, 4).Add(3 * time.Hour)

	w1, w2, w3 := "60.05", "61.10", "62.20"
	height, sugar := "170.10", "5.55"

	r1, err := svc.CreateRecord(ctx, alice.ID, request.RecordCreateReq{
		HealthMetrics: request.HealthMetrics{WeightKg: &w1, HeightCm: &height},
		MeasuredAt:    day1.Format("2006-01-02 15:04:05"),
		Source:        entity.SourceDevice,
	})
	if err != nil {
		t.Fatalf("新增记录失败: %v", err)
	}
	if r1.WeightKg == nil || *r1.WeightKg != "60.05" || r1.Source != entity.SourceDevice {
		t.Fatalf("新增记录返回异常: %+v", r1)
	}
	if _, err := svc.CreateRecord(ctx, alice.ID, healthRecordReq(day1Late.Format("2006-01-02 15:04:05"), &w2)); err != nil {
		t.Fatalf("新增第二条记录失败: %v", err)
	}
	if _, err := svc.CreateRecord(ctx, alice.ID, request.RecordCreateReq{
		HealthMetrics: request.HealthMetrics{WeightKg: &w3, BloodSugarMmol: &sugar},
		MeasuredAt:    day3.Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("新增第三条记录失败: %v", err)
	}
	// 所有指标均为 NULL 的合法记录。
	nullRecord, err := svc.CreateRecord(ctx, alice.ID, request.RecordCreateReq{
		MeasuredAt: day5.Format("2006-01-02T15:04:05"),
	})
	if err != nil {
		t.Fatalf("新增 NULL 记录失败: %v", err)
	}
	if nullRecord.WeightKg != nil || nullRecord.BloodSugarMmol != nil {
		t.Fatalf("NULL 不应被写成 0: %+v", nullRecord)
	}
	if nullRecord.Source != entity.SourceManual {
		t.Fatalf("source 默认值应为 manual: %q", nullRecord.Source)
	}

	// 分页 + 倒序。
	q := request.RecordQuery{PageQuery: request.PageQuery{Page: 1, PageSize: 2}}
	list, total, page, pageSize, err := svc.ListRecords(ctx, alice.ID, q)
	if err != nil {
		t.Fatalf("查询记录失败: %v", err)
	}
	if total != 4 || len(list) != 2 || page != 1 || pageSize != 2 {
		t.Fatalf("分页结果异常: total=%d len=%d page=%d size=%d", total, len(list), page, pageSize)
	}
	if !list[0].MeasuredAt.Equal(day5) {
		t.Fatalf("应按 measured_at 倒序，首条=%v", list[0].MeasuredAt)
	}

	// pageSize 越界必须被收敛到 100。
	_, _, _, pageSize, err = svc.ListRecords(ctx, alice.ID, request.RecordQuery{PageQuery: request.PageQuery{Page: 0, PageSize: 1000}})
	if err != nil || pageSize != 100 {
		t.Fatalf("pageSize 未收敛: size=%d err=%v", pageSize, err)
	}

	// 指标过滤：只有 3 条记录带 weight。
	list, total, _, _, err = svc.ListRecords(ctx, alice.ID, request.RecordQuery{PageQuery: request.PageQuery{Page: 1, PageSize: 10}, Metric: "weightKg"})
	if err != nil {
		t.Fatalf("按指标查询失败: %v", err)
	}
	if total != 3 || len(list) != 3 {
		t.Fatalf("指标过滤异常: total=%d len=%d", total, len(list))
	}
	for _, item := range list {
		if item.WeightKg == nil {
			t.Fatalf("指标过滤返回了 NULL 记录: %+v", item)
		}
	}

	// 时间范围：day1 与 day3 共 3 条，day5 不计入。
	list, total, _, _, err = svc.ListRecords(ctx, alice.ID, request.RecordQuery{
		PageQuery: request.PageQuery{Page: 1, PageSize: 10},
		StartAt:   day1.Format("2006-01-02"),
		EndAt:     day3.Format("2006-01-02"),
	})
	if err != nil || total != 3 || len(list) != 3 {
		t.Fatalf("时间范围查询异常: total=%d len=%d err=%v", total, len(list), err)
	}

	// 归属校验：bob 看不到、查不到、删不掉 alice 的记录。
	if _, total, _, _, err := svc.ListRecords(ctx, bob.ID, request.RecordQuery{PageQuery: request.PageQuery{Page: 1, PageSize: 10}}); err != nil || total != 0 {
		t.Fatalf("bob 不应看到 alice 的记录: total=%d err=%v", total, err)
	}
	firstID := parseUint(t, list[len(list)-1].ID)
	if _, err := svc.GetRecord(ctx, bob.ID, firstID); !errors.Is(err, errcode.ErrHealthRecordNotFound) {
		t.Fatalf("bob 查询 alice 记录应返回 40004，实际: %v", err)
	}
	if err := svc.DeleteRecord(ctx, bob.ID, firstID); !errors.Is(err, errcode.ErrHealthRecordNotFound) {
		t.Fatalf("bob 删除 alice 记录应返回 40004，实际: %v", err)
	}
	if _, err := svc.GetRecord(ctx, alice.ID, firstID); err != nil {
		t.Fatalf("alice 查询自己的记录失败: %v", err)
	}
	if err := svc.DeleteRecord(ctx, alice.ID, firstID); err != nil {
		t.Fatalf("alice 删除自己的记录失败: %v", err)
	}
	if _, err := svc.GetRecord(ctx, alice.ID, firstID); !errors.Is(err, errcode.ErrHealthRecordNotFound) {
		t.Fatalf("删除后应返回 40004，实际: %v", err)
	}

	// 参数错误。
	if _, _, _, _, err := svc.ListRecords(ctx, alice.ID, request.RecordQuery{StartAt: "2026/01/01"}); !errors.Is(err, errcode.ErrHealthDateInvalid) {
		t.Fatalf("非法日期应返回 40005，实际: %v", err)
	}
	if _, _, _, _, err := svc.ListRecords(ctx, alice.ID, request.RecordQuery{Metric: "notExists"}); !errors.Is(err, errcode.ErrHealthMetricInvalid) {
		t.Fatalf("非法指标应返回 40006，实际: %v", err)
	}
}

func TestHealthMeasuredAtValidation(t *testing.T) {
	db := openSvcDB(t)
	resetSvcTables(t, db)
	alice := seedHealthUser(t, db, "record_time")
	svc := newHealthService(db)
	ctx := context.Background()

	future := time.Now().UTC().Add(2 * time.Hour).Format(time.RFC3339)
	if _, err := svc.CreateRecord(ctx, alice.ID, request.RecordCreateReq{MeasuredAt: future}); !errors.Is(err, errcode.ErrMeasuredAtInvalid) {
		t.Fatalf("未来测量时间应返回 40001，实际: %v", err)
	}
	if _, err := svc.CreateRecord(ctx, alice.ID, request.RecordCreateReq{MeasuredAt: "2026/01/01 10:00:00"}); !errors.Is(err, errcode.ErrHealthMeasuredInvalid) {
		t.Fatalf("非法测量时间应返回 40009，实际: %v", err)
	}
	if _, err := svc.GetRecord(ctx, alice.ID, 0); !errors.Is(err, errcode.ErrInvalidParam) {
		t.Fatalf("id=0 应返回 10001，实际: %v", err)
	}
	if _, err := svc.CreateRecord(ctx, alice.ID, request.RecordCreateReq{
		MeasuredAt: "2026-01-01 10:00:00", Source: "unknown",
	}); !errors.Is(err, errcode.ErrHealthValueInvalid) {
		t.Fatalf("非法来源应返回 40010，实际: %v", err)
	}
	imported, err := svc.CreateRecord(ctx, alice.ID, request.RecordCreateReq{
		MeasuredAt: "2026-01-01 10:00:00", Source: entity.SourceImport,
	})
	if err != nil || imported.Source != entity.SourceImport {
		t.Fatalf("import 来源应被接受: %+v err=%v", imported, err)
	}
}

func TestHealthTrendAggregationAndIsolation(t *testing.T) {
	db := openSvcDB(t)
	resetSvcTables(t, db)
	alice := seedHealthUser(t, db, "trend_alice")
	bob := seedHealthUser(t, db, "trend_bob")
	svc := newHealthService(db)
	ctx := context.Background()

	now := time.Now().UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0)
	d1 := monthStart.AddDate(0, 0, 4).Add(8 * time.Hour)
	d1b := d1.Add(4 * time.Hour)
	d2 := monthStart.AddDate(0, 0, 19).Add(9 * time.Hour)
	d3 := d2

	w1, w2, w3, bobW := "70.00", "72.00", "73.50", "100.00"
	sugar := "5.50"
	for _, item := range []struct {
		uid uint64
		at  time.Time
		req request.RecordCreateReq
	}{
		{alice.ID, d1, request.RecordCreateReq{HealthMetrics: request.HealthMetrics{WeightKg: &w1}, MeasuredAt: d1.Format("2006-01-02 15:04:05")}},
		{alice.ID, d1b, request.RecordCreateReq{HealthMetrics: request.HealthMetrics{WeightKg: &w2}, MeasuredAt: d1b.Format("2006-01-02 15:04:05")}},
		{alice.ID, d2, request.RecordCreateReq{HealthMetrics: request.HealthMetrics{WeightKg: &w3, BloodSugarMmol: &sugar}, MeasuredAt: d2.Format("2006-01-02 15:04:05")}},
		{alice.ID, d3, request.RecordCreateReq{HealthMetrics: request.HealthMetrics{BloodSugarMmol: &sugar}, MeasuredAt: d3.Format("2006-01-02 15:04:05")}},
		{bob.ID, d1, request.RecordCreateReq{HealthMetrics: request.HealthMetrics{WeightKg: &bobW}, MeasuredAt: d1.Format("2006-01-02 15:04:05")}},
	} {
		if _, err := svc.CreateRecord(ctx, item.uid, item.req); err != nil {
			t.Fatalf("准备趋势数据失败: %v", err)
		}
	}

	out, err := svc.Trend(ctx, alice.ID, request.TrendQuery{
		Metric:      "weightKg",
		StartDate:   d1.Format("2006-01-02"),
		EndDate:     d2.Format("2006-01-02"),
		Granularity: "day",
	})
	if err != nil {
		t.Fatalf("趋势查询失败: %v", err)
	}
	if out.MetricName != "体重" || out.Unit != "kg" || out.Granularity != "day" {
		t.Fatalf("趋势元信息异常: %+v", out)
	}
	if len(out.Points) != 2 {
		t.Fatalf("NULL 记录不应进入聚合，期望 2 个点，实际 %d: %+v", len(out.Points), out.Points)
	}
	if out.Points[0].Time != d1.Format("2006-01-02") || out.Points[0].Value != "71.00" || out.Points[0].SampleCount != 2 {
		t.Fatalf("day 桶聚合异常: %+v", out.Points[0])
	}
	if out.Points[1].Value != "73.50" || out.Points[1].SampleCount != 1 {
		t.Fatalf("day 桶聚合异常: %+v", out.Points[1])
	}
	// 用户隔离：bob 的 100.00 没有混入平均值。
	if out.Points[0].Value == "85.50" {
		t.Fatal("趋势数据未按用户隔离")
	}

	// month 粒度：同月三点求平均，NULL 不计入。
	out, err = svc.Trend(ctx, alice.ID, request.TrendQuery{
		Metric:      "weightKg",
		StartDate:   monthStart.Format("2006-01-02"),
		EndDate:     d2.Format("2006-01-02"),
		Granularity: "month",
	})
	if err != nil {
		t.Fatalf("month 趋势查询失败: %v", err)
	}
	if len(out.Points) != 1 || out.Points[0].Value != "71.83" || out.Points[0].SampleCount != 3 {
		t.Fatalf("month 聚合异常: %+v", out.Points)
	}

	// week 粒度：周一起桶；d1/d1b 同周，d2 至少晚两周。
	out, err = svc.Trend(ctx, alice.ID, request.TrendQuery{
		Metric:      "weightKg",
		StartDate:   d1.Format("2006-01-02"),
		EndDate:     d2.Format("2006-01-02"),
		Granularity: "week",
	})
	if err != nil {
		t.Fatalf("week 趋势查询失败: %v", err)
	}
	if len(out.Points) != 2 || out.Points[0].Value != "71.00" || out.Points[0].SampleCount != 2 ||
		out.Points[1].Value != "73.50" || out.Points[1].SampleCount != 1 {
		t.Fatalf("week 聚合异常: %+v", out.Points)
	}

	// limit 只保留最近的点。
	out, err = svc.Trend(ctx, alice.ID, request.TrendQuery{
		Metric:      "weightKg",
		StartDate:   d1.Format("2006-01-02"),
		EndDate:     d2.Format("2006-01-02"),
		Granularity: "day",
		Limit:       1,
	})
	if err != nil || len(out.Points) != 1 || out.Points[0].Time != out.EndDate {
		t.Fatalf("limit 截断异常: %+v err=%v", out, err)
	}

	// 无数据返回空集合，而不是 nil 或伪造 0。
	out, err = svc.Trend(ctx, alice.ID, request.TrendQuery{Metric: "totalCholesterolMmol"})
	if err != nil {
		t.Fatalf("空趋势查询失败: %v", err)
	}
	if out.Points == nil || len(out.Points) != 0 {
		t.Fatalf("空数据应返回空集合: %+v", out.Points)
	}

	// 参数校验。
	if _, err := svc.Trend(ctx, alice.ID, request.TrendQuery{Metric: "notExists"}); !errors.Is(err, errcode.ErrHealthMetricInvalid) {
		t.Fatalf("非法指标应返回 40006，实际: %v", err)
	}
	if _, err := svc.Trend(ctx, alice.ID, request.TrendQuery{Metric: "weightKg", Granularity: "hour"}); !errors.Is(err, errcode.ErrHealthGranularity) {
		t.Fatalf("非法粒度应返回 40007，实际: %v", err)
	}
	if _, err := svc.Trend(ctx, alice.ID, request.TrendQuery{Metric: "weightKg", StartDate: "2026/01/01"}); !errors.Is(err, errcode.ErrHealthDateInvalid) {
		t.Fatalf("非法日期应返回 40005，实际: %v", err)
	}
	if _, err := svc.Trend(ctx, alice.ID, request.TrendQuery{
		Metric: "weightKg", StartDate: "2025-01-01", EndDate: "2026-06-01",
	}); !errors.Is(err, errcode.ErrHealthRangeInvalid) {
		t.Fatalf("超过 366 天应返回 40008，实际: %v", err)
	}
	// 含首尾正好 366 个自然日允许；再多一天拒绝。
	if _, err := svc.Trend(ctx, alice.ID, request.TrendQuery{
		Metric: "weightKg", StartDate: "2024-01-01", EndDate: "2024-12-31",
	}); err != nil {
		t.Fatalf("366 个自然日应允许，实际: %v", err)
	}
	if _, err := svc.Trend(ctx, alice.ID, request.TrendQuery{
		Metric: "weightKg", StartDate: "2024-01-01", EndDate: "2025-01-01",
	}); !errors.Is(err, errcode.ErrHealthRangeInvalid) {
		t.Fatalf("367 个自然日应返回 40008，实际: %v", err)
	}
	if _, err := svc.Trend(ctx, alice.ID, request.TrendQuery{
		Metric: "weightKg", StartDate: "2026-06-01", EndDate: "2026-01-01",
	}); !errors.Is(err, errcode.ErrHealthRangeInvalid) {
		t.Fatalf("start 晚于 end 应返回 40008，实际: %v", err)
	}
	if _, err := svc.Trend(ctx, alice.ID, request.TrendQuery{Metric: "weightKg", Limit: 1001}); !errors.Is(err, errcode.ErrInvalidParam) {
		t.Fatalf("limit 过大应返回 10001，实际: %v", err)
	}
}

func parseUint(t *testing.T, s string) uint64 {
	t.Helper()
	var v uint64
	if _, err := fmt.Sscan(s, &v); err != nil {
		t.Fatalf("解析 ID %q 失败: %v", s, err)
	}
	return v
}
