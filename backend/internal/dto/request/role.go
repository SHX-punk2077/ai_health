package request

// RoleCreateReq 新建角色。roleCode 是权限判定与种子数据依赖的稳定标识，创建后不可修改。
type RoleCreateReq struct {
	RoleCode    string   `json:"roleCode" binding:"required,max=50"`
	RoleName    string   `json:"roleName" binding:"required,max=50"`
	Description string   `json:"description" binding:"max=255"`
	Status      *uint8   `json:"status" binding:"omitempty,oneof=0 1"`
	MenuIDs     []uint64 `json:"menuIds"`
}

// RoleUpdateReq 部分更新；roleCode 不允许修改。
// MenuIDs 为 nil 表示不动授权，传 [] 表示清空授权，由独立事务完成。
type RoleUpdateReq struct {
	RoleName    *string   `json:"roleName" binding:"omitempty,max=50"`
	Description *string   `json:"description" binding:"omitempty,max=255"`
	Status      *uint8    `json:"status" binding:"omitempty,oneof=0 1"`
	MenuIDs     *[]uint64 `json:"menuIds"`
}

// RoleQuery 角色列表筛选。
type RoleQuery struct {
	Keyword string `form:"keyword" binding:"max=50"`
	Status  *uint8 `form:"status"`
}

// AssignMenusReq 角色菜单授权；菜单 ID 由后端查库校验，不信任前端。
type AssignMenusReq struct {
	MenuIDs *[]uint64 `json:"menuIds" binding:"required"`
}
