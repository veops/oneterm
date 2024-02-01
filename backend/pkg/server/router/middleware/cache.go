package middleware

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/sync/singleflight"

	"github.com/veops/oneterm/pkg/logger"
	"github.com/veops/oneterm/pkg/server/storage/cache/redis"
)

var (
	cachePrefix = "App::HttpCache"
)

type responseCache struct {
	Status int
	Header http.Header
	Data   []byte
}

func (c *responseCache) fillWithCacheWriter(cacheWriter *responseCacheWriter) {
	c.Status = cacheWriter.Status()
	c.Data = cacheWriter.body.Bytes()
	c.Header = cacheWriter.Header().Clone()
}

// responseCacheWriter
type responseCacheWriter struct {
	gin.ResponseWriter
	body bytes.Buffer
}

func (w *responseCacheWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func replyWithCache(c *gin.Context, respCache *responseCache) {
	c.Writer.WriteHeader(respCache.Status)

	for key, values := range respCache.Header {
		for _, val := range values {
			c.Writer.Header().Set(key, val)
		}
	}

	if _, err := c.Writer.Write(respCache.Data); err != nil {
		logger.L.Error(err.Error())
	}
	c.Abort()
}

func RCache(ctx context.Context, defaultExpire time.Duration) gin.HandlerFunc {
	sfGroup := singleflight.Group{}
	return func(c *gin.Context) {

		cacheKey := fmt.Sprintf("%s::%s", cachePrefix, c.Request.RequestURI)
		cacheDuration := defaultExpire

		// read cache first
		{
			respCache := &responseCache{}
			err := RGet(ctx, cacheKey, &respCache)
			if err == nil {
				replyWithCache(c, respCache)
				return
			}
		}

		cacheWriter := &responseCacheWriter{ResponseWriter: c.Writer}
		c.Writer = cacheWriter

		inFlight := false
		rawRespCache, _, _ := sfGroup.Do(cacheKey, func() (any, error) {
			forgetTimer := time.AfterFunc(time.Second*15, func() {
				sfGroup.Forget(cacheKey)
			})
			defer forgetTimer.Stop()

			c.Next()

			inFlight = true
			respCache := &responseCache{}
			respCache.fillWithCacheWriter(cacheWriter)
			// only cache 2xx response
			if !c.IsAborted() && cacheWriter.Status() < 300 && cacheWriter.Status() >= 200 {
				if err := RSet(ctx, cacheKey, respCache, cacheDuration); err != nil {
					logger.L.Error(err.Error())
				}
			}
			return respCache, nil
		})

		if !inFlight {
			replyWithCache(c, rawRespCache.(*responseCache))
			return
		}
	}
}

// redis

func RSet(ctx context.Context, key string, value any, expire time.Duration) error {
	payload, err := Serialize(value)
	if err != nil {
		return err
	}

	_, err = redis.RC.SetEx(ctx, key, payload, expire).Result()
	return err
}

func RDelete(ctx context.Context, key string) error {
	_, err := redis.RC.Del(ctx, key).Result()
	return err
}

func RGet(ctx context.Context, key string, value any) error {
	r, err := redis.RC.Get(ctx, key).Bytes()
	if err != nil {
		return err
	}
	return Deserialize(r, value)
}

// codec

func Serialize(value any) ([]byte, error) {
	var b bytes.Buffer
	encoder := gob.NewEncoder(&b)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func Deserialize(byt []byte, ptr any) (err error) {
	b := bytes.NewBuffer(byt)
	decoder := gob.NewDecoder(b)
	if err = decoder.Decode(ptr); err != nil {
		return err
	}
	return nil
}
