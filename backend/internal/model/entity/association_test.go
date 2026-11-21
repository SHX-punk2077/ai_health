package entity_test

import (
	"testing"
	"time"

	"gorm.io/gorm"

	"ai_health/internal/model/entity"
)

func TestUserRoleManyToMany(t *testing.T) {
	db := openTestDB(t)

	user := createUser(t, db, "henry")
	admin := createRole(t, db, "admin")
	editor := createRole(t, db, "editor")

	if err := db.Model(user).Association("Roles").Append(admin, editor); err != nil {
		t.Fatalf("追加用户角色失败: %v", err)
	}
	assertCount(t, db, &entity.SysUserRole{}, 2)
	if got := countRows(t, db, &entity.SysRole{}); got != 2 {
		t.Fatalf("追加角色不应删除角色本身, role 数 = %d", got)
	}

	var loaded entity.SysUser
	if err := db.Preload("Roles", func(db *gorm.DB) *gorm.DB {
		return db.Order("sys_role.id ASC")
	}).First(&loaded, user.ID).Error; err != nil {
		t.Fatalf("预加载用户角色失败: %v", err)
	}
	if len(loaded.Roles) != 2 || loaded.Roles[0].ID != admin.ID || loaded.Roles[1].ID != editor.ID {
		t.Fatalf("SysUser.Roles = %+v", loaded.Roles)
	}

	var loadedRole entity.SysRole
	if err := db.Preload("Users").First(&loadedRole, admin.ID).Error; err != nil {
		t.Fatalf("预加载角色用户失败: %v", err)
	}
	if len(loadedRole.Users) != 1 || loadedRole.Users[0].ID != user.ID {
		t.Fatalf("SysRole.Users = %+v", loadedRole.Users)
	}

	if err := db.Model(user).Association("Roles").Delete(editor); err != nil {
		t.Fatalf("删除用户角色关联失败: %v", err)
	}
	assertCount(t, db, &entity.SysUserRole{}, 1)
	assertCount(t, db, &entity.SysRole{}, 2)

	count := db.Model(user).Association("Roles").Count()
	if count != 1 {
		t.Fatalf("Roles.Count = %d, want 1", count)
	}

	if err := db.Model(user).Association("Roles").Clear(); err != nil {
		t.Fatalf("清空用户角色失败: %v", err)
	}
	assertCount(t, db, &entity.SysUserRole{}, 0)
	assertCount(t, db, &entity.SysRole{}, 2)

	// 直接写中间表后删除角色，应级联删除关联行。
	mustCreate(t, db, &entity.SysUserRole{UserID: user.ID, RoleID: admin.ID})
	if err := db.Delete(admin).Error; err != nil {
		t.Fatalf("删除角色失败: %v", err)
	}
	assertCount(t, db, &entity.SysUserRole{}, 0)
	assertCount(t, db, &entity.SysRole{}, 1)
}

func TestRoleMenuManyToMany(t *testing.T) {
	db := openTestDB(t)

	role := createRole(t, db, "ops")
	root := createMenu(t, db, "ops", nil, entity.MenuTypeDir)
	child := createMenu(t, db, "ops:server", &root.ID, entity.MenuTypeMenu)

	if err := db.Model(role).Association("Menus").Append(root, child); err != nil {
		t.Fatalf("追加角色菜单失败: %v", err)
	}
	assertCount(t, db, &entity.SysRoleMenu{}, 2)

	var loaded entity.SysRole
	if err := db.Preload("Menus", func(db *gorm.DB) *gorm.DB {
		return db.Order("sys_menu.id ASC")
	}).First(&loaded, role.ID).Error; err != nil {
		t.Fatalf("预加载角色菜单失败: %v", err)
	}
	if len(loaded.Menus) != 2 || loaded.Menus[0].ID != root.ID || loaded.Menus[1].ID != child.ID {
		t.Fatalf("SysRole.Menus = %+v", loaded.Menus)
	}

	var loadedMenu entity.SysMenu
	if err := db.Preload("Roles").First(&loadedMenu, child.ID).Error; err != nil {
		t.Fatalf("预加载菜单角色失败: %v", err)
	}
	if len(loadedMenu.Roles) != 1 || loadedMenu.Roles[0].ID != role.ID {
		t.Fatalf("SysMenu.Roles = %+v", loadedMenu.Roles)
	}

	if err := db.Model(role).Association("Menus").Delete(child); err != nil {
		t.Fatalf("删除角色菜单关联失败: %v", err)
	}
	assertCount(t, db, &entity.SysRoleMenu{}, 1)
	assertCount(t, db, &entity.SysMenu{}, 2)

	count := db.Model(role).Association("Menus").Count()
	if count != 1 {
		t.Fatalf("Menus.Count = %d, want 1", count)
	}

	// 删除菜单应级联删除 sys_role_menu。
	if err := db.Delete(root).Error; err != nil {
		t.Fatalf("删除菜单失败: %v", err)
	}
	assertCount(t, db, &entity.SysRoleMenu{}, 0)

	// 菜单自关联：删除父菜单后子菜单 parent_id 置 NULL。
	mustCreate(t, db, &entity.SysRoleMenu{RoleID: role.ID, MenuID: child.ID})
	if err := db.Delete(child).Error; err != nil {
		t.Fatalf("删除子菜单失败: %v", err)
	}
	assertCount(t, db, &entity.SysRoleMenu{}, 0)
}

func TestForeignKeyConstraints(t *testing.T) {
	db := openTestDB(t)

	user := createUser(t, db, "ivan")
	role := createRole(t, db, "viewer")
	menu := createMenu(t, db, "viewer:read", nil, entity.MenuTypeButton)
	sportType := createSportType(t, db, "swimming")

	const missingID uint64 = 1 << 60

	cases := []struct {
		name  string
		model any
	}{
		{"oauth 用户不存在", &entity.SysOauthIdentity{UserID: missingID, Provider: "wechat", OpenID: "openid-missing"}},
		{"健康档案用户不存在", &entity.HealthProfile{UserID: missingID}},
		{"健康记录用户不存在", &entity.HealthRecord{UserID: missingID, MeasuredAt: time.Now().UTC(), Source: entity.SourceManual}},
		{"运动详情类型不存在", &entity.SportInfo{SportTypeID: missingID, Status: 1}},
		{"运动疾病类型不存在", &entity.SportDiseaseDetail{SportTypeID: missingID, DiseaseName: "x"}},
		{"AI 会话用户不存在", &entity.AIConversation{UserID: missingID, Title: "x", Status: 1}},
		{"AI 消息会话不存在", &entity.AIMessage{ConversationID: missingID, MessageRole: entity.RoleUser, Content: "x", SequenceNo: 1}},
		{"用户角色-用户不存在", &entity.SysUserRole{UserID: missingID, RoleID: role.ID}},
		{"用户角色-角色不存在", &entity.SysUserRole{UserID: user.ID, RoleID: missingID}},
		{"角色菜单-角色不存在", &entity.SysRoleMenu{RoleID: missingID, MenuID: menu.ID}},
		{"角色菜单-菜单不存在", &entity.SysRoleMenu{RoleID: role.ID, MenuID: missingID}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := db.Create(tc.model).Error
			if code := mysqlErrCode(err); code != 1452 {
				t.Fatalf("外键错误码 = %d, want 1452 (err=%v)", code, err)
			}
		})
	}

	info := &entity.SportInfo{SportTypeID: sportType.ID, Status: 1}
	mustCreate(t, db, info)
	disease := &entity.SportDiseaseDetail{SportTypeID: sportType.ID, DiseaseName: "哮喘"}
	mustCreate(t, db, disease)

	if err := db.Delete(sportType).Error; mysqlErrCode(err) != 1451 {
		t.Fatalf("被引用的运动类型应 RESTRICT(1451), got code=%d err=%v", mysqlErrCode(err), err)
	}
	assertCount(t, db, &entity.SportType{}, 1)

	if err := db.Delete(info).Error; err != nil {
		t.Fatalf("删除运动详情失败: %v", err)
	}
	if err := db.Delete(sportType).Error; mysqlErrCode(err) != 1451 {
		t.Fatalf("仍有疾病详情引用时应 RESTRICT(1451), got code=%d err=%v", mysqlErrCode(err), err)
	}
	if err := db.Delete(disease).Error; err != nil {
		t.Fatalf("删除疾病详情失败: %v", err)
	}
	if err := db.Delete(sportType).Error; err != nil {
		t.Fatalf("删除运动类型失败: %v", err)
	}
	assertCount(t, db, &entity.SportType{}, 0)

	// sys_menu.parent_id ON DELETE SET NULL。
	parent := createMenu(t, db, "parent", nil, entity.MenuTypeDir)
	child := createMenu(t, db, "child", &parent.ID, entity.MenuTypeMenu)
	if err := db.Delete(parent).Error; err != nil {
		t.Fatalf("删除父菜单失败: %v", err)
	}
	var loadedChild entity.SysMenu
	if err := db.First(&loadedChild, child.ID).Error; err != nil {
		t.Fatalf("读取子菜单失败: %v", err)
	}
	if loadedChild.ParentID != nil {
		t.Fatalf("子菜单 parent_id 应置 NULL, got %v", *loadedChild.ParentID)
	}

	// 删除角色级联清理 sys_user_role / sys_role_menu。
	extraRole := createRole(t, db, "temp")
	mustCreate(t, db, &entity.SysUserRole{UserID: user.ID, RoleID: extraRole.ID})
	mustCreate(t, db, &entity.SysRoleMenu{RoleID: extraRole.ID, MenuID: menu.ID})
	if err := db.Delete(extraRole).Error; err != nil {
		t.Fatalf("删除临时角色失败: %v", err)
	}
	assertCount(t, db, &entity.SysUserRole{}, 0)
	assertCount(t, db, &entity.SysRoleMenu{}, 0)
}

func TestManyToManyAssociationLifecycle(t *testing.T) {
	db := openTestDB(t)

	user := createUser(t, db, "judy")
	admin := createRole(t, db, "admin")
	editor := createRole(t, db, "editor")
	viewer := createRole(t, db, "viewer")

	if err := db.Model(user).Association("Roles").Append(admin, editor); err != nil {
		t.Fatalf("Append 失败: %v", err)
	}
	var roles []entity.SysRole
	if err := db.Model(user).Association("Roles").Find(&roles); err != nil {
		t.Fatalf("Association Find 失败: %v", err)
	}
	if len(roles) != 2 {
		t.Fatalf("Association Find 数量 = %d, want 2", len(roles))
	}

	if err := db.Model(user).Association("Roles").Replace(editor, viewer); err != nil {
		t.Fatalf("Replace 失败: %v", err)
	}
	count := db.Model(user).Association("Roles").Count()
	if count != 2 {
		t.Fatalf("Replace 后 Count = %d, want 2", count)
	}
	assertCount(t, db, &entity.SysUserRole{}, 2)

	if err := db.Model(user).Association("Roles").Delete(editor, viewer); err != nil {
		t.Fatalf("Delete 失败: %v", err)
	}
	count = db.Model(user).Association("Roles").Count()
	if count != 0 {
		t.Fatalf("Delete 后 Count = %d, want 0", count)
	}
	assertCount(t, db, &entity.SysUserRole{}, 0)

	if err := db.Model(admin).Association("Users").Append(user); err != nil {
		t.Fatalf("反向 Append 失败: %v", err)
	}
	count = db.Model(admin).Association("Users").Count()
	if count != 1 {
		t.Fatalf("反向 Users.Count = %d, want 1", count)
	}
	if err := db.Model(admin).Association("Users").Clear(); err != nil {
		t.Fatalf("反向 Clear 失败: %v", err)
	}
	assertCount(t, db, &entity.SysUserRole{}, 0)

	root := createMenu(t, db, "dashboard", nil, entity.MenuTypeDir)
	if err := db.Model(viewer).Association("Menus").Append(root); err != nil {
		t.Fatalf("角色追加菜单失败: %v", err)
	}
	if err := db.Model(viewer).Association("Menus").Clear(); err != nil {
		t.Fatalf("角色清空菜单失败: %v", err)
	}
	assertCount(t, db, &entity.SysRoleMenu{}, 0)
}
