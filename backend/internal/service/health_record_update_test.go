package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"ai_health/internal/dto/request"
	"ai_health/internal/pkg/errcode"
)

func decodeUpdateReq(t *testing.T, body string) request.RecordUpdateReq {
	t.Helper()
	var req request.RecordUpdateReq
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatalf("解析更新请求失败: %v, body=%s", err, body)
	}
	return req
}

func recordID(t *testing.T, raw string) uint64 {
	t.Helper()
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		t.Fatalf("解析记录 ID 失败: %v", err)
	}
	return id
}

func strPtr(v string) *string { return &v }
func u16Ptr(v uint16) *uint16 { return &v }

func TestHealthRecordUpdatePartialNullAndProtectedFields(t *testing.T) {
	db := openSvcDB(t)
	resetSvcTables(t, db)
	alice := seedHealthUser(t, db, "health_update_alice")
	svc := newHealthService(db)
	ctx := context.Background()

	weight := "70.25"
	created, err := svc.CreateRecord(ctx, alice.ID, request.RecordCreateReq{
		MeasuredAt:        "2026-11-20T10:00:00Z",
		Source:            "manual",
		HeightCm:          strPtr("175.50"),
		WeightKg:          &weight,
		SystolicPressure:  u16Ptr(120),
		DiastolicPressure: u16Ptr(80),
		HeartRate:         u16Ptr(72),
		Remark:            strPtr("初始备注"),
	})
	if err != nil {
		t.Fatalf("创建记录失败: %v", err)
	}
	id := recordID(t, created.ID)

	// 部分更新：只改体重，其他字段、归属、创建时间、测量时间保持不变。
	updated, err := svc.UpdateRecord(ctx, alice.ID, id, decodeUpdateReq(t, `{"weightKg":"72.00"}`))
	if err != nil {
		t.Fatalf("部分更新失败: %v", err)
	}
	if updated.WeightKg == nil || *updated.WeightKg != "72.00" {
		t.Fatalf("体重应更新为 72.00，实际: %+v", updated.WeightKg)
	}
	if updated.HeightCm == nil || *updated.HeightCm != "175.50" {
		t.Fatalf("未传身高应保持原值，实际: %+v", updated.HeightCm)
	}
	if updated.SystolicPressure == nil || *updated.SystolicPressure != 120 {
		t.Fatalf("未传收缩压应保持原值，实际: %+v", updated.SystolicPressure)
	}
	if updated.MeasuredAt != created.MeasuredAt {
		t.Fatalf("未传 measuredAt 应保持原值: before=%v after=%v", created.MeasuredAt, updated.MeasuredAt)
	}
	if updated.UserID != created.UserID || updated.ID != created.ID {
		t.Fatalf("受保护元数据被改变: before=%+v after=%+v", created, updated)
	}
	// MySQL DATETIME(3) 会按毫秒四舍五入，创建响应中的纳秒值不能直接与重查值做精确比较。
	if diff := updated.CreatedAt.Sub(created.CreatedAt); diff < -time.Millisecond || diff > time.Millisecond {
		t.Fatalf("createdAt 被改变: before=%v after=%v", created.CreatedAt, updated.CreatedAt)
	}

	// 显式 null 保持原值；DECIMAL/文本空串清空为 NULL。
	updated, err = svc.UpdateRecord(ctx, alice.ID, id, decodeUpdateReq(t, `{"weightKg":null,"heightCm":"","systolicPressure":null,"remark":""}`))
	if err != nil {
		t.Fatalf("null/空串更新失败: %v", err)
	}
	if updated.WeightKg == nil || *updated.WeightKg != "72.00" {
		t.Fatalf("显式 null 应保持体重，实际: %+v", updated.WeightKg)
	}
	if updated.SystolicPressure == nil || *updated.SystolicPressure != 120 {
		t.Fatalf("显式 null 应保持收缩压，实际: %+v", updated.SystolicPressure)
	}
	if updated.HeightCm != nil {
		t.Fatalf("空串应清空身高，实际: %+v", updated.HeightCm)
	}
	if updated.Remark != nil {
		t.Fatalf("空串应清空备注，实际: %+v", updated.Remark)
	}

	// 全部字段都是显式 null：语义上无变化，但仍应确认记录存在并返回当前值。
	updated, err = svc.UpdateRecord(ctx, alice.ID, id, decodeUpdateReq(t, `{"weightKg":null,"diastolicPressure":null,"measuredAt":null}`))
	if err != nil {
		t.Fatalf("全 null 更新应成功: %v", err)
	}
	if updated.WeightKg == nil || *updated.WeightKg != "72.00" || updated.DiastolicPressure == nil || *updated.DiastolicPressure != 80 {
		t.Fatalf("全 null 更新不应改变数据: %+v", updated)
	}

	// 更新为相同值：MySQL RowsAffected 可能为 0，但不能误判为记录不存在。
	if _, err := svc.UpdateRecord(ctx, alice.ID, id, decodeUpdateReq(t, `{"weightKg":"72.00"}`)); err != nil {
		t.Fatalf("相同值更新不应返回不存在: %v", err)
	}

	// 修改测量时间（合法）与备注。
	updated, err = svc.UpdateRecord(ctx, alice.ID, id, decodeUpdateReq(t, `{"measuredAt":"2026-11-19T09:00:00Z","remark":"重新测量"}`))
	if err != nil {
		t.Fatalf("更新测量时间失败: %v", err)
	}
	wantAt, _ := time.Parse(time.RFC3339, "2026-11-19T09:00:00Z")
	if !updated.MeasuredAt.Equal(wantAt) {
		t.Fatalf("测量时间未更新: %v", updated.MeasuredAt)
	}
	if updated.Remark == nil || *updated.Remark != "重新测量" {
		t.Fatalf("备注未更新: %+v", updated.Remark)
	}
}

func TestHealthRecordUpdateRejectsProtectedFieldsAndBadInput(t *testing.T) {
	db := openSvcDB(t)
	resetSvcTables(t, db)
	alice := seedHealthUser(t, db, "health_update_bad_alice")
	bob := seedHealthUser(t, db, "health_update_bad_bob")
	svc := newHealthService(db)
	ctx := context.Background()

	weight := "68.40"
	created, err := svc.CreateRecord(ctx, alice.ID, request.RecordCreateReq{
		MeasuredAt: "2026-11-18T10:00:00Z",
		WeightKg:   &weight,
	})
	if err != nil {
		t.Fatalf("创建记录失败: %v", err)
	}
	id := recordID(t, created.ID)

	for _, body := range []string{
		`{}`,
		`{"id":"1"}`,
		`{"id":null}`,
		`{"userId":"2"}`,
		`{"user_id":"2"}`,
		`{"createdAt":"2026-01-01T00:00:00Z"}`,
		`{"created_at":"2026-01-01T00:00:00Z"}`,
		`{"source":"device"}`,
	} {
		if _, err := svc.UpdateRecord(ctx, alice.ID, id, decodeUpdateReq(t, body)); !errors.Is(err, errcode.ErrInvalidParam) {
			t.Fatalf("受保护字段/空对象应返回 10001: body=%s err=%v", body, err)
		}
	}

	for _, tc := range []struct {
		body string
		want *errcode.Error
	}{
		{`{"weightKg":"12.345"}`, errcode.ErrHealthValueInvalid},
		{`{"weightKg":"-1.00"}`, errcode.ErrHealthValueInvalid},
		{`{"systolicPressure":301}`, errcode.ErrHealthValueInvalid},
		{`{"smokingStatus":3}`, errcode.ErrHealthValueInvalid},
		{`{"measuredAt":"not-a-time"}`, errcode.ErrHealthMeasuredInvalid},
		{`{"measuredAt":"2999-01-01T00:00:00Z"}`, errcode.ErrMeasuredAtInvalid},
	} {
		if _, err := svc.UpdateRecord(ctx, alice.ID, id, decodeUpdateReq(t, tc.body)); !errors.Is(err, tc.want) {
			t.Fatalf("非法更新应返回 %d: body=%s err=%v", tc.want.Code, tc.body, err)
		}
	}

	// 失败请求不能造成部分更新。
	after, err := svc.GetRecord(ctx, alice.ID, id)
	if err != nil {
		t.Fatalf("读取记录失败: %v", err)
	}
	if after.WeightKg == nil || *after.WeightKg != "68.40" {
		t.Fatalf("非法更新不应改变体重: %+v", after.WeightKg)
	}

	// 跨用户更新：统一 40004，且不能改变对方数据。
	bobWeight := "80.00"
	bobRec, err := svc.CreateRecord(ctx, bob.ID, request.RecordCreateReq{
		MeasuredAt: "2026-11-18T11:00:00Z",
		WeightKg:   &bobWeight,
	})
	if err != nil {
		t.Fatalf("创建 bob 记录失败: %v", err)
	}
	bobID := recordID(t, bobRec.ID)
	if _, err := svc.UpdateRecord(ctx, alice.ID, bobID, decodeUpdateReq(t, `{"weightKg":"1.00"}`)); !errors.Is(err, errcode.ErrHealthRecordNotFound) {
		t.Fatalf("跨用户更新应返回 40004，实际: %v", err)
	}
	bobAfter, err := svc.GetRecord(ctx, bob.ID, bobID)
	if err != nil || bobAfter.WeightKg == nil || *bobAfter.WeightKg != "80.00" {
		t.Fatalf("跨用户更新不应改变数据: rec=%+v err=%v", bobAfter, err)
	}

	if _, err := svc.UpdateRecord(ctx, alice.ID, 99999999, decodeUpdateReq(t, `{"weightKg":"70.00"}`)); !errors.Is(err, errcode.ErrHealthRecordNotFound) {
		t.Fatalf("不存在记录应返回 40004，实际: %v", err)
	}
}

func TestHealthRecordBackfillOrderingAndWeightWarning(t *testing.T) {
	db := openSvcDB(t)
	resetSvcTables(t, db)
	alice := seedHealthUser(t, db, "health_backfill_alice")
	svc := newHealthService(db)
	ctx := context.Background()

	base := "70.00"
	if _, err := svc.CreateRecord(ctx, alice.ID, request.RecordCreateReq{
		MeasuredAt: "2026-11-01T08:00:00Z",
		WeightKg:   &base,
	}); err != nil {
		t.Fatalf("创建基准记录失败: %v", err)
	}
	latest := "70.20"
	if _, err := svc.CreateRecord(ctx, alice.ID, request.RecordCreateReq{
		MeasuredAt: "2026-11-10T08:00:00Z",
		WeightKg:   &latest,
	}); err != nil {
		t.Fatalf("创建较新记录失败: %v", err)
	}

	// 历史补录：测量时间介于两条已有记录之间，体重较前一条明显变化。
	backfill := "76.50"
	rec, err := svc.CreateRecord(ctx, alice.ID, request.RecordCreateReq{
		MeasuredAt: "2026-11-05T08:00:00Z",
		WeightKg:   &backfill,
	})
	if err != nil {
		t.Fatalf("历史补录失败: %v", err)
	}
	found := false
	for _, w := range rec.Warnings {
		if w.Metric == "weightKg" {
			found = true
		}
	}
	if !found {
		t.Fatalf("历史补录的明显体重变化应返回提醒: %+v", rec.Warnings)
	}

	// 补录不能改变“最新有效值”的排序：仍应取 measured_at 最新的 70.20。
	summary, err := svc.Summary(ctx, alice.ID)
	if err != nil {
		t.Fatalf("汇总失败: %v", err)
	}
	if summary.LatestWeight == nil || *summary.LatestWeight != "70.20" {
		t.Fatalf("历史补录不应覆盖最新体重: %+v", summary.LatestWeight)
	}
	wantAt := time.Date(2026, 11, 10, 8, 0, 0, 0, time.UTC)
	if got := summary.MeasuredAt["weightKg"]; !got.Equal(wantAt) {
		t.Fatalf("最新体重时间错误: %v", got)
	}
}

func TestHealthRecordUpdateConcurrentDifferentValues(t *testing.T) {
	db := openSvcDB(t)
	resetSvcTables(t, db)
	alice := seedHealthUser(t, db, "health_update_distinct")
	svc := newHealthService(db)
	ctx := context.Background()

	weight := "70.00"
	created, err := svc.CreateRecord(ctx, alice.ID, request.RecordCreateReq{
		MeasuredAt: "2026-11-16T10:00:00Z",
		WeightKg:   &weight,
	})
	if err != nil {
		t.Fatalf("创建记录失败: %v", err)
	}
	id := recordID(t, created.ID)

	const workers = 8
	want := make(map[string]struct{}, workers)
	var wg sync.WaitGroup
	errCh := make(chan error, workers)
	start := make(chan struct{})
	for i := 0; i < workers; i++ {
		value := fmt.Sprintf("7%d.00", i+1)
		want[value] = struct{}{}
		wg.Add(1)
		go func(v string) {
			defer wg.Done()
			<-start
			if _, err := svc.UpdateRecord(ctx, alice.ID, id, decodeUpdateReq(t, fmt.Sprintf(`{"weightKg":%q}`, v))); err != nil {
				errCh <- fmt.Errorf("并发写入体重 %s: %w", v, err)
			}
		}(value)
	}
	close(start)
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}

	got, err := svc.GetRecord(ctx, alice.ID, id)
	if err != nil {
		t.Fatalf("并发更新后读取失败: %v", err)
	}
	if got.WeightKg == nil {
		t.Fatalf("并发更新后体重为空: %+v", got)
	}
	if _, ok := want[*got.WeightKg]; !ok {
		t.Fatalf("最终体重必须是某次成功写入的值（最后写入者生效），实际: %s", *got.WeightKg)
	}
}

func TestHealthRecordUpdateBloodPressureIsAtomic(t *testing.T) {
	db := openSvcDB(t)
	resetSvcTables(t, db)
	alice := seedHealthUser(t, db, "health_update_bp_atomic")
	svc := newHealthService(db)
	ctx := context.Background()

	weight := "70.00"
	created, err := svc.CreateRecord(ctx, alice.ID, request.RecordCreateReq{
		MeasuredAt:        "2026-11-15T10:00:00Z",
		WeightKg:          &weight,
		SystolicPressure:  u16Ptr(120),
		DiastolicPressure: u16Ptr(80),
	})
	if err != nil {
		t.Fatalf("创建记录失败: %v", err)
	}
	id := recordID(t, created.ID)

	// 单侧更新也必须与库中另一侧合并校验，非法值不能落库。
	if _, err := svc.UpdateRecord(ctx, alice.ID, id, decodeUpdateReq(t, `{"systolicPressure":75}`)); !errors.Is(err, errcode.ErrHealthValueInvalid) {
		t.Fatalf("收缩压不高于舒张压应返回 40010，实际: %v", err)
	}
	unchanged, err := svc.GetRecord(ctx, alice.ID, id)
	if err != nil {
		t.Fatalf("读取记录失败: %v", err)
	}
	if unchanged.SystolicPressure == nil || *unchanged.SystolicPressure != 120 ||
		unchanged.DiastolicPressure == nil || *unchanged.DiastolicPressure != 80 {
		t.Fatalf("失败的血压更新不应改变数据: %+v", unchanged)
	}

	// 并发分别修改收缩压/舒张压：两次更新各自对旧值都合法，合并后可能非法。
	// 行锁必须让两次更新串行，并让后一次基于最新库值重新校验。
	type result struct {
		name string
		err  error
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	go func() {
		<-start
		_, err := svc.UpdateRecord(ctx, alice.ID, id, decodeUpdateReq(t, `{"systolicPressure":85}`))
		results <- result{name: "systolic", err: err}
	}()
	go func() {
		<-start
		_, err := svc.UpdateRecord(ctx, alice.ID, id, decodeUpdateReq(t, `{"diastolicPressure":90}`))
		results <- result{name: "diastolic", err: err}
	}()
	close(start)

	var success, rejected int
	for i := 0; i < 2; i++ {
		r := <-results
		switch {
		case r.err == nil:
			success++
		case errors.Is(r.err, errcode.ErrHealthValueInvalid):
			rejected++
		default:
			t.Fatalf("%s 更新返回非预期错误: %v", r.name, r.err)
		}
	}
	if success != 1 || rejected != 1 {
		t.Fatalf("并发交叉更新血压应恰好一个成功、一个被拒，实际 success=%d rejected=%d", success, rejected)
	}

	got, err := svc.GetRecord(ctx, alice.ID, id)
	if err != nil {
		t.Fatalf("读取最终血压失败: %v", err)
	}
	if got.SystolicPressure == nil || got.DiastolicPressure == nil || *got.SystolicPressure <= *got.DiastolicPressure {
		t.Fatalf("最终血压必须满足收缩压大于舒张压: %+v", got)
	}
	if !((*got.SystolicPressure == 85 && *got.DiastolicPressure == 80) ||
		(*got.SystolicPressure == 120 && *got.DiastolicPressure == 90)) {
		t.Fatalf("最终血压必须是某一次成功更新的结果，实际: %d/%d", *got.SystolicPressure, *got.DiastolicPressure)
	}

	// 同时修改两侧都合法时，两次更新都成功，结果确定。
	secondWeight := "70.00"
	second, err := svc.CreateRecord(ctx, alice.ID, request.RecordCreateReq{
		MeasuredAt:        "2026-11-14T10:00:00Z",
		WeightKg:          &secondWeight,
		SystolicPressure:  u16Ptr(120),
		DiastolicPressure: u16Ptr(80),
	})
	if err != nil {
		t.Fatalf("创建第二条记录失败: %v", err)
	}
	secondID := recordID(t, second.ID)

	var wg sync.WaitGroup
	errCh := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := svc.UpdateRecord(ctx, alice.ID, secondID, decodeUpdateReq(t, `{"systolicPressure":130,"diastolicPressure":85}`)); err != nil {
				errCh <- fmt.Errorf("同时更新双压: %w", err)
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}
	secondGot, err := svc.GetRecord(ctx, alice.ID, secondID)
	if err != nil {
		t.Fatalf("读取第二条记录失败: %v", err)
	}
	if secondGot.SystolicPressure == nil || *secondGot.SystolicPressure != 130 ||
		secondGot.DiastolicPressure == nil || *secondGot.DiastolicPressure != 85 {
		t.Fatalf("同时更新双压结果错误: %+v", secondGot)
	}
}

func TestHealthRecordUpdateConcurrent(t *testing.T) {
	db := openSvcDB(t)
	resetSvcTables(t, db)
	alice := seedHealthUser(t, db, "health_update_concurrent")
	svc := newHealthService(db)
	ctx := context.Background()

	weight := "70.00"
	created, err := svc.CreateRecord(ctx, alice.ID, request.RecordCreateReq{
		MeasuredAt: "2026-11-17T10:00:00Z",
		WeightKg:   &weight,
		HeartRate:  u16Ptr(70),
		Remark:     strPtr("start"),
	})
	if err != nil {
		t.Fatalf("创建记录失败: %v", err)
	}
	id := recordID(t, created.ID)

	var wg sync.WaitGroup
	errCh := make(chan error, 20)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := svc.UpdateRecord(ctx, alice.ID, id, decodeUpdateReq(t, `{"weightKg":"71.50"}`)); err != nil {
				errCh <- fmt.Errorf("并发更新体重: %w", err)
			}
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := svc.UpdateRecord(ctx, alice.ID, id, decodeUpdateReq(t, `{"heartRate":78}`)); err != nil {
				errCh <- fmt.Errorf("并发更新心率: %w", err)
			}
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := svc.UpdateRecord(ctx, alice.ID, id, decodeUpdateReq(t, `{"remark":"done"}`)); err != nil {
				errCh <- fmt.Errorf("并发更新备注: %w", err)
			}
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := svc.UpdateRecord(ctx, alice.ID, id, decodeUpdateReq(t, `{"dailyWaterMl":1500}`)); err != nil {
				errCh <- fmt.Errorf("并发更新饮水: %w", err)
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}

	got, err := svc.GetRecord(ctx, alice.ID, id)
	if err != nil {
		t.Fatalf("并发更新后读取失败: %v", err)
	}
	if got.WeightKg == nil || *got.WeightKg != "71.50" {
		t.Fatalf("并发更新后体重不正确: %+v", got.WeightKg)
	}
	if got.HeartRate == nil || *got.HeartRate != 78 {
		t.Fatalf("并发更新后心率不正确: %+v", got.HeartRate)
	}
	if got.Remark == nil || *got.Remark != "done" {
		t.Fatalf("并发更新后备注不正确: %+v", got.Remark)
	}
	if got.DailyWaterMl == nil || *got.DailyWaterMl != 1500 {
		t.Fatalf("并发更新后饮水量不正确: %+v", got.DailyWaterMl)
	}
}
