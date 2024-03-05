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
	ggateway "github.com/veops/oneterm/pkg/server/global/gateway"
	"github.com/veops/oneterm/pkg/server/model"
	"github.com/veops/oneterm/pkg/server/storage/db/mysql"
)

var (
	ctx, cancel = context.WithCancel(context.Background())
	d           = time.Hour * 2
)

func Run() (err error) {
	tk := time.NewTicker(d)
	for {
		select {
		case <-tk.C:
			CheckUpdate()
		case <-ctx.Done():
			return
		}
	}
}

func Stop(err error) {
	defer cancel()
}

func CheckUpdate(ids ...int) (err error) {
	defer func() {
		fmt.Println(err)
	}()
	assets := make([]*model.Asset, 0)
	db := mysql.DB.
		Model(assets)
	if len(ids) > 0 {
		db = db.Where("id IN ?", ids)
	} else {
		db = db.Where("updated_at <= ?", time.Now().Add(-d).Unix())
	}
	if err = db.
		Find(&assets).Error; err != nil {
		logger.L.Debug("get assets to test connectable failed", zap.Error(err))
		return
	}
	gids := lo.Without(lo.Uniq(lo.Map(assets, func(a *model.Asset, _ int) int { return a.GatewayId })), 0)
	gateways := make([]*model.Gateway, 0)
	if len(gids) > 0 {
		if err = mysql.DB.
			Model(gateways).
			Where("id IN ?", gids).
			Find(&assets).Error; err != nil {
			logger.L.Debug("get gatewats to test connectable failed", zap.Error(err))
			return
		}
	}
	gatewayMap := lo.SliceToMap(gateways, func(g *model.Gateway) (int, *model.Gateway) { return g.Id, g })

	all, oks := lo.Map(assets, func(a *model.Asset, _ int) int { return a.Id }), make([]int, 0)
	gid2sid := make(map[ggateway.GatewayTunnelKey][]string)
	for _, a := range assets {
		if checkOne(gid2sid, a, gatewayMap[a.GatewayId]) {
			oks = append(oks, a.Id)
		}
	}
	for k, v := range gid2sid {
		ggateway.GetGatewayManager().Close(k, v...)
	}
	if len(oks) > 0 {
		if err := mysql.DB.Model(assets).Where("id IN ?", oks).Update("connectable", true).Error; err != nil {
			logger.L.Debug("update connectable to ok failed", zap.Error(err))
		}
	}
	if len(oks) < len(all) {
		if err := mysql.DB.Model(assets).Where("id IN ?", lo.Without(all, oks...)).Update("connectable", false).Error; err != nil {
			logger.L.Debug("update connectable to fail failed", zap.Error(err))
		}
	}
	return
}

func checkOne(gid2sid map[ggateway.GatewayTunnelKey][]string, asset *model.Asset, gateway *model.Gateway) bool {
	sid := uuid.New().String()
	for _, p := range asset.Protocols {
		ip, port := asset.Ip, cast.ToInt(strings.Split(p, ":")[1])
		if asset.GatewayId != 0 {
			g, err := ggateway.GetGatewayManager().Open(sid, ip, port, gateway)
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
