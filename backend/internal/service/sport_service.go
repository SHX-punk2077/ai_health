package service

import (
	"ai_health/internal/pkg/cache"
	"ai_health/internal/repository"
)

// SportService 运动类型、运动建议、疾病运动禁忌。
//
// TODO(实现清单)：
//  1. ListTypes：全量字典缓存 aihealth:cache:sport:types（TTL 1h）
//  2. ListInfos：按 typeCode 关联查询，预加载 sport_type
//  3. ListDiseases：支持 diseaseName 模糊 + riskLevel 过滤
//  4. 管理端写操作需 sport:create / sport:update 权限
type SportService struct {
	repo  *repository.SportRepo
	store *cache.Store
}

func NewSportService(repo *repository.SportRepo, store *cache.Store) *SportService {
	return &SportService{repo: repo, store: store}
}
