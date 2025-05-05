package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/veops/oneterm/internal/api/router"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/service"
	"github.com/veops/oneterm/pkg/config"
	"github.com/veops/oneterm/pkg/db"
	"github.com/veops/oneterm/pkg/logger"
)

var (
	ctx, cancel = context.WithCancel(context.Background())
	srv         = &http.Server{}
)

func initDB() {
	cfg := db.ConfigFromGlobal()

	if err := db.Init(cfg,
		model.DefaultAccount, model.DefaultAsset, model.DefaultAuthorization, model.DefaultCommand,
		model.DefaultConfig, model.DefaultFileHistory, model.DefaultGateway, model.DefaultHistory,
		model.DefaultNode, model.DefaultPublicKey, model.DefaultSession, model.DefaultSessionCmd,
		model.DefaultShare,
	); err != nil {
		logger.L().Fatal("Failed to init database", zap.Error(err))
	}

	if err := db.DropIndex(&model.Authorization{}, "asset_account_id_del"); err != nil {
		logger.L().Fatal("Failed to drop index", zap.Error(err))
	}

}

// 初始化服务
func initServices() {
	// 初始化授权服务
	service.InitAuthorizationService()

	// 初始化文件服务
	service.InitFileService()

	// 其他服务初始化...
}

func RunApi() error {
	initDB()

	// 初始化服务层
	initServices()

	r := gin.New()

	router.SetupRouter(r)

	srv.Addr = fmt.Sprintf("%s:%d", config.Cfg.Http.Host, config.Cfg.Http.Port)
	srv.Handler = r

	logger.L().Info("Starting HTTP server",
		zap.String("address", srv.Addr))

	err := srv.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		logger.L().Fatal("Start HTTP server failed", zap.Error(err))
	}

	return err
}

func StopApi() {
	defer cancel()

	logger.L().Info("Stopping HTTP server")
	if err := srv.Shutdown(ctx); err != nil {
		logger.L().Error("Stop HTTP server failed", zap.Error(err))
	}
}
