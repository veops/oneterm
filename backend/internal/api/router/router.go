package router

import (
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"strings"

	"github.com/veops/oneterm/internal/api/controller"
	"github.com/veops/oneterm/internal/api/docs"
	"github.com/veops/oneterm/internal/api/middleware"
)

func SetupRouter(r *gin.Engine) {
	r.SetTrustedProxies([]string{"0.0.0.0/0", "::/0"})
	r.MaxMultipartMemory = 1 << 20 // 1MB to prevent memory overflow
	r.Use(gin.Recovery(), middleware.LoggerMiddleware())

	// Start web session cleanup routine
	controller.StartSessionCleanupRoutine()

	// Subdomain proxy middleware for asset- subdomains
	webProxy := controller.NewWebProxyController()
	r.Use(func(c *gin.Context) {
		host := c.Request.Host

		// Check if this is an asset subdomain request
		if strings.HasPrefix(host, "asset-") {
			// Allow API requests to pass through to normal routing
			if strings.HasPrefix(c.Request.URL.Path, "/api/oneterm/v1/") {
				c.Next()
				return
			}

			// Handle external redirect requests
			if c.Request.URL.Path == "/external" {
				webProxy.HandleExternalRedirect(c)
				return
			}

			// Handle normal proxy requests
			webProxy.ProxyWebRequest(c)
			return
		}

		c.Next()
	})

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
			account.POST("/:id/credentials", c.GetAccountCredentials)
			account.GET("/:id/credentials2", c.GetAccountCredentials2)
		}

		asset := v1.Group("asset")
		{
			asset.POST("", c.CreateAsset)
			asset.DELETE("/:id", c.DeleteAsset)
			asset.PUT("/:id", c.UpdateAsset)
			asset.GET("", c.GetAssets)
			asset.GET("/:id/permissions", c.GetAssetPermissions)
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

			// Legacy asset-based file operations (for backward compatibility)
			file.GET("/ls/:asset_id/:account_id", c.FileLS)
			file.POST("/mkdir/:asset_id/:account_id", c.FileMkdir)
			file.POST("/upload/:asset_id/:account_id", c.FileUpload)
			file.GET("/download/:asset_id/:account_id", c.FileDownload)

			sftpFile := file.Group("/session/:session_id")
			{
				sftpFile.GET("/ls", c.SftpFileLS)
				sftpFile.POST("/mkdir", c.SftpFileMkdir)
				sftpFile.POST("/upload", c.SftpFileUpload)
				sftpFile.GET("/download", c.SftpFileDownload)
			}

			// File transfer progress tracking
			file.GET("/transfer/progress/id/:transfer_id", c.TransferProgressById)
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
			authorization.DELETE("/:id", c.DeleteAuthorization)
			authorization.GET("", c.GetAuthorizations)
		}

		authorizationV2 := v1.Group("/authorization_v2")
		{
			authorizationV2.POST("", c.CreateAuthorizationV2)
			authorizationV2.GET("", c.GetAuthorizationsV2)
			authorizationV2.GET("/:id", c.GetAuthorizationV2)
			authorizationV2.PUT("/:id", c.UpdateAuthorizationV2)
			authorizationV2.DELETE("/:id", c.DeleteAuthorizationV2)
			authorizationV2.POST("/:id/clone", c.CloneAuthorizationV2)
			authorizationV2.POST("/check", c.CheckPermissionV2)
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

		// Storage management routes
		storage := v1.Group("/storage")
		{
			storage.GET("/configs", c.ListStorageConfigs)
			storage.GET("/configs/:id", c.GetStorageConfig)
			storage.POST("/configs", c.CreateStorageConfig)
			storage.PUT("/configs/:id", c.UpdateStorageConfig)
			storage.DELETE("/configs/:id", c.DeleteStorageConfig)
			storage.POST("/test-connection", c.TestStorageConnection)
			storage.GET("/health", c.GetStorageHealth)
			// storage.GET("/metrics", c.GetStorageMetrics)
			// storage.POST("/metrics/refresh", c.RefreshStorageMetrics)
			storage.PUT("/configs/:id/set-primary", c.SetPrimaryStorage)
			storage.PUT("/configs/:id/toggle", c.ToggleStorageProvider)
		}

		// Time template management routes
		timeTemplate := v1.Group("/time_template")
		{
			timeTemplate.POST("", c.CreateTimeTemplate)
			timeTemplate.DELETE("/:id", c.DeleteTimeTemplate)
			timeTemplate.PUT("/:id", c.UpdateTimeTemplate)
			timeTemplate.GET("", c.GetTimeTemplates)
			timeTemplate.GET("/builtin", c.GetBuiltInTimeTemplates)
			timeTemplate.POST("/check", c.CheckTimeAccess)
			timeTemplate.POST("/init", c.InitBuiltInTemplates)
		}

		// Command template management routes
		commandTemplate := v1.Group("/command_template")
		{
			commandTemplate.POST("", c.CreateCommandTemplate)
			commandTemplate.DELETE("/:id", c.DeleteCommandTemplate)
			commandTemplate.PUT("/:id", c.UpdateCommandTemplate)
			commandTemplate.GET("", c.GetCommandTemplates)
			commandTemplate.GET("/builtin", c.GetBuiltInCommandTemplates)
			commandTemplate.GET("/:id/commands", c.GetTemplateCommands)
		}

		// Web proxy management API routes
		webProxyGroup := v1.Group("/web_proxy")
		{
			webProxyGroup.GET("/config/:asset_id", webProxy.GetWebAssetConfig)
			webProxyGroup.POST("/start", webProxy.StartWebSession)
			webProxyGroup.GET("/external_redirect", webProxy.HandleExternalRedirect)
			webProxyGroup.POST("/close", webProxy.CloseWebSession)
			webProxyGroup.GET("/sessions/:asset_id", webProxy.GetActiveWebSessions)
		}

		// Web proxy routes that don't require auth (heartbeat, cleanup)
		webProxyNoAuth := v1AuthAbandoned.Group("/web_proxy")
		{
			webProxyNoAuth.POST("/heartbeat", webProxy.UpdateWebSessionHeartbeat)
			webProxyNoAuth.POST("/cleanup", webProxy.CleanupWebSession)
		}
	}
}
