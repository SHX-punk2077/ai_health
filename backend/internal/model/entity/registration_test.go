package entity_test

import (
	"context"
	"testing"

	"ai_health/internal/model/entity"
	"ai_health/internal/repository"
)

// TestCreateWithRoleTransaction 验证注册时“建用户 + 绑默认角色”是同一事务。
func TestCreateWithRoleTransaction(t *testing.T) {
	db := openTestDB(t)
	createRole(t, db, "user")

	repo := repository.NewUserRepo(db)
	u := &entity.SysUser{Username: "tx-user", Status: 1}
	if err := repo.CreateWithRole(context.Background(), u, "user"); err != nil {
		t.Fatalf("注册事务失败: %v", err)
	}
	if u.ID == 0 {
		t.Fatal("注册后用户 ID 未回填")
	}
	assertCount(t, db, &entity.SysUser{}, 1)
	assertCount(t, db, &entity.SysUserRole{}, 1)

	// 角色不存在时，用户插入也必须回滚。
	bad := &entity.SysUser{Username: "tx-rollback", Status: 1}
	if err := repo.CreateWithRole(context.Background(), bad, "missing-role"); err == nil {
		t.Fatal("缺失角色时注册应失败")
	}
	var n int64
	if err := db.Model(&entity.SysUser{}).Where("username = ?", "tx-rollback").Count(&n).Error; err != nil {
		t.Fatalf("检查回滚失败: %v", err)
	}
	if n != 0 {
		t.Fatalf("事务未回滚，残留用户数 = %d", n)
	}
}
