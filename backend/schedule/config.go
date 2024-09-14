package schedule

import (
	"time"

	redis "github.com/veops/oneterm/cache"
	mysql "github.com/veops/oneterm/db"
	"github.com/veops/oneterm/model"
)

func UpdateConfig() {
	cfg := &model.Config{}
	defer func() {
		redis.SetEx(ctx, "config", cfg, time.Hour)
		model.GlobalConfig.Store(cfg)
	}()
	err := redis.Get(ctx, "config", cfg)
	if err == nil {
		return
	}
	err = mysql.DB.Model(cfg).First(cfg).Error
	if err != nil {
		return
	}
}
