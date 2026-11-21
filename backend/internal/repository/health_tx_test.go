package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"ai_health/internal/model/entity"
	"ai_health/internal/repository"
)

func repoStrPtr(v string) *string { return &v }

func repoDecimalPtr(t *testing.T, raw string) *decimal.Decimal {
	t.Helper()
	d, err := decimal.NewFromString(raw)
	if err != nil {
		t.Fatalf("解析 DECIMAL %q 失败: %v", raw, err)
	}
	return &d
}

func TestHealthRepoTransactionRollbackAndLockOwnership(t *testing.T) {
	db := openTestDB(t)
	resetTables(t, db)
	alice := mustUser(t, db, "health_tx_alice")
	bob := mustUser(t, db, "health_tx_bob")
	ctx := context.Background()

	if err := db.Create(&entity.HealthProfile{UserID: alice.ID, RealName: repoStrPtr("alice")}).Error; err != nil {
		t.Fatalf("创建档案失败: %v", err)
	}
	weight := "70.00"
	rec := entity.HealthRecord{
		UserID:     alice.ID,
		WeightKg:   repoDecimalPtr(t, weight),
		MeasuredAt: time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC),
		Source:     entity.SourceManual,
	}
	if err := db.Create(&rec).Error; err != nil {
		t.Fatalf("创建记录失败: %v", err)
	}

	repo := repository.NewHealthRepo(db)
	rollback := errors.New("强制回滚")
	err := repo.Transaction(ctx, func(tx *repository.HealthRepo) error {
		locked, err := tx.LockRecord(ctx, alice.ID, rec.ID)
		if err != nil {
			return err
		}
		if locked == nil || locked.ID != rec.ID {
			t.Fatalf("应能锁定自己的记录: %+v", locked)
		}
		// 他人记录不能被锁定，也不能借事务绕过 user_id 条件。
		if other, err := tx.LockRecord(ctx, bob.ID, rec.ID); err != nil || other != nil {
			t.Fatalf("不应锁定他人记录: rec=%+v err=%v", other, err)
		}
		if profile, err := tx.LockProfile(ctx, bob.ID); err != nil || profile != nil {
			t.Fatalf("不应锁定他人档案: profile=%+v err=%v", profile, err)
		}
		if _, err := tx.UpdateRecord(ctx, alice.ID, rec.ID, map[string]any{"weight_kg": "71.00"}); err != nil {
			return err
		}
		extra := entity.HealthRecord{
			UserID:     alice.ID,
			MeasuredAt: time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC),
		}
		if err := tx.CreateRecord(ctx, &extra); err != nil {
			return err
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("事务应返回原始错误，实际: %v", err)
	}

	var count int64
	if err := db.Model(&entity.HealthRecord{}).Where("user_id = ?", alice.ID).Count(&count).Error; err != nil {
		t.Fatalf("统计记录失败: %v", err)
	}
	if count != 1 {
		t.Fatalf("事务回滚后只应保留原记录，实际 %d 条", count)
	}
	var after entity.HealthRecord
	if err := db.First(&after, rec.ID).Error; err != nil {
		t.Fatalf("读取原记录失败: %v", err)
	}
	if after.WeightKg == nil || after.WeightKg.StringFixed(2) != "70.00" {
		t.Fatalf("事务回滚后体重不应改变: %+v", after.WeightKg)
	}

	// 成功事务提交后更新可见。
	if err := repo.Transaction(ctx, func(tx *repository.HealthRepo) error {
		locked, err := tx.LockRecord(ctx, alice.ID, rec.ID)
		if err != nil {
			return err
		}
		if locked == nil {
			return errors.New("记录不存在")
		}
		_, err = tx.UpdateRecord(ctx, alice.ID, rec.ID, map[string]any{"weight_kg": "72.00"})
		return err
	}); err != nil {
		t.Fatalf("提交事务失败: %v", err)
	}
	committed, err := repo.GetRecord(ctx, alice.ID, rec.ID)
	if err != nil || committed == nil || committed.WeightKg == nil || committed.WeightKg.StringFixed(2) != "72.00" {
		t.Fatalf("事务提交后体重应为 72.00: rec=%+v err=%v", committed, err)
	}
}
