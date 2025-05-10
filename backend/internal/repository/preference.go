package repository

import (
	"context"

	"github.com/veops/oneterm/internal/model"
	dbpkg "github.com/veops/oneterm/pkg/db"
	"gorm.io/gorm"
)

// UserPreferenceRepository defines the interface for user preference operations
type UserPreferenceRepository interface {
	GetByUserID(ctx context.Context, userID int) (*model.UserPreference, error)
	UpsertPreference(ctx context.Context, pref *model.UserPreference, fields []string) error
}

// userPreferenceRepository implements UserPreferenceRepository
type userPreferenceRepository struct{}

// NewUserPreferenceRepository creates a new user preference repository
func NewUserPreferenceRepository() UserPreferenceRepository {
	return &userPreferenceRepository{}
}

// GetByUserID retrieves user preferences by user ID
func (r *userPreferenceRepository) GetByUserID(ctx context.Context, userID int) (*model.UserPreference, error) {
	var preference model.UserPreference
	err := dbpkg.DB.Where("user_id = ?", userID).First(&preference).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// Return empty preference if not found
			return &model.UserPreference{UserID: userID}, nil
		}
		return nil, err
	}
	return &preference, nil
}

// UpsertPreference creates or updates user preferences
func (r *userPreferenceRepository) UpsertPreference(ctx context.Context, pref *model.UserPreference, fields []string) error {
	existing, err := r.GetByUserID(ctx, pref.UserID)
	if err != nil {
		return err
	}

	// If record exists, do a partial update
	if existing.ID > 0 {
		// Handle settings specially to merge instead of replace
		if len(pref.Settings) > 0 {
			if existing.Settings == nil {
				existing.Settings = model.JSON{}
			}
			for k, v := range pref.Settings {
				existing.Settings[k] = v
			}
			pref.Settings = existing.Settings
		}

		// Nothing to update
		if len(fields) == 0 {
			return nil
		}

		// Set ID to ensure we update the correct record
		pref.ID = existing.ID

		// Use Select to update only specified fields, including zero values
		return dbpkg.DB.Model(pref).Select(fields).Updates(pref).Error
	}

	// Create new record if it doesn't exist
	return dbpkg.DB.Create(pref).Error
}
