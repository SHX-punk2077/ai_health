package entity_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"ai_health/internal/model/entity"
	"ai_health/internal/repository"
)

// TestAppendMessageSequenceConcurrent 验证同一会话的并发写入不会拿到重复 sequence_no。
// 依赖 AppendMessage 的会话行锁；数据库唯一索引是最后一道防线。
func TestAppendMessageSequenceConcurrent(t *testing.T) {
	db := openTestDB(t)

	user := createUser(t, db, "concurrent-ai")
	conv := createConversation(t, db, user.ID)
	repo := repository.NewAIRepo(db)

	const workers = 8
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			msg := &entity.AIMessage{
				ConversationID: conv.ID,
				MessageRole:    entity.RoleUser,
				Content:        fmt.Sprintf("并发消息 %d", i),
			}
			if err := repo.AppendMessage(context.Background(), msg); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("并发写入消息失败: %v", err)
	}

	var messages []entity.AIMessage
	if err := db.Where("conversation_id = ?", conv.ID).Order("sequence_no ASC").Find(&messages).Error; err != nil {
		t.Fatalf("查询消息失败: %v", err)
	}
	if len(messages) != workers {
		t.Fatalf("消息数 = %d, want %d", len(messages), workers)
	}
	for i, msg := range messages {
		if msg.SequenceNo != uint32(i+1) {
			t.Fatalf("第 %d 条消息 sequence_no = %d, want %d", i, msg.SequenceNo, i+1)
		}
	}
}
