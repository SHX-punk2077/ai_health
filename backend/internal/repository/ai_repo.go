package repository

import (
	"context"

	"gorm.io/gorm"

	"ai_health/internal/model/entity"
)

// AIRepo AI 会话与消息。
type AIRepo struct {
	db *gorm.DB
}

func NewAIRepo(db *gorm.DB) *AIRepo { return &AIRepo{db: db} }

func (r *AIRepo) DB() *gorm.DB { return r.db }

func (r *AIRepo) CreateConversation(ctx context.Context, c *entity.AIConversation) error {
	return r.db.WithContext(ctx).Create(c).Error
}

// GetConversation 带 user_id 条件，防止越权读取他人会话。
func (r *AIRepo) GetConversation(ctx context.Context, uid, id uint64) (*entity.AIConversation, error) {
	var c entity.AIConversation
	err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, uid).Limit(1).Find(&c).Error
	if err != nil || c.ID == 0 {
		return nil, err
	}
	return &c, nil
}

func (r *AIRepo) ListConversations(ctx context.Context, uid uint64, offset, limit int) ([]entity.AIConversation, int64, error) {
	q := r.db.WithContext(ctx).Model(&entity.AIConversation{}).Where("user_id = ?", uid)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []entity.AIConversation
	err := q.Order("updated_at DESC").Offset(offset).Limit(limit).Find(&list).Error
	return list, total, err
}

// NextSequence 取下一序号；并发下依赖 uk_ai_message_conversation_sequence 兜底。
func (r *AIRepo) NextSequence(ctx context.Context, cid uint64) (uint32, error) {
	var max uint32
	err := r.db.WithContext(ctx).Model(&entity.AIMessage{}).
		Where("conversation_id = ?", cid).
		Select("COALESCE(MAX(sequence_no), 0)").Scan(&max).Error
	return max + 1, err
}

func (r *AIRepo) CreateMessage(ctx context.Context, m *entity.AIMessage) error {
	return r.db.WithContext(ctx).Create(m).Error
}

// RecentMessages 取最近 n 条作为上下文，再反转成时间正序。
func (r *AIRepo) RecentMessages(ctx context.Context, cid uint64, n int) ([]entity.AIMessage, error) {
	var list []entity.AIMessage
	err := r.db.WithContext(ctx).
		Where("conversation_id = ?", cid).
		Order("sequence_no DESC").Limit(n).Find(&list).Error
	for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
		list[i], list[j] = list[j], list[i]
	}
	return list, err
}

func (r *AIRepo) TouchConversation(ctx context.Context, cid uint64, title string) error {
	updates := map[string]any{"updated_at": gorm.Expr("CURRENT_TIMESTAMP(3)")}
	if title != "" {
		updates["title"] = title
	}
	return r.db.WithContext(ctx).Model(&entity.AIConversation{}).
		Where("id = ?", cid).Updates(updates).Error
}
