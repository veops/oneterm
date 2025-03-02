package schedule

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"go.uber.org/zap"

	ggateway "github.com/veops/oneterm/internal/gateway"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/service"
	dbpkg "github.com/veops/oneterm/pkg/db"
	"github.com/veops/oneterm/pkg/logger"
	"github.com/veops/oneterm/pkg/utils"
)

func UpdateConnectables(ids ...int) (err error) {
	defer func() {
		if err != nil {
			logger.L().Warn("check connectable failed", zap.Error(err))
		}
	}()
	assets := make([]*model.Asset, 0)
	db := dbpkg.DB.
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
		if err = dbpkg.DB.
			Model(gateways).
			Where("id IN ?", gids).
			Find(&gateways).Error; err != nil {
			logger.L().Debug("get gatewats to test connectable failed", zap.Error(err))
			return
		}
	}
	for _, g := range gateways {
		g.Password = utils.DecryptAES(g.Password)
		g.Pk = utils.DecryptAES(g.Pk)
		g.Phrase = utils.DecryptAES(g.Phrase)
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
		if err := dbpkg.DB.Model(assets).Where("id IN ?", oks).Update("connectable", true).Error; err != nil {
			logger.L().Debug("update connectable to ok failed", zap.Error(err))
		}
	}
	if len(oks) < len(all) {
		if err := dbpkg.DB.Model(assets).Where("id IN ?", lo.Without(all, oks...)).Update("connectable", false).Error; err != nil {
			logger.L().Debug("update connectable to fail failed", zap.Error(err))
		}
	}
	return
}

func updateConnectable(asset *model.Asset, gateway *model.Gateway) (sid string, ok bool) {
	sid = uuid.New().String()
	ps := strings.Join(lo.Map(asset.Protocols, func(p string, _ int) string { return strings.Split(p, ":")[0] }), ",")
	ip, port, err := service.Proxy(true, sid, ps, asset, gateway)
	if err != nil {
		logger.L().Debug("connectable proxy failed", zap.String("protocol", ps), zap.Error(err))
		return
	}
	var hostPort string
	if strings.Contains(ip, ":") {
		hostPort = fmt.Sprintf("[%s]:%d", ip, port)
	} else {
		hostPort = fmt.Sprintf("%s:%d", ip, port)
	}
	conn, err := net.DialTimeout("tcp", hostPort, time.Second)
	if err != nil {
		logger.L().Debug("dail failed", zap.String("addr", hostPort), zap.Error(err))
		return
	}
	defer conn.Close()
	if asset.GatewayId != 0 {
		t := ggateway.GetGatewayTunnelBySessionId(sid)
		if t == nil {
			return
		}
		if err = <-t.Opened; err != nil {
			return
		}
	}
	ok = true
	return
}
