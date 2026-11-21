package request

// ConversationCreateReq 新建会话，标题可空（取首条提问自动生成）。
type ConversationCreateReq struct {
	Title string `json:"title" binding:"omitempty,max=150"`
}

// ChatReq 提问。ConversationID 为空时自动建会话。
type ChatReq struct {
	ConversationID *uint64 `json:"conversationId"`
	Content        string  `json:"content" binding:"required,min=1,max=4000"`
	Stream         bool    `json:"stream"`
}

// MessageQuery 历史消息分页。
type MessageQuery struct {
	PageQuery
	BeforeSeq *uint32 `form:"beforeSeq"`
}
