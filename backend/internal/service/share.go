package service

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"

	"github.com/veops/oneterm/internal/acl"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/repository"
)

// ShareService handles share business logic
type ShareService struct {
	repo repository.ShareRepository
}

// NewShareService creates a new share service
func NewShareService() *ShareService {
	return &ShareService{
		repo: repository.NewShareRepository(),
	}
}

// HasPermission checks if the user has permission on the share
func (s *ShareService) HasPermission(ctx context.Context, share *model.Share, action string) bool {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	if acl.IsAdmin(currentUser) {
		return true
	}

	_, assetIds, accountIds, err := getNodeAssetAccoutIdsByAction(ctx, action)
	if err != nil {
		return false
	}

	return lo.Contains(assetIds, share.AssetId) || lo.Contains(accountIds, share.AccountId)
}

// CreateShares creates new shares
func (s *ShareService) CreateShares(ctx context.Context, shares []*model.Share) ([]string, error) {
	// Add UUID and return the UUIDs
	uuids := lo.Map(shares, func(share *model.Share, _ int) string {
		share.Uuid = uuid.New().String()
		return share.Uuid
	})

	err := s.repo.CreateShares(ctx, shares)
	if err != nil {
		return nil, err
	}

	return uuids, nil
}

// BuildQuery constructs a query for shares
func (s *ShareService) BuildQuery(ctx *gin.Context, isAdmin bool) (*gorm.DB, error) {
	var assetIds, accountIds []int

	// If not admin, get accessible assets and accounts
	if !isAdmin {
		var err error
		_, assetIds, accountIds, err = getNodeAssetAccoutIdsByAction(ctx, acl.GRANT)
		if err != nil {
			return nil, err
		}
	}

	return s.repo.BuildQuery(ctx, assetIds, accountIds)
}

// GetShareByID retrieves a share by ID
func (s *ShareService) GetShareByID(ctx context.Context, id int) (*model.Share, error) {
	return s.repo.GetShareByID(ctx, id)
}

// GetShareByUUID retrieves a share by UUID
func (s *ShareService) GetShareByUUID(ctx context.Context, uuid string) (*model.Share, error) {
	return s.repo.GetShareByUUID(ctx, uuid)
}

// ValidateShareForConnection validates a share for connection
func (s *ShareService) ValidateShareForConnection(ctx context.Context, uuid string) (*model.Share, error) {
	share, err := s.repo.GetShareByUUID(ctx, uuid)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	if now.Before(share.Start) || now.After(share.End) {
		return nil, fmt.Errorf("share expired or not started")
	}

	if share.NoLimit {
		return share, nil
	}

	rowsAffected, err := s.repo.DecrementShareTimes(ctx, share.Uuid)
	if err != nil {
		return nil, err
	}

	if rowsAffected != 1 {
		return nil, fmt.Errorf("no times left")
	}

	return share, nil
}

// SetupConnectionParams sets up connection parameters from share
func (s *ShareService) SetupConnectionParams(ctx *gin.Context, share *model.Share) {
	// Filter out and add new parameters
	ctx.Params = lo.Filter(ctx.Params, func(p gin.Param, _ int) bool {
		return !lo.Contains([]string{"account_id", "asset_id", "protocol"}, p.Key)
	})

	ctx.Params = append(ctx.Params, gin.Param{Key: "account_id", Value: cast.ToString(share.AccountId)})
	ctx.Params = append(ctx.Params, gin.Param{Key: "asset_id", Value: cast.ToString(share.AssetId)})
	ctx.Params = append(ctx.Params, gin.Param{Key: "protocol", Value: cast.ToString(share.Protocol)})

	// Set share context values
	ctx.Set("shareId", share.Id)
	ctx.Set("session", &acl.Session{})
	ctx.Set("shareEnd", share.End)
}

// Helper functions that should be moved to appropriate services
func getNodeAssetAccoutIdsByAction(ctx context.Context, action string) (nodeIds, assetIds, accountIds []int, err error) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	eg := &errgroup.Group{}
	ch := make(chan bool)

	eg.Go(func() (err error) {
		defer close(ch)
		res, err := acl.GetRoleResources(ctx, currentUser.GetRid(), "node")
		if err != nil {
			return
		}
		res = lo.Filter(res, func(r *acl.Resource, _ int) bool { return lo.Contains(r.Permissions, action) })
		resIds := lo.Map(res, func(r *acl.Resource, _ int) int { return r.ResourceId })
		nodes, err := repository.GetAllFromCacheDb(ctx, model.DefaultNode)
		if err != nil {
			return
		}
		nodes = lo.Filter(nodes, func(n *model.Node, _ int) bool { return lo.Contains(resIds, n.ResourceId) })
		nodeIds = lo.Map(nodes, func(n *model.Node, _ int) int { return n.Id })
		nodeIds, err = repository.HandleSelfChild(ctx, nodeIds...)
		return
	})

	eg.Go(func() (err error) {
		res, err := acl.GetRoleResources(ctx, currentUser.GetRid(), "asset")
		if err != nil {
			return
		}
		res = lo.Filter(res, func(r *acl.Resource, _ int) bool { return lo.Contains(r.Permissions, action) })
		resIds := lo.Map(res, func(r *acl.Resource, _ int) int { return r.ResourceId })
		<-ch
		assets, err := repository.GetAllFromCacheDb(ctx, model.DefaultAsset)
		if err != nil {
			return
		}
		assets = lo.Filter(assets, func(a *model.Asset, _ int) bool {
			return lo.Contains(resIds, a.ResourceId) || lo.Contains(nodeIds, a.ParentId)
		})
		assetIds = lo.Map(assets, func(a *model.Asset, _ int) int { return a.Id })
		return
	})

	eg.Go(func() (err error) {
		res, err := acl.GetRoleResources(ctx, currentUser.GetRid(), "account")
		if err != nil {
			return
		}
		res = lo.Filter(res, func(r *acl.Resource, _ int) bool { return lo.Contains(r.Permissions, action) })
		resIds := lo.Map(res, func(r *acl.Resource, _ int) int { return r.ResourceId })
		accounts, err := repository.GetAllFromCacheDb(ctx, model.DefaultAccount)
		if err != nil {
			return
		}
		accounts = lo.Filter(accounts, func(a *model.Account, _ int) bool { return lo.Contains(resIds, a.ResourceId) })
		accountIds = lo.Map(accounts, func(a *model.Account, _ int) int { return a.Id })
		return
	})

	err = eg.Wait()

	return
}
