package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"ai_health/internal/bootstrap"
	"ai_health/internal/config"
	"ai_health/internal/pkg/cache"
	"ai_health/internal/pkg/logger"
)

// @title           AI 健康管理系统 API
// @version         1.0
// @description     基于 Gin + GORM 的健康档案 / 健康记录 / 运动建议 / AI 问答服务
// @BasePath        /api/v1
// @securityDefinitions.apikey Bearer
// @in header
// @name Authorization
func main() {
	cfgPath := flag.String("config", envOr("AIH_CONFIG", "configs/config.yaml"), "配置文件路径")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	lg, err := logger.Init(cfg.Log)
	if err != nil {
		log.Fatalf("init logger: %v", err)
	}
	defer func() { _ = lg.Sync() }()

	if cfg.AI.Provider != "mock" && cfg.AI.APIKey == "" {
		lg.Warn("ai.api_key 未配置（provider=" + cfg.AI.Provider + "），AI 会话问答将不可用，其余业务不受影响")
	}

	db, err := bootstrap.NewDB(cfg)
	if err != nil {
		lg.Fatal("connect mysql failed: " + err.Error())
	}
	defer func() {
		if err := bootstrap.CloseDB(db); err != nil {
			lg.Warn("close mysql: " + err.Error())
		}
	}()

	rdb, err := bootstrap.NewRedis(cfg)
	if err != nil {
		lg.Warn("connect redis failed, cache will be degraded: " + err.Error())
	}

	app := bootstrap.New(cfg, db, cache.New(rdb))

	srv := &http.Server{
		Addr:         cfg.Server.Addr,
		Handler:      app.Router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	go func() {
		lg.Info("http server listening on " + cfg.Server.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			lg.Fatal("listen: " + err.Error())
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		lg.Error("server forced to shutdown: " + err.Error())
	}
	lg.Info("server exited")
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
