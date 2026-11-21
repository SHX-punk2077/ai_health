package request

// MenuCreateReq 菜单/按钮节点。parentId 为空或 0 表示根节点。
type MenuCreateReq struct {
	ParentID   *uint64 `json:"parentId"`
	MenuType   uint8   `json:"menuType" binding:"required,oneof=1 2 3"`
	Name       string  `json:"name" binding:"required,max=100"`
	Title      string  `json:"title" binding:"required,max=100"`
	Path       string  `json:"path" binding:"max=255"`
	Component  string  `json:"component" binding:"max=255"`
	Redirect   string  `json:"redirect" binding:"max=255"`
	Icon       string  `json:"icon" binding:"max=100"`
	Permission string  `json:"permission" binding:"max=100"`
	SortOrder  int     `json:"sortOrder"`
	IsHidden   *bool   `json:"isHidden"`
	IsEnabled  *bool   `json:"isEnabled"`
}

// MenuUpdateReq 部分更新。parentId=0 表示移动到根节点，未传表示不改父级。
type MenuUpdateReq struct {
	ParentID   *uint64 `json:"parentId"`
	MenuType   *uint8  `json:"menuType" binding:"omitempty,oneof=1 2 3"`
	Name       *string `json:"name" binding:"omitempty,max=100"`
	Title      *string `json:"title" binding:"omitempty,max=100"`
	Path       *string `json:"path" binding:"omitempty,max=255"`
	Component  *string `json:"component" binding:"omitempty,max=255"`
	Redirect   *string `json:"redirect" binding:"omitempty,max=255"`
	Icon       *string `json:"icon" binding:"omitempty,max=100"`
	Permission *string `json:"permission" binding:"omitempty,max=100"`
	SortOrder  *int    `json:"sortOrder"`
	IsHidden   *bool   `json:"isHidden"`
	IsEnabled  *bool   `json:"isEnabled"`
}

// MenuQuery 菜单列表筛选。
type MenuQuery struct {
	OnlyEnabled bool `form:"onlyEnabled"`
}
