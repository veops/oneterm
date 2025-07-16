package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/gin-gonic/gin"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/repository"
	dbpkg "github.com/veops/oneterm/pkg/db"
)

// TimeTemplateService handles business logic for time templates
type TimeTemplateService struct {
	repo repository.ITimeTemplateRepository
}

// NewTimeTemplateService creates a new time template service
func NewTimeTemplateService() *TimeTemplateService {
	repo := repository.NewTimeTemplateRepository(dbpkg.DB)
	return &TimeTemplateService{
		repo: repo,
	}
}

// BuildQuery builds the base query for time templates
func (s *TimeTemplateService) BuildQuery(ctx *gin.Context) (*gorm.DB, error) {
	db := dbpkg.DB.Model(model.DefaultTimeTemplate)

	// Apply search filter
	if search := ctx.Query("search"); search != "" {
		db = db.Where("name LIKE ? OR description LIKE ?", "%"+search+"%", "%"+search+"%")
	}

	// Apply category filter
	if category := ctx.Query("category"); category != "" {
		db = db.Where("category = ?", category)
	}

	// Apply active filter
	if activeStr := ctx.Query("active"); activeStr != "" {
		active := activeStr == "true"
		db = db.Where("is_active = ?", active)
	}

	return db, nil
}

// CreateTimeTemplate creates a new time template
func (s *TimeTemplateService) CreateTimeTemplate(ctx context.Context, template *model.TimeTemplate) error {
	// Validate the template
	if err := s.ValidateTimeTemplate(template); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Check for duplicate names
	existing, err := s.repo.GetByName(ctx, template.Name)
	if err != nil {
		return fmt.Errorf("failed to check existing template: %w", err)
	}
	if existing != nil {
		return errors.New("template with this name already exists")
	}

	// Set default values
	if template.Timezone == "" {
		template.Timezone = "Asia/Shanghai"
	}
	template.IsBuiltIn = false
	template.UsageCount = 0

	return s.repo.Create(ctx, template)
}

// GetTimeTemplate retrieves a time template by ID
func (s *TimeTemplateService) GetTimeTemplate(ctx context.Context, id int) (*model.TimeTemplate, error) {
	return s.repo.GetByID(ctx, id)
}

// GetTimeTemplateByName retrieves a time template by name
func (s *TimeTemplateService) GetTimeTemplateByName(ctx context.Context, name string) (*model.TimeTemplate, error) {
	return s.repo.GetByName(ctx, name)
}

// ListTimeTemplates retrieves time templates with pagination and filters
func (s *TimeTemplateService) ListTimeTemplates(ctx context.Context, offset, limit int, category string, active *bool) ([]*model.TimeTemplate, int64, error) {
	return s.repo.List(ctx, offset, limit, category, active)
}

// UpdateTimeTemplate updates an existing time template
func (s *TimeTemplateService) UpdateTimeTemplate(ctx context.Context, template *model.TimeTemplate) error {
	// Validate the template
	if err := s.ValidateTimeTemplate(template); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Check if template exists
	existing, err := s.repo.GetByID(ctx, template.Id)
	if err != nil {
		return fmt.Errorf("failed to get existing template: %w", err)
	}
	if existing == nil {
		return errors.New("time template not found")
	}

	// Don't allow changing built-in status
	template.IsBuiltIn = existing.IsBuiltIn
	template.UsageCount = existing.UsageCount

	return s.repo.Update(ctx, template)
}

// DeleteTimeTemplate deletes a time template
func (s *TimeTemplateService) DeleteTimeTemplate(ctx context.Context, id int) error {
	return s.repo.Delete(ctx, id)
}

// UseTimeTemplate increments usage count and returns the template
func (s *TimeTemplateService) UseTimeTemplate(ctx context.Context, id int) (*model.TimeTemplate, error) {
	template, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if template == nil {
		return nil, errors.New("time template not found")
	}

	// Increment usage count
	if err := s.repo.IncrementUsage(ctx, id); err != nil {
		// Log error but don't fail the operation
		fmt.Printf("Failed to increment usage count for template %d: %v\n", id, err)
	}

	return template, nil
}

// GetBuiltInTemplates retrieves all built-in time templates
func (s *TimeTemplateService) GetBuiltInTemplates(ctx context.Context) ([]*model.TimeTemplate, error) {
	return s.repo.GetBuiltInTemplates(ctx)
}

// InitializeBuiltInTemplates initializes built-in time templates
func (s *TimeTemplateService) InitializeBuiltInTemplates(ctx context.Context) error {
	return s.repo.InitBuiltInTemplates(ctx)
}

// ValidateTimeTemplate validates a time template
func (s *TimeTemplateService) ValidateTimeTemplate(template *model.TimeTemplate) error {
	if template.Name == "" {
		return errors.New("template name is required")
	}

	if len(template.Name) > 128 {
		return errors.New("template name too long (max 128 characters)")
	}

	if template.Category == "" {
		return errors.New("template category is required")
	}

	// Validate category
	validCategories := []string{"work", "duty", "maintenance", "emergency", "always", "custom"}
	categoryValid := false
	for _, cat := range validCategories {
		if template.Category == cat {
			categoryValid = true
			break
		}
	}
	if !categoryValid {
		return fmt.Errorf("invalid category: %s. Valid categories: %v", template.Category, validCategories)
	}

	// Validate timezone
	if template.Timezone != "" {
		if _, err := time.LoadLocation(template.Timezone); err != nil {
			return fmt.Errorf("invalid timezone: %s", template.Timezone)
		}
	}

	// Validate time ranges
	if len(template.TimeRanges) == 0 {
		return errors.New("at least one time range is required")
	}

	for i, timeRange := range template.TimeRanges {
		if err := s.validateTimeRange(timeRange); err != nil {
			return fmt.Errorf("invalid time range %d: %w", i+1, err)
		}
	}

	return nil
}

// validateTimeRange validates a single time range
func (s *TimeTemplateService) validateTimeRange(timeRange model.TimeRange) error {
	// Validate time format (HH:MM)
	timePattern := regexp.MustCompile(`^([0-1]?[0-9]|2[0-3]):[0-5][0-9]$`)

	if !timePattern.MatchString(timeRange.StartTime) {
		return fmt.Errorf("invalid start time format: %s (expected HH:MM)", timeRange.StartTime)
	}

	if !timePattern.MatchString(timeRange.EndTime) {
		return fmt.Errorf("invalid end time format: %s (expected HH:MM)", timeRange.EndTime)
	}

	// Parse and validate time logic
	startMinutes, err := s.parseTimeToMinutes(timeRange.StartTime)
	if err != nil {
		return fmt.Errorf("invalid start time: %w", err)
	}

	endMinutes, err := s.parseTimeToMinutes(timeRange.EndTime)
	if err != nil {
		return fmt.Errorf("invalid end time: %w", err)
	}

	if startMinutes >= endMinutes {
		return errors.New("start time must be before end time")
	}

	// Validate weekdays
	if len(timeRange.Weekdays) == 0 {
		return errors.New("at least one weekday must be specified")
	}

	for _, day := range timeRange.Weekdays {
		if day < 1 || day > 7 {
			return fmt.Errorf("invalid weekday: %d (must be 1-7, where 1=Monday, 7=Sunday)", day)
		}
	}

	return nil
}

// parseTimeToMinutes converts HH:MM format to minutes since midnight
func (s *TimeTemplateService) parseTimeToMinutes(timeStr string) (int, error) {
	parts := strings.Split(timeStr, ":")
	if len(parts) != 2 {
		return 0, errors.New("invalid time format")
	}

	hour, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, err
	}

	minute, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, err
	}

	return hour*60 + minute, nil
}

// CheckTimeAccess checks if current time is within the template's allowed time ranges
func (s *TimeTemplateService) CheckTimeAccess(ctx context.Context, templateID int, timezone string) (bool, error) {
	template, err := s.repo.GetByID(ctx, templateID)
	if err != nil {
		return false, err
	}
	if template == nil {
		return false, errors.New("time template not found")
	}

	return s.IsTimeInTemplate(template, timezone), nil
}

// IsTimeInTemplate checks if current time matches any time range in the template
func (s *TimeTemplateService) IsTimeInTemplate(template *model.TimeTemplate, timezone string) bool {
	// Use template's timezone if not specified
	if timezone == "" {
		timezone = template.Timezone
	}
	if timezone == "" {
		timezone = "Asia/Shanghai" // Default timezone
	}

	// Load timezone location
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		// Fall back to UTC if timezone is invalid
		loc = time.UTC
	}

	now := time.Now().In(loc)
	currentWeekday := int(now.Weekday())
	if currentWeekday == 0 {
		currentWeekday = 7 // Convert Sunday from 0 to 7
	}

	currentMinutes := now.Hour()*60 + now.Minute()

	// Check each time range
	for _, timeRange := range template.TimeRanges {
		// Check if current weekday is in the allowed weekdays
		weekdayMatch := false
		for _, day := range timeRange.Weekdays {
			if day == currentWeekday {
				weekdayMatch = true
				break
			}
		}
		if !weekdayMatch {
			continue
		}

		// Check if current time is in the allowed time range
		startMinutes, err := s.parseTimeToMinutes(timeRange.StartTime)
		if err != nil {
			continue
		}

		endMinutes, err := s.parseTimeToMinutes(timeRange.EndTime)
		if err != nil {
			continue
		}

		if currentMinutes >= startMinutes && currentMinutes <= endMinutes {
			return true
		}
	}

	return false
}
