package request

// RoleCreateReq 新建角色，roleCode 一经确定不建议修改。
type RoleCreateReq struct {
	RoleCode    string   `json:"roleCode" binding:"required,oneof=super_admin admin user"`
	RoleName    string   `json:"roleName" binding:"required,max=50"`
	Description string   `json:"description" binding:"max=255"`
	Status      *uint8   `json:"status" binding:"omitempty,oneof=0 1"`
	MenuIDs     []uint64 `json:"menuIds"`
}

// RoleUpdateReq 部分更新。
type RoleUpdateReq struct {
	RoleName    *string  `json:"roleName" binding:"omitempty,max=50"`
	Description *string  `json:"description" binding:"omitempty,max=255"`
	Status      *uint8   `json:"status" binding:"omitempty,oneof=0 1"`
	MenuIDs     []uint64 `json:"menuIds"`
}

// AssignMenusReq 菜单授权。
type AssignMenusReq struct {
	MenuIDs []uint64 `json:"menuIds" binding:"required"`
}
