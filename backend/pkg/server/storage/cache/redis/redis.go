package redis

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/veops/oneterm/pkg/conf"
)

var (
	// RC redis cache client
	RC *redis.Client
)

func Init(cfg *conf.RedisConfig) (err error) {
	if cfg == nil {
		return
	}

	ctx := context.Background()
	readTimeout := time.Duration(30) * time.Second
	writeTimeout := time.Duration(30) * time.Second
	RC = redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		DB:           cfg.Db,
		Password:     cfg.Password,
		PoolSize:     cfg.PoolSize,
		MaxIdleConns: cfg.MaxIdle,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	})

	if _, err = RC.Ping(ctx).Result(); err != nil {
		return err
	}
	return nil
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
