package service_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"gorm.io/gorm"

	"ai_health/internal/consts"
	"ai_health/internal/dto/request"
	"ai_health/internal/model/entity"
	"ai_health/internal/pkg/cache"
	"ai_health/internal/pkg/errcode"
	"ai_health/internal/pkg/hash"
	"ai_health/internal/repository"
	"ai_health/internal/service"
)

// 本文件覆盖 RBAC 阶段的核心业务：角色 CRUD、菜单树、授权 / 撤销、
// 用户角色分配、禁用角色与禁用菜单的权限语义、事务回滚。
// 依赖 auth_service_test.go 中的 openSvcDB / newStore / resetSvcTables。

func newRoleService(t *testing.T, db *gorm.DB, store *cache.Store) *service.RoleService {
	t.Helper()
	return service.NewRoleService(repository.NewRoleRepo(db), repository.NewMenuRepo(db), repository.NewUserRepo(db), store)
}

func newMenuService(t *testing.T, db *gorm.DB, store *cache.Store) *service.MenuService {
	t.Helper()
	return service.NewMenuService(repository.NewMenuRepo(db), repository.NewRoleRepo(db), repository.NewUserRepo(db), store)
}

func newUserService(t *testing.T, db *gorm.DB, store *cache.Store) *service.UserService {
	t.Helper()
	return service.NewUserService(
		newAuthService(t, db, store),
		repository.NewUserRepo(db),
		repository.NewRoleRepo(db),
		store,
	)
}

func mustRole(t *testing.T, db *gorm.DB, code, name string, status uint8) *entity.SysRole {
	t.Helper()
	role := &entity.SysRole{RoleCode: code, RoleName: name, Status: status}
	if err := repository.NewRoleRepo(db).CreateWithMenus(context.Background(), role, nil); err != nil {
		t.Fatalf("创建角色 %s 失败: %v", code, err)
	}
	return role
}

func mustMenu(t *testing.T, db *gorm.DB, name, title string, menuType uint8, permission string, parentID *uint64, enabled bool) *entity.SysMenu {
	t.Helper()
	var perm *string
	if permission != "" {
		p := permission
		perm = &p
	}
	m := &entity.SysMenu{
		ParentID:   parentID,
		MenuType:   menuType,
		Name:       name,
		Title:      title,
		Permission: perm,
		IsEnabled:  enabled,
	}
	if err := repository.NewMenuRepo(db).Create(context.Background(), m); err != nil {
		t.Fatalf("创建菜单 %s 失败: %v", name, err)
	}
	return m
}

func mustUser(t *testing.T, db *gorm.DB, username string, roleIDs ...uint64) *entity.SysUser {
	t.Helper()
	pwd, err := hash.Hash("Passw0rd!23")
	if err != nil {
		t.Fatalf("生成密码哈希失败: %v", err)
	}
	u := &entity.SysUser{Username: username, PasswordHash: &pwd, Status: consts.StatusEnabled}
	if err := repository.NewUserRepo(db).CreateWithRoles(context.Background(), u, roleIDs); err != nil {
		t.Fatalf("创建用户 %s 失败: %v", username, err)
	}
	return u
}

func ptrUint8(v uint8) *uint8    { return &v }
func ptrUint64(v uint64) *uint64 { return &v }
func ptrBool(v bool) *bool       { return &v }

func assertPerms(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("权限集合 = %v, want %v", got, want)
	}
	seen := make(map[string]bool, len(got))
	for _, p := range got {
		seen[p] = true
	}
	for _, w := range want {
		if !seen[w] {
			t.Fatalf("权限集合 = %v, 缺少 %q", got, w)
		}
	}
}

func TestRoleListSearchAndDeleteGuard(t *testing.T) {
	db := openSvcDB(t)
	resetSvcTables(t, db)
	store := newStore(t)
	seedDefaultRole(t, db)

	auditor := mustRole(t, db, "auditor", "审计员", consts.StatusEnabled)
	legacy := mustRole(t, db, "legacy", "历史角色", consts.StatusDisabled)
	svc := newRoleService(t, db, store)
	ctx := context.Background()

	all, err := svc.List(ctx, &request.RoleQuery{})
	if err != nil {
		t.Fatalf("角色列表失败: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("角色数量 = %d, want 3", len(all))
	}

	filtered, err := svc.List(ctx, &request.RoleQuery{Keyword: "audit"})
	if err != nil || len(filtered) != 1 || filtered[0].ID != auditor.ID {
		t.Fatalf("关键字筛选异常: list=%+v err=%v", filtered, err)
	}
	disabled, err := svc.List(ctx, &request.RoleQuery{Status: ptrUint8(consts.StatusDisabled)})
	if err != nil || len(disabled) != 1 || disabled[0].ID != legacy.ID {
		t.Fatalf("状态筛选异常: list=%+v err=%v", disabled, err)
	}

	if _, err := svc.Detail(ctx, 999999); !errors.Is(err, errcode.ErrNotFound) {
		t.Fatalf("不存在角色应返回 ErrNotFound，实际: %v", err)
	}

	// 系统内置角色不可删除。
	userRole, err := repository.NewRoleRepo(db).GetByCode(ctx, consts.RoleUser)
	if err != nil || userRole == nil {
		t.Fatalf("读取 user 角色失败: role=%+v err=%v", userRole, err)
	}
	if err := svc.Delete(ctx, userRole.ID); !errors.Is(err, errcode.ErrSystemRole) {
		t.Fatalf("删除内置角色应返回 ErrSystemRole，实际: %v", err)
	}

	// 被用户使用的角色不可删除。
	owner := mustUser(t, db, "role-owner", auditor.ID)
	if err := svc.Delete(ctx, auditor.ID); !errors.Is(err, errcode.ErrRoleInUse) {
		t.Fatalf("删除使用中角色应返回 ErrRoleInUse，实际: %v", err)
	}
	if _, err := svc.Detail(ctx, auditor.ID); err != nil {
		t.Fatalf("删除失败后角色应仍存在: %v", err)
	}
	if err := repository.NewUserRepo(db).ReplaceRoles(ctx, owner.ID, nil); err != nil {
		t.Fatalf("解除用户角色失败: %v", err)
	}
	if err := svc.Delete(ctx, auditor.ID); err != nil {
		t.Fatalf("角色无人使用后应可删除: %v", err)
	}
	if _, err := svc.Detail(ctx, auditor.ID); !errors.Is(err, errcode.ErrNotFound) {
		t.Fatalf("角色删除后应返回 ErrNotFound，实际: %v", err)
	}

	// 软删除用户的历史绑定不应阻塞角色删除。
	ghostRole := mustRole(t, db, "ghosted", "幽灵角色", consts.StatusEnabled)
	ghostUser := mustUser(t, db, "ghost-user", ghostRole.ID)
	if _, err := repository.NewUserRepo(db).SoftDelete(ctx, ghostUser.ID); err != nil {
		t.Fatalf("软删除用户失败: %v", err)
	}
	if err := svc.Delete(ctx, ghostRole.ID); err != nil {
		t.Fatalf("仅剩软删用户时角色应可删除: %v", err)
	}
}

func TestRoleCreateValidationAndDisabledStatus(t *testing.T) {
	db := openSvcDB(t)
	resetSvcTables(t, db)
	store := newStore(t)
	seedDefaultRole(t, db)
	svc := newRoleService(t, db, store)
	ctx := context.Background()

	if _, err := svc.Create(ctx, &request.RoleCreateReq{RoleCode: "BadCode", RoleName: "非法"}); !errors.Is(err, errcode.ErrInvalidParam) {
		t.Fatalf("非法编码应返回 ErrInvalidParam，实际: %v", err)
	}
	if _, err := svc.Create(ctx, &request.RoleCreateReq{RoleCode: "super_admin", RoleName: "超管"}); !errors.Is(err, errcode.ErrSystemRole) {
		t.Fatalf("重复创建 super_admin 应返回 ErrSystemRole，实际: %v", err)
	}

	disabled, err := svc.Create(ctx, &request.RoleCreateReq{
		RoleCode: "temp_disabled",
		RoleName: "临时禁用角色",
		Status:   ptrUint8(consts.StatusDisabled),
	})
	if err != nil {
		t.Fatalf("创建禁用角色失败: %v", err)
	}
	if disabled.Status != consts.StatusDisabled {
		t.Fatalf("禁用状态未落库: %+v", disabled)
	}
	// 禁用角色不能继续授权。
	if err := svc.AssignMenus(ctx, disabled.ID, []uint64{}); !errors.Is(err, errcode.ErrRoleDisabled) {
		t.Fatalf("禁用角色授权应返回 ErrRoleDisabled，实际: %v", err)
	}

	if _, err := svc.Create(ctx, &request.RoleCreateReq{RoleCode: "temp_disabled", RoleName: "重复"}); !errors.Is(err, errcode.ErrRoleExists) {
		t.Fatalf("重复编码应返回 ErrRoleExists，实际: %v", err)
	}
	if _, err := svc.Create(ctx, &request.RoleCreateReq{RoleCode: "with_menu", RoleName: "带菜单", MenuIDs: []uint64{999999}}); !errors.Is(err, errcode.ErrInvalidParam) {
		t.Fatalf("非法菜单应返回 ErrInvalidParam，实际: %v", err)
	}
}

func TestMenuTreeParentValidationAndCascade(t *testing.T) {
	db := openSvcDB(t)
	resetSvcTables(t, db)
	store := newStore(t)
	seedDefaultRole(t, db)
	svc := newMenuService(t, db, store)
	ctx := context.Background()

	root := mustMenu(t, db, "System", "系统管理", entity.MenuTypeDir, "", nil, true)
	child := mustMenu(t, db, "RoleManagement", "角色管理", entity.MenuTypeMenu, "role:list", &root.ID, true)
	button := mustMenu(t, db, "RoleCreate", "新增角色", entity.MenuTypeButton, "role:create", &child.ID, true)
	grand := mustMenu(t, db, "RoleDetail", "角色详情", entity.MenuTypeMenu, "role:detail", &child.ID, true)

	tree, err := svc.Tree(ctx, &request.MenuQuery{})
	if err != nil {
		t.Fatalf("菜单树失败: %v", err)
	}
	if len(tree) != 1 || tree[0].ID != root.ID {
		t.Fatalf("根节点异常: %+v", tree)
	}
	if len(tree[0].Children) != 1 || tree[0].Children[0].ID != child.ID {
		t.Fatalf("子节点异常: %+v", tree[0].Children)
	}
	if len(tree[0].Children[0].Children) != 2 {
		t.Fatalf("孙节点数量 = %d, want 2", len(tree[0].Children[0].Children))
	}

	// 按钮必须有权限位；按钮不能作为父节点。
	if _, err := svc.Create(ctx, &request.MenuCreateReq{MenuType: entity.MenuTypeButton, Name: "NoPerm", Title: "无权限按钮"}); !errors.Is(err, errcode.ErrInvalidParam) {
		t.Fatalf("按钮缺少 permission 应返回 ErrInvalidParam，实际: %v", err)
	}
	if _, err := svc.Create(ctx, &request.MenuCreateReq{ParentID: &button.ID, MenuType: entity.MenuTypeMenu, Name: "UnderButton", Title: "按钮子节点"}); !errors.Is(err, errcode.ErrMenuParent) {
		t.Fatalf("按钮作为父级应返回 ErrMenuParent，实际: %v", err)
	}
	if _, err := svc.Create(ctx, &request.MenuCreateReq{MenuType: entity.MenuTypeMenu, Name: "RoleManagement", Title: "重名"}); !errors.Is(err, errcode.ErrMenuExists) {
		t.Fatalf("重名菜单应返回 ErrMenuExists，实际: %v", err)
	}

	// 循环父级：自身、后代都拒绝。
	if _, err := svc.Update(ctx, child.ID, &request.MenuUpdateReq{ParentID: &child.ID}); !errors.Is(err, errcode.ErrMenuCycle) {
		t.Fatalf("父级为自身应返回 ErrMenuCycle，实际: %v", err)
	}
	if _, err := svc.Update(ctx, child.ID, &request.MenuUpdateReq{ParentID: &grand.ID}); !errors.Is(err, errcode.ErrMenuCycle) {
		t.Fatalf("父级为后代应返回 ErrMenuCycle，实际: %v", err)
	}
	if _, err := svc.Update(ctx, root.ID, &request.MenuUpdateReq{ParentID: &child.ID}); !errors.Is(err, errcode.ErrMenuCycle) {
		t.Fatalf("祖先挂到后代应返回 ErrMenuCycle，实际: %v", err)
	}

	// 删除仍有子节点的菜单必须拒绝。
	if err := svc.Delete(ctx, root.ID); !errors.Is(err, errcode.ErrMenuHasChildren) {
		t.Fatalf("删除有子节点菜单应返回 ErrMenuHasChildren，实际: %v", err)
	}

	// 禁用父菜单级联禁用全部后代，且不再允许在其下创建节点。
	if _, err := svc.Update(ctx, child.ID, &request.MenuUpdateReq{IsEnabled: ptrBool(false)}); err != nil {
		t.Fatalf("禁用菜单失败: %v", err)
	}
	repo := repository.NewMenuRepo(db)
	for _, id := range []uint64{child.ID, button.ID, grand.ID} {
		m, err := repo.GetByID(ctx, id)
		if err != nil || m == nil {
			t.Fatalf("读取菜单 %d 失败: %v", id, err)
		}
		if m.IsEnabled {
			t.Fatalf("菜单 %d 应被级联禁用", id)
		}
	}
	if _, err := svc.Create(ctx, &request.MenuCreateReq{ParentID: &child.ID, MenuType: entity.MenuTypeMenu, Name: "UnderDisabled", Title: "禁用父级下"}); !errors.Is(err, errcode.ErrMenuParent) {
		t.Fatalf("禁用父级下创建应返回 ErrMenuParent，实际: %v", err)
	}
	if _, err := svc.Update(ctx, grand.ID, &request.MenuUpdateReq{IsEnabled: ptrBool(true)}); !errors.Is(err, errcode.ErrMenuParent) {
		t.Fatalf("父级禁用时启用子级应返回 ErrMenuParent，实际: %v", err)
	}

	// 叶子节点可以删除。
	if err := svc.Delete(ctx, button.ID); err != nil {
		t.Fatalf("删除叶子按钮失败: %v", err)
	}
}

func TestMenuMyMenusFiltersButtons(t *testing.T) {
	db := openSvcDB(t)
	resetSvcTables(t, db)
	store := newStore(t)
	seedDefaultRole(t, db)

	role := mustRole(t, db, "viewer", "查看者", consts.StatusEnabled)
	root := mustMenu(t, db, "Health", "健康管理", entity.MenuTypeDir, "", nil, true)
	child := mustMenu(t, db, "HealthProfile", "健康档案", entity.MenuTypeMenu, "health:read", &root.ID, true)
	button := mustMenu(t, db, "HealthEdit", "编辑档案", entity.MenuTypeButton, "health:update", &child.ID, true)
	u := mustUser(t, db, "viewer-user", role.ID)
	if err := repository.NewRoleRepo(db).ReplaceMenus(context.Background(), role.ID, []uint64{root.ID, child.ID, button.ID}); err != nil {
		t.Fatalf("授权菜单失败: %v", err)
	}

	svc := newMenuService(t, db, store)
	tree, err := svc.MyMenus(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("我的菜单失败: %v", err)
	}
	if len(tree) != 1 || tree[0].ID != root.ID {
		t.Fatalf("我的菜单根节点异常: %+v", tree)
	}
	if len(tree[0].Children) != 1 || tree[0].Children[0].ID != child.ID {
		t.Fatalf("我的菜单子节点异常: %+v", tree[0].Children)
	}
	if len(tree[0].Children[0].Children) != 0 {
		t.Fatalf("按钮不应出现在前端菜单树: %+v", tree[0].Children[0].Children)
	}
}

func TestRoleAssignMenusInvalidatesPermissionCache(t *testing.T) {
	db := openSvcDB(t)
	resetSvcTables(t, db)
	store := newStore(t)
	seedDefaultRole(t, db)

	role := mustRole(t, db, "ops", "运维", consts.StatusEnabled)
	m1 := mustMenu(t, db, "OpsRead", "查看运维", entity.MenuTypeButton, "ops:read", nil, true)
	m2 := mustMenu(t, db, "OpsWrite", "编辑运维", entity.MenuTypeButton, "ops:write", nil, true)
	u := mustUser(t, db, "ops-user", role.ID)

	svc := newRoleService(t, db, store)
	auth := newAuthService(t, db, store)
	ctx := context.Background()

	if err := svc.AssignMenus(ctx, role.ID, []uint64{m1.ID}); err != nil {
		t.Fatalf("授权失败: %v", err)
	}
	perms, err := auth.Permissions(ctx, u.ID)
	if err != nil {
		t.Fatalf("读取权限失败: %v", err)
	}
	assertPerms(t, perms, "ops:read")

	cacheKey := fmt.Sprintf(consts.KeyPermissions, u.ID)
	if _, ok := store.Get(ctx, cacheKey); !ok {
		t.Fatal("权限查询后应写入缓存")
	}
	if err := svc.AssignMenus(ctx, role.ID, []uint64{m1.ID, m2.ID}); err != nil {
		t.Fatalf("重新授权失败: %v", err)
	}
	if _, ok := store.Get(ctx, cacheKey); ok {
		t.Fatal("授权变更后必须立即删除权限缓存")
	}
	perms, err = auth.Permissions(ctx, u.ID)
	if err != nil {
		t.Fatalf("读取权限失败: %v", err)
	}
	assertPerms(t, perms, "ops:read", "ops:write")

	// 撤销全部授权。
	if err := svc.AssignMenus(ctx, role.ID, []uint64{}); err != nil {
		t.Fatalf("清空授权失败: %v", err)
	}
	perms, err = auth.Permissions(ctx, u.ID)
	if err != nil {
		t.Fatalf("读取权限失败: %v", err)
	}
	assertPerms(t, perms)

	// 非法菜单 ID 不能写库，原授权保持不变。
	if err := svc.AssignMenus(ctx, role.ID, []uint64{m1.ID, 999999}); !errors.Is(err, errcode.ErrInvalidParam) {
		t.Fatalf("非法菜单应返回 ErrInvalidParam，实际: %v", err)
	}
	ids, err := svc.MenuIDs(ctx, role.ID)
	if err != nil || len(ids) != 0 {
		t.Fatalf("非法请求后授权不应变化: ids=%v err=%v", ids, err)
	}

	// 角色禁用后权限立即失效，也不能继续授权。
	if _, err := svc.Update(ctx, role.ID, &request.RoleUpdateReq{Status: ptrUint8(consts.StatusDisabled)}); err != nil {
		t.Fatalf("禁用角色失败: %v", err)
	}
	perms, err = auth.Permissions(ctx, u.ID)
	if err != nil {
		t.Fatalf("读取权限失败: %v", err)
	}
	assertPerms(t, perms)
	if err := svc.AssignMenus(ctx, role.ID, []uint64{m1.ID}); !errors.Is(err, errcode.ErrRoleDisabled) {
		t.Fatalf("禁用角色授权应返回 ErrRoleDisabled，实际: %v", err)
	}
	if _, err := svc.Update(ctx, role.ID, &request.RoleUpdateReq{Status: ptrUint8(consts.StatusDisabled)}); err != nil {
		t.Fatalf("重复禁用应幂等: %v", err)
	}

	_ = m2
}

func TestDisabledMenuExcludedFromPermissions(t *testing.T) {
	db := openSvcDB(t)
	resetSvcTables(t, db)
	store := newStore(t)
	seedDefaultRole(t, db)

	role := mustRole(t, db, "menu-scope", "菜单范围", consts.StatusEnabled)
	enabledMenu := mustMenu(t, db, "ScopeRead", "读取", entity.MenuTypeButton, "scope:read", nil, true)
	disabledMenu := mustMenu(t, db, "ScopeWrite", "写入", entity.MenuTypeButton, "scope:write", nil, false)
	u := mustUser(t, db, "scope-user", role.ID)

	if err := repository.NewRoleRepo(db).ReplaceMenus(context.Background(), role.ID, []uint64{enabledMenu.ID, disabledMenu.ID}); err != nil {
		t.Fatalf("授权失败: %v", err)
	}
	auth := newAuthService(t, db, store)
	perms, err := auth.Permissions(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("读取权限失败: %v", err)
	}
	assertPerms(t, perms, "scope:read")

	// 运行期禁用菜单后，权限缓存应失效且权限位不再生效。
	svc := newMenuService(t, db, store)
	if _, err := svc.Update(context.Background(), enabledMenu.ID, &request.MenuUpdateReq{IsEnabled: ptrBool(false)}); err != nil {
		t.Fatalf("禁用菜单失败: %v", err)
	}
	perms, err = auth.Permissions(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("读取权限失败: %v", err)
	}
	assertPerms(t, perms)
}

func TestCascadeDisabledMenuInvalidatesChildOnlyRoleCache(t *testing.T) {
	db := openSvcDB(t)
	resetSvcTables(t, db)
	store := newStore(t)
	seedDefaultRole(t, db)

	role := mustRole(t, db, "cascade-role", "级联角色", consts.StatusEnabled)
	parent := mustMenu(t, db, "CascadeParent", "级联父级", entity.MenuTypeDir, "", nil, true)
	child := mustMenu(t, db, "CascadeChild", "级联子级", entity.MenuTypeButton, "cascade:run", &parent.ID, true)
	u := mustUser(t, db, "cascade-user", role.ID)
	if err := repository.NewRoleRepo(db).ReplaceMenus(context.Background(), role.ID, []uint64{child.ID}); err != nil {
		t.Fatalf("仅授权子级失败: %v", err)
	}

	auth := newAuthService(t, db, store)
	perms, err := auth.Permissions(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("读取权限失败: %v", err)
	}
	assertPerms(t, perms, "cascade:run")

	// 角色只关联被级联禁用的子级：禁用父级后权限缓存必须立即失效。
	svc := newMenuService(t, db, store)
	if _, err := svc.Update(context.Background(), parent.ID, &request.MenuUpdateReq{IsEnabled: ptrBool(false)}); err != nil {
		t.Fatalf("禁用父级失败: %v", err)
	}
	perms, err = auth.Permissions(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("读取权限失败: %v", err)
	}
	assertPerms(t, perms)
}

func TestUserRoleAssignmentGuards(t *testing.T) {
	db := openSvcDB(t)
	resetSvcTables(t, db)
	store := newStore(t)
	seedDefaultRole(t, db)

	superRole := mustRole(t, db, consts.RoleSuperAdmin, "超级管理员", consts.StatusEnabled)
	adminRole := mustRole(t, db, consts.RoleAdmin, "管理员", consts.StatusEnabled)
	userRole, err := repository.NewRoleRepo(db).GetByCode(context.Background(), consts.RoleUser)
	if err != nil || userRole == nil {
		t.Fatalf("读取 user 角色失败: %+v %v", userRole, err)
	}
	disabledRole := mustRole(t, db, "frozen", "冻结角色", consts.StatusDisabled)

	actor := mustUser(t, db, "root", superRole.ID)
	target := mustUser(t, db, "kate", userRole.ID)
	plain := mustUser(t, db, "bob", userRole.ID)
	svc := newUserService(t, db, store)
	ctx := context.Background()

	if err := svc.AssignRoles(ctx, actor.ID, target.ID, []uint64{adminRole.ID}); err != nil {
		t.Fatalf("分配角色失败: %v", err)
	}
	roles, err := svc.Roles(ctx, target.ID)
	if err != nil || len(roles) != 1 || roles[0].ID != adminRole.ID {
		t.Fatalf("角色分配结果异常: roles=%+v err=%v", roles, err)
	}

	// 管理端资料更新不能顺带改角色（RoleIDs 被忽略）。
	nick := "Kate"
	if _, err := svc.Update(ctx, actor.ID, target.ID, &request.UserUpdateReq{Nickname: &nick, RoleIDs: []uint64{superRole.ID}}); err != nil {
		t.Fatalf("更新资料失败: %v", err)
	}
	roles, err = svc.Roles(ctx, target.ID)
	if err != nil || len(roles) != 1 || roles[0].ID != adminRole.ID {
		t.Fatalf("资料接口不应修改角色: roles=%+v err=%v", roles, err)
	}

	// 非超管不能授予 super_admin。
	if err := svc.AssignRoles(ctx, plain.ID, target.ID, []uint64{superRole.ID}); !errors.Is(err, errcode.ErrForbidden) {
		t.Fatalf("非超管授予超管应返回 ErrForbidden，实际: %v", err)
	}
	// 禁用角色不能分配。
	if err := svc.AssignRoles(ctx, actor.ID, target.ID, []uint64{disabledRole.ID}); !errors.Is(err, errcode.ErrRoleDisabled) {
		t.Fatalf("分配禁用角色应返回 ErrRoleDisabled，实际: %v", err)
	}
	// 清空角色。
	if err := svc.AssignRoles(ctx, actor.ID, target.ID, nil); err != nil {
		t.Fatalf("清空角色失败: %v", err)
	}
	roles, err = svc.Roles(ctx, target.ID)
	if err != nil || len(roles) != 0 {
		t.Fatalf("清空角色失败: roles=%+v err=%v", roles, err)
	}

	// 最后一个超管不能被降级 / 禁用 / 删除。
	if err := svc.AssignRoles(ctx, actor.ID, actor.ID, []uint64{userRole.ID}); !errors.Is(err, errcode.ErrLastSuperAdmin) {
		t.Fatalf("降级最后一个超管应返回 ErrLastSuperAdmin，实际: %v", err)
	}
	if err := svc.UpdateStatus(ctx, actor.ID, actor.ID, consts.StatusDisabled); !errors.Is(err, errcode.ErrForbidden) {
		t.Fatalf("禁用自己应返回 ErrForbidden，实际: %v", err)
	}
	if err := svc.Delete(ctx, actor.ID, actor.ID); !errors.Is(err, errcode.ErrForbidden) {
		t.Fatalf("删除自己应返回 ErrForbidden，实际: %v", err)
	}

	// 有第二个超管后，原超管可以降级。
	if err := svc.AssignRoles(ctx, actor.ID, target.ID, []uint64{superRole.ID}); err != nil {
		t.Fatalf("授予第二个超管失败: %v", err)
	}
	if err := svc.AssignRoles(ctx, actor.ID, actor.ID, []uint64{userRole.ID}); err != nil {
		t.Fatalf("存在第二个超管时应允许降级: %v", err)
	}
}

func TestUserAdminCreateUpdateStatusAndResetPassword(t *testing.T) {
	db := openSvcDB(t)
	resetSvcTables(t, db)
	store := newStore(t)
	seedDefaultRole(t, db)
	svc := newUserService(t, db, store)
	auth := newAuthService(t, db, store)
	ctx := context.Background()

	created, err := svc.Create(ctx, 0, &request.UserCreateReq{
		Username: "newbie",
		Password: "Passw0rd!23",
		Nickname: "新人",
	})
	if err != nil {
		t.Fatalf("管理员建号失败: %v", err)
	}
	if len(created.Roles) != 1 || created.Roles[0] != consts.RoleUser {
		t.Fatalf("默认角色应为 user: %+v", created.Roles)
	}

	list, total, err := svc.List(ctx, &request.UserQuery{Keyword: "newb"})
	if err != nil || total != 1 || len(list) != 1 || list[0].ID != created.ID {
		t.Fatalf("用户列表异常: list=%+v total=%d err=%v", list, total, err)
	}

	// 管理端禁用账号后，旧密码不能再登录。
	if err := svc.UpdateStatus(ctx, 0, created.ID, consts.StatusDisabled); err != nil {
		t.Fatalf("禁用账号失败: %v", err)
	}
	if _, err := auth.Login(ctx, &request.LoginReq{Account: "newbie", Password: "Passw0rd!23"}, "10.9.0.1"); !errors.Is(err, errcode.ErrUserDisabled) {
		t.Fatalf("禁用账号登录应返回 ErrUserDisabled，实际: %v", err)
	}
	if err := svc.UpdateStatus(ctx, 0, created.ID, consts.StatusEnabled); err != nil {
		t.Fatalf("启用账号失败: %v", err)
	}

	if err := svc.ResetPassword(ctx, created.ID, "Another!Pass9"); err != nil {
		t.Fatalf("重置密码失败: %v", err)
	}
	if _, err := auth.Login(ctx, &request.LoginReq{Account: "newbie", Password: "Another!Pass9"}, "10.9.0.2"); err != nil {
		t.Fatalf("新密码应能登录: %v", err)
	}

	if err := svc.Delete(ctx, 0, created.ID); err != nil {
		t.Fatalf("删除用户失败: %v", err)
	}
	if _, err := svc.Detail(ctx, created.ID); !errors.Is(err, errcode.ErrUserNotFound) {
		t.Fatalf("删除后应返回 ErrUserNotFound，实际: %v", err)
	}
}

func TestAuthorizationTransactionsRollback(t *testing.T) {
	db := openSvcDB(t)
	resetSvcTables(t, db)
	seedDefaultRole(t, db)
	ctx := context.Background()

	roleRepo := repository.NewRoleRepo(db)
	userRepo := repository.NewUserRepo(db)
	menuRepo := repository.NewMenuRepo(db)

	menu := mustMenu(t, db, "TxMenu", "事务菜单", entity.MenuTypeButton, "tx:read", nil, true)
	role := mustRole(t, db, "tx_role", "事务角色", consts.StatusEnabled)
	if err := roleRepo.ReplaceMenus(ctx, role.ID, []uint64{menu.ID}); err != nil {
		t.Fatalf("初始授权失败: %v", err)
	}

	// 授权删除 + 插入同事务：重复 menu_id 触发唯一键错误，旧的授权必须保留。
	if err := roleRepo.ReplaceMenus(ctx, role.ID, []uint64{menu.ID, menu.ID}); err == nil {
		t.Fatal("重复 menu_id 应触发唯一键错误")
	}
	ids, err := roleRepo.ListMenuIDs(ctx, role.ID)
	if err != nil {
		t.Fatalf("读取授权失败: %v", err)
	}
	if len(ids) != 1 || ids[0] != menu.ID {
		t.Fatalf("事务回滚后授权应保持原样: %v", ids)
	}

	// 更新角色 + 授权同一事务：非法菜单导致 FK 失败，角色名不能变化。
	if _, err := roleRepo.UpdateWithMenus(ctx, role.ID, map[string]any{"role_name": "被回滚的名字"}, []uint64{999999}); err == nil {
		t.Fatal("非法菜单应触发外键错误")
	}
	reloaded, err := roleRepo.GetByID(ctx, role.ID)
	if err != nil || reloaded == nil {
		t.Fatalf("读取角色失败: %v", err)
	}
	if reloaded.RoleName != "事务角色" {
		t.Fatalf("角色名被部分提交: %q", reloaded.RoleName)
	}

	// 创建角色 + 授权同一事务：失败后角色不能残留。
	ghost := &entity.SysRole{RoleCode: "ghost_role", RoleName: "幽灵角色", Status: consts.StatusEnabled}
	if err := roleRepo.CreateWithMenus(ctx, ghost, []uint64{999999}); err == nil {
		t.Fatal("创建角色绑定非法菜单应失败")
	}
	exists, err := roleRepo.ExistsCode(ctx, "ghost_role", 0)
	if err != nil || exists {
		t.Fatalf("事务回滚后角色不应存在: exists=%v err=%v", exists, err)
	}

	// 创建用户 + 绑定角色同一事务：非法角色不能留下无角色用户。
	pwd, _ := hash.Hash("Passw0rd!23")
	ghostUser := &entity.SysUser{Username: "ghost-user", PasswordHash: &pwd, Status: consts.StatusEnabled}
	if err := userRepo.CreateWithRoles(ctx, ghostUser, []uint64{999999}); err == nil {
		t.Fatal("创建用户绑定非法角色应失败")
	}
	got, err := userRepo.GetByAccount(ctx, "ghost-user")
	if err != nil || got != nil {
		t.Fatalf("事务回滚后用户不应存在: user=%+v err=%v", got, err)
	}

	_ = menuRepo
}
