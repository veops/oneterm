package schedule

import (
	"context"
	"time"
)

var (
	ctx, cancel = context.WithCancel(context.Background())
)

func init() {
	UpdateConfig()
}

func RunSchedule() (err error) {
	tk2h := time.NewTicker(time.Hour * 2)
	tk1m := time.NewTicker(time.Minute)
	for {
		select {
		case <-ctx.Done():
			return
		case <-tk2h.C:
			UpdateConnectables()
		case <-tk1m.C:
			UpdateConfig()
		}
	}
}

func StopSchedule() {
	defer cancel()
}
