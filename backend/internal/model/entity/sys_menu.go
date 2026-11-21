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
	ID         uint64    `gorm:"column:id;type:bigint unsigned;primaryKey;autoIncrement" json:"id,string"`
	ParentID   *uint64   `gorm:"column:parent_id;type:bigint unsigned;index:idx_sys_menu_parent_sort,priority:1" json:"parentId,string"`
	MenuType   uint8     `gorm:"column:menu_type;type:tinyint unsigned;not null;default:1" json:"menuType"`
	Name       string    `gorm:"column:name;type:varchar(100);not null;uniqueIndex:uk_sys_menu_name" json:"name"`
	Title      string    `gorm:"column:title;type:varchar(100);not null" json:"title"`
	Path       *string   `gorm:"column:path;type:varchar(255)" json:"path"`
	Component  *string   `gorm:"column:component;type:varchar(255)" json:"component"`
	Redirect   *string   `gorm:"column:redirect;type:varchar(255)" json:"redirect"`
	Icon       *string   `gorm:"column:icon;type:varchar(100)" json:"icon"`
	Permission *string   `gorm:"column:permission;type:varchar(100);index:idx_sys_menu_permission" json:"permission"`
	SortOrder  int       `gorm:"column:sort_order;type:int;not null;default:0;index:idx_sys_menu_parent_sort,priority:2" json:"sortOrder"`
	IsHidden   bool      `gorm:"column:is_hidden;type:tinyint(1);not null;default:0" json:"isHidden"`
	IsEnabled  bool      `gorm:"column:is_enabled;type:tinyint(1);not null;default:1" json:"isEnabled"`
	CreatedAt  time.Time `gorm:"column:created_at;type:datetime(3);not null;autoCreateTime:milli" json:"createdAt"`
	UpdatedAt  time.Time `gorm:"column:updated_at;type:datetime(3);not null;autoUpdateTime:milli" json:"updatedAt"`

	// Parent / Children 自关联，fk_sys_menu_parent 由 migration 管理（ON DELETE SET NULL）。
	Parent   *SysMenu   `gorm:"foreignKey:ParentID;references:ID;constraint:fk_sys_menu_parent,OnDelete:SET NULL,OnUpdate:CASCADE" json:"-"`
	Children []*SysMenu `gorm:"foreignKey:ParentID;references:ID;constraint:fk_sys_menu_parent,OnDelete:SET NULL,OnUpdate:CASCADE" json:"children,omitempty"`
	// Roles 拥有该菜单权限的角色。
	Roles []SysRole `gorm:"many2many:sys_role_menu;foreignKey:ID;joinForeignKey:MenuID;references:ID;joinReferences:RoleID;constraint:-" json:"-"`
}

func (SysMenu) TableName() string { return "sys_menu" }

// IsButton 按钮节点只产权限位，不出现在侧边栏。
func (m *SysMenu) IsButton() bool { return m.MenuType == MenuTypeButton }
