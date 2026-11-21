package entity

import "time"

// 消息角色，与 Chat Completions 协议的 role 对齐（DeepSeek / 通义千问 / GLM / Kimi 均兼容）。
const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

// AIMessage 会话消息，conversation_id+sequence_no 唯一，保证顺序可重放。
type AIMessage struct {
	ID             uint64    `gorm:"column:id;type:bigint unsigned;primaryKey;autoIncrement" json:"id,string"`
	ConversationID uint64    `gorm:"column:conversation_id;type:bigint unsigned;not null;uniqueIndex:uk_ai_message_conversation_sequence,priority:1;index:idx_ai_message_conversation_created,priority:1" json:"conversationId,string"`
	MessageRole    string    `gorm:"column:message_role;type:varchar(20);not null" json:"messageRole"`
	Content        string    `gorm:"column:content;type:mediumtext;not null" json:"content"`
	ModelName      *string   `gorm:"column:model_name;type:varchar(100)" json:"modelName"`
	TokenCount     *uint32   `gorm:"column:token_count;type:int unsigned" json:"tokenCount"`
	SequenceNo     uint32    `gorm:"column:sequence_no;type:int unsigned;not null;uniqueIndex:uk_ai_message_conversation_sequence,priority:2" json:"sequenceNo"`
	CreatedAt      time.Time `gorm:"column:created_at;type:datetime(3);not null;autoCreateTime:milli;index:idx_ai_message_conversation_created,priority:2" json:"createdAt"`

	Conversation *AIConversation `gorm:"foreignKey:ConversationID;references:ID;constraint:fk_ai_message_conversation,OnDelete:CASCADE,OnUpdate:CASCADE" json:"-"`
}

func (AIMessage) TableName() string { return "ai_message" }
