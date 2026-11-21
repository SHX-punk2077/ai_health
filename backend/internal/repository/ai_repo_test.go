package repository_test

import (
	"context"
	"sync"
	"testing"

	"ai_health/internal/model/entity"
	"ai_health/internal/repository"
)

func TestAIRepoConversationOwnership(t *testing.T) {
	db := openTestDB(t)
	resetTables(t, db)
	ctx := context.Background()

	alice := mustUser(t, db, "alice_ai")
	bob := mustUser(t, db, "bob_ai")

	repo := repository.NewAIRepo(db)
	conv := &entity.AIConversation{UserID: alice.ID, Title: "减脂计划", Status: 1}
	if err := repo.CreateConversation(ctx, conv); err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}

	got, err := repo.GetConversation(ctx, alice.ID, conv.ID)
	if err != nil || got == nil {
		t.Fatalf("本人应能读取会话: %v", err)
	}
	other, err := repo.GetConversation(ctx, bob.ID, conv.ID)
	if err != nil {
		t.Fatalf("越权读取不应报错，只应查不到: %v", err)
	}
	if other != nil {
		t.Fatal("越权读取必须返回 nil")
	}

	if n, err := repo.CloseConversation(ctx, bob.ID, conv.ID, 2); err != nil || n != 0 {
		t.Fatalf("他人不应能关闭会话: rows=%d err=%v", n, err)
	}
	if n, err := repo.DeleteConversation(ctx, bob.ID, conv.ID); err != nil || n != 0 {
		t.Fatalf("他人不应能删除会话: rows=%d err=%v", n, err)
	}
	if n, err := repo.CloseConversation(ctx, alice.ID, conv.ID, 2); err != nil || n != 1 {
		t.Fatalf("本人关闭应成功: rows=%d err=%v", n, err)
	}
}

func TestAIRepoListConversationsIsolated(t *testing.T) {
	db := openTestDB(t)
	resetTables(t, db)
	ctx := context.Background()

	alice := mustUser(t, db, "alice_list")
	bob := mustUser(t, db, "bob_list")
	repo := repository.NewAIRepo(db)

	for i := 0; i < 3; i++ {
		if err := repo.CreateConversation(ctx, &entity.AIConversation{UserID: alice.ID, Title: "a", Status: 1}); err != nil {
			t.Fatalf("创建会话失败: %v", err)
		}
	}
	if err := repo.CreateConversation(ctx, &entity.AIConversation{UserID: bob.ID, Title: "b", Status: 1}); err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}

	list, total, err := repo.ListConversations(ctx, alice.ID, 0, 10)
	if err != nil {
		t.Fatalf("ListConversations: %v", err)
	}
	if total != 3 || len(list) != 3 {
		t.Fatalf("只能看到自己的 3 个会话，实际 total=%d len=%d", total, len(list))
	}
}

// sequence_no 必须在并发下连续且不重复：AppendMessage 用会话行锁串行化 MAX+1。
func TestAIRepoAppendMessageConcurrentSequence(t *testing.T) {
	db := openTestDB(t)
	resetTables(t, db)
	ctx := context.Background()

	u := mustUser(t, db, "seq_user")
	repo := repository.NewAIRepo(db)
	conv := &entity.AIConversation{UserID: u.ID, Title: "并发", Status: 1}
	if err := repo.CreateConversation(ctx, conv); err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}

	const n = 8
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			m := &entity.AIMessage{ConversationID: conv.ID, MessageRole: entity.RoleUser, Content: "hi"}
			if err := repo.AppendMessage(ctx, m); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("并发写入失败: %v", err)
	}

	msgs, total, err := repo.ListMessages(ctx, conv.ID, nil, 0, 50)
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if total != n || len(msgs) != n {
		t.Fatalf("消息数应为 %d，实际 total=%d len=%d", n, total, len(msgs))
	}
	for i, m := range msgs {
		if m.SequenceNo != uint32(i+1) {
			t.Fatalf("序号应为 1..%d 连续，实际第 %d 条为 %d", n, i, m.SequenceNo)
		}
	}
}
