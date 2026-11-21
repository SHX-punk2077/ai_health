package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"ai_health/internal/model/entity"
)

// RecordFilter 健康记录查询条件，nil 表示该端不限制。
// EndAt 为开区间上界（measured_at < EndAt），便于表达“包含 endAt 当天”。
type RecordFilter struct {
	StartAt *time.Time
	EndAt   *time.Time
	// Metric 非空时只保留该指标列非 NULL 的记录。
	Metric string
}

// TrendRow 趋势聚合的单点，按桶起始日升序返回。
type TrendRow struct {
	Bucket      string
	Value       decimal.Decimal
	SampleCount int64
}

// healthMetricColumns 把对外指标名映射到真实列名。
// 所有外部传入的 metric 都必须先经过这里，禁止直接拼进 SQL。
var healthMetricColumns = map[string]string{
	"heightCm":             "height_cm",
	"weightKg":             "weight_kg",
	"bloodSugarMmol":       "blood_sugar_mmol",
	"systolicPressure":     "systolic_pressure",
	"diastolicPressure":    "diastolic_pressure",
	"totalCholesterolMmol": "total_cholesterol_mmol",
	"heartRate":            "heart_rate",
	"visionLeft":           "vision_left",
	"visionRight":          "vision_right",
	"sleepDurationHours":   "sleep_duration_hours",
	"dailyWaterMl":         "daily_water_ml",
}

// HealthMetricColumn 返回指标对应的数据库列；ok=false 表示不支持的指标。
func HealthMetricColumn(metric string) (string, bool) {
	col, ok := healthMetricColumns[metric]
	return col, ok
}

// HealthMetricNames 返回受支持的指标名，供服务层校验和文档使用。
func HealthMetricNames() []string {
	return []string{
		"heightCm", "weightKg", "bloodSugarMmol", "systolicPressure",
		"diastolicPressure", "totalCholesterolMmol", "heartRate",
		"visionLeft", "visionRight", "sleepDurationHours", "dailyWaterMl",
	}
}

// HealthRepo 健康档案与健康记录。
type HealthRepo struct {
	db *gorm.DB
}

func NewHealthRepo(db *gorm.DB) *HealthRepo { return &HealthRepo{db: db} }

func (r *HealthRepo) DB() *gorm.DB { return r.db }

// Transaction 在同一数据库事务中执行 fn，并把绑定该事务的 HealthRepo 传给回调。
// 服务层需要“锁定-校验-写入”原子化时使用；回调返回错误会整体回滚。
func (r *HealthRepo) Transaction(ctx context.Context, fn func(tx *HealthRepo) error) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("repository: 健康事务缺少数据库连接")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&HealthRepo{db: tx})
	})
}

// GetProfile 一人一份，未建档返回 (nil, nil)。
func (r *HealthRepo) GetProfile(ctx context.Context, uid uint64) (*entity.HealthProfile, error) {
	var p entity.HealthProfile
	err := r.db.WithContext(ctx).Where("user_id = ?", uid).Limit(1).Find(&p).Error
	if err != nil || p.ID == 0 {
		return nil, err
	}
	return &p, nil
}

// LockProfile 在事务内对当前用户档案加行锁（SELECT ... FOR UPDATE）。
// 未建档返回 (nil, nil)；所有条件都包含 user_id，不能锁定他人档案。
func (r *HealthRepo) LockProfile(ctx context.Context, uid uint64) (*entity.HealthProfile, error) {
	if uid == 0 {
		return nil, fmt.Errorf("repository: 档案锁定缺少 user_id")
	}
	var p entity.HealthProfile
	err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ?", uid).
		Limit(1).
		Find(&p).Error
	if err != nil || p.ID == 0 {
		return nil, err
	}
	return &p, nil
}

// CreateProfile 依赖 uk_health_profile_user_id 保证一人一份；
// 并发重复创建由唯一键拒绝，调用方负责把 1062 转成业务错误。
func (r *HealthRepo) CreateProfile(ctx context.Context, p *entity.HealthProfile) error {
	return r.db.WithContext(ctx).Create(p).Error
}

// UpdateProfile 只更新 updates 中显式给出的列，绝不包含 user_id。
// 返回受影响行数；MySQL 在“值未变化”时可能返回 0，不能据此判断不存在。
func (r *HealthRepo) UpdateProfile(ctx context.Context, uid uint64, updates map[string]any) (int64, error) {
	if _, ok := updates["user_id"]; ok {
		return 0, fmt.Errorf("repository: health_profile 不允许更新 user_id")
	}
	if len(updates) == 0 {
		return 0, nil
	}
	tx := r.db.WithContext(ctx).
		Model(&entity.HealthProfile{}).
		Where("user_id = ?", uid).
		Updates(updates)
	return tx.RowsAffected, tx.Error
}

// ListRecords 按 measured_at 倒序分页，走 idx_health_record_user_measured。
func (r *HealthRepo) ListRecords(ctx context.Context, uid uint64, f RecordFilter, offset, limit int) ([]entity.HealthRecord, int64, error) {
	q := r.db.WithContext(ctx).Model(&entity.HealthRecord{}).Where("user_id = ?", uid)
	if f.StartAt != nil {
		q = q.Where("measured_at >= ?", *f.StartAt)
	}
	if f.EndAt != nil {
		q = q.Where("measured_at < ?", *f.EndAt)
	}
	if f.Metric != "" {
		col, ok := healthMetricColumns[f.Metric]
		if !ok {
			return nil, 0, fmt.Errorf("repository: 不支持的健康指标 %q", f.Metric)
		}
		q = q.Where(col + " IS NOT NULL")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []entity.HealthRecord
	err := q.Order("measured_at DESC, id DESC").Offset(offset).Limit(limit).Find(&list).Error
	return list, total, err
}

// GetRecord 带 user_id 条件，防止越权读取他人记录。
func (r *HealthRepo) GetRecord(ctx context.Context, uid, id uint64) (*entity.HealthRecord, error) {
	var rec entity.HealthRecord
	err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, uid).Limit(1).Find(&rec).Error
	if err != nil || rec.ID == 0 {
		return nil, err
	}
	return &rec, nil
}

func (r *HealthRepo) CreateRecord(ctx context.Context, rec *entity.HealthRecord) error {
	return r.db.WithContext(ctx).Create(rec).Error
}

// LockRecord 在事务内锁定当前用户的一条记录（SELECT ... FOR UPDATE）。
// 条件同时包含 id 和 user_id；不存在或不属于当前用户都返回 (nil, nil)。
func (r *HealthRepo) LockRecord(ctx context.Context, uid, id uint64) (*entity.HealthRecord, error) {
	if uid == 0 || id == 0 {
		return nil, fmt.Errorf("repository: 记录锁定缺少有效的 user_id 或 id")
	}
	var rec entity.HealthRecord
	err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND user_id = ?", id, uid).
		Limit(1).
		Find(&rec).Error
	if err != nil || rec.ID == 0 {
		return nil, err
	}
	return &rec, nil
}

// DeleteRecord 健康记录不做软删，直接物理删除并带 user_id 防越权。
func (r *HealthRepo) DeleteRecord(ctx context.Context, uid, id uint64) (int64, error) {
	tx := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, uid).Delete(&entity.HealthRecord{})
	return tx.RowsAffected, tx.Error
}

// UpdateRecord 只更新 updates 中显式给出的列，并始终附加 user_id 条件。
// user_id、id、created_at、source 由服务层拒绝；这里再兜底一次，防止内部调用绕过。
// MySQL 在“值未变化”时可能返回 RowsAffected=0，不能据此判断记录不存在。
func (r *HealthRepo) UpdateRecord(ctx context.Context, uid, id uint64, updates map[string]any) (int64, error) {
	if uid == 0 || id == 0 {
		return 0, fmt.Errorf("repository: 健康记录更新缺少有效的 user_id 或 id")
	}
	for _, protected := range []string{"id", "user_id", "created_at", "source"} {
		if _, ok := updates[protected]; ok {
			return 0, fmt.Errorf("repository: health_record 不允许更新 %s", protected)
		}
	}
	if len(updates) == 0 {
		return 0, nil
	}
	tx := r.db.WithContext(ctx).
		Model(&entity.HealthRecord{}).
		Where("id = ? AND user_id = ?", id, uid).
		Updates(updates)
	return tx.RowsAffected, tx.Error
}

// PreviousWeight 返回 measured_at、id 排序下严格早于给定位置的最近一条有效体重。
// 更新场景传入 excludeID，避免把当前记录自身当作历史值；只按 measured_at 排序，不使用 created_at。
func (r *HealthRepo) PreviousWeight(ctx context.Context, uid uint64, before time.Time, excludeID uint64) (*decimal.Decimal, *time.Time, error) {
	var row struct {
		WeightKg   decimal.Decimal `gorm:"column:weight_kg"`
		MeasuredAt time.Time       `gorm:"column:measured_at"`
	}
	q := r.db.WithContext(ctx).
		Table("health_record").
		Select("weight_kg, measured_at").
		Where("user_id = ? AND weight_kg IS NOT NULL", uid)
	if excludeID == 0 {
		q = q.Where("measured_at < ?", before)
	} else {
		q = q.Where("(measured_at < ? OR (measured_at = ? AND id < ?))", before, before, excludeID)
	}
	err := q.Order("measured_at DESC, id DESC").Limit(1).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	measuredAt := row.MeasuredAt
	return &row.WeightKg, &measuredAt, nil
}

// MetricRange 是一个指标在健康汇总中可接受的闭区间。
// 它用于过滤数据库里历史遗留的 0、负数等工程上无效的值，不代表医学正常范围。
type MetricRange struct {
	Min decimal.Decimal
	Max decimal.Decimal
}

// LatestMetricRow 是单个指标最近一次有效测量。
type LatestMetricRow struct {
	Metric     string
	Value      string
	MeasuredAt time.Time
}

// LatestMetrics 为每个指标各取 measured_at DESC, id DESC 的第一条非 NULL 值。
// ranges 中的指标会额外按闭区间过滤，使最新一次“无效值”不会遮蔽更早的有效值。
// 使用一个 UNION ALL 查询，避免为每个指标各发一条 SQL；metric 必须来自白名单。
func (r *HealthRepo) LatestMetrics(ctx context.Context, uid uint64, metrics []string, ranges map[string]MetricRange) ([]LatestMetricRow, error) {
	if uid == 0 {
		return nil, fmt.Errorf("repository: 健康汇总缺少 user_id")
	}
	if len(metrics) == 0 {
		return nil, nil
	}
	parts := make([]string, 0, len(metrics))
	args := make([]any, 0, len(metrics))
	for _, metric := range metrics {
		col, ok := healthMetricColumns[metric]
		if !ok {
			return nil, fmt.Errorf("repository: 不支持的健康指标 %q", metric)
		}
		parts = append(parts, fmt.Sprintf(
			"(SELECT '%s' AS metric, CAST(%s AS CHAR) AS value, measured_at "+
				"FROM health_record WHERE user_id = ? AND %s IS NOT NULL "+
				"ORDER BY measured_at DESC, id DESC LIMIT 1)",
			metric, col, col,
		))
		args = append(args, uid)
		if rg, ok := ranges[metric]; ok {
			parts[len(parts)-1] = fmt.Sprintf(
				"(SELECT '%s' AS metric, CAST(%s AS CHAR) AS value, measured_at "+
					"FROM health_record WHERE user_id = ? AND %s IS NOT NULL "+
					"AND %s >= ? AND %s <= ? "+
					"ORDER BY measured_at DESC, id DESC LIMIT 1)",
				metric, col, col, col, col,
			)
			args = append(args, rg.Min, rg.Max)
		}
	}
	var rows []LatestMetricRow
	err := r.db.WithContext(ctx).Raw(strings.Join(parts, " UNION ALL "), args...).Scan(&rows).Error
	return rows, err
}

// LatestMeasuredAt 返回当前用户最近一条记录的测量时间；没有记录返回 nil。
func (r *HealthRepo) LatestMeasuredAt(ctx context.Context, uid uint64) (*time.Time, error) {
	var row struct {
		MeasuredAt time.Time `gorm:"column:measured_at"`
	}
	err := r.db.WithContext(ctx).
		Table("health_record").
		Select("measured_at").
		Where("user_id = ?", uid).
		Order("measured_at DESC, id DESC").
		Limit(1).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row.MeasuredAt, nil
}

// Trend 在 MySQL 内完成分组聚合，避免把全部历史记录加载到内存。
// WHERE 已过滤 NULL，因此空桶不会出现，也不会把缺失值补成 0。
func (r *HealthRepo) Trend(ctx context.Context, uid uint64, metric string, f RecordFilter, granularity string) ([]TrendRow, error) {
	col, ok := healthMetricColumns[metric]
	if !ok {
		return nil, fmt.Errorf("repository: 不支持的趋势指标 %q", metric)
	}
	bucketExpr, ok := trendBucketExpr(granularity)
	if !ok {
		return nil, fmt.Errorf("repository: 不支持的聚合粒度 %q", granularity)
	}

	q := r.db.WithContext(ctx).
		Table("health_record").
		Select(bucketExpr+" AS bucket, CAST(AVG("+col+") AS DECIMAL(14,2)) AS value, COUNT("+col+") AS sample_count").
		Where("user_id = ? AND "+col+" IS NOT NULL", uid)
	if f.StartAt != nil {
		q = q.Where("measured_at >= ?", *f.StartAt)
	}
	if f.EndAt != nil {
		q = q.Where("measured_at < ?", *f.EndAt)
	}

	var rows []TrendRow
	err := q.Group(bucketExpr).Order("bucket ASC").Scan(&rows).Error
	return rows, err
}

func trendBucketExpr(granularity string) (string, bool) {
	switch granularity {
	case "day":
		return "DATE_FORMAT(measured_at, '%Y-%m-%d')", true
	case "week":
		return "DATE_FORMAT(DATE_SUB(measured_at, INTERVAL WEEKDAY(measured_at) DAY), '%Y-%m-%d')", true
	case "month":
		return "DATE_FORMAT(measured_at, '%Y-%m-01')", true
	default:
		return "", false
	}
}
