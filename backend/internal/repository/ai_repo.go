package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

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

// CloseConversation 关闭 / 恢复会话，带 user_id 防越权。
func (r *AIRepo) CloseConversation(ctx context.Context, uid, id uint64, status uint8) (int64, error) {
	res := r.db.WithContext(ctx).Model(&entity.AIConversation{}).
		Where("id = ? AND user_id = ?", id, uid).Update("status", status)
	return res.RowsAffected, res.Error
}

// DeleteConversation 软删会话，消息由外键保留；列表查询自动过滤。
func (r *AIRepo) DeleteConversation(ctx context.Context, uid, id uint64) (int64, error) {
	res := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, uid).Delete(&entity.AIConversation{})
	return res.RowsAffected, res.Error
}

// NextSequence 只用于只读预估，不可作为并发写入的序号来源；
// 写入请使用 AppendMessage，由会话行锁串行化 MAX+1。
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

// AppendMessage 在同一个事务内锁定会话行，再计算 MAX(sequence_no)+1 并插入。
// 同一会话的并发写入因此串行化；uk_ai_message_conversation_sequence 仍是最终唯一性保障。
func (r *AIRepo) AppendMessage(ctx context.Context, m *entity.AIMessage) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var conv entity.AIConversation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id").Where("id = ?", m.ConversationID).First(&conv).Error; err != nil {
			return err
		}
		var max uint32
		if err := tx.Model(&entity.AIMessage{}).
			Where("conversation_id = ?", m.ConversationID).
			Select("COALESCE(MAX(sequence_no), 0)").Scan(&max).Error; err != nil {
			return err
		}
		m.SequenceNo = max + 1
		return tx.Create(m).Error
	})
}

// ListMessages 分页读取消息：先按 sequence_no 倒序取一页，再反转为正序返回。
func (r *AIRepo) ListMessages(ctx context.Context, cid uint64, beforeSeq *uint32, offset, limit int) ([]entity.AIMessage, int64, error) {
	q := r.db.WithContext(ctx).Model(&entity.AIMessage{}).Where("conversation_id = ?", cid)
	if beforeSeq != nil && *beforeSeq > 0 {
		q = q.Where("sequence_no < ?", *beforeSeq)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []entity.AIMessage
	if err := q.Order("sequence_no DESC").Offset(offset).Limit(limit).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
		list[i], list[j] = list[j], list[i]
	}
	return list, total, nil
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
