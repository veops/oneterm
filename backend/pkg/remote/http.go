package remote

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/spf13/cast"
	"go.uber.org/zap"

	"github.com/veops/oneterm/pkg/cache"
	"github.com/veops/oneterm/pkg/config"
	"github.com/veops/oneterm/pkg/logger"
)

var (
	RC = resty.NewWithClient(&http.Client{}).SetRetryCount(3)
)

func GetAclToken(ctx context.Context) (res string, err error) {
	res, err = cache.RC.Get(ctx, "aclToken").Result()
	if err == nil {
		return
	}
	aclConfig := config.Cfg.Auth.Acl

	url := fmt.Sprintf("%s%s", aclConfig.Url, "/acl/apps/token")
	secretHash := md5.Sum([]byte(aclConfig.SecretKey))
	secretKey := hex.EncodeToString(secretHash[:])

	data := make(map[string]string)
	resp, err := RC.R().
		SetBody(map[string]any{"app_id": aclConfig.AppId, "secret_key": secretKey}).
		SetResult(&data).
		Post(url)
	if err = HandleErr(err, resp, func(dt map[string]any) bool { return dt["token"] != "" }); err != nil {
		return
	}

	res = data["token"]
	_, err = cache.RC.SetNX(ctx, "aclToken", res, time.Hour).Result()
	return
}

func HandleErr(e error, resp *resty.Response, isOk func(dt map[string]any) bool) (err error) {
	pc, _, _, _ := runtime.Caller(1)

	defer func() {
		if err != nil {
			bs, _ := json.Marshal(resp.Request.Body)
			logger.L().Error(fmt.Sprintf("%s failed", runtime.FuncForPC(pc).Name()), zap.String("url", resp.Request.URL), zap.String("req", string(bs)), zap.String("resp", resp.String()))
		}
	}()

	err = e
	if err != nil {
		return err
	}

	dt := make(map[string]any)
	err = json.Unmarshal(resp.Body(), &dt)
	if err != nil {
		return err
	}

	if resp.StatusCode() != 200 || (isOk != nil && !isOk(dt)) {
		err = &RemoteError{
			HttpCode: resp.StatusCode(),
			Resp:     dt,
		}
		return
	}
	return nil
}

type RemoteError struct {
	HttpCode int
	Resp     map[string]any
}

func (r *RemoteError) Error() string {
	return cast.ToString(r.Resp["message"])
}
