package bootstrap

import (
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"ai_health/internal/config"
)

// NewDB 初始化 GORM：连接池、慢查询阈值、默认禁用 AutoMigrate。
func NewDB(cfg *config.Config) (*gorm.DB, error) {
	lg := gormlogger.Default.LogMode(gormlogger.Warn)
	if cfg.Server.Mode == "debug" {
		lg = gormlogger.Default.LogMode(gormlogger.Info)
	}

	db, err := gorm.Open(mysql.Open(cfg.MySQL.DSN), &gorm.Config{
		Logger:                 lg,
		SkipDefaultTransaction: true,
		NowFunc:                func() time.Time { return time.Now().Local() },
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(cfg.MySQL.MaxOpen)
	sqlDB.SetMaxIdleConns(cfg.MySQL.MaxIdle)
	sqlDB.SetConnMaxLifetime(time.Hour)
	return db, sqlDB.Ping()
}
