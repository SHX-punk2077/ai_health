package seed

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	glogger "gorm.io/gorm/logger"

	"ai_health/internal/consts"
	"ai_health/internal/model/entity"
	"ai_health/internal/pkg/hash"
)

var seedTestTables = []string{
	"sys_user",
	"sys_user_role",
	"sys_role",
	"sys_role_menu",
	"sys_menu",
	"sys_oauth_identity",
	"health_profile",
	"health_record",
	"sport_type",
	"sport_info",
	"sport_disease_detail",
	"ai_conversation",
	"ai_message",
}

func openSeedTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("AIH_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("未设置 AIH_TEST_MYSQL_DSN，跳过 Seed 数据库集成测试")
	}
	cfg, err := mysqlDriver.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("解析 AIH_TEST_MYSQL_DSN 失败: %v", err)
	}
	if !strings.HasSuffix(cfg.DBName, "_test") {
		t.Fatalf("Seed 集成测试只允许 _test 库，实际 %q", cfg.DBName)
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		SkipDefaultTransaction: true,
		Logger:                 glogger.Default.LogMode(glogger.Silent),
		NowFunc:                func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		t.Fatalf("连接测试库失败: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("获取测试库连接失败: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func resetSeedTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	err := db.Connection(func(tx *gorm.DB) error {
		if err := tx.Exec("SET FOREIGN_KEY_CHECKS = 0").Error; err != nil {
			return err
		}
		for _, table := range seedTestTables {
			if err := tx.Exec("TRUNCATE TABLE " + table).Error; err != nil {
				return err
			}
		}
		return tx.Exec("SET FOREIGN_KEY_CHECKS = 1").Error
	})
	if err != nil {
		t.Fatalf("清理测试库失败: %v", err)
	}
}

func seedRole(t *testing.T, db *gorm.DB, code, name string) entity.SysRole {
	t.Helper()
	role := entity.SysRole{RoleCode: code, RoleName: name, Status: consts.StatusEnabled}
	if err := db.Create(&role).Error; err != nil {
		t.Fatalf("创建角色 %s 失败: %v", code, err)
	}
	return role
}

func seedUser(t *testing.T, db *gorm.DB, username, nickname, password string, role entity.SysRole) entity.SysUser {
	t.Helper()
	pwd, err := hash.Hash(password)
	if err != nil {
		t.Fatalf("生成密码哈希失败: %v", err)
	}
	u := entity.SysUser{Username: username, PasswordHash: &pwd, Nickname: &nickname, Status: consts.StatusEnabled}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("创建用户 %s 失败: %v", username, err)
	}
	if err := db.Exec("INSERT INTO sys_user_role (user_id, role_id) VALUES (?, ?)", u.ID, role.ID).Error; err != nil {
		t.Fatalf("绑定用户角色失败: %v", err)
	}
	return u
}

func countRows(t *testing.T, db *gorm.DB, model any, query string, args ...any) int64 {
	t.Helper()
	var count int64
	if err := db.Model(model).Where(query, args...).Count(&count).Error; err != nil {
		t.Fatalf("统计 %T 失败: %v", model, err)
	}
	return count
}

func loadDemoUsers(t *testing.T, db *gorm.DB) map[string]entity.SysUser {
	t.Helper()
	var users []entity.SysUser
	if err := db.Where("username IN ?", []string{"demo_user_a", "demo_user_b", "demo_user_c"}).Find(&users).Error; err != nil {
		t.Fatalf("读取演示用户失败: %v", err)
	}
	out := make(map[string]entity.SysUser, len(users))
	for _, u := range users {
		out[u.Username] = u
	}
	return out
}

func TestSeedApplyCreatesIsolatedDemoDataAndIsIdempotent(t *testing.T) {
	db := openSeedTestDB(t)
	resetSeedTables(t, db)
	ctx := context.Background()
	anchor := time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC)
	plan := BuildPlan(anchor, 42)
	wantRecords := 0
	for _, u := range plan.Users {
		wantRecords += len(u.Records)
	}

	stats, err := apply(ctx, db, plan, true)
	if err != nil {
		t.Fatalf("首次 Apply 失败: %v", err)
	}
	if stats.Users != 3 || stats.Profiles != 2 || stats.Records != wantRecords {
		t.Fatalf("首次 Apply 统计错误: %+v，期望用户 3、档案 2、记录 %d", stats, wantRecords)
	}

	users := loadDemoUsers(t, db)
	if len(users) != 3 {
		t.Fatalf("演示用户数量 = %d，期望 3", len(users))
	}
	ids := make([]uint64, 0, 3)
	for _, name := range []string{"demo_user_a", "demo_user_b", "demo_user_c"} {
		u, ok := users[name]
		if !ok {
			t.Fatalf("缺少演示用户 %s", name)
		}
		ids = append(ids, u.ID)
		if u.Status != consts.StatusEnabled || u.DeletedAt.Valid {
			t.Fatalf("%s 状态异常: %+v", name, u)
		}
		if u.PasswordHash == nil || !hash.Verify(*u.PasswordHash, DemoPassword) {
			t.Fatalf("%s 的演示密码不正确", name)
		}
		roles, err := rolesOfUser(db, u.ID)
		if err != nil {
			t.Fatalf("读取 %s 角色失败: %v", name, err)
		}
		if len(roles) != 1 || roles[0].RoleCode != demoSeedRoleCode {
			t.Fatalf("%s 必须只绑定 %s 角色，实际: %+v", name, demoSeedRoleCode, roles)
		}
	}

	var role entity.SysRole
	if err := db.Where("role_code = ?", demoSeedRoleCode).First(&role).Error; err != nil {
		t.Fatalf("缺少演示数据专用角色: %v", err)
	}
	if role.Status != consts.StatusDisabled {
		t.Fatalf("演示数据专用角色必须禁用，实际 status=%d", role.Status)
	}
	if got := countRows(t, db, &entity.SysRoleMenu{}, "role_id = ?", role.ID); got != 0 {
		t.Fatalf("演示数据专用角色不能绑定菜单，实际 %d 个", got)
	}

	if got := countRows(t, db, &entity.HealthProfile{}, "1 = 1"); got != 2 {
		t.Fatalf("健康档案数量 = %d，期望 2", got)
	}
	if got := countRows(t, db, &entity.HealthRecord{}, "user_id IN ?", ids); got != int64(wantRecords) {
		t.Fatalf("演示健康记录数量 = %d，期望 %d", got, wantRecords)
	}
	if got := countRows(t, db, &entity.HealthProfile{}, "user_id = ?", users["demo_user_c"].ID); got != 0 {
		t.Fatalf("demo_user_c 不应有健康档案，实际 %d", got)
	}
	if got := countRows(t, db, &entity.HealthRecord{}, "user_id NOT IN ?", ids); got != 0 {
		t.Fatalf("不应写入非演示用户的健康记录，实际 %d 条", got)
	}
	if got := countRows(t, db, &entity.SysUserRole{}, "1 = 1"); got != 3 {
		t.Fatalf("演示用户角色绑定数量 = %d，期望 3", got)
	}

	var beforeA entity.HealthProfile
	if err := db.Where("user_id = ?", users["demo_user_a"].ID).First(&beforeA).Error; err != nil {
		t.Fatalf("读取 demo_user_a 档案失败: %v", err)
	}
	createdAt := make(map[string]time.Time, len(users))
	for name, u := range users {
		createdAt[name] = u.CreatedAt
	}

	// 同 anchor+seed 再执行一次：用户不重复、记录数不增长、档案内容确定。
	stats2, err := apply(ctx, db, plan, true)
	if err != nil {
		t.Fatalf("重复 Apply 失败: %v", err)
	}
	if stats2 != stats {
		t.Fatalf("重复 Apply 统计不一致: first=%+v second=%+v", stats, stats2)
	}
	users2 := loadDemoUsers(t, db)
	if len(users2) != 3 {
		t.Fatalf("重复执行后演示用户数量 = %d，期望 3", len(users2))
	}
	for name, u := range users2 {
		if u.ID != users[name].ID {
			t.Fatalf("%s 的用户 ID 不应改变", name)
		}
		if !u.CreatedAt.Equal(createdAt[name]) {
			t.Fatalf("%s 的 createdAt 不应被重复执行修改", name)
		}
	}
	if got := countRows(t, db, &entity.HealthProfile{}, "1 = 1"); got != 2 {
		t.Fatalf("重复执行后档案数量 = %d，期望 2", got)
	}
	if got := countRows(t, db, &entity.HealthRecord{}, "user_id IN ?", ids); got != int64(wantRecords) {
		t.Fatalf("重复执行后记录数量 = %d，期望 %d", got, wantRecords)
	}
	if got := countRows(t, db, &entity.SysUserRole{}, "1 = 1"); got != 3 {
		t.Fatalf("重复执行后角色绑定数量 = %d，期望 3", got)
	}

	var afterA entity.HealthProfile
	if err := db.Where("user_id = ?", users["demo_user_a"].ID).First(&afterA).Error; err != nil {
		t.Fatalf("重复执行后读取 demo_user_a 档案失败: %v", err)
	}
	if beforeA.HeightCm == nil || afterA.HeightCm == nil || beforeA.HeightCm.String() != afterA.HeightCm.String() ||
		beforeA.WeightKg == nil || afterA.WeightKg == nil || beforeA.WeightKg.String() != afterA.WeightKg.String() ||
		beforeA.RealName == nil || afterA.RealName == nil || *beforeA.RealName != *afterA.RealName {
		t.Fatalf("重复执行后档案内容不一致: before=%+v after=%+v", beforeA, afterA)
	}
}

func TestSeedApplyLeavesOrdinaryUsersUntouched(t *testing.T) {
	db := openSeedTestDB(t)
	resetSeedTables(t, db)
	ctx := context.Background()
	anchor := time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC)
	plan := BuildPlan(anchor, 42)

	userRole := seedRole(t, db, consts.RoleUser, "普通用户")
	ordinary := seedUser(t, db, "ordinary_health_user", "普通用户", "Original#123", userRole)
	profile := entity.HealthProfile{UserID: ordinary.ID, RealName: strPtrSeed("普通用户档案")}
	if err := db.Create(&profile).Error; err != nil {
		t.Fatalf("创建普通用户档案失败: %v", err)
	}
	record := entity.HealthRecord{UserID: ordinary.ID, MeasuredAt: anchor.AddDate(0, 0, -1), Remark: strPtrSeed("普通用户记录")}
	if err := db.Create(&record).Error; err != nil {
		t.Fatalf("创建普通用户记录失败: %v", err)
	}

	if _, err := apply(ctx, db, plan, true); err != nil {
		t.Fatalf("Apply 失败: %v", err)
	}

	var after entity.SysUser
	if err := db.Unscoped().First(&after, ordinary.ID).Error; err != nil {
		t.Fatalf("读取普通用户失败: %v", err)
	}
	if after.PasswordHash == nil || !hash.Verify(*after.PasswordHash, "Original#123") {
		t.Fatal("普通用户密码被 Seed 修改")
	}
	if after.Username != ordinary.Username || after.Nickname == nil || *after.Nickname != "普通用户" ||
		after.Status != ordinary.Status || after.DeletedAt.Valid {
		t.Fatalf("普通用户资料被 Seed 修改: %+v", after)
	}
	roles, err := rolesOfUser(db, ordinary.ID)
	if err != nil || len(roles) != 1 || roles[0].RoleCode != consts.RoleUser {
		t.Fatalf("普通用户角色被 Seed 修改: roles=%+v err=%v", roles, err)
	}
	var afterProfile entity.HealthProfile
	if err := db.First(&afterProfile, profile.ID).Error; err != nil {
		t.Fatalf("普通用户档案被删除: %v", err)
	}
	if afterProfile.RealName == nil || *afterProfile.RealName != "普通用户档案" {
		t.Fatalf("普通用户档案内容被修改: %+v", afterProfile)
	}
	var afterRecord entity.HealthRecord
	if err := db.First(&afterRecord, record.ID).Error; err != nil {
		t.Fatalf("普通用户记录被删除: %v", err)
	}
	if afterRecord.Remark == nil || *afterRecord.Remark != "普通用户记录" {
		t.Fatalf("普通用户记录内容被修改: %+v", afterRecord)
	}
}

func TestSeedApplyRejectsSameNameOrdinaryUser(t *testing.T) {
	db := openSeedTestDB(t)
	resetSeedTables(t, db)
	ctx := context.Background()
	plan := BuildPlan(time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC), 42)

	userRole := seedRole(t, db, consts.RoleUser, "普通用户")
	occupied := seedUser(t, db, "demo_user_a", "真实用户", "Real#User#123", userRole)
	profile := entity.HealthProfile{UserID: occupied.ID, RealName: strPtrSeed("真实档案")}
	if err := db.Create(&profile).Error; err != nil {
		t.Fatalf("创建真实档案失败: %v", err)
	}
	record := entity.HealthRecord{UserID: occupied.ID, MeasuredAt: time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)}
	if err := db.Create(&record).Error; err != nil {
		t.Fatalf("创建真实记录失败: %v", err)
	}

	_, err := apply(ctx, db, plan, true)
	if err == nil || !strings.Contains(err.Error(), "非演示用户") {
		t.Fatalf("同名普通用户应被拒绝，实际: %v", err)
	}

	var after entity.SysUser
	if err := db.Unscoped().First(&after, occupied.ID).Error; err != nil {
		t.Fatalf("读取被占用用户失败: %v", err)
	}
	if after.PasswordHash == nil || !hash.Verify(*after.PasswordHash, "Real#User#123") {
		t.Fatal("拒绝前不得修改同名用户密码")
	}
	if after.Nickname == nil || *after.Nickname != "真实用户" || after.Status != consts.StatusEnabled || after.DeletedAt.Valid {
		t.Fatalf("拒绝前不得修改同名用户资料: %+v", after)
	}
	roles, err := rolesOfUser(db, occupied.ID)
	if err != nil || len(roles) != 1 || roles[0].RoleCode != consts.RoleUser {
		t.Fatalf("拒绝前不得修改同名用户角色: roles=%+v err=%v", roles, err)
	}
	if got := countRows(t, db, &entity.HealthProfile{}, "user_id = ?", occupied.ID); got != 1 {
		t.Fatalf("同名用户的档案不应被删除，实际 %d", got)
	}
	if got := countRows(t, db, &entity.HealthRecord{}, "user_id = ?", occupied.ID); got != 1 {
		t.Fatalf("同名用户的记录不应被删除，实际 %d", got)
	}
	if got := countRows(t, db, &entity.SysRole{}, "role_code = ?", demoSeedRoleCode); got != 0 {
		t.Fatalf("失败事务不应留下演示角色，实际 %d", got)
	}
}

func TestSeedApplyRejectsDemoUserWithExtraRole(t *testing.T) {
	db := openSeedTestDB(t)
	resetSeedTables(t, db)
	ctx := context.Background()
	anchor := time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC)
	plan := BuildPlan(anchor, 42)

	if _, err := apply(ctx, db, plan, true); err != nil {
		t.Fatalf("首次 Apply 失败: %v", err)
	}
	users := loadDemoUsers(t, db)
	a := users["demo_user_a"]

	admin := seedRole(t, db, consts.RoleAdmin, "管理员")
	if err := db.Exec("INSERT INTO sys_user_role (user_id, role_id) VALUES (?, ?)", a.ID, admin.ID).Error; err != nil {
		t.Fatalf("添加额外角色失败: %v", err)
	}
	tampered, err := hash.Hash("Tampered#123")
	if err != nil {
		t.Fatalf("生成测试密码哈希失败: %v", err)
	}
	if err := db.Model(&entity.SysUser{}).Where("id = ?", a.ID).Update("password_hash", tampered).Error; err != nil {
		t.Fatalf("设置测试密码失败: %v", err)
	}
	recordsBefore := countRows(t, db, &entity.HealthRecord{}, "user_id = ?", a.ID)

	_, err = apply(ctx, db, plan, true)
	if err == nil || !strings.Contains(err.Error(), "额外角色") {
		t.Fatalf("带额外角色的演示用户应被拒绝，实际: %v", err)
	}
	var after entity.SysUser
	if err := db.First(&after, a.ID).Error; err != nil {
		t.Fatalf("读取演示用户失败: %v", err)
	}
	if after.PasswordHash == nil || !hash.Verify(*after.PasswordHash, "Tampered#123") {
		t.Fatal("拒绝前不得重置演示用户密码")
	}
	roles, err := rolesOfUser(db, a.ID)
	if err != nil || len(roles) != 2 {
		t.Fatalf("拒绝时不应静默清理额外角色: roles=%+v err=%v", roles, err)
	}
	if got := countRows(t, db, &entity.HealthRecord{}, "user_id = ?", a.ID); got != recordsBefore {
		t.Fatalf("拒绝时不得删除/重建健康记录: before=%d after=%d", recordsBefore, got)
	}
}

func TestSeedApplyRollsBackOnDatabaseError(t *testing.T) {
	db := openSeedTestDB(t)
	resetSeedTables(t, db)
	ctx := context.Background()
	plan := BuildPlan(time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC), 42)

	// 人为制造主键冲突，让事务在写入最后一条记录时失败。
	plan.Users[2].Records[0].ID = 999999999
	plan.Users[2].Records[1].ID = 999999999

	if _, err := apply(ctx, db, plan, true); err == nil {
		t.Fatal("数据库错误必须向上返回")
	}
	if got := countRows(t, db, &entity.SysUser{}, "username IN ?", []string{"demo_user_a", "demo_user_b", "demo_user_c"}); got != 0 {
		t.Fatalf("失败事务不应留下演示用户，实际 %d", got)
	}
	if got := countRows(t, db, &entity.SysRole{}, "role_code = ?", demoSeedRoleCode); got != 0 {
		t.Fatalf("失败事务不应留下演示角色，实际 %d", got)
	}
	if got := countRows(t, db, &entity.HealthProfile{}, "1 = 1"); got != 0 {
		t.Fatalf("失败事务不应留下档案，实际 %d", got)
	}
	if got := countRows(t, db, &entity.HealthRecord{}, "1 = 1"); got != 0 {
		t.Fatalf("失败事务不应留下记录，实际 %d", got)
	}
}

func TestSeedPublicApplyRejectsTestDatabase(t *testing.T) {
	db := openSeedTestDB(t)
	resetSeedTables(t, db)
	plan := BuildPlan(time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC), 42)

	if _, err := Apply(context.Background(), db, plan); err == nil || !strings.Contains(err.Error(), "测试数据库") {
		t.Fatalf("公开 Apply 必须拒绝 _test 库，实际: %v", err)
	}
	if got := countRows(t, db, &entity.SysUser{}, "1 = 1"); got != 0 {
		t.Fatalf("被拒绝的 Apply 不应写入用户，实际 %d", got)
	}
}

func strPtrSeed(v string) *string { return &v }
