package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/mitchellh/mapstructure"
	"github.com/oklog/run"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/veops/oneterm/pkg/conf"
	"github.com/veops/oneterm/pkg/logger"
	sshproto "github.com/veops/oneterm/pkg/proto/ssh"
	cfg "github.com/veops/oneterm/pkg/proto/ssh/config"
	"github.com/veops/oneterm/pkg/proto/ssh/handler"
	"github.com/veops/oneterm/pkg/proto/ssh/tasks"
)

const (
	componentServer = "./server"
)

var (
	configFilePath string
)

var cmdRun = &cobra.Command{
	Use:     "ssh",
	Example: fmt.Sprintf("%s ssh -c apps", componentServer),
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
	handler.I18nInit(conf.Cfg.I18nDir)

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
			_ = sshproto.Run(fmt.Sprintf("%s:%d", cfg.SSHConfig.Ip, cfg.SSHConfig.Port),
				cfg.SSHConfig.Api,
				cfg.SSHConfig.Token,
				cfg.SSHConfig.PrivateKeyPath,
				conf.Cfg.SecretKey)
			<-cancel
			return nil
		}, func(err error) {
			close(cancel)
		})
	}
	{
		ctx, cancel := context.WithCancel(context.Background())
		gr.Add(func() error {
			tasks.LoopCheck(ctx, cfg.SSHConfig.Api, cfg.SSHConfig.Token)
			return nil
		}, func(err error) {
			cancel()
		})
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

	if sc, ok := conf.Cfg.Protocols["ssh"]; ok {
		er := mapstructure.Decode(sc, &cfg.SSHConfig)
		if er != nil {
			panic(er)
		}
		switch v := sc.(type) {
		case map[string]interface{}:
			if v1, ok := v["ip"]; ok {
				cfg.SSHConfig.Ip = v1.(string)
			} else {
				cfg.SSHConfig.Ip = "127.0.0.1"
			}
			if v1, ok := v["port"]; ok {
				cfg.SSHConfig.Port = v1.(int)
			} else {
				cfg.SSHConfig.Port = 45622
			}
			//cfg.SSHConfig.Api = fmt.Sprintf("%v", v["api"])
			//cfg.SSHConfig.Token = fmt.Sprintf("%v", v["token"])
			//cfg.SSHConfig.WebUser = v["webUser"]
		}
	}
}
