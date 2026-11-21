package bootstrap

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"ai_health/internal/config"
	"ai_health/internal/handler"
	"ai_health/internal/pkg/cache"
	jwtmgr "ai_health/internal/pkg/jwt"
	"ai_health/internal/repository"
	"ai_health/internal/router"
	"ai_health/internal/service"
)

// App 聚合运行期依赖，main 只关心启动与关闭。
type App struct {
	Router *gin.Engine
}

// New 依赖装配入口：repo → service → handler → router，顺序不可颠倒。
func New(cfg *config.Config, db *gorm.DB, store *cache.Store) *App {
	tokens := jwtmgr.New(cfg.JWT.Secret, cfg.JWT.Issuer, cfg.JWT.AccessTTL, cfg.JWT.RefreshTTL)

	userRepo := repository.NewUserRepo(db)
	roleRepo := repository.NewRoleRepo(db)
	menuRepo := repository.NewMenuRepo(db)
	healthRepo := repository.NewHealthRepo(db)
	sportRepo := repository.NewSportRepo(db)
	aiRepo := repository.NewAIRepo(db)

	authSvc := service.NewAuthService(userRepo, roleRepo, menuRepo, store, tokens)
	userSvc := service.NewUserService(authSvc, userRepo, roleRepo, store)
	roleSvc := service.NewRoleService(roleRepo, menuRepo, userRepo, store)
	menuSvc := service.NewMenuService(menuRepo, roleRepo, userRepo, store)
	healthSvc := service.NewHealthService(healthRepo, store)
	sportSvc := service.NewSportService(sportRepo, store)
	aiSvc := service.NewAIService(aiRepo, store, cfg)

	h := handler.New(
		handler.NewAuthHandler(authSvc),
		handler.NewUserHandler(userSvc),
		handler.NewRoleHandler(roleSvc),
		handler.NewMenuHandler(menuSvc),
		handler.NewHealthHandler(healthSvc),
		handler.NewSportHandler(sportSvc),
		handler.NewAIHandler(aiSvc),
	)

	return &App{Router: router.New(cfg, h, tokens, store, authSvc)}
}
