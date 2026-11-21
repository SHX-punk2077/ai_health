package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// 访问日志只记录请求元数据，不得输出健康请求体、query 或其中的测量值。
func TestLoggerDoesNotLogHealthPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)

	core, observed := observer.New(zap.InfoLevel)
	previous := zap.L()
	zap.ReplaceGlobals(zap.New(core))
	t.Cleanup(func() { zap.ReplaceGlobals(previous) })

	r := gin.New()
	r.Use(Logger())
	r.POST("/api/v1/health/records", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	body := `{"weightKg":"72.00","bloodSugarMmol":"5.60","remark":"private-remark"}`
	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/health/records?metric=weightKg&token=private-token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:4567"

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	entries := observed.All()
	if len(entries) != 1 {
		t.Fatalf("log entries = %d, want 1", len(entries))
	}
	entry := entries[0]
	if entry.Message != "access" {
		t.Fatalf("message = %q, want access", entry.Message)
	}

	fields := entry.ContextMap()
	allowed := map[string]bool{
		"requestId": true, "method": true, "path": true,
		"status": true, "cost": true, "ip": true, "uid": true,
	}
	for key := range fields {
		if !allowed[key] {
			t.Fatalf("访问日志出现未预期字段 %q: %#v", key, fields)
		}
	}

	for _, sensitive := range []string{"72.00", "5.60", "private-remark", "private-token", "weightKg"} {
		if strings.Contains(entry.Message, sensitive) {
			t.Fatalf("日志 message 泄露 %q", sensitive)
		}
		for key, value := range fields {
			if strings.Contains(fmt.Sprint(value), sensitive) {
				t.Fatalf("日志字段 %s 泄露 %q: %v", key, sensitive, value)
			}
		}
	}
}
