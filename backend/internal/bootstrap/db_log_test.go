package bootstrap

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"ai_health/internal/config"
)

func TestDBLoggerParameterizesSensitiveValues(t *testing.T) {
	var buf bytes.Buffer
	cfg := &config.Config{}
	cfg.Server.Mode = "debug"
	lg := newDBLogger(cfg, &buf)

	const sql = "INSERT INTO health_record (user_id, weight_kg) VALUES (?, ?)"
	pf, ok := lg.(interface {
		ParamsFilter(context.Context, string, ...interface{}) (string, []interface{})
	})
	if !ok {
		t.Fatal("GORM logger 未实现 ParamsFilter")
	}
	paramSQL, params := pf.ParamsFilter(context.Background(), sql, 101, "72.00")
	if len(params) != 0 {
		t.Fatalf("参数化日志不应保留参数，实际 %d 个", len(params))
	}
	lg.Trace(context.Background(), time.Now().Add(-time.Millisecond), func() (string, int64) {
		return paramSQL, 1
	}, nil)

	out := buf.String()
	if !strings.Contains(out, "weight_kg") || !strings.Contains(out, "?") {
		t.Fatalf("日志缺少 SQL 占位符: %s", out)
	}
	if strings.Contains(out, "72.00") {
		t.Fatalf("日志泄露了健康参数值: %s", out)
	}
}
