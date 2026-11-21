package entity

import (
	"time"

	"gorm.io/gorm"
)

// AIConversation AI 会话。updated_at 用于会话列表倒序，软删保留审计。
type AIConversation struct {
	ID        uint64         `gorm:"column:id;primaryKey;autoIncrement" json:"id,string"`
	UserID    uint64         `gorm:"column:user_id;not null" json:"userId,string"`
	Title     string         `gorm:"column:title;type:varchar(150);not null;default:新对话" json:"title"`
	Status    uint8          `gorm:"column:status;type:tinyint unsigned;not null;default:1" json:"status"`
	CreatedAt time.Time      `gorm:"column:created_at;autoCreateTime:milli" json:"createdAt"`
	UpdatedAt time.Time      `gorm:"column:updated_at;autoUpdateTime:milli" json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`

	Messages []AIMessage `gorm:"foreignKey:ConversationID;references:ID" json:"messages,omitempty"`
}

func (AIConversation) TableName() string { return "ai_conversation" }

// DefaultConversationTitle 新建会话标题。
const DefaultConversationTitle = "新对话"
