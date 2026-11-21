package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"ai_health/internal/model/entity"
)

// RecordFilter 健康记录查询条件，nil 表示该端不限制。
type RecordFilter struct {
	StartAt *time.Time
	EndAt   *time.Time
}

// TrendRow 趋势聚合的单点，按 measured_at 升序返回。
type TrendRow struct {
	MeasuredAt time.Time
	Value      decimal.Decimal
}

// trendColumns 把对外的指标名映射到真实列名，禁止把外部字符串直接拼进 SQL。
var trendColumns = map[string]string{
	"weightKg":         "weight_kg",
	"bloodSugarMmol":   "blood_sugar_mmol",
	"systolicPressure": "systolic_pressure",
}

// HealthRepo 健康档案与健康记录。
type HealthRepo struct {
	db *gorm.DB
}

func NewHealthRepo(db *gorm.DB) *HealthRepo { return &HealthRepo{db: db} }

func (r *HealthRepo) DB() *gorm.DB { return r.db }

// GetProfile 一人一份，未建档返回 (nil, nil)。
func (r *HealthRepo) GetProfile(ctx context.Context, uid uint64) (*entity.HealthProfile, error) {
	var p entity.HealthProfile
	err := r.db.WithContext(ctx).Where("user_id = ?", uid).Limit(1).Find(&p).Error
	if err != nil || p.ID == 0 {
		return nil, err
	}
	return &p, nil
}

// SaveProfile upsert：命中唯一键 user_id 时更新，否则插入。
func (r *HealthRepo) SaveProfile(ctx context.Context, p *entity.HealthProfile) error {
	return r.db.WithContext(ctx).Save(p).Error
}

func (r *HealthRepo) CreateProfile(ctx context.Context, p *entity.HealthProfile) error {
	return r.db.WithContext(ctx).Create(p).Error
}

// ListRecords 按 measured_at 倒序分页，走 idx_health_record_user_measured。
func (r *HealthRepo) ListRecords(ctx context.Context, uid uint64, f RecordFilter, offset, limit int) ([]entity.HealthRecord, int64, error) {
	q := r.db.WithContext(ctx).Model(&entity.HealthRecord{}).Where("user_id = ?", uid)
	if f.StartAt != nil {
		q = q.Where("measured_at >= ?", *f.StartAt)
	}
	if f.EndAt != nil {
		q = q.Where("measured_at <= ?", *f.EndAt)
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

// DeleteRecord 健康记录不做软删，直接物理删除并带 user_id 防越权。
func (r *HealthRepo) DeleteRecord(ctx context.Context, uid, id uint64) (int64, error) {
	tx := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, uid).Delete(&entity.HealthRecord{})
	return tx.RowsAffected, tx.Error
}

// Trend 取最近 limit 个非空采样点并按时间升序，供折线图直接使用。
func (r *HealthRepo) Trend(ctx context.Context, uid uint64, metric string, f RecordFilter, limit int) ([]TrendRow, error) {
	col, ok := trendColumns[metric]
	if !ok {
		return nil, fmt.Errorf("repository: 不支持的趋势指标 %q", metric)
	}
	q := r.db.WithContext(ctx).
		Table("health_record").
		Select("measured_at AS measured_at, CAST("+col+" AS DECIMAL(12,2)) AS value").
		Where("user_id = ? AND "+col+" IS NOT NULL", uid)
	if f.StartAt != nil {
		q = q.Where("measured_at >= ?", *f.StartAt)
	}
	if f.EndAt != nil {
		q = q.Where("measured_at <= ?", *f.EndAt)
	}
	if limit <= 0 {
		limit = 30
	}
	var rows []TrendRow
	if err := q.Order("measured_at DESC").Limit(limit).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	return rows, nil
}
