package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"ai_health/internal/config"
)

// Init 初始化全局 zap logger。
func Init(cfg config.LogConfig) (*zap.Logger, error) {
	lvl := zap.NewAtomicLevel()
	if err := lvl.UnmarshalText([]byte(cfg.Level)); err != nil {
		lvl.SetLevel(zapcore.InfoLevel)
	}

	var c zap.Config
	if cfg.Format == "json" {
		c = zap.NewProductionConfig()
	} else {
		c = zap.NewDevelopmentConfig()
		c.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}
	c.Level = lvl
	c.Encoding = map[bool]string{true: "json", false: "console"}[cfg.Format == "json"]

	l, err := c.Build(zap.AddCallerSkip(1))
	if err != nil {
		return nil, err
	}
	zap.ReplaceGlobals(l)
	return l, nil
}
