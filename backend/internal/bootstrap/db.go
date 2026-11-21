package bootstrap

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	mysqlcfg "github.com/go-sql-driver/mysql"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"ai_health/internal/config"
	"ai_health/internal/model/entity"
)

// NewDB 初始化 GORM：连接池、UTC 时间基准、join 表注册，不在启动时执行 AutoMigrate。
func NewDB(cfg *config.Config) (*gorm.DB, error) {
	lg := newDBLogger(cfg, os.Stdout)

	dsn, err := normalizeDSN(cfg.MySQL.DSN)
	if err != nil {
		return nil, err
	}
	db, err := gorm.Open(gormmysql.Open(dsn), &gorm.Config{
		Logger:                 lg,
		SkipDefaultTransaction: true,
		// 应用侧统一以 UTC 生成 created_at/updated_at，避免依赖容器本地时区。
		NowFunc: func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}

	// 多对多关联使用显式 join 实体，避免 GORM 推导出不存在的中间表。
	if err := entity.SetupJoinTables(db); err != nil {
		return nil, fmt.Errorf("setup join tables: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(cfg.MySQL.MaxOpen)
	sqlDB.SetMaxIdleConns(cfg.MySQL.MaxIdle)
	sqlDB.SetConnMaxLifetime(time.Hour)
	sqlDB.SetConnMaxIdleTime(10 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping mysql: %w", err)
	}
	return db, nil
}

// newDBLogger 构造 GORM 日志器。ParameterizedQueries 让 SQL 日志只保留占位符，
// 避免 debug/慢查询日志把健康数值等业务参数展开到标准输出。
func newDBLogger(cfg *config.Config, out io.Writer) gormlogger.Interface {
	level := gormlogger.Warn
	if cfg.Server.Mode == "debug" {
		level = gormlogger.Info
	}
	slow := cfg.MySQL.SlowThreshold
	if slow <= 0 {
		slow = 200 * time.Millisecond
	}
	return gormlogger.New(log.New(out, "\r\n", log.LstdFlags), gormlogger.Config{
		SlowThreshold:             slow,
		LogLevel:                  level,
		IgnoreRecordNotFoundError: true,
		Colorful:                  false,
		ParameterizedQueries:      true,
	})
}

// normalizeDSN 把运行时 DSN 强制归一到 UTC 会话：
// parseTime=true、loc=UTC、time_zone='+00:00'，并给未配置的 DSN 补 5s 拨号超时。
// 即使部署环境误写了 loc=Local 或容器时区为 Asia/Shanghai，DATETIME(3) 仍按 UTC 存取。
func normalizeDSN(raw string) (string, error) {
	dsnCfg, err := mysqlcfg.ParseDSN(raw)
	if err != nil {
		return "", fmt.Errorf("parse mysql dsn: %w", err)
	}
	dsnCfg.ParseTime = true
	dsnCfg.Loc = time.UTC
	if dsnCfg.Timeout == 0 {
		dsnCfg.Timeout = 5 * time.Second
	}
	if dsnCfg.Params == nil {
		dsnCfg.Params = make(map[string]string, 1)
	}
	dsnCfg.Params["time_zone"] = "'+00:00'"
	return dsnCfg.FormatDSN(), nil
}

// CloseDB 关闭底层连接池，供进程退出时释放连接。
func CloseDB(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
