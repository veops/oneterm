package repository

import (
	"context"
	"errors"

	"github.com/samber/lo"
	"gorm.io/gorm"

	"github.com/veops/oneterm/internal/model"
)

type IAuthorizationRepository interface {
	UpsertAuthorization(ctx context.Context, auth *model.Authorization) error
	DeleteAuthorization(ctx context.Context, auth *model.Authorization) error
	GetAuthorizations(ctx context.Context, nodeId, assetId, accountId int) ([]*model.Authorization, int64, error)
	GetAuthorizationById(ctx context.Context, id int) (*model.Authorization, error)
	GetAuthorizationByFields(ctx context.Context, nodeId, assetId, accountId int) (*model.Authorization, error)
	GetAuthsByAsset(ctx context.Context, asset *model.Asset) ([]*model.Authorization, error)
	GetAuthorizationIds(ctx context.Context, resourceIds []int) ([]*model.AuthorizationIds, error)
	GetAuthorizationIdsByAssetAccount(ctx context.Context, assetId, accountId int) ([]*model.AuthorizationIds, error)
}

type AuthorizationRepository struct {
	db *gorm.DB
}

func NewAuthorizationRepository(db *gorm.DB) IAuthorizationRepository {
	return &AuthorizationRepository{
		db: db,
	}
}

func (r *AuthorizationRepository) UpsertAuthorization(ctx context.Context, auth *model.Authorization) error {
	t := &model.Authorization{}
	err := r.db.Model(t).
		Where("node_id=? AND asset_id=? AND account_id=?", auth.NodeId, auth.AssetId, auth.AccountId).
		First(t).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		// New record
		return r.db.Create(auth).Error
	}

	// Update existing record
	auth.Id = t.Id
	auth.ResourceId = t.ResourceId
	return r.db.Save(auth).Error
}

// GetAuthorizationById 根据ID获取授权
func (r *AuthorizationRepository) GetAuthorizationById(ctx context.Context, id int) (*model.Authorization, error) {
	auth := &model.Authorization{}
	err := r.db.Model(auth).Where("id = ?", id).First(auth).Error
	if err != nil {
		return nil, err
	}
	return auth, nil
}

// GetAuthorizationByFields 根据字段获取授权
func (r *AuthorizationRepository) GetAuthorizationByFields(ctx context.Context, nodeId, assetId, accountId int) (*model.Authorization, error) {
	auth := &model.Authorization{}
	err := r.db.Model(auth).
		Where("node_id = ? AND asset_id = ? AND account_id = ?", nodeId, assetId, accountId).
		First(auth).Error
	if err != nil {
		return nil, err
	}
	return auth, nil
}

func (r *AuthorizationRepository) DeleteAuthorization(ctx context.Context, auth *model.Authorization) error {
	return r.db.Delete(auth).Error
}

func (r *AuthorizationRepository) GetAuthorizations(ctx context.Context, nodeId, assetId, accountId int) ([]*model.Authorization, int64, error) {
	db := r.db.Model(&model.Authorization{})

	if nodeId > 0 {
		db = db.Where("node_id = ?", nodeId)
	}
	if assetId > 0 {
		db = db.Where("asset_id = ?", assetId)
	}
	if accountId > 0 {
		db = db.Where("account_id = ?", accountId)
	}

	var count int64
	if err := db.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	var auths []*model.Authorization
	if err := db.Find(&auths).Error; err != nil {
		return nil, 0, err
	}

	return auths, count, nil
}

func (r *AuthorizationRepository) GetAuthsByAsset(ctx context.Context, asset *model.Asset) ([]*model.Authorization, error) {
	var data []*model.Authorization
	err := r.db.Model(&model.Authorization{}).
		Where("asset_id=? AND account_id IN ? AND node_id=0", asset.Id, lo.Without(lo.Keys(asset.Authorization), 0)).
		Find(&data).Error
	return data, err
}

func (r *AuthorizationRepository) GetAuthorizationIds(ctx context.Context, resourceIds []int) ([]*model.AuthorizationIds, error) {
	var authIds []*model.AuthorizationIds
	err := r.db.Model(&model.Authorization{}).Find(&authIds).Where("resource_id IN ?", resourceIds).Error
	return authIds, err
}

func (r *AuthorizationRepository) GetAuthorizationIdsByAssetAccount(ctx context.Context, assetId, accountId int) ([]*model.AuthorizationIds, error) {
	var authIds []*model.AuthorizationIds
	err := r.db.Model(&model.Authorization{}).
		Where("asset_id = ? AND account_id = ?", assetId, accountId).
		Find(&authIds).Error
	return authIds, err
}
