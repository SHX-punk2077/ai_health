package repository

import (
	"context"

	"gorm.io/gorm"

	"ai_health/internal/model/entity"
)

// DiseaseFilter 疾病注意事项查询条件。
type DiseaseFilter struct {
	Name      string
	TypeID    *uint64
	RiskLevel *uint8
}

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

// GetInfo 运动建议详情，未找到返回 (nil, nil)。
func (r *SportRepo) GetInfo(ctx context.Context, id uint64) (*entity.SportInfo, error) {
	var info entity.SportInfo
	err := r.db.WithContext(ctx).Preload("SportType").Where("id = ?", id).Limit(1).Find(&info).Error
	if err != nil || info.ID == 0 {
		return nil, err
	}
	return &info, nil
}

// ListDiseases 按疾病名 / 运动类型 / 风险等级筛选，risk_level 高的优先展示。
func (r *SportRepo) ListDiseases(ctx context.Context, f DiseaseFilter, offset, limit int) ([]entity.SportDiseaseDetail, int64, error) {
	q := r.db.WithContext(ctx).Model(&entity.SportDiseaseDetail{})
	if f.Name != "" {
		q = q.Where("disease_name LIKE ?", "%"+f.Name+"%")
	}
	if f.TypeID != nil {
		q = q.Where("sport_type_id = ?", *f.TypeID)
	}
	if f.RiskLevel != nil {
		q = q.Where("risk_level = ?", *f.RiskLevel)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []entity.SportDiseaseDetail
	err := q.Preload("SportType").
		Order("risk_level DESC, id ASC").
		Offset(offset).Limit(limit).Find(&list).Error
	return list, total, err
}
