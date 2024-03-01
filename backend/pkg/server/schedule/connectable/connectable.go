package connectable

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"go.uber.org/zap"

	"github.com/veops/oneterm/pkg/logger"
	"github.com/veops/oneterm/pkg/server/guacd"
	"github.com/veops/oneterm/pkg/server/model"
	"github.com/veops/oneterm/pkg/server/storage/db/mysql"
)

var (
	ctx, cancel = context.WithCancel(context.Background())
)

func Run() (err error) {
	tk := time.NewTicker(time.Hour)
	for {
		select {
		case <-tk.C:
			assets := make([]*model.Asset, 0)
			if err := mysql.DB.
				Model(assets).
				Where("updated_at <= ?", time.Now().Add(-time.Hour).Unix()).
				Find(&assets).Error; err != nil {
				logger.L.Debug("get assets to test connectable failed", zap.Error(err))
				continue
			}
			gateways := make([]*model.Gateway, 0)
			if err := mysql.DB.
				Model(gateways).
				Where("id IN ?", lo.Without(lo.Uniq(lo.Map(assets, func(a *model.Asset, _ int) int { return a.GatewayId }))), 0).
				Find(&assets).Error; err != nil {
				logger.L.Debug("get gatewats to test connectable failed", zap.Error(err))
				continue
			}
			gatewayMap := lo.SliceToMap(gateways, func(g *model.Gateway) (int, *model.Gateway) { return g.Id, g })

			all, oks := lo.Map(assets, func(a *model.Asset, _ int) int { return a.Id }), make([]int, 0)
			gid2sid := make(map[guacd.GatewayTunnelKey][]string)
			for _, a := range assets {
				if checkOne(gid2sid, a, gatewayMap[a.GatewayId]) {
					oks = append(oks, a.Id)
				}
			}
			for k, v := range gid2sid {
				guacd.GlobalGatewayManager.Close(k, v...)
			}
			if len(oks) > 0 {
				if err = mysql.DB.Where("id IN ?", oks).Update("connectable", true).Error; err != nil {
					logger.L.Debug("update connectable to ok failed", zap.Error(err))
				}
			}
			if len(oks) < len(all) {
				if err = mysql.DB.Where("id IN ?", lo.Without(all, oks...)).Update("connectable", false).Error; err != nil {
					logger.L.Debug("update connectable to fail failed", zap.Error(err))
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

func Stop(err error) {
	defer cancel()
}

func checkOne(gid2sid map[guacd.GatewayTunnelKey][]string, asset *model.Asset, gateway *model.Gateway) bool {
	sid := uuid.New().String()
	for _, p := range asset.Protocols {
		ss := strings.Split(p, ":")
		ip, port := ss[0], cast.ToInt(ss[1])
		if asset.GatewayId != 0 {
			g, err := guacd.GlobalGatewayManager.Open(sid, ip, port, gateway)
			if err != nil {
				continue
			}
			gid2sid[g.Key] = append(gid2sid[g.Key], sid)
			ip, port = g.LocalIp, g.LocalPort
		}
		net, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", ip, port), time.Second*3)
		if err != nil {
			continue
		}
		net.Close()
		return true
	}
	return false
}
