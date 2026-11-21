package bootstrap

import (
	"testing"
	"time"

	mysqlcfg "github.com/go-sql-driver/mysql"
)

func TestNormalizeDSNForcesUTCAndTimeout(t *testing.T) {
	got, err := normalizeDSN("user:pass@tcp(127.0.0.1:3306)/ai_health?charset=utf8mb4&loc=Local&timeout=30s")
	if err != nil {
		t.Fatalf("normalizeDSN 失败: %v", err)
	}
	cfg, err := mysqlcfg.ParseDSN(got)
	if err != nil {
		t.Fatalf("解析归一化 DSN 失败: %v", err)
	}
	if !cfg.ParseTime {
		t.Fatal("parseTime 应为 true")
	}
	if cfg.Loc != time.UTC {
		t.Fatalf("loc = %v, want UTC", cfg.Loc)
	}
	if cfg.Timeout != 30*time.Second {
		t.Fatalf("已配置的 timeout 被覆盖: %v", cfg.Timeout)
	}
	if got := cfg.Params["time_zone"]; got != "'+00:00'" {
		t.Fatalf("time_zone = %q, want '+00:00'", got)
	}

	// 未配置 timeout 时补 5s，避免启动阶段无限等待。
	got, err = normalizeDSN("user:pass@tcp(127.0.0.1:3306)/ai_health")
	if err != nil {
		t.Fatalf("normalizeDSN 失败: %v", err)
	}
	cfg, err = mysqlcfg.ParseDSN(got)
	if err != nil {
		t.Fatalf("解析归一化 DSN 失败: %v", err)
	}
	if cfg.Timeout != 5*time.Second {
		t.Fatalf("默认 timeout = %v, want 5s", cfg.Timeout)
	}
}

func TestNormalizeDSNRejectsInvalid(t *testing.T) {
	if _, err := normalizeDSN("not-a-dsn"); err == nil {
		t.Fatal("非法 DSN 应返回错误")
	}
}
