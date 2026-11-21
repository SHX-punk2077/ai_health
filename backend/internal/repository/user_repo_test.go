package repository_test

import (
	"context"
	"testing"

	"gorm.io/gorm"

	"ai_health/internal/model/entity"
	"ai_health/internal/repository"
)

func TestUserRepoCreateWithRole(t *testing.T) {
	db := openTestDB(t)
	resetTables(t, db)
	ctx := context.Background()

	mustRole(t, db, "user", "普通用户")
	repo := repository.NewUserRepo(db)

	pwd := "$2a$10$abcdefghijklmnopqrstuv"
	u := &entity.SysUser{Username: "alice", PasswordHash: &pwd, Status: 1}
	if err := repo.CreateWithRole(ctx, u, "user"); err != nil {
		t.Fatalf("CreateWithRole: %v", err)
	}
	if u.ID == 0 {
		t.Fatal("创建后应回填自增 ID")
	}

	got, err := repo.GetByAccount(ctx, "alice")
	if err != nil || got == nil {
		t.Fatalf("GetByAccount 失败: %v, user=%v", err, got)
	}
	if got.ID != u.ID {
		t.Fatalf("ID 不一致: %d != %d", got.ID, u.ID)
	}

	var links int64
	if err := db.Model(&entity.SysUserRole{}).Where("user_id = ?", u.ID).Count(&links).Error; err != nil {
		t.Fatalf("统计角色关联失败: %v", err)
	}
	if links != 1 {
		t.Fatalf("注册应绑定 1 个默认角色，实际 %d", links)
	}
}

// 角色不存在时事务必须整体回滚：不能留下「无角色用户」。
func TestUserRepoCreateWithRoleRollback(t *testing.T) {
	db := openTestDB(t)
	resetTables(t, db)
	ctx := context.Background()

	repo := repository.NewUserRepo(db)
	pwd := "$2a$10$abcdefghijklmnopqrstuv"
	u := &entity.SysUser{Username: "bob", PasswordHash: &pwd, Status: 1}

	if err := repo.CreateWithRole(ctx, u, "not_exists_role"); err == nil {
		t.Fatal("角色不存在时应返回错误")
	}

	var n int64
	if err := db.Model(&entity.SysUser{}).Where("username = ?", "bob").Count(&n).Error; err != nil {
		t.Fatalf("统计用户失败: %v", err)
	}
	if n != 0 {
		t.Fatalf("事务应回滚，用户数应为 0，实际 %d", n)
	}
	var links int64
	if err := db.Model(&entity.SysUserRole{}).Count(&links).Error; err != nil {
		t.Fatalf("统计关联失败: %v", err)
	}
	if links != 0 {
		t.Fatalf("事务应回滚，角色关联应为 0，实际 %d", links)
	}
}

// 数据库唯一索引是重复用户名/邮箱的最终防线（应用层 Exists 只是提前提示）。
func TestUserRepoDuplicateUsernameRejected(t *testing.T) {
	db := openTestDB(t)
	resetTables(t, db)
	ctx := context.Background()

	repo := repository.NewUserRepo(db)
	pwd := "$2a$10$abcdefghijklmnopqrstuv"
	if err := repo.Create(ctx, &entity.SysUser{Username: "carol", PasswordHash: &pwd, Status: 1}); err != nil {
		t.Fatalf("首次创建失败: %v", err)
	}
	if err := repo.Create(ctx, &entity.SysUser{Username: "carol", PasswordHash: &pwd, Status: 1}); err == nil {
		t.Fatal("重复用户名应被唯一索引拒绝")
	}
}

func TestUserRepoExistsExcept(t *testing.T) {
	db := openTestDB(t)
	resetTables(t, db)
	ctx := context.Background()

	repo := repository.NewUserRepo(db)
	pwd := "$2a$10$abcdefghijklmnopqrstuv"
	email := "dave@example.com"
	u := &entity.SysUser{Username: "dave", PasswordHash: &pwd, Email: &email, Status: 1}
	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("创建失败: %v", err)
	}

	exists, err := repo.ExistsExcept(ctx, "email", email, 0)
	if err != nil || !exists {
		t.Fatalf("他人邮箱应判定为已占用: exists=%v err=%v", exists, err)
	}
	exists, err = repo.ExistsExcept(ctx, "email", email, u.ID)
	if err != nil || exists {
		t.Fatalf("排除自己后不应判定占用: exists=%v err=%v", exists, err)
	}
	exists, err = repo.Exists(ctx, "email", "")
	if err != nil || exists {
		t.Fatalf("空串不参与唯一校验: exists=%v err=%v", exists, err)
	}
}

func TestUserRepoSoftDeleteAndUpdateFields(t *testing.T) {
	db := openTestDB(t)
	resetTables(t, db)
	ctx := context.Background()

	repo := repository.NewUserRepo(db)
	pwd := "$2a$10$abcdefghijklmnopqrstuv"
	u := &entity.SysUser{Username: "erin", PasswordHash: &pwd, Status: 1}
	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("创建失败: %v", err)
	}

	nick := "新昵称"
	if _, err := repo.UpdateFields(ctx, u.ID, map[string]any{"nickname": nick, "password_hash": "$2a$10$new"}); err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	got, err := repo.GetByID(ctx, u.ID)
	if err != nil || got == nil {
		t.Fatalf("查询失败: %v", err)
	}
	if got.Nickname == nil || *got.Nickname != nick {
		t.Fatalf("昵称未更新: %+v", got.Nickname)
	}

	if _, err := repo.SoftDelete(ctx, u.ID); err != nil {
		t.Fatalf("软删失败: %v", err)
	}
	if got, err := repo.GetByID(ctx, u.ID); err != nil || got != nil {
		t.Fatalf("软删后应查不到: user=%v err=%v", got, err)
	}

	// 软删行仍在表里，可直接用 Unscoped 验证，防止误写成物理删除。
	var raw entity.SysUser
	if err := db.Unscoped().Where("id = ?", u.ID).First(&raw).Error; err != nil {
		t.Fatalf("软删后原始行应保留: %v", err)
	}
	if !raw.DeletedAt.Valid {
		t.Fatal("deleted_at 应被写入")
	}
}

// NULL 字段读写：可选列允许为空，回读不能报错。
func TestUserRepoNullableFields(t *testing.T) {
	db := openTestDB(t)
	resetTables(t, db)
	ctx := context.Background()

	repo := repository.NewUserRepo(db)
	pwd := "$2a$10$abcdefghijklmnopqrstuv"
	u := &entity.SysUser{Username: "frank", PasswordHash: &pwd, Status: 1}
	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	got, err := repo.GetByID(ctx, u.ID)
	if err != nil || got == nil {
		t.Fatalf("查询失败: %v", err)
	}
	if got.Email != nil || got.Phone != nil || got.Nickname != nil || got.LastLoginAt != nil {
		t.Fatalf("未写入的可选列应为 NULL: %+v", got)
	}
}

var _ = gorm.ErrRecordNotFound
