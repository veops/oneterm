package redis

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/veops/oneterm/conf"
	"github.com/veops/oneterm/logger"
)

var (
	// RC redis cache client
	RC *redis.Client
)

func init() {
	ctx := context.Background()
	RC = redis.NewClient(&redis.Options{
		Addr:     conf.Cfg.Redis.Addr,
		Password: conf.Cfg.Redis.Password,
	})

	if _, err := RC.Ping(ctx).Result(); err != nil {
		logger.L().Fatal("ping redis failed", zap.Error(err))
	}
}

func Get(ctx context.Context, key string, dst any) (err error) {
	bs, err := RC.Get(ctx, key).Bytes()
	if err != nil {
		return
	}
	return json.Unmarshal(bs, dst)
}

func SetEx(ctx context.Context, key string, src any, exp time.Duration) (err error) {
	bs, err := json.Marshal(src)
	if err != nil {
		return
	}
	return RC.SetEx(ctx, key, bs, exp).Err()
}
