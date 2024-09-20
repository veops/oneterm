package schedule

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"go.uber.org/zap"

	mysql "github.com/veops/oneterm/db"
	ggateway "github.com/veops/oneterm/gateway"
	"github.com/veops/oneterm/logger"
	"github.com/veops/oneterm/model"
	"github.com/veops/oneterm/util"
)

func UpdateConnectables(ids ...int) (err error) {
	defer func() {
		if err != nil {
			logger.L().Warn("check connectable failed", zap.Error(err))
		}
	}()
	assets := make([]*model.Asset, 0)
	db := mysql.DB.
		Model(assets)
	if len(ids) > 0 {
		db = db.Where("id IN ?", ids)
	} else {
		db = db.Where("updated_at <= ?", time.Now().Add(-time.Hour))
	}
	if err = db.
		Find(&assets).Error; err != nil {
		logger.L().Debug("get assets to test connectable failed", zap.Error(err))
		return
	}
	gids := lo.Without(lo.Uniq(lo.Map(assets, func(a *model.Asset, _ int) int { return a.GatewayId })), 0)
	gateways := make([]*model.Gateway, 0)
	if len(gids) > 0 {
		if err = mysql.DB.
			Model(gateways).
			Where("id IN ?", gids).
			Find(&gateways).Error; err != nil {
			logger.L().Debug("get gatewats to test connectable failed", zap.Error(err))
			return
		}
	}
	for _, g := range gateways {
		g.Password = util.DecryptAES(g.Password)
		g.Pk = util.DecryptAES(g.Pk)
		g.Phrase = util.DecryptAES(g.Phrase)
	}
	gatewayMap := lo.SliceToMap(gateways, func(g *model.Gateway) (int, *model.Gateway) { return g.Id, g })

	all, oks := lo.Map(assets, func(a *model.Asset, _ int) int { return a.Id }), make([]int, 0)
	sids := make([]string, 0)
	for _, a := range assets {
		sid, ok := updateConnectable(a, gatewayMap[a.GatewayId])
		if ok {
			oks = append(oks, a.Id)
		}
		sids = append(sids, sid)
	}
	defer ggateway.GetGatewayManager().Close(sids...)
	if len(oks) > 0 {
		if err := mysql.DB.Model(assets).Where("id IN ?", oks).Update("connectable", true).Error; err != nil {
			logger.L().Debug("update connectable to ok failed", zap.Error(err))
		}
	}
	if len(oks) < len(all) {
		if err := mysql.DB.Model(assets).Where("id IN ?", lo.Without(all, oks...)).Update("connectable", false).Error; err != nil {
			logger.L().Debug("update connectable to fail failed", zap.Error(err))
		}
	}
	return
}

func updateConnectable(asset *model.Asset, gateway *model.Gateway) (sid string, ok bool) {
	sid = uuid.New().String()
	for _, p := range asset.Protocols {
		ip, port := asset.Ip, cast.ToInt(strings.Split(p, ":")[1])
		var (
			gt  *ggateway.GatewayTunnel
			err error
		)
		if asset.GatewayId != 0 {
			gt, err = ggateway.GetGatewayManager().Open(sid, ip, port, gateway)
			if err != nil {
				logger.L().Debug("open gateway failed", zap.Error(err))
				continue
			}
			ip, port = gt.LocalIp, gt.LocalPort
			<-gt.Opened
		}
		addr := fmt.Sprintf("%s:%d", ip, port)
		net, err := net.DialTimeout("tcp", addr, time.Second*3)
		if err != nil {
			logger.L().Debug("dail failed", zap.String("addr", addr), zap.Error(err))
			continue
		}
		defer net.Close()

		ok = true
		return
	}
	return
}
