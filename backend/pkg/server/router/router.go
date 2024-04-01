package router

import (
	"context"
	"encoding/gob"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/viper"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/veops/oneterm/docs"
	"github.com/veops/oneterm/pkg/conf"
	"github.com/veops/oneterm/pkg/logger"
	"github.com/veops/oneterm/pkg/server/router/middleware"
	"github.com/veops/oneterm/pkg/util"
)

var routeGroup []*GroupRoute

func Server(cfg *conf.ConfigYaml) *http.Server {
	routeConfig()

	srv := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Http.Host, cfg.Http.Port),
		Handler: setupRouter(),
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.L.Error(err.Error())
			os.Exit(1)
		}
	}()

	logger.L.Info(fmt.Sprintf("start on server:%s", srv.Addr))
	return srv
}

func GracefulExit(srv *http.Server, ch chan struct{}) {
	<-ch

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.L.Error(err.Error())
	}
	logger.L.Info("Shutdown server ...")
}

func routeConfig() {
	var commonRoute []Route
	commonRoute = append(commonRoute, routes...)
	routeGroup = []*GroupRoute{
		{
			Prefix: "/api/oneterm/v1",
			GroupMiddleware: gin.HandlersChain{
				middleware.Error2Resp(),
				middleware.RecoveryWithWriter(),
			},
			SubRoutes: commonRoute,
		},
		{
			Prefix:    "",
			SubRoutes: baseRoutes,
		},
	}
}

func setupRouter() *gin.Engine {
	r := gin.New()
	r.SetTrustedProxies([]string{"0.0.0.0/0", "::/0"})
	r.MaxMultipartMemory = 128 << 20
	r.Use(
		middleware.GinLogger(logger.L),
		middleware.LogRequest(),
		middleware.Cors(),
		middleware.GinRecovery(logger.L, true))
	// sso
	gob.Register(map[string]any{}) // important!
	store := cookie.NewStore([]byte(viper.GetString("gateway.secretKey")))
	r.Use(sessions.Sessions("session", store))

	routeGroupsMap := make(map[string]*gin.RouterGroup)
	for _, gRoute := range routeGroup {
		if _, ok := routeGroupsMap[gRoute.Prefix]; !ok {
			routeGroupsMap[gRoute.Prefix] = r.Group(gRoute.Prefix)
		}
		for _, gMiddleware := range gRoute.GroupMiddleware {
			routeGroupsMap[gRoute.Prefix].Use(gMiddleware)
		}

		for _, subRoute := range gRoute.SubRoutes {
			length := len(subRoute.Middleware) + 2
			routes := make([]any, length)
			routes[0] = subRoute.Pattern
			for i, v := range subRoute.Middleware {
				routes[i+1] = v
			}
			routes[length-1] = subRoute.HandlerFunc

			util.CallReflect(
				routeGroupsMap[gRoute.Prefix],
				subRoute.Method,
				routes...)
		}
	}
	r.Handle("GET", "/metrics", gin.WrapH(promhttp.Handler()))
	// swagger
	docs.SwaggerInfo.Title = "ONETERM API"
	docs.SwaggerInfo.BasePath = "/api/oneterm/v1"
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	return r
}
