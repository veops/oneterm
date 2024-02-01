package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/veops/oneterm/pkg/logger"
	"github.com/veops/oneterm/pkg/server/controller"
	"github.com/veops/oneterm/pkg/server/router/middleware"
)

var (
	c = controller.NewController()

	baseRoutes = []Route{
		{
			Name:    "a health check, just for monitoring",
			Method:  "GET",
			Pattern: "/-/health",
			HandlerFunc: func(ctx *gin.Context) {
				ctx.String(http.StatusOK, "OK")
			},
		},
		{
			Name:    "favicon.ico",
			Method:  "GET",
			Pattern: "/favicon.ico",
			HandlerFunc: func(ctx *gin.Context) {
			},
		},
		{
			Name:    "change the log level",
			Method:  "PUT",
			Pattern: "/-/log/level",
			HandlerFunc: func(ctx *gin.Context) {
				logger.AtomicLevel.ServeHTTP(ctx.Writer, ctx.Request)
			},
		},
	}

	routes = []Route{
		// account
		{
			Name:        "create a account",
			Method:      "POST",
			Pattern:     "account",
			HandlerFunc: c.CreateAccount,
			Middleware:  gin.HandlersChain{middleware.Auth()},
		},
		{
			Name:        "delete a account",
			Method:      "DELETE",
			Pattern:     "account/:id",
			HandlerFunc: c.DeleteAccount,
			Middleware:  gin.HandlersChain{middleware.Auth()},
		},
		{
			Name:        "update a account",
			Method:      "PUT",
			Pattern:     "account/:id",
			HandlerFunc: c.UpdateAccount,
			Middleware:  gin.HandlersChain{middleware.Auth()},
		},
		{
			Name:        "query accounts",
			Method:      "GET",
			Pattern:     "account",
			HandlerFunc: c.GetAccounts,
			Middleware:  gin.HandlersChain{middleware.Auth()},
		},
		// asset
		{
			Name:        "create a asset",
			Method:      "POST",
			Pattern:     "asset",
			HandlerFunc: c.CreateAsset,
			Middleware:  gin.HandlersChain{middleware.Auth()},
		},
		{
			Name:        "delete a asset",
			Method:      "DELETE",
			Pattern:     "asset/:id",
			HandlerFunc: c.DeleteAsset,
			Middleware:  gin.HandlersChain{middleware.Auth()},
		},
		{
			Name:        "update a asset",
			Method:      "PUT",
			Pattern:     "asset/:id",
			HandlerFunc: c.UpdateAsset,
			Middleware:  gin.HandlersChain{middleware.Auth()},
		},
		{
			Name:        "query assets",
			Method:      "GET",
			Pattern:     "asset",
			HandlerFunc: c.GetAssets,
			Middleware:  gin.HandlersChain{middleware.Auth()},
		},
		{
			Name:        "update asset by server",
			Method:      "PUT",
			Pattern:     "asset/update_by_server",
			HandlerFunc: c.UpdateByServer,
			Middleware:  gin.HandlersChain{middleware.AuthToken()},
		},
		{
			Name:        "query asset by server",
			Method:      "GET",
			Pattern:     "asset/query_by_server",
			HandlerFunc: c.QueryByServer,
			Middleware:  gin.HandlersChain{middleware.AuthToken()},
		},
		// command
		{
			Name:        "create a command",
			Method:      "POST",
			Pattern:     "command",
			HandlerFunc: c.CreateCommand,
			Middleware:  gin.HandlersChain{middleware.Auth()},
		},
		{
			Name:        "delete a command",
			Method:      "DELETE",
			Pattern:     "command/:id",
			HandlerFunc: c.DeleteCommand,
			Middleware:  gin.HandlersChain{middleware.Auth()},
		},
		{
			Name:        "update a command",
			Method:      "PUT",
			Pattern:     "command/:id",
			HandlerFunc: c.UpdateCommand,
			Middleware:  gin.HandlersChain{middleware.Auth()},
		},
		{
			Name:        "query commands",
			Method:      "GET",
			Pattern:     "command",
			HandlerFunc: c.GetCommands,
			Middleware:  gin.HandlersChain{middleware.Auth()},
		},
		{
			Name:        "modify config",
			Method:      "POST",
			Pattern:     "config",
			HandlerFunc: c.PostConfig,
			Middleware:  gin.HandlersChain{middleware.Auth()},
		},
		{
			Name:        "query config",
			Method:      "GET",
			Pattern:     "config",
			HandlerFunc: c.GetConfig,
			Middleware:  gin.HandlersChain{middleware.Auth()},
		},
		// gateway
		{
			Name:        "create a gateway",
			Method:      "POST",
			Pattern:     "gateway",
			HandlerFunc: c.CreateGateway,
			Middleware:  gin.HandlersChain{middleware.Auth()},
		},
		{
			Name:        "delete a gateway",
			Method:      "DELETE",
			Pattern:     "gateway/:id",
			HandlerFunc: c.DeleteGateway,
			Middleware:  gin.HandlersChain{middleware.Auth()},
		},
		{
			Name:        "update a gateway",
			Method:      "PUT",
			Pattern:     "gateway/:id",
			HandlerFunc: c.UpdateGateway,
			Middleware:  gin.HandlersChain{middleware.Auth()},
		},
		{
			Name:        "query gateways",
			Method:      "GET",
			Pattern:     "gateway",
			HandlerFunc: c.GetGateways,
			Middleware:  gin.HandlersChain{middleware.Auth()},
		},
		// node
		{
			Name:        "create a node",
			Method:      "POST",
			Pattern:     "node",
			HandlerFunc: c.CreateNode,
			Middleware:  gin.HandlersChain{middleware.Auth()},
		},
		{
			Name:        "delete a node",
			Method:      "DELETE",
			Pattern:     "node/:id",
			HandlerFunc: c.DeleteNode,
			Middleware:  gin.HandlersChain{middleware.Auth()},
		},
		{
			Name:        "update a node",
			Method:      "PUT",
			Pattern:     "node/:id",
			HandlerFunc: c.UpdateNode,
			Middleware:  gin.HandlersChain{middleware.Auth()},
		},
		{
			Name:        "query nodes",
			Method:      "GET",
			Pattern:     "node",
			HandlerFunc: c.GetNodes,
			Middleware:  gin.HandlersChain{middleware.Auth()},
		},
		// publicKey
		{
			Name:        "create a publicKey",
			Method:      "POST",
			Pattern:     "public_key",
			HandlerFunc: c.CreatePublicKey,
			Middleware:  gin.HandlersChain{middleware.Auth()},
		},
		{
			Name:        "delete a publicKey",
			Method:      "DELETE",
			Pattern:     "public_key/:id",
			HandlerFunc: c.DeletePublicKey,
			Middleware:  gin.HandlersChain{middleware.Auth()},
		},
		{
			Name:        "update a publicKey",
			Method:      "PUT",
			Pattern:     "public_key/:id",
			HandlerFunc: c.UpdatePublicKey,
			Middleware:  gin.HandlersChain{middleware.Auth()},
		},
		{
			Name:        "query publicKeys",
			Method:      "GET",
			Pattern:     "public_key",
			HandlerFunc: c.GetPublicKeys,
			Middleware:  gin.HandlersChain{middleware.Auth()},
		},
		{
			Name:        "auth by publicKey or password",
			Method:      "POST",
			Pattern:     "public_key/auth",
			HandlerFunc: c.Auth,
			Middleware:  gin.HandlersChain{middleware.AuthToken()},
		},
		//stat
		{
			Name:        "query stat asset type",
			Method:      "GET",
			Pattern:     "stat/assettype",
			HandlerFunc: c.StatAssetType,
			Middleware:  gin.HandlersChain{middleware.Auth()},
		},
		{
			Name:        "query stat count",
			Method:      "GET",
			Pattern:     "stat/count",
			HandlerFunc: c.StatCount,
			Middleware:  gin.HandlersChain{middleware.Auth()},
		},
		{
			Name:        "query stat count of user",
			Method:      "GET",
			Pattern:     "stat/count/ofuser",
			HandlerFunc: c.StatCountOfUser,
			Middleware:  gin.HandlersChain{middleware.Auth()},
		},
		{
			Name:        "query stat account",
			Method:      "GET",
			Pattern:     "stat/account",
			HandlerFunc: c.StatAccount,
			Middleware:  gin.HandlersChain{middleware.Auth()},
		},
		{
			Name:        "query stat asset",
			Method:      "GET",
			Pattern:     "stat/asset",
			HandlerFunc: c.StatAsset,
			Middleware:  gin.HandlersChain{middleware.Auth()},
		},
		{
			Name:        "query stat rank of user",
			Method:      "GET",
			Pattern:     "stat/rank/ofuser",
			HandlerFunc: c.StatRankOfUser,
			Middleware:  gin.HandlersChain{middleware.Auth()},
		},
		//session
		{
			Name:        "query session",
			Method:      "GET",
			Pattern:     "session",
			HandlerFunc: c.GetSessions,
			Middleware:  gin.HandlersChain{middleware.Auth()},
		},
		{
			Name:        "query session cmds",
			Method:      "GET",
			Pattern:     "session/:session_id/cmd",
			HandlerFunc: c.GetSessionCmds,
			Middleware:  gin.HandlersChain{middleware.Auth()},
		},
		{
			Name:        "query session option asset",
			Method:      "GET",
			Pattern:     "session/option/asset",
			HandlerFunc: c.GetSessionOptionAsset,
			Middleware:  gin.HandlersChain{middleware.Auth()},
		},
		{
			Name:        "query session option client ip",
			Method:      "GET",
			Pattern:     "session/option/clientip",
			HandlerFunc: c.GetSessionOptionClientIp,
			Middleware:  gin.HandlersChain{middleware.Auth()},
		},
		{
			Name:        "query session replay",
			Method:      "GET",
			Pattern:     "session/replay/:session_id",
			HandlerFunc: c.GetSessionReplay,
			Middleware:  gin.HandlersChain{middleware.Auth()},
		},
		{
			Name:        "create sesssin replay",
			Method:      "POST",
			Pattern:     "session/replay/:session_id",
			HandlerFunc: c.CreateSessionReplay,
			Middleware:  gin.HandlersChain{middleware.AuthToken()},
		},
		{
			Name:        "upsert session",
			Method:      "POST",
			Pattern:     "session",
			HandlerFunc: c.UpsertSession,
			Middleware:  gin.HandlersChain{middleware.AuthToken()},
		},
		{
			Name:        "create sesssin cmd",
			Method:      "POST",
			Pattern:     "session/cmd",
			HandlerFunc: c.CreateSessionCmd,
			Middleware:  gin.HandlersChain{middleware.AuthToken()},
		},
		//history
		{
			Name:        "query history",
			Method:      "GET",
			Pattern:     "history",
			HandlerFunc: c.GetHistories,
			Middleware:  gin.HandlersChain{middleware.Auth()},
		},
		{
			Name:        "query history type mapping",
			Method:      "GET",
			Pattern:     "history/type/mapping",
			HandlerFunc: c.GetHistoryTypeMapping,
			Middleware:  gin.HandlersChain{middleware.Auth()},
		},
		//connect
		{
			Name:        "connect",
			Method:      "POST",
			Pattern:     "connect/:asset_id/:account_id/:protocol",
			HandlerFunc: c.Connect,
			Middleware:  gin.HandlersChain{middleware.Auth()},
		},
		{
			Name:        "connecting",
			Method:      "GET",
			Pattern:     "connect/:session_id",
			HandlerFunc: c.Connecting,
			Middleware:  gin.HandlersChain{middleware.Auth()},
		},
		{
			Name:        "connect monitor",
			Method:      "GET",
			Pattern:     "connect/monitor/:session_id",
			HandlerFunc: c.ConnectMonitor,
			Middleware:  gin.HandlersChain{middleware.Auth()},
		},
		{
			Name:        "connect close",
			Method:      "POST",
			Pattern:     "connect/close/:session_id",
			HandlerFunc: c.ConnectClose,
			Middleware:  gin.HandlersChain{middleware.Auth()},
		},
	}
)

type Route struct {
	Name        string
	Method      string
	Pattern     string
	HandlerFunc func(ctx *gin.Context)
	Middleware  []gin.HandlerFunc
}

type GroupRoute struct {
	Prefix          string
	GroupMiddleware gin.HandlersChain
	SubRoutes       []Route
}
