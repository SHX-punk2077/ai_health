package entity

import (
	"time"

	"gorm.io/gorm"
)

// AIConversation AI 会话。updated_at 用于会话列表倒序，软删保留审计。
type AIConversation struct {
	ID        uint64         `gorm:"column:id;type:bigint unsigned;primaryKey;autoIncrement" json:"id,string"`
	UserID    uint64         `gorm:"column:user_id;type:bigint unsigned;not null;index:idx_ai_conversation_user_updated,priority:1" json:"userId,string"`
	Title     string         `gorm:"column:title;type:varchar(150);not null;default:新对话" json:"title"`
	Status    uint8          `gorm:"column:status;type:tinyint unsigned;not null;default:1" json:"status"`
	CreatedAt time.Time      `gorm:"column:created_at;type:datetime(3);not null;autoCreateTime:milli" json:"createdAt"`
	UpdatedAt time.Time      `gorm:"column:updated_at;type:datetime(3);not null;autoUpdateTime:milli;index:idx_ai_conversation_user_updated,priority:2,sort:desc" json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;type:datetime(3);index:idx_ai_conversation_deleted" json:"-"`

	User     *SysUser    `gorm:"foreignKey:UserID;references:ID;constraint:fk_ai_conversation_user,OnDelete:CASCADE,OnUpdate:CASCADE" json:"-"`
	Messages []AIMessage `gorm:"foreignKey:ConversationID;references:ID;constraint:fk_ai_message_conversation,OnDelete:CASCADE,OnUpdate:CASCADE" json:"messages,omitempty"`
}

func (AIConversation) TableName() string { return "ai_conversation" }

// DefaultConversationTitle 新建会话标题。
const DefaultConversationTitle = "新对话"
