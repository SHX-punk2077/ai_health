package service

import (
	"ai_health/internal/config"
	"ai_health/internal/pkg/cache"
	"ai_health/internal/repository"
)

// AIService AI 问答：上下文拼装、大模型调用（OpenAI 兼容协议，国内厂商优先）、消息落库。
//
// TODO(实现清单)：
//  1. Ask：拼 prompt（健康档案 + 近期记录 + 运动建议 + 疾病禁忌）→ aiclient.Chat
//  2. 落库顺序：user 消息 sequence_no=n → assistant 消息 n+1，同一事务
//  3. 首轮问答后把会话标题改写为前 20 字
//  4. aihealth:ai:lock:{uid} 防重复提交；aihealth:ai:ctx:{uid} 缓存档案摘要
//  5. 流式输出走 SSE，写入 server.write_timeout 之外需单独处理
type AIService struct {
	repo  *repository.AIRepo
	store *cache.Store
	cfg   *config.Config
}

func NewAIService(repo *repository.AIRepo, store *cache.Store, cfg *config.Config) *AIService {
	return &AIService{repo: repo, store: store, cfg: cfg}
}
