package main

import (
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/oklog/run"
	"github.com/veops/oneterm/api"
	"github.com/veops/oneterm/logger"
	"go.uber.org/zap"
)

func main() {
	rg := run.Group{}
	{
		term := make(chan os.Signal, 1)
		signal.Notify(term, os.Interrupt, syscall.SIGTERM)
		rg.Add(func() error {
			<-term
			return errors.New("terminated")
		}, func(err error) {})
	}
	{
		rg.Add(func() error {
			return api.RunApi()
		}, func(err error) {
			api.StopApi()
		})
	}

	if err := rg.Run(); err != nil {
		logger.L().Fatal("", zap.Error(err))
	}
}
