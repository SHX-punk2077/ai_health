package request

// RegisterReq 注册入参。手机号/邮箱可选，但填了必须唯一。
type RegisterReq struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Password string `json:"password" binding:"required,min=8,max=64"`
	Nickname string `json:"nickname" binding:"max=50"`
	Email    string `json:"email" binding:"omitempty,email,max=100"`
	Phone    string `json:"phone" binding:"omitempty,min=6,max=20"`
}

// LoginReq 登录入参，支持用户名 / 邮箱 / 手机号任一。
type LoginReq struct {
	Account  string `json:"account" binding:"required,min=3,max=100"`
	Password string `json:"password" binding:"required,min=8,max=64"`
}

// RefreshReq 刷新令牌入参。
type RefreshReq struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}
