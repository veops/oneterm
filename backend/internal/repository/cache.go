package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/veops/oneterm/internal/model"
	redis "github.com/veops/oneterm/pkg/cache"
	"github.com/veops/oneterm/pkg/db"
)

func GetAllFromCacheDb[T model.Model](ctx context.Context, m T) (res []T, err error) {
	k := fmt.Sprintf("all-%s", m.TableName())
	if err = redis.Get(ctx, k, &res); err == nil {
		return
	}
	if err = db.DB.Model(m).Find(&res).Error; err != nil {
		return
	}
	redis.SetEx(ctx, k, res, time.Hour)
	return
}

func DeleteAllFromCacheDb(ctx context.Context, m model.Model) (err error) {
	k := fmt.Sprintf("all-%s", m.TableName())
	err = redis.RC.Del(ctx, k).Err()
	return
}
