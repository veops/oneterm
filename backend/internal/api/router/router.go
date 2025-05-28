package router

import (
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/veops/oneterm/internal/api/controller"
	"github.com/veops/oneterm/internal/api/docs"
	"github.com/veops/oneterm/internal/api/middleware"
)

func SetupRouter(r *gin.Engine) {
	r.SetTrustedProxies([]string{"0.0.0.0/0", "::/0"})
	r.MaxMultipartMemory = 128 << 20
	r.Use(gin.Recovery(), middleware.LoggerMiddleware())

	docs.SwaggerInfo.Title = "ONETERM API"
	docs.SwaggerInfo.BasePath = "/api/oneterm/v1"
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	c := controller.Controller{}

	v1 := r.Group("/api/oneterm/v1", middleware.Error2RespMiddleware(), middleware.AuthMiddleware())
	v1AuthAbandoned := r.Group("/api/oneterm/v1", middleware.Error2RespMiddleware())
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
			config.POST("", c.PostConfig)
		}
		config2 := v1AuthAbandoned.Group("config")
		{
			config2.GET("", c.GetConfig)
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

		r.GET("/api/oneterm/v1/share/connect/:uuid", middleware.Error2RespMiddleware(), c.ConnectShare)

		authorization := v1.Group("/authorization")
		{
			authorization.POST("", c.UpsertAuthorization)
			authorization.DELETE("/:id", c.DeleteAccount)
			authorization.GET("", c.GetAuthorizations)
		}

		quickCommand := v1.Group("/quick_command")
		{
			quickCommand.POST("", c.CreateQuickCommand)
			quickCommand.GET("", c.GetQuickCommands)
			quickCommand.DELETE("/:id", c.DeleteQuickCommand)
			quickCommand.PUT("/:id", c.UpdateQuickCommand)
		}

		preference := v1.Group("/preference")
		{
			preference.GET("", c.GetPreference)
			preference.PUT("", c.UpdatePreference)
		}

		// RDP file transfer routes
		rdpGroup := v1.Group("/rdp")
		{
			rdpGroup.GET("/sessions/:session_id/files", c.RDPFileList)
			rdpGroup.POST("/sessions/:session_id/files/upload", c.RDPFileUpload)
			rdpGroup.GET("/sessions/:session_id/files/download", c.RDPFileDownload)
			rdpGroup.POST("/sessions/:session_id/files/mkdir", c.RDPFileMkdir)
		}
	}
}
