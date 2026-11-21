package repository

import (
	"context"

	"gorm.io/gorm"

	"ai_health/internal/model/entity"
)

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

// ListRecords 按 measured_at 倒序分页，走 idx_health_record_user_measured。
func (r *HealthRepo) ListRecords(ctx context.Context, uid uint64, offset, limit int) ([]entity.HealthRecord, int64, error) {
	q := r.db.WithContext(ctx).Model(&entity.HealthRecord{}).Where("user_id = ?", uid)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []entity.HealthRecord
	err := q.Order("measured_at DESC, id DESC").Offset(offset).Limit(limit).Find(&list).Error
	return list, total, err
}

func (r *HealthRepo) CreateRecord(ctx context.Context, rec *entity.HealthRecord) error {
	return r.db.WithContext(ctx).Create(rec).Error
}

// DeleteRecord 健康记录不做软删，直接物理删除并带 user_id 防越权。
func (r *HealthRepo) DeleteRecord(ctx context.Context, uid, id uint64) (int64, error) {
	tx := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, uid).Delete(&entity.HealthRecord{})
	return tx.RowsAffected, tx.Error
}
