package request

// UserCreateReq 管理员建号入参。
type UserCreateReq struct {
	Username string   `json:"username" binding:"required,min=3,max=50"`
	Password string   `json:"password" binding:"required,min=8,max=64"`
	Nickname string   `json:"nickname" binding:"max=50"`
	Email    string   `json:"email" binding:"omitempty,email,max=100"`
	Phone    string   `json:"phone" binding:"omitempty,min=6,max=20"`
	Status   *uint8   `json:"status" binding:"omitempty,oneof=0 1"`
	RoleIDs  []uint64 `json:"roleIds"`
}

// UserUpdateReq 部分更新，指针字段为 nil 表示不修改。
type UserUpdateReq struct {
	Nickname *string  `json:"nickname" binding:"omitempty,max=50"`
	Email    *string  `json:"email" binding:"omitempty,email,max=100"`
	Phone    *string  `json:"phone" binding:"omitempty,min=6,max=20"`
	Avatar   *string  `json:"avatar" binding:"omitempty,max=500"`
	Status   *uint8   `json:"status" binding:"omitempty,oneof=0 1"`
	RoleIDs  []uint64 `json:"roleIds"`
}

// UserQuery 列表筛选。
type UserQuery struct {
	PageQuery
	Keyword string  `form:"keyword" binding:"max=50"`
	Status  *uint8  `form:"status"`
	RoleID  *uint64 `form:"roleId"`
}

// PasswordReq 改密入参。
type PasswordReq struct {
	OldPassword string `json:"oldPassword" binding:"required,min=8,max=64"`
	NewPassword string `json:"newPassword" binding:"required,min=8,max=64"`
}

// ResetPasswordReq 管理员重置。
type ResetPasswordReq struct {
	NewPassword string `json:"newPassword" binding:"required,min=8,max=64"`
}

// AssignRolesReq 角色分配。
type AssignRolesReq struct {
	RoleIDs []uint64 `json:"roleIds" binding:"required"`
}
