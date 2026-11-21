package repository_test

import (
	"context"
	"sort"
	"testing"

	"ai_health/internal/model/entity"
	"ai_health/internal/repository"
)

func TestRoleAndMenuReposRBAC(t *testing.T) {
	db := openTestDB(t)
	resetTables(t, db)
	ctx := context.Background()

	admin := mustRole(t, db, "admin", "管理员")
	user := mustRole(t, db, "user", "普通用户")
	disabled := entity.SysRole{RoleCode: "auditor", RoleName: "审计", Status: 1}
	if err := db.Create(&disabled).Error; err != nil {
		t.Fatalf("创建禁用角色失败: %v", err)
	}
	// status 带 default:1，零值会被 GORM 跳过，禁用状态必须显式更新。
	if err := db.Model(&entity.SysRole{}).Where("id = ?", disabled.ID).Update("status", 0).Error; err != nil {
		t.Fatalf("设置角色为禁用失败: %v", err)
	}

	u := mustUser(t, db, "grace")
	for _, rid := range []uint64{admin.ID, user.ID, disabled.ID} {
		if err := db.Create(&entity.SysUserRole{UserID: u.ID, RoleID: rid}).Error; err != nil {
			t.Fatalf("绑定角色失败: %v", err)
		}
	}

	roleRepo := repository.NewRoleRepo(db)

	codes, err := roleRepo.CodesByUser(ctx, u.ID)
	if err != nil {
		t.Fatalf("CodesByUser: %v", err)
	}
	sort.Strings(codes)
	if len(codes) != 2 || codes[0] != "admin" || codes[1] != "user" {
		t.Fatalf("启用角色应为 admin/user，实际 %v", codes)
	}

	all, err := roleRepo.ListByUser(ctx, u.ID)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("管理端应看到全部 3 个角色（含禁用），实际 %d", len(all))
	}

	// 菜单与权限：禁用菜单、空权限位都不应出现在权限集合里。
	menuRepo := repository.NewMenuRepo(db)
	m1 := mustMenu(t, db, "health_write", "health:write", true)
	m2 := mustMenu(t, db, "sport_read", "sport:read", true)
	m3 := mustMenu(t, db, "sport_delete", "sport:delete", false)
	m4 := mustMenu(t, db, "dir_home", "", true)

	rows := []entity.SysRoleMenu{
		{RoleID: admin.ID, MenuID: m1.ID},
		{RoleID: admin.ID, MenuID: m3.ID},
		{RoleID: user.ID, MenuID: m2.ID},
		{RoleID: user.ID, MenuID: m4.ID},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("绑定菜单失败: %v", err)
	}

	perms, err := menuRepo.PermissionNames(ctx, u.ID)
	if err != nil {
		t.Fatalf("PermissionNames: %v", err)
	}
	sort.Strings(perms)
	if len(perms) != 2 || perms[0] != "health:write" || perms[1] != "sport:read" {
		t.Fatalf("权限集合错误（应排除禁用菜单与空权限）: %v", perms)
	}

	menus, err := menuRepo.ListByUser(ctx, u.ID)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(menus) != 3 {
		t.Fatalf("可见菜单应为 3 个（含无权限位目录，排除禁用项），实际 %d", len(menus))
	}

	// 全量覆盖授权：先授权两个，再清空。
	if err := roleRepo.ReplaceMenus(ctx, user.ID, []uint64{m1.ID, m2.ID}); err != nil {
		t.Fatalf("ReplaceMenus: %v", err)
	}
	ids, err := roleRepo.ListMenuIDs(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListMenuIDs: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("期望 2 条授权，实际 %d", len(ids))
	}
	if err := roleRepo.ReplaceMenus(ctx, user.ID, nil); err != nil {
		t.Fatalf("清空授权失败: %v", err)
	}
	ids, err = roleRepo.ListMenuIDs(ctx, user.ID)
	if err != nil || len(ids) != 0 {
		t.Fatalf("清空后应为空: ids=%v err=%v", ids, err)
	}
}

func TestMenuRepoListAllAndGet(t *testing.T) {
	db := openTestDB(t)
	resetTables(t, db)
	ctx := context.Background()

	menuRepo := repository.NewMenuRepo(db)
	enabled := mustMenu(t, db, "enabled_menu", "a:read", true)
	mustMenu(t, db, "disabled_menu", "a:write", false)

	all, err := menuRepo.ListAll(ctx, false)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("全量应为 2 个，实际 %d", len(all))
	}
	on, err := menuRepo.ListAll(ctx, true)
	if err != nil {
		t.Fatalf("ListAll(enabled): %v", err)
	}
	if len(on) != 1 || on[0].ID != enabled.ID {
		t.Fatalf("仅启用项应为 1 个: %+v", on)
	}

	got, err := menuRepo.GetByID(ctx, enabled.ID)
	if err != nil || got == nil {
		t.Fatalf("GetByID: %v", err)
	}
	missing, err := menuRepo.GetByID(ctx, 999999)
	if err != nil || missing != nil {
		t.Fatalf("不存在应返回 (nil, nil): %v %v", missing, err)
	}
}
