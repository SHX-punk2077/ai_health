package response

import "time"

// UserItem 用户列表项（不含密码字段）。
type UserItem struct {
	ID          uint64     `json:"id,string"`
	Username    string     `json:"username"`
	Nickname    string     `json:"nickname"`
	Email       string     `json:"email"`
	Phone       string     `json:"phone"`
	Avatar      string     `json:"avatar"`
	Status      uint8      `json:"status"`
	LastLoginAt *time.Time `json:"lastLoginAt"`
	CreatedAt   time.Time  `json:"createdAt"`
	Roles       []string   `json:"roles"`
}

// MenuNode 前端路由树节点。
type MenuNode struct {
	ID         uint64      `json:"id,string"`
	ParentID   *uint64     `json:"parentId,string"`
	MenuType   uint8       `json:"menuType"`
	Name       string      `json:"name"`
	Title      string      `json:"title"`
	Path       string      `json:"path"`
	Component  string      `json:"component"`
	Redirect   string      `json:"redirect"`
	Icon       string      `json:"icon"`
	Permission string      `json:"permission"`
	SortOrder  int         `json:"sortOrder"`
	IsHidden   bool        `json:"isHidden"`
	IsEnabled  bool        `json:"isEnabled"`
	Children   []*MenuNode `json:"children,omitempty"`
}
