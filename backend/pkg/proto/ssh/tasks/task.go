package tasks

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/veops/oneterm/pkg/logger"
	"github.com/veops/oneterm/pkg/proto/ssh/api"
	"github.com/veops/oneterm/pkg/server/model"
)

func checkPort(server string, port int, timeout time.Duration) bool {
	address := fmt.Sprintf("%s:%d", server, port)
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func checkAssets(assets []*model.Asset) (result map[int]map[string]any, err error) {
	type St struct {
		Id    int
		State bool
	}
	result = map[int]map[string]any{}
	statusChan := make(chan *St, len(assets))
	var wg sync.WaitGroup
	for i := range assets {
		wg.Add(1)
		go func(asset *model.Asset) {
			defer wg.Done()
			var connected bool
			for _, protocol := range asset.Protocols {
				tmp := strings.Split(protocol, ":")
				port := 22
				var er error
				if len(tmp) == 2 {
					port, er = strconv.Atoi(tmp[1])
					if er != nil {
						continue
					}
				}
				connected = checkPort(asset.Ip, port, time.Second*5)
				if connected {
					break
				}
			}
			statusChan <- &St{Id: asset.Id, State: connected}
		}(assets[i])
	}

	wg.Wait()
	for i := 0; i < len(statusChan); i++ {
		v := <-statusChan
		result[v.Id] = map[string]any{"connectable": v.State}
	}
	return
}

func LoopCheck(ctx context.Context, host, token string) {
	assetServer := api.NewAssetServer(host, token)
	ticker := time.NewTicker(time.Second)
	for {
		select {
		case <-ticker.C:
			ticker.Reset(time.Hour)
			res, err := assetServer.AllAssets()
			if err != nil {
				logger.L.Error(err.Error(), zap.String("module", "LoopCheck"))
				break
			}
			status, err := checkAssets(res)
			if err != nil {
				logger.L.Error(err.Error(), zap.String("module", "LoopCheck"))
			}
			err = assetServer.ChangeState(status)
			if err != nil {
				logger.L.Error(err.Error(), zap.String("module", "LoopCheck"))
			}
		case <-ctx.Done():
			return
		}
	}
}
