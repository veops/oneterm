// Package app

package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/oklog/run"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/veops/oneterm/pkg/conf"
	"github.com/veops/oneterm/pkg/logger"
	"github.com/veops/oneterm/pkg/server/cmdb"
	"github.com/veops/oneterm/pkg/server/controller"
	"github.com/veops/oneterm/pkg/server/router"
	"github.com/veops/oneterm/pkg/server/storage/cache/local"
	"github.com/veops/oneterm/pkg/server/storage/cache/redis"
	"github.com/veops/oneterm/pkg/server/storage/db/mysql"
)

const (
	componentServer = "./server"
)

var (
	configFilePath string
)

var cmdRun = &cobra.Command{
	Use:     "run",
	Example: fmt.Sprintf("%s run -c apps", componentServer),
	Short:   "run",
	Long:    `a run test`,
	Args:    cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		Run()
		os.Exit(0)
	},
}

func NewServerCommand() *cobra.Command {
	command := &cobra.Command{
		Use: componentServer,
	}

	cmdRun.PersistentFlags().StringVarP(&configFilePath, "config", "c", "./", "config path")
	command.AddCommand(cmdRun)
	return command
}

func Run() {
	parseConfig(configFilePath)
	gr := run.Group{}
	ctx, logCancel := context.WithCancel(context.Background())

	if err := logger.Init(ctx, conf.Cfg.Log); err != nil {
		fmt.Println("err init failed", err)
		os.Exit(1)
	}

	if err := mysql.Init(conf.Cfg.Mysql); err != nil {
		logger.L.Error("mysql init failed: " + err.Error())
		os.Exit(1)
	}

	if err := redis.Init(conf.Cfg.Redis); err != nil {
		logger.L.Error("redis init failed: " + err.Error())
		os.Exit(1)
	}

	if err := local.Init(); err != nil {
		logger.L.Error("local init failed: " + err.Error())
		os.Exit(1)
	}

	if err := controller.Init(); err != nil {
		logger.L.Error("local init failed: " + err.Error())
		os.Exit(1)
	}

	{
		// Termination handler.
		term := make(chan os.Signal, 1)
		signal.Notify(term, os.Interrupt, syscall.SIGTERM)
		gr.Add(
			func() error {
				<-term
				logger.L.Warn("Received SIGTERM, exiting gracefully...")
				return nil
			},
			func(err error) {},
		)
	}
	{
		cancel := make(chan struct{})
		gr.Add(func() error {
			gin.SetMode(conf.Cfg.Mode)
			srv := router.Server(conf.Cfg)
			router.GracefulExit(srv, cancel)
			return nil
		}, func(err error) {
			close(cancel)
		})
		gr.Add(cmdb.Run, cmdb.Stop)
	}

	if err := gr.Run(); err != nil {
		logger.L.Error(err.Error())
	}

	logger.L.Info("exiting")
	logCancel()
}

func parseConfig(filePath string) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(filePath)
	viper.AddConfigPath(".")

	err := viper.ReadInConfig()
	if err != nil { // Handle errors reading the config file
		panic(fmt.Errorf("fatal error config file: %s", err))
	}

	if err = viper.Unmarshal(&conf.Cfg); err != nil {
		panic(fmt.Sprintf("parse config from config.yaml failed:%s", err))
	}
}
