package service

import (
	"context"
	"encoding/json"

	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/repository"
)

// UserPreferenceService defines the interface for user preference operations
type UserPreferenceService interface {
	GetUserPreference(ctx context.Context, userID int) (*model.UserPreference, error)
	UpdateUserPreference(ctx context.Context, userID int, prefData map[string]any) error
	GetDefaultPreference() *model.UserPreference
}

// userPreferenceService implements UserPreferenceService
type userPreferenceService struct {
	repo repository.UserPreferenceRepository
}

// DefaultUserPreferenceService singleton instance
var DefaultUserPreferenceService = NewUserPreferenceService()

// NewUserPreferenceService creates a new user preference service
func NewUserPreferenceService() UserPreferenceService {
	return &userPreferenceService{
		repo: repository.NewUserPreferenceRepository(),
	}
}

// GetUserPreference retrieves user preferences by user ID
// If no preferences exist, it returns the default preferences
func (s *userPreferenceService) GetUserPreference(ctx context.Context, userID int) (*model.UserPreference, error) {
	pref, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// If no preferences found, return default preferences with user ID
	if pref.ID == 0 {
		defaultPref := s.GetDefaultPreference()
		defaultPref.UserID = userID
		return defaultPref, nil
	}

	return pref, nil
}

// UpdateUserPreference updates user preferences
func (s *userPreferenceService) UpdateUserPreference(ctx context.Context, userID int, prefData map[string]any) error {
	// Create a list of fields to update
	var fields []string
	for field := range prefData {
		// Skip metadata fields
		if field != "id" && field != "user_id" && field != "created_at" && field != "updated_at" {
			fields = append(fields, field)
		}
	}

	// If no fields to update, return early
	if len(fields) == 0 {
		return nil
	}

	// Convert map to JSON
	jsonData, err := json.Marshal(prefData)
	if err != nil {
		return err
	}

	// Parse preference object
	var pref model.UserPreference
	if err := json.Unmarshal(jsonData, &pref); err != nil {
		return err
	}

	// Ensure user ID is set
	pref.UserID = userID

	// Call repository to update preferences
	return s.repo.UpsertPreference(ctx, &pref, fields)
}

// GetDefaultPreference returns the default terminal preferences
func (s *userPreferenceService) GetDefaultPreference() *model.UserPreference {
	return &model.UserPreference{
		Theme:       "default",
		FontFamily:  "monospace",
		FontSize:    12,
		LineHeight:  1.2,
		CursorStyle: "block",
		Settings: model.JSON{
			"cursorBlink":           true,
			"scrollback":            1000,
			"bellStyle":             "sound",
			"enableBold":            true,
			"enableItalic":          true,
			"tabStopWidth":          8,
			"wordSeparator":         " ()[]{}',\"`",
			"allowTransparency":     false,
			"screenReaderMode":      false,
			"rightClickSelectsWord": true,
		},
	}
}
