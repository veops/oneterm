package schedule

import (
	"fmt"
	"time"

	redis "github.com/veops/oneterm/cache"
	mysql "github.com/veops/oneterm/db"
	"github.com/veops/oneterm/model"
)

func UpdateConfig() {
	cfg := &model.Config{}
	err := redis.Get(ctx, "config", cfg)
	if err == nil {
		return
	}
	err = mysql.DB.Model(cfg).First(cfg).Error
	if err != nil {
		return
	}
	redis.SetEx(ctx, "config", cfg, time.Hour)
	model.GlobalConfig.Store(cfg)

	fmt.Println("--------------------------", *model.GlobalConfig.Load())

}
