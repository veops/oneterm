package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/zap"

	"github.com/veops/oneterm/api/controller"
	"github.com/veops/oneterm/conf"
	"github.com/veops/oneterm/docs"
	"github.com/veops/oneterm/logger"
)

var (
	ctx, cancel = context.WithCancel(context.Background())
	srv         = &http.Server{}
)

func RunApi() error {
	c := controller.Controller{}
	r := gin.New()
	r.SetTrustedProxies([]string{"0.0.0.0/0", "::/0"})
	r.MaxMultipartMemory = 128 << 20
	r.Use(gin.Recovery(), ginLogger())

	docs.SwaggerInfo.Title = "ONETERM API"
	docs.SwaggerInfo.BasePath = "/api/oneterm/v1"
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	v1 := r.Group("/api/oneterm/v1", Error2Resp(), auth())
	{
		account := v1.Group("account")
		{
			account.POST("", c.CreateAccount)
			account.DELETE("/:id", c.DeleteAccount)
			account.PUT("/:id", c.UpdateAccount)
			account.GET("", c.GetAccounts)
		}

		asset := v1.Group("asset")
		{
			asset.POST("", c.CreateAsset)
			asset.DELETE("/:id", c.DeleteAsset)
			asset.PUT("/:id", c.UpdateAsset)
			asset.GET("", c.GetAssets)
		}

		node := v1.Group("node")
		{
			node.POST("", c.CreateNode)
			node.DELETE("/:id", c.DeleteNode)
			node.PUT("/:id", c.UpdateNode)
			node.GET("", c.GetNodes)
		}

		publicKey := v1.Group("public_key")
		{
			publicKey.POST("", c.CreatePublicKey)
			publicKey.DELETE("/:id", c.DeletePublicKey)
			publicKey.PUT("/:id", c.UpdatePublicKey)
			publicKey.GET("", c.GetPublicKeys)
		}

		gateway := v1.Group("gateway")
		{
			gateway.POST("", c.CreateGateway)
			gateway.DELETE("/:id", c.DeleteGateway)
			gateway.PUT("/:id", c.UpdateGateway)
			gateway.GET("", c.GetGateways)
		}

		stat := v1.Group("stat")
		{
			stat.GET("assettype", c.StatAssetType)
			stat.GET("count", c.StatCount)
			stat.GET("count/ofuser", c.StatCountOfUser)
			stat.GET("account", c.StatAccount)
			stat.GET("asset", c.StatAsset)
			stat.GET("rank/ofuser", c.StatRankOfUser)
		}

		command := v1.Group("command")
		{
			command.POST("", c.CreateCommand)
			command.DELETE("/:id", c.DeleteCommand)
			command.PUT("/:id", c.UpdateCommand)
			command.GET("", c.GetCommands)
		}

		session := v1.Group("session")
		{
			session.GET("", c.GetSessions)
			session.GET("/:session_id/cmd", c.GetSessionCmds)
			session.GET("/option/asset", c.GetSessionOptionAsset)
			session.GET("/option/clientip", c.GetSessionOptionClientIp)
			session.GET("/replay/:session_id", c.GetSessionReplay)
		}

		connect := v1.Group("connect")
		{
			connect.GET("/:asset_id/:account_id/:protocol", c.Connect)
			connect.GET("/monitor/:session_id", c.ConnectMonitor)
			connect.POST("/close/:session_id", c.ConnectClose)
		}

		file := v1.Group("file")
		{
			file.GET("/history", c.GetFileHistory)
			file.GET("/ls/:asset_id/:account_id", c.FileLS)
			file.POST("/mkdir/:asset_id/:account_id", c.FileMkdir)
			file.POST("/upload/:asset_id/:account_id", c.FileUpload)
			file.GET("/download/:asset_id/:account_id", c.FileDownload)
		}

		config := v1.Group("config")
		{
			config.GET("", c.GetConfig)
			config.POST("", c.PostConfig)
		}

		history := v1.Group("history")
		{
			history.GET("", c.GetHistories)
			history.GET("/type/mapping", c.GetHistoryTypeMapping)
		}

		share := v1.Group("/share")
		{
			share.POST("", c.CreateShare)
			share.DELETE("/:id", c.DeleteShare)
			share.GET("", c.GetShare)
		}
		r.GET("/api/oneterm/v1/share/connect/:uuid", Error2Resp(), c.ConnectShare)

		authorization := v1.Group("/authorization")
		{
			authorization.POST("", c.UpsertAuthorization)
			authorization.DELETE("/:id", c.DeleteAccount)
			authorization.GET("", c.GetAuthorizations)
		}
	}

	srv.Addr = fmt.Sprintf("%s:%d", conf.Cfg.Http.Host, conf.Cfg.Http.Port)
	srv.Handler = r
	err := srv.ListenAndServe()
	if err != nil {
		logger.L().Fatal("start http failed", zap.Error(err))
	}
	return err
}

func StopApi() {
	defer cancel()
	srv.Shutdown(ctx)
}
