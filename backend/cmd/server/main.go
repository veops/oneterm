package main

import (
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/oklog/run"
	"go.uber.org/zap"

	"github.com/veops/oneterm/internal/api"
	"github.com/veops/oneterm/internal/schedule"
	"github.com/veops/oneterm/internal/sshsrv"
	"github.com/veops/oneterm/pkg/logger"
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
	{
		rg.Add(func() error {
			return sshsrv.RunSsh()
		}, func(err error) {
			sshsrv.StopSsh()
		})
	}
	{
		rg.Add(func() error {
			return schedule.RunSchedule()
		}, func(err error) {
			schedule.StopSchedule()
		})
	}

	if err := rg.Run(); err != nil {
		logger.L().Fatal("", zap.Error(err))
	}
}
