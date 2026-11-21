package entity_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	driver "github.com/go-sql-driver/mysql"
	"github.com/shopspring/decimal"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"ai_health/internal/model/entity"
)

// 集成测试使用独立测试库：DSN 必须指向以 _test 结尾的库，
// 默认跳过，只有设置 AIH_TEST_MYSQL_DSN 时才执行，避免破坏开发库。

var (
	schemaOnce sync.Once
	schemaErr  error
)

// testTables 按建表顺序排列；清空时无需关心顺序（会话关闭外键检查）。
var testTables = []string{
	"sys_user",
	"sys_oauth_identity",
	"sys_role",
	"sys_menu",
	"sys_user_role",
	"sys_role_menu",
	"health_profile",
	"health_record",
	"sport_type",
	"sport_info",
	"sport_disease_detail",
	"ai_conversation",
	"ai_message",
}

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("AIH_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("未设置 AIH_TEST_MYSQL_DSN，跳过需要 MySQL 的集成测试")
	}

	cfg, err := driver.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("解析 AIH_TEST_MYSQL_DSN 失败: %v", err)
	}
	if !strings.HasSuffix(cfg.DBName, "_test") {
		t.Fatalf("测试库名必须以 _test 结尾，拒绝在 %q 上执行测试", cfg.DBName)
	}
	cfg.ParseTime = true
	cfg.Loc = time.UTC
	cfg.MultiStatements = true
	if cfg.Params == nil {
		cfg.Params = make(map[string]string)
	}
	// 会话时区固定 UTC，验证 DATETIME(3) 的存储/读取基准。
	cfg.Params["time_zone"] = "'+00:00'"

	db, err := gorm.Open(mysql.Open(cfg.FormatDSN()), &gorm.Config{
		Logger:                 logger.Default.LogMode(logger.Silent),
		SkipDefaultTransaction: true,
		NowFunc:                func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		t.Fatalf("连接测试库失败: %v", err)
	}
	if err := entity.SetupJoinTables(db); err != nil {
		t.Fatalf("注册 join 表失败: %v", err)
	}

	schemaOnce.Do(func() { schemaErr = resetSchema(db) })
	if schemaErr != nil {
		t.Fatalf("初始化测试库结构失败: %v", schemaErr)
	}

	truncateAll(t, db)
	verifySessionUTC(t, db)
	return db
}

func resetSchema(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	ctx := context.Background()
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=0"); err != nil {
		return err
	}
	defer conn.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=1")

	for i := len(testTables) - 1; i >= 0; i-- {
		if _, err := conn.ExecContext(ctx, "DROP TABLE IF EXISTS "+testTables[i]); err != nil {
			return fmt.Errorf("drop %s: %w", testTables[i], err)
		}
	}

	raw, err := os.ReadFile("../../../migrations/000001_init.up.sql")
	if err != nil {
		return err
	}
	for _, stmt := range strings.Split(string(raw), ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("执行迁移失败: %v; SQL: %.200s", err, stmt)
		}
	}
	return nil
}

func truncateAll(t *testing.T, db *gorm.DB) {
	t.Helper()

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("获取 sql.DB 失败: %v", err)
	}
	ctx := context.Background()
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		t.Fatalf("获取连接失败: %v", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=0"); err != nil {
		t.Fatalf("关闭外键检查失败: %v", err)
	}
	for _, table := range testTables {
		if _, err := conn.ExecContext(ctx, "TRUNCATE TABLE "+table); err != nil {
			t.Fatalf("清空 %s 失败: %v", table, err)
		}
	}
	if _, err := conn.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=1"); err != nil {
		t.Fatalf("恢复外键检查失败: %v", err)
	}
}

func verifySessionUTC(t *testing.T, db *gorm.DB) {
	t.Helper()
	var zone string
	if err := db.Raw("SELECT @@session.time_zone").Scan(&zone).Error; err != nil {
		t.Fatalf("读取会话时区失败: %v", err)
	}
	if zone != "+00:00" {
		t.Fatalf("会话时区 = %q, want +00:00", zone)
	}
}

func mysqlErrCode(err error) uint16 {
	var me *driver.MySQLError
	if errors.As(err, &me) {
		return me.Number
	}
	return 0
}

func ptr[T any](v T) *T { return &v }

func dec(s string) *decimal.Decimal {
	d := decimal.RequireFromString(s)
	return &d
}

func createUser(t *testing.T, db *gorm.DB, username string) *entity.SysUser {
	t.Helper()
	u := &entity.SysUser{Username: username, Status: 1}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}
	return u
}

func createRole(t *testing.T, db *gorm.DB, code string) *entity.SysRole {
	t.Helper()
	r := &entity.SysRole{RoleCode: code, RoleName: code, Status: 1}
	if err := db.Create(r).Error; err != nil {
		t.Fatalf("创建角色失败: %v", err)
	}
	return r
}

func createMenu(t *testing.T, db *gorm.DB, name string, parentID *uint64, menuType uint8) *entity.SysMenu {
	t.Helper()
	m := &entity.SysMenu{
		ParentID:  parentID,
		MenuType:  menuType,
		Name:      name,
		Title:     name,
		IsEnabled: true,
	}
	if err := db.Create(m).Error; err != nil {
		t.Fatalf("创建菜单失败: %v", err)
	}
	return m
}

func createSportType(t *testing.T, db *gorm.DB, code string) *entity.SportType {
	t.Helper()
	s := &entity.SportType{TypeCode: code, TypeName: code, Status: 1}
	if err := db.Create(s).Error; err != nil {
		t.Fatalf("创建运动类型失败: %v", err)
	}
	return s
}

func createConversation(t *testing.T, db *gorm.DB, userID uint64) *entity.AIConversation {
	t.Helper()
	c := &entity.AIConversation{UserID: userID, Title: "健康咨询", Status: 1}
	if err := db.Create(c).Error; err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	return c
}

func createProfile(t *testing.T, db *gorm.DB, userID uint64) *entity.HealthProfile {
	t.Helper()
	p := &entity.HealthProfile{UserID: userID}
	if err := db.Create(p).Error; err != nil {
		t.Fatalf("创建健康档案失败: %v", err)
	}
	return p
}

func mustCreate(t *testing.T, db *gorm.DB, value any) {
	t.Helper()
	if err := db.Create(value).Error; err != nil {
		t.Fatalf("创建 %T 失败: %v", value, err)
	}
}

func countRows(t *testing.T, db *gorm.DB, model any) int64 {
	t.Helper()
	var n int64
	if err := db.Model(model).Count(&n).Error; err != nil {
		t.Fatalf("统计 %T 失败: %v", model, err)
	}
	return n
}

func assertCount(t *testing.T, db *gorm.DB, model any, want int64) {
	t.Helper()
	if got := countRows(t, db, model); got != want {
		t.Fatalf("%T 行数 = %d, want %d", model, got, want)
	}
}
