package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config 对应 configs/config.yaml，敏感项允许用 AIH_ 前缀的环境变量覆盖。
type Config struct {
	Server ServerConfig `mapstructure:"server"`
	MySQL  MySQLConfig  `mapstructure:"mysql"`
	Redis  RedisConfig  `mapstructure:"redis"`
	JWT    JWTConfig    `mapstructure:"jwt"`
	AI     AIConfig     `mapstructure:"ai"`
	CORS   CORSConfig   `mapstructure:"cors"`
	Log    LogConfig    `mapstructure:"log"`
}

type ServerConfig struct {
	Addr            string        `mapstructure:"addr"`
	Mode            string        `mapstructure:"mode"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

type MySQLConfig struct {
	DSN           string        `mapstructure:"dsn"`
	MaxOpen       int           `mapstructure:"max_open"`
	MaxIdle       int           `mapstructure:"max_idle"`
	SlowThreshold time.Duration `mapstructure:"slow_threshold"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

type JWTConfig struct {
	Secret     string        `mapstructure:"secret"`
	Issuer     string        `mapstructure:"issuer"`
	AccessTTL  time.Duration `mapstructure:"access_ttl"`
	RefreshTTL time.Duration `mapstructure:"refresh_ttl"`
}

// AIConfig 大模型接入配置。国内服务商全部提供 OpenAI 兼容的 /chat/completions 协议，
// 因此切换厂商只需要换 provider（决定 base_url 与 model 默认值）+ 填 api_key。
type AIConfig struct {
	Provider     string        `mapstructure:"provider"`
	BaseURL      string        `mapstructure:"base_url"`
	APIKey       string        `mapstructure:"api_key"`
	Model        string        `mapstructure:"model"`
	Timeout      time.Duration `mapstructure:"timeout"`
	MaxTokens    int           `mapstructure:"max_tokens"`
	Temperature  float32       `mapstructure:"temperature"`
	HistoryLimit int           `mapstructure:"history_limit"`
}

type aiPreset struct {
	BaseURL string
	Model   string
}

// aiPresets 国内主流大模型的 OpenAI 兼容端点与默认模型（openai 仅作备用）。
//
//	deepseek  DeepSeek 开放平台   platform.deepseek.com
//	qwen      阿里云百炼/通义千问  bailian.console.aliyun.com
//	glm       智谱 BigModel       open.bigmodel.cn
//	moonshot  月之暗面 Kimi       platform.moonshot.cn
var aiPresets = map[string]aiPreset{
	"deepseek": {BaseURL: "https://api.deepseek.com/v1", Model: "deepseek-chat"},
	"qwen":     {BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1", Model: "qwen-plus"},
	"glm":      {BaseURL: "https://open.bigmodel.cn/api/paas/v4", Model: "glm-4-flash"},
	"moonshot": {BaseURL: "https://api.moonshot.cn/v1", Model: "moonshot-v1-8k"},
	"openai":   {BaseURL: "https://api.openai.com/v1", Model: "gpt-4o-mini"},
	"mock":     {},
}

// ApplyDefaults 归一化 provider，并在 base_url / model 留空时按 provider 预置。
func (a *AIConfig) ApplyDefaults() error {
	a.Provider = strings.ToLower(strings.TrimSpace(a.Provider))
	if a.Provider == "" {
		a.Provider = "deepseek"
	}
	preset, ok := aiPresets[a.Provider]
	if !ok {
		return fmt.Errorf("config: 不支持的 ai.provider=%q（可选 deepseek/qwen/glm/moonshot/openai/mock）", a.Provider)
	}
	if a.BaseURL == "" {
		a.BaseURL = preset.BaseURL
	}
	if a.Model == "" {
		a.Model = preset.Model
	}
	return nil
}

type CORSConfig struct {
	AllowOrigins []string `mapstructure:"allow_origins"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// Load 读取 YAML，并用 AIH_* 环境变量覆盖敏感项。
func Load(file string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(file)
	v.SetEnvPrefix("AIH")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config %s: %w", file, err)
	}

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	override := func(env string, dst *string) {
		if s := os.Getenv(env); s != "" {
			*dst = s
		}
	}
	override("AIH_SERVER_MODE", &cfg.Server.Mode)
	override("AIH_MYSQL_DSN", &cfg.MySQL.DSN)
	override("AIH_REDIS_ADDR", &cfg.Redis.Addr)
	override("AIH_REDIS_PASSWORD", &cfg.Redis.Password)
	override("AIH_JWT_SECRET", &cfg.JWT.Secret)
	override("AIH_AI_API_KEY", &cfg.AI.APIKey)
	override("AIH_AI_BASE_URL", &cfg.AI.BaseURL)
	override("AIH_AI_PROVIDER", &cfg.AI.Provider)
	override("AIH_AI_MODEL", &cfg.AI.Model)

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate 负责启动期兜底：敏感配置缺失直接失败，避免带病上线。
func (c *Config) Validate() error {
	if len(c.JWT.Secret) < 32 {
		return fmt.Errorf("config: jwt.secret 必须至少 32 字节（用 AIH_JWT_SECRET 注入）")
	}
	if c.MySQL.DSN == "" {
		return fmt.Errorf("config: mysql.dsn 不能为空")
	}
	if err := c.AI.ApplyDefaults(); err != nil {
		return err
	}
	if c.AI.Provider != "mock" && c.AI.APIKey == "" {
		return fmt.Errorf("config: ai.api_key 不能为空（provider=%s，用 AIH_AI_API_KEY 注入）", c.AI.Provider)
	}
	if c.Server.Addr == "" {
		c.Server.Addr = ":8080"
	}
	return nil
}
