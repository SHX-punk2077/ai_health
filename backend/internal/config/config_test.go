package config

import (
	"testing"
)

func TestAIApplyDefaults(t *testing.T) {
	tests := []struct {
		name      string
		provider  string
		baseURL   string
		model     string
		wantURL   string
		wantModel string
		wantErr   bool
	}{
		{name: "空 provider 回落到 deepseek", provider: "", wantURL: "https://api.deepseek.com/v1", wantModel: "deepseek-chat"},
		{name: "大小写归一", provider: "QWEN", wantURL: "https://dashscope.aliyuncs.com/compatible-mode/v1", wantModel: "qwen-plus"},
		{name: "glm 预置", provider: "glm", wantURL: "https://open.bigmodel.cn/api/paas/v4", wantModel: "glm-4-flash"},
		{name: "显式配置优先", provider: "moonshot", baseURL: "https://proxy.example.com/v1", model: "moonshot-v1-32k", wantURL: "https://proxy.example.com/v1", wantModel: "moonshot-v1-32k"},
		{name: "mock 不填端点", provider: "mock", wantURL: "", wantModel: ""},
		{name: "未知 provider 报错", provider: "ernie", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ai := AIConfig{Provider: tt.provider, BaseURL: tt.baseURL, Model: tt.model}
			err := ai.ApplyDefaults()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("期望报错，实际通过：%+v", ai)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ai.BaseURL != tt.wantURL || ai.Model != tt.wantModel {
				t.Fatalf("base_url/model = %q/%q, want %q/%q", ai.BaseURL, ai.Model, tt.wantURL, tt.wantModel)
			}
		})
	}
}

// AI Key 不是启动硬性条件：AI 模块未启用时后端必须能正常起来。
func TestValidateAllowsMissingAPIKey(t *testing.T) {
	base := Config{
		JWT:   JWTConfig{Secret: "0123456789abcdef0123456789abcdef"},
		MySQL: MySQLConfig{DSN: "root:root@tcp(127.0.0.1:3306)/ai_health"},
	}

	mock := base
	mock.AI = AIConfig{Provider: "mock"}
	if err := mock.Validate(); err != nil {
		t.Fatalf("mock 不应要求 api_key: %v", err)
	}

	real := base
	real.AI = AIConfig{Provider: "deepseek"}
	if err := real.Validate(); err != nil {
		t.Fatalf("缺 api_key 不应阻塞启动（只应告警）: %v", err)
	}

	withKey := base
	withKey.AI = AIConfig{Provider: "deepseek", APIKey: "sk-test"}
	if err := withKey.Validate(); err != nil {
		t.Fatalf("填了 api_key 不应报错: %v", err)
	}
	if withKey.AI.BaseURL == "" || withKey.AI.Model == "" {
		t.Fatalf("Validate 应补齐端点：%+v", withKey.AI)
	}
}
