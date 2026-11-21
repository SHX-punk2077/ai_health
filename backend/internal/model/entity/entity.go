// Package entity 严格对应 backend/migrations 中的 MySQL 8 表结构。
//
// 约定：
//   - 表名一律手写 TableName()，不依赖 GORM 的复数命名策略；
//   - BIGINT UNSIGNED -> uint64，TINYINT UNSIGNED -> uint8，
//     SMALLINT UNSIGNED -> uint16，INT UNSIGNED -> uint32；
//   - DECIMAL(p,s) -> *decimal.Decimal，DATETIME(3) -> time.Time，DATE -> *time.Time；
//   - 可空列一律用指针，软删列用 gorm.DeletedAt；
//   - 每个列都显式声明 type，索引名 / 外键名与 migration 完全一致；
//   - 关联字段只用于 Preload / Joins，统一 json:"-"，避免污染接口响应。
//
// 生产环境禁用 AutoMigrate，schema 的唯一来源是 backend/migrations；
// 因此这里的标签是对既有表结构的声明，改表必须先写 migration。
package entity
