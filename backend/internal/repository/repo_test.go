package repository_test

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	driver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"ai_health/internal/model/entity"
)

// 集成测试统一跑在独立测试库（DSN 必须以 _test 结尾），
// 默认跳过，只有设置 AIH_TEST_MYSQL_DSN 时才执行，避免误伤开发库。
var (
	dbOnce sync.Once
	testDB *gorm.DB
	dbErr  error
)

// allTables 清库顺序无关（清空前关闭外键检查），覆盖本包用到的 12 张表。
var allTables = []string{
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

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("AIH_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("未设置 AIH_TEST_MYSQL_DSN，跳过需要 MySQL 的集成测试")
	}

	dbOnce.Do(func() {
		cfg, err := driver.ParseDSN(dsn)
		if err != nil {
			dbErr = fmt.Errorf("解析 AIH_TEST_MYSQL_DSN 失败: %w", err)
			return
		}
		if !strings.HasSuffix(cfg.DBName, "_test") {
			dbErr = fmt.Errorf("测试库名必须以 _test 结尾，拒绝在 %q 上执行测试", cfg.DBName)
			return
		}
		testDB, dbErr = gorm.Open(mysql.Open(dsn), &gorm.Config{
			SkipDefaultTransaction: true,
			Logger:                 logger.Default.LogMode(logger.Silent),
			NowFunc:                func() time.Time { return time.Now().UTC() },
		})
	})
	if dbErr != nil {
		t.Fatalf("连接测试库失败: %v", dbErr)
	}
	return testDB
}

// resetTables 每个用例前清空全部业务表，保证用例之间互不污染。
func resetTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	err := db.Connection(func(tx *gorm.DB) error {
		if err := tx.Exec("SET FOREIGN_KEY_CHECKS = 0").Error; err != nil {
			return err
		}
		for _, tb := range allTables {
			if err := tx.Exec("TRUNCATE TABLE " + tb).Error; err != nil {
				return err
			}
		}
		return tx.Exec("SET FOREIGN_KEY_CHECKS = 1").Error
	})
	if err != nil {
		t.Fatalf("清理测试库失败: %v", err)
	}
}

func mustRole(t *testing.T, db *gorm.DB, code, name string) entity.SysRole {
	t.Helper()
	role := entity.SysRole{RoleCode: code, RoleName: name, Status: 1}
	if err := db.Create(&role).Error; err != nil {
		t.Fatalf("创建角色 %s 失败: %v", code, err)
	}
	return role
}

func mustUser(t *testing.T, db *gorm.DB, username string) entity.SysUser {
	t.Helper()
	pwd := "hashed-placeholder"
	u := entity.SysUser{Username: username, PasswordHash: &pwd, Status: 1}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("创建用户 %s 失败: %v", username, err)
	}
	return u
}

func mustMenu(t *testing.T, db *gorm.DB, name, permission string, enabled bool) entity.SysMenu {
	t.Helper()
	var perm *string
	if permission != "" {
		p := permission
		perm = &p
	}
	m := entity.SysMenu{Name: name, Title: name, MenuType: entity.MenuTypeButton, Permission: perm, IsEnabled: enabled}
	if err := db.Create(&m).Error; err != nil {
		t.Fatalf("创建菜单 %s 失败: %v", name, err)
	}
	// GORM 对带 default 标签的零值字段会跳过写入，is_enabled=false 必须显式更新。
	if !enabled {
		if err := db.Model(&entity.SysMenu{}).Where("id = ?", m.ID).Update("is_enabled", false).Error; err != nil {
			t.Fatalf("设置菜单 %s 为禁用失败: %v", name, err)
		}
		m.IsEnabled = false
	}
	return m
}
