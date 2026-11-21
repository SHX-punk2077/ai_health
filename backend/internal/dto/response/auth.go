// Package response 定义出参结构，控制对外暴露字段。
package response

// Token 双令牌。
type Token struct {
	AccessToken      string `json:"accessToken"`
	RefreshToken     string `json:"refreshToken"`
	TokenType        string `json:"tokenType"`
	ExpiresIn        int64  `json:"expiresIn"`
	RefreshExpiresIn int64  `json:"refreshExpiresIn"`
}

// UserInfo 当前登录用户。
type UserInfo struct {
	ID          uint64   `json:"id,string"`
	Username    string   `json:"username"`
	Nickname    string   `json:"nickname"`
	Avatar      string   `json:"avatar"`
	Email       string   `json:"email"`
	Phone       string   `json:"phone"`
	Status      uint8    `json:"status"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
}

// Login 登录响应。
type Login struct {
	Token
	User *UserInfo `json:"user"`
}
