package repository

import (
	"context"
	"encoding/json"

	"github.com/veops/oneterm/internal/model"
	"gorm.io/gorm"
)

// IAuthorizationV2Repository defines the interface for authorization V2 repository
type IAuthorizationV2Repository interface {
	Create(ctx context.Context, auth *model.AuthorizationV2) error
	GetById(ctx context.Context, id int) (*model.AuthorizationV2, error)
	Update(ctx context.Context, auth *model.AuthorizationV2) error
	Delete(ctx context.Context, id int) error
	GetUserRules(ctx context.Context, userRids []int) ([]*model.AuthorizationV2, error)
	GetAll(ctx context.Context) ([]*model.AuthorizationV2, error)
	GetByResourceIds(ctx context.Context, resourceIds []int) ([]*model.AuthorizationV2, error)
}

// AuthorizationV2Repository implements IAuthorizationV2Repository
type AuthorizationV2Repository struct {
	db *gorm.DB
}

// NewAuthorizationV2Repository creates a new authorization V2 repository
func NewAuthorizationV2Repository(db *gorm.DB) IAuthorizationV2Repository {
	return &AuthorizationV2Repository{db: db}
}

// Create creates a new authorization V2 rule
func (r *AuthorizationV2Repository) Create(ctx context.Context, auth *model.AuthorizationV2) error {
	return r.db.Create(auth).Error
}

// GetById retrieves an authorization V2 rule by ID
func (r *AuthorizationV2Repository) GetById(ctx context.Context, id int) (*model.AuthorizationV2, error) {
	auth := &model.AuthorizationV2{}
	err := r.db.Where("id = ?", id).First(auth).Error
	return auth, err
}

// Update updates an authorization V2 rule
func (r *AuthorizationV2Repository) Update(ctx context.Context, auth *model.AuthorizationV2) error {
	return r.db.Model(auth).Where("id = ?", auth.Id).Updates(auth).Error
}

// Delete deletes an authorization V2 rule
func (r *AuthorizationV2Repository) Delete(ctx context.Context, id int) error {
	return r.db.Where("id = ?", id).Delete(&model.AuthorizationV2{}).Error
}

// GetUserRules retrieves authorization rules for specific user role IDs
func (r *AuthorizationV2Repository) GetUserRules(ctx context.Context, userRids []int) ([]*model.AuthorizationV2, error) {
	if len(userRids) == 0 {
		return []*model.AuthorizationV2{}, nil
	}

	jsonRids, err := json.Marshal(userRids)
	if err != nil {
		return nil, err
	}

	var rules []*model.AuthorizationV2
	err = r.db.Where("enabled = ? AND JSON_OVERLAPS(rids, ?)", true, string(jsonRids)).
		Order("id ASC").
		Find(&rules).Error
	return rules, err
}

// GetAll retrieves all authorization V2 rules
func (r *AuthorizationV2Repository) GetAll(ctx context.Context) ([]*model.AuthorizationV2, error) {
	var rules []*model.AuthorizationV2
	err := r.db.Order("id ASC").Find(&rules).Error
	return rules, err
}

// GetByResourceIds retrieves authorization rules by resource IDs
func (r *AuthorizationV2Repository) GetByResourceIds(ctx context.Context, resourceIds []int) ([]*model.AuthorizationV2, error) {
	if len(resourceIds) == 0 {
		return []*model.AuthorizationV2{}, nil
	}

	var rules []*model.AuthorizationV2
	err := r.db.Where("resource_id IN ? AND enabled = ?", resourceIds, true).
		Order("id ASC").
		Find(&rules).Error
	return rules, err
}
