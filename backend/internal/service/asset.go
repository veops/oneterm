package service

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/veops/oneterm/internal/acl"
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
		asset.Authorization = make(model.AuthorizationMap)
	}

	// Handle backward compatibility: convert old format to new format
	// This handles cases where frontend still sends old format: Map[int, Slice[int]]
	s.ensureAuthorizationFormat(asset)
}

// ensureAuthorizationFormat ensures asset.Authorization is in the correct V2 format
// Handles backward compatibility with old V1 format
func (s *AssetService) ensureAuthorizationFormat(asset *model.Asset) {
	// Check if we need to convert from old format to new format
	// This is needed for backward compatibility
	for accountId, authData := range asset.Authorization {
		// If permissions is nil, set default permissions (connect only for V1 compatibility)
		if authData.Permissions == nil {
			authData.Permissions = &model.AuthPermissions{
				Connect:      true,  // Default: allow connect (V1 behavior)
				FileUpload:   false, // Default: deny file upload
				FileDownload: false, // Default: deny file download
				Copy:         false, // Default: deny copy
				Paste:        false, // Default: deny paste
				Share:        false, // Default: deny share
			}
			asset.Authorization[accountId] = authData
		}
	}
}

// AttachNodeChain attaches node chain to assets
func (s *AssetService) AttachNodeChain(ctx context.Context, assets []*model.Asset) error {
	return s.repo.AttachNodeChain(ctx, assets)
}

// BuildQuery constructs asset query with basic filters
func (s *AssetService) BuildQuery(ctx *gin.Context) (*gorm.DB, error) {
	return s.repo.BuildQuery(ctx)
}

// FilterByParentId filters assets by parent ID
func (s *AssetService) FilterByParentId(db *gorm.DB, parentId int) (*gorm.DB, error) {
	return s.repo.FilterByParentId(db, parentId)
}

// GetAssetIdsByAuthorization gets asset IDs by authorization using efficient V2 method
func (s *AssetService) GetAssetIdsByAuthorization(ctx *gin.Context) ([]int, []int, []int, error) {
	// Use efficient V2 method: get authorized resource IDs from ACL, then find V2 rules
	authV2Service := NewAuthorizationV2Service()
	return authV2Service.GetAuthorizationScopeByACL(ctx)
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

// BuildQueryWithAuthorization constructs asset query with integrated V2 authorization filter
func (s *AssetService) BuildQueryWithAuthorization(ctx *gin.Context) (*gorm.DB, error) {
	// Start with base query
	db, err := s.repo.BuildQuery(ctx)
	if err != nil {
		return nil, err
	}

	currentUser, _ := acl.GetSessionFromCtx(ctx)

	// Administrators have access to all assets
	if acl.IsAdmin(currentUser) {
		return db, nil
	}

	// Apply V2 authorization filter directly at database level
	authV2Service := NewAuthorizationV2Service()
	_, assetIds, _, err := authV2Service.GetAuthorizationScopeByACL(ctx)
	if err != nil {
		return nil, err
	}

	// Filter by authorized asset IDs at database level (much more efficient)
	if len(assetIds) == 0 {
		// No access to any assets
		db = db.Where("1 = 0") // Returns empty result set efficiently
	} else {
		db = db.Where("id IN ?", assetIds)
	}

	return db, nil
}
