package service

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/repository"
	"github.com/veops/oneterm/internal/schedule"
	"gorm.io/gorm"
)

// AssetService handles asset business logic
type AssetService struct {
	repo repository.AssetRepository
}

// NewAssetService creates a new asset service
func NewAssetService() *AssetService {
	return &AssetService{
		repo: repository.NewAssetRepository(),
	}
}

// GetById retrieves an asset by its ID
func (s *AssetService) GetById(ctx context.Context, id int) (*model.Asset, error) {
	return s.repo.GetById(ctx, id)
}

// PreprocessAssetData preprocesses asset data before saving
func (s *AssetService) PreprocessAssetData(asset *model.Asset) {
	asset.Ip = strings.TrimSpace(asset.Ip)
	asset.Protocols = lo.Map(asset.Protocols, func(s string, _ int) string { return strings.TrimSpace(s) })
	if asset.Authorization == nil {
		asset.Authorization = make(model.Map[int, model.Slice[int]])
	}
}

// AttachNodeChain attaches node chain to assets
func (s *AssetService) AttachNodeChain(ctx context.Context, assets []*model.Asset) error {
	return s.repo.AttachNodeChain(ctx, assets)
}

// ApplyAuthorizationFilters applies authorization filters to assets
func (s *AssetService) ApplyAuthorizationFilters(ctx *gin.Context, assets []*model.Asset, authorizationIds []*model.AuthorizationIds, nodeIds, accountIds []int) {
	s.repo.ApplyAuthorizationFilters(ctx, assets, authorizationIds, nodeIds, accountIds)
}

// BuildQuery constructs asset query with basic filters
func (s *AssetService) BuildQuery(ctx *gin.Context) (*gorm.DB, error) {
	return s.repo.BuildQuery(ctx)
}

// FilterByParentId filters assets by parent ID
func (s *AssetService) FilterByParentId(db *gorm.DB, parentId int) (*gorm.DB, error) {
	return s.repo.FilterByParentId(db, parentId)
}

// GetAssetIdsByAuthorization gets asset IDs by authorization
func (s *AssetService) GetAssetIdsByAuthorization(ctx *gin.Context, authorizationIds []*model.AuthorizationIds) ([]int, []int, []int, error) {
	return s.repo.GetAssetIdsByAuthorization(ctx, authorizationIds)
}

// GetIdsByAuthorizationIds extracts node IDs, asset IDs, and account IDs from authorization IDs
func (s *AssetService) GetIdsByAuthorizationIds(ctx *gin.Context, authorizationIds []*model.AuthorizationIds) ([]int, []int, []int) {
	return s.repo.GetIdsByAuthorizationIds(ctx, authorizationIds)
}

// GetAssetIdsByNodeAccount gets asset IDs by node IDs and account IDs
func (s *AssetService) GetAssetIdsByNodeAccount(ctx context.Context, nodeIds, accountIds []int) ([]int, error) {
	return s.repo.GetAssetIdsByNodeAccount(ctx, nodeIds, accountIds)
}

// UpdateConnectables updates asset connectability status
func (s *AssetService) UpdateConnectables(ids ...int) error {
	return schedule.UpdateAssetConnectables(ids...)
}
