package response

import (
	"time"

	"ai_health/internal/model/entity"
)

// Conversation 会话列表项。
type Conversation struct {
	ID         uint64    `json:"id,string"`
	Title      string    `json:"title"`
	Status     uint8     `json:"status"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
	LastMsg    string    `json:"lastMessage,omitempty"`
	MessageCnt int64     `json:"messageCount,omitempty"`
}

// Message 消息项。
type Message struct {
	entity.AIMessage
}

// Chat 一次问答的落库结果。
type Chat struct {
	ConversationID uint64 `json:"conversationId,string"`
	MessageID      uint64 `json:"messageId,string"`
	Content        string `json:"content"`
	Model          string `json:"model"`
	TokenCount     int    `json:"tokenCount"`
}
