package schedule

import (
	"fmt"
	"math"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/tunneling"
	dbpkg "github.com/veops/oneterm/pkg/db"
	"github.com/veops/oneterm/pkg/logger"
	"github.com/veops/oneterm/pkg/utils"
)

// ConnectableResult represents the result of a connectivity check
type ConnectableResult struct {
	AssetID   int
	SessionID string
	Success   bool
	Error     error
}

func UpdateConnectables(ids ...int) (err error) {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		if err != nil {
			logger.L().Warn("Connectivity check failed",
				zap.Error(err),
				zap.Duration("duration", duration))
		} else {
			logger.L().Info("Connectivity check completed",
				zap.Duration("duration", duration))
		}
	}()

	// Load assets to check
	assets, err := getAssetsToCheck(ids...)
	if err != nil {
		return err
	}

	if len(assets) == 0 {
		logger.L().Debug("No assets to check connectivity")
		return nil
	}

	logger.L().Info("Starting connectivity check",
		zap.Int("total_assets", len(assets)),
		zap.Int("batch_size", scheduleConfig.BatchSize),
		zap.Int("concurrent_workers", scheduleConfig.ConcurrentWorkers))

	// Load and decrypt gateways
	gatewayMap, err := getGatewayMap(assets)
	if err != nil {
		return err
	}

	// Process assets in concurrent batches
	results := processConcurrentBatches(assets, gatewayMap)

	// Update database with results
	return updateConnectableStatus(results)
}

func getAssetsToCheck(ids ...int) ([]*model.Asset, error) {
	assets := make([]*model.Asset, 0)
	db := dbpkg.DB.Model(assets)

	if len(ids) > 0 {
		db = db.Where("id IN ?", ids)
	} else {
		// Only check assets not updated within the configured interval OR offline assets
		checkInterval := scheduleConfig.ConnectableCheckInterval
		db = db.Where("updated_at <= ? OR connectable = ?",
			time.Now().Add(-checkInterval).Add(-time.Second*30), false)
	}

	// Only select fields needed for connectivity check, exclude authorization to avoid V1/V2 compatibility issues
	if err := db.Select("id", "name", "ip", "protocols", "gateway_id", "connectable", "updated_at").Find(&assets).Error; err != nil {
		logger.L().Error("Failed to get assets for connectivity check", zap.Error(err))
		return nil, err
	}

	return assets, nil
}

func getGatewayMap(assets []*model.Asset) (map[int]*model.Gateway, error) {
	gids := lo.Without(lo.Uniq(lo.Map(assets, func(a *model.Asset, _ int) int {
		return a.GatewayId
	})), 0)

	if len(gids) == 0 {
		return make(map[int]*model.Gateway), nil
	}

	gateways := make([]*model.Gateway, 0)
	if err := dbpkg.DB.Model(gateways).Where("id IN ?", gids).Find(&gateways).Error; err != nil {
		logger.L().Error("Failed to get gateways for connectivity check", zap.Error(err))
		return nil, err
	}

	// Decrypt gateway credentials
	for _, g := range gateways {
		g.Password = utils.DecryptAES(g.Password)
		g.Pk = utils.DecryptAES(g.Pk)
		g.Phrase = utils.DecryptAES(g.Phrase)
	}

	return lo.SliceToMap(gateways, func(g *model.Gateway) (int, *model.Gateway) {
		return g.Id, g
	}), nil
}

func processConcurrentBatches(assets []*model.Asset, gatewayMap map[int]*model.Gateway) []ConnectableResult {
	batchSize := scheduleConfig.BatchSize
	concurrentWorkers := scheduleConfig.ConcurrentWorkers

	totalBatches := int(math.Ceil(float64(len(assets)) / float64(batchSize)))
	resultChan := make(chan []ConnectableResult, totalBatches)
	semaphore := make(chan struct{}, concurrentWorkers)

	var wg sync.WaitGroup

	// Process assets in batches
	for i := 0; i < len(assets); i += batchSize {
		end := min(i+batchSize, len(assets))
		batch := assets[i:end]

		wg.Add(1)
		go func(batch []*model.Asset, batchNum int) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			logger.L().Debug("Processing connectivity batch",
				zap.Int("batch_number", batchNum+1),
				zap.Int("batch_size", len(batch)))

			batchResults := processBatch(batch, gatewayMap)
			resultChan <- batchResults
		}(batch, i/batchSize)
	}

	// Close result channel when all goroutines complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect all results
	var allResults []ConnectableResult
	for batchResults := range resultChan {
		allResults = append(allResults, batchResults...)
	}

	return allResults
}

func processBatch(assets []*model.Asset, gatewayMap map[int]*model.Gateway) []ConnectableResult {
	results := make([]ConnectableResult, len(assets))

	for i, asset := range assets {
		gateway := gatewayMap[asset.GatewayId]
		sessionID, success := updateConnectable(asset, gateway)
		results[i] = ConnectableResult{
			AssetID:   asset.Id,
			SessionID: sessionID,
			Success:   success,
		}
	}

	return results
}

func updateConnectableStatus(results []ConnectableResult) error {
	if len(results) == 0 {
		return nil
	}

	// Collect session IDs for cleanup
	sessionIDs := make([]string, 0, len(results))
	successfulAssets := make([]int, 0)
	failedAssets := make([]int, 0)

	for _, result := range results {
		sessionIDs = append(sessionIDs, result.SessionID)
		if result.Success {
			successfulAssets = append(successfulAssets, result.AssetID)
		} else {
			failedAssets = append(failedAssets, result.AssetID)
		}
	}

	// Clean up tunnels
	defer tunneling.CloseTunnels(sessionIDs...)

	// Update successful assets
	if len(successfulAssets) > 0 {
		if err := dbpkg.DB.Model(&model.Asset{}).
			Where("id IN ?", successfulAssets).
			Update("connectable", true).Error; err != nil {
			logger.L().Error("Failed to update successful assets",
				zap.Error(err),
				zap.Int("count", len(successfulAssets)))
			return err
		}
		logger.L().Debug("Updated successful assets", zap.Int("count", len(successfulAssets)))
	}

	// Update failed assets
	if len(failedAssets) > 0 {
		if err := dbpkg.DB.Model(&model.Asset{}).
			Where("id IN ?", failedAssets).
			Update("connectable", false).Error; err != nil {
			logger.L().Error("Failed to update failed assets",
				zap.Error(err),
				zap.Int("count", len(failedAssets)))
			return err
		}
		logger.L().Debug("Updated failed assets", zap.Int("count", len(failedAssets)))
	}

	logger.L().Info("Connectivity status updated",
		zap.Int("successful", len(successfulAssets)),
		zap.Int("failed", len(failedAssets)),
		zap.Int("total", len(results)))

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func updateConnectable(asset *model.Asset, gateway *model.Gateway) (sid string, ok bool) {
	sid = uuid.New().String()
	ps := strings.Join(lo.Map(asset.Protocols, func(p string, _ int) string {
		return strings.Split(p, ":")[0]
	}), ",")

	ip, port, err := tunneling.Proxy(true, sid, ps, asset, gateway)
	if err != nil {
		logger.L().Debug("Proxy connection failed",
			zap.String("protocol", ps),
			zap.String("asset_ip", asset.Ip),
			zap.Int("asset_id", asset.Id),
			zap.Error(err))
		return
	}

	var hostPort string
	if strings.Contains(ip, ":") {
		hostPort = fmt.Sprintf("[%s]:%d", ip, port)
	} else {
		hostPort = fmt.Sprintf("%s:%d", ip, port)
	}

	// Use configurable timeout
	conn, err := net.DialTimeout("tcp", hostPort, scheduleConfig.ConnectTimeout)
	if err != nil {
		logger.L().Debug("TCP connection failed",
			zap.String("address", hostPort),
			zap.Int("asset_id", asset.Id),
			zap.Duration("timeout", scheduleConfig.ConnectTimeout),
			zap.Error(err))
		return
	}
	defer conn.Close()

	// Verify gateway tunnel if using gateway
	if asset.GatewayId != 0 {
		t := tunneling.GetTunnelBySessionId(sid)
		if t == nil {
			logger.L().Debug("Gateway tunnel not found",
				zap.String("session_id", sid),
				zap.Int("asset_id", asset.Id),
				zap.Int("gateway_id", asset.GatewayId))
			return
		}

		select {
		case err = <-t.Opened:
			if err != nil {
				logger.L().Debug("Gateway tunnel failed to open",
					zap.String("session_id", sid),
					zap.Int("asset_id", asset.Id),
					zap.Int("gateway_id", asset.GatewayId),
					zap.Error(err))
				return
			}
		case <-time.After(scheduleConfig.ConnectTimeout):
			logger.L().Debug("Gateway tunnel open timeout",
				zap.String("session_id", sid),
				zap.Int("asset_id", asset.Id),
				zap.Int("gateway_id", asset.GatewayId),
				zap.Duration("timeout", scheduleConfig.ConnectTimeout))
			return
		}
	}

	logger.L().Debug("Asset connectivity check successful",
		zap.Int("asset_id", asset.Id),
		zap.String("address", hostPort))
	ok = true
	return
}

// UpdateAssetConnectables is used by service/asset.go to update connectables
func UpdateAssetConnectables(ids ...int) error {
	return UpdateConnectables(ids...)
}
