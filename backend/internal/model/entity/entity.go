// Package entity 严格对应 ai_health_mysql8_init.sql 的表结构。
// 约定：表名一律手写 TableName()（GORM 默认复数命名与本项目不一致）；
// BIGINT UNSIGNED -> uint64，DECIMAL -> decimal.Decimal，DATETIME(3) -> time.Time，
// 可空列一律用指针，软删列用 gorm.DeletedAt。
package entity

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// 编译期占位，避免 import 未使用报错；实际使用集中在各实体文件。
var (
	_ = time.Now
	_ = decimal.Zero
	_ = gorm.ErrRecordNotFound
)
