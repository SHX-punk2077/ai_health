package entity

import "time"

// 菜单类型：1 目录，2 菜单，3 按钮。
const (
	MenuTypeDir    uint8 = 1
	MenuTypeMenu   uint8 = 2
	MenuTypeButton uint8 = 3
)

// SysMenu 菜单 / 按钮权限，parent_id 自关联成树。
type SysMenu struct {
	ID         uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id,string"`
	ParentID   *uint64   `gorm:"column:parent_id" json:"parentId,string"`
	MenuType   uint8     `gorm:"column:menu_type;type:tinyint unsigned;not null;default:1" json:"menuType"`
	Name       string    `gorm:"column:name;type:varchar(100);not null" json:"name"`
	Title      string    `gorm:"column:title;type:varchar(100);not null" json:"title"`
	Path       *string   `gorm:"column:path;type:varchar(255)" json:"path"`
	Component  *string   `gorm:"column:component;type:varchar(255)" json:"component"`
	Redirect   *string   `gorm:"column:redirect;type:varchar(255)" json:"redirect"`
	Icon       *string   `gorm:"column:icon;type:varchar(100)" json:"icon"`
	Permission *string   `gorm:"column:permission;type:varchar(100)" json:"permission"`
	SortOrder  int       `gorm:"column:sort_order;not null;default:0" json:"sortOrder"`
	IsHidden   uint8     `gorm:"column:is_hidden;type:tinyint(1);not null;default:0" json:"isHidden"`
	IsEnabled  uint8     `gorm:"column:is_enabled;type:tinyint(1);not null;default:1" json:"isEnabled"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime:milli" json:"createdAt"`
	UpdatedAt  time.Time `gorm:"column:updated_at;autoUpdateTime:milli" json:"updatedAt"`

	Children []*SysMenu `gorm:"-" json:"children,omitempty"`
}

func (SysMenu) TableName() string { return "sys_menu" }

// IsButton 按钮节点只产权限位，不出现在侧边栏。
func (m *SysMenu) IsButton() bool { return m.MenuType == MenuTypeButton }
