package schedule

import (
	"time"

	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/pkg/cache"
	dbpkg "github.com/veops/oneterm/pkg/db"
)

func UpdateConfig() {
	cfg := &model.Config{}
	defer func() {
		cache.SetEx(ctx, "config", cfg, time.Hour)
		model.GlobalConfig.Store(cfg)
	}()
	err := cache.Get(ctx, "config", cfg)
	if err == nil {
		return
	}
	err = dbpkg.DB.Model(cfg).First(cfg).Error
	if err != nil {
		return
	}
}
