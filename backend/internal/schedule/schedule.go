package schedule

import (
	"context"
	"time"

	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/pkg/logger"
	"go.uber.org/zap"
)

var (
	ctx, cancel    = context.WithCancel(context.Background())
	scheduleConfig = model.GetDefaultScheduleConfig()
)

func init() {
	UpdateConfig()
}

func RunSchedule() (err error) {
	logger.L().Info("Starting scheduler with configuration",
		zap.Duration("connectable_check_interval", scheduleConfig.ConnectableCheckInterval),
		zap.Duration("config_update_interval", scheduleConfig.ConfigUpdateInterval),
		zap.Int("batch_size", scheduleConfig.BatchSize),
		zap.Int("concurrent_workers", scheduleConfig.ConcurrentWorkers))

	connectableTicker := time.NewTicker(scheduleConfig.ConnectableCheckInterval)
	// configTicker := time.NewTicker(scheduleConfig.ConfigUpdateInterval)

	defer connectableTicker.Stop()
	// defer configTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.L().Info("Scheduler stopped")
			return
		case <-connectableTicker.C:
			go func() {
				if err := UpdateConnectables(); err != nil {
					logger.L().Error("Failed to update connectables", zap.Error(err))
				}
			}()
			// case <-configTicker.C:
			// 	UpdateConfig()
		}
	}
}

func StopSchedule() {
	defer cancel()
	logger.L().Info("Stopping scheduler")
}
