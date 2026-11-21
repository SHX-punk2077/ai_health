package service

import (
	"ai_health/internal/pkg/cache"
	"ai_health/internal/repository"
)

// HealthService 健康档案 / 健康记录 / 趋势统计。
//
// TODO(实现清单)：
//  1. GetProfile / SaveProfile：upsert + 写后失效 aihealth:cache:profile:{uid}
//  2. CreateRecord：measuredAt 不得晚于当前时间，source 默认 manual
//  3. ListRecords：分页 + 时间区间过滤，走 idx_health_record_user_measured
//  4. Trend：按 metric 取近 N 条，返回单位与点位
//  5. 越权校验：所有读写都必须带 user_id 条件
type HealthService struct {
	repo  *repository.HealthRepo
	store *cache.Store
}

func NewHealthService(repo *repository.HealthRepo, store *cache.Store) *HealthService {
	return &HealthService{repo: repo, store: store}
}
