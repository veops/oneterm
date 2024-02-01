package local

import (
	"context"
	"time"

	"github.com/allegro/bigcache/v3"
)

var (
	// LC local cache client
	LC *bigcache.BigCache
)

func Init() error {
	var err error
	if LC, err = bigcache.New(context.Background(), bigcache.DefaultConfig(time.Minute*10)); err != nil {
		return err
	}
	return nil
}
