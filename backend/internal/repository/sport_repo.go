package repository

import (
	"context"

	"gorm.io/gorm"

	"ai_health/internal/model/entity"
)

// SportRepo 运动类型、运动建议、疾病禁忌。
type SportRepo struct {
	db *gorm.DB
}

func NewSportRepo(db *gorm.DB) *SportRepo { return &SportRepo{db: db} }

func (r *SportRepo) DB() *gorm.DB { return r.db }

// ListTypes 运动类型字典，数据量小可直接缓存全量。
func (r *SportRepo) ListTypes(ctx context.Context) ([]entity.SportType, error) {
	var list []entity.SportType
	err := r.db.WithContext(ctx).Where("status = 1").Order("id ASC").Find(&list).Error
	return list, err
}

// GetTypeByCode 供 AI 模块把用户诉求映射到运动类型。
func (r *SportRepo) GetTypeByCode(ctx context.Context, code string) (*entity.SportType, error) {
	var t entity.SportType
	err := r.db.WithContext(ctx).Where("type_code = ?", code).Limit(1).Find(&t).Error
	if err != nil || t.ID == 0 {
		return nil, err
	}
	return &t, nil
}

// ListInfos 运动建议列表，预加载类型。
func (r *SportRepo) ListInfos(ctx context.Context, typeID *uint64, offset, limit int) ([]entity.SportInfo, int64, error) {
	q := r.db.WithContext(ctx).Model(&entity.SportInfo{})
	if typeID != nil {
		q = q.Where("sport_type_id = ?", *typeID)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []entity.SportInfo
	err := q.Preload("SportType").Order("id DESC").Offset(offset).Limit(limit).Find(&list).Error
	return list, total, err
}

// ListDiseases 按疾病名匹配禁忌，risk_level 高的优先展示。
func (r *SportRepo) ListDiseases(ctx context.Context, disease string, limit int) ([]entity.SportDiseaseDetail, error) {
	q := r.db.WithContext(ctx).Model(&entity.SportDiseaseDetail{})
	if disease != "" {
		q = q.Where("disease_name LIKE ?", "%"+disease+"%")
	}
	var list []entity.SportDiseaseDetail
	err := q.Order("risk_level DESC").Limit(limit).Find(&list).Error
	return list, err
}
