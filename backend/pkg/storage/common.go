package storage

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// PathStrategy defines different file path generation strategies
type PathStrategy string

const (
	// FlatStrategy stores all files in a single directory
	FlatStrategy PathStrategy = "flat"
	// DateHierarchyStrategy organizes files by date hierarchy (YYYY-MM-DD)
	DateHierarchyStrategy PathStrategy = "date_hierarchy"
)

// StorageRetentionConfig holds retention and archival configuration
type StorageRetentionConfig struct {
	RetentionDays  int  `json:"retention_days" mapstructure:"retention_days"`   // Keep files for N days
	ArchiveDays    int  `json:"archive_days" mapstructure:"archive_days"`       // Archive files after N days
	CleanupEnabled bool `json:"cleanup_enabled" mapstructure:"cleanup_enabled"` // Enable automatic cleanup
	ArchiveEnabled bool `json:"archive_enabled" mapstructure:"archive_enabled"` // Enable automatic archival
}

// DefaultRetentionConfig returns default retention configuration
func DefaultRetentionConfig() StorageRetentionConfig {
	return StorageRetentionConfig{
		RetentionDays:  30,
		ArchiveDays:    7,
		CleanupEnabled: true,
		ArchiveEnabled: true,
	}
}

// PathGenerator generates storage paths based on strategy
type PathGenerator struct {
	Strategy PathStrategy
	BaseDir  string
}

// NewPathGenerator creates a new path generator
func NewPathGenerator(strategy PathStrategy, baseDir string) *PathGenerator {
	return &PathGenerator{
		Strategy: strategy,
		BaseDir:  baseDir,
	}
}

// GenerateReplayPath generates path for session replay files
func (pg *PathGenerator) GenerateReplayPath(sessionID string, timestamp time.Time) string {
	return pg.generatePath("replays", sessionID+".cast", timestamp)
}

// GenerateRDPFilePath generates path for RDP files
func (pg *PathGenerator) GenerateRDPFilePath(assetID int, remotePath string, timestamp time.Time) string {
	// Sanitize remote path for safe storage
	safeName := strings.ReplaceAll(remotePath, "/", "_")
	safeName = strings.ReplaceAll(safeName, "\\", "_")
	filename := fmt.Sprintf("asset_%d_%s", assetID, safeName)
	return pg.generatePath("rdp_files", filename, timestamp)
}

// GenerateArchivePath generates path for archived files
func (pg *PathGenerator) GenerateArchivePath(category string, archiveDate time.Time) string {
	archiveName := fmt.Sprintf("%s_%s.tar.gz", category, archiveDate.Format("2006-01"))
	return filepath.Join(pg.BaseDir, "archived", archiveName)
}

// generatePath generates path based on strategy
func (pg *PathGenerator) generatePath(category, filename string, timestamp time.Time) string {
	switch pg.Strategy {
	case DateHierarchyStrategy:
		dateDir := timestamp.Format("2006-01-02")
		return filepath.Join(pg.BaseDir, category, dateDir, filename)
	case FlatStrategy:
		fallthrough
	default:
		return filepath.Join(pg.BaseDir, category, filename)
	}
}

// ParseDateFromPath extracts date from hierarchical path
func (pg *PathGenerator) ParseDateFromPath(filePath string) (time.Time, error) {
	if pg.Strategy != DateHierarchyStrategy {
		return time.Time{}, fmt.Errorf("path strategy does not support date parsing")
	}

	rel, err := filepath.Rel(pg.BaseDir, filePath)
	if err != nil {
		return time.Time{}, err
	}

	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) < 2 {
		return time.Time{}, fmt.Errorf("invalid path format for date extraction")
	}

	// Expected format: category/YYYY-MM-DD/filename
	dateStr := parts[1]
	return time.Parse("2006-01-02", dateStr)
}

// ListDatedDirectories returns all date directories for a category
func (pg *PathGenerator) ListDatedDirectories(category string) ([]string, error) {
	if pg.Strategy != DateHierarchyStrategy {
		return nil, fmt.Errorf("path strategy does not support date directories")
	}

	categoryPath := filepath.Join(pg.BaseDir, category)
	return listDirectoriesByPattern(categoryPath, "2006-01-02")
}

// GetPathsOlderThan returns file paths older than specified days
func (pg *PathGenerator) GetPathsOlderThan(category string, days int) ([]string, error) {
	cutoffDate := time.Now().AddDate(0, 0, -days)
	var oldPaths []string

	if pg.Strategy == DateHierarchyStrategy {
		dirs, err := pg.ListDatedDirectories(category)
		if err != nil {
			return nil, err
		}

		for _, dir := range dirs {
			dirDate, err := time.Parse("2006-01-02", filepath.Base(dir))
			if err != nil {
				continue
			}
			if dirDate.Before(cutoffDate) {
				oldPaths = append(oldPaths, dir)
			}
		}
	}

	return oldPaths, nil
}

// listDirectoriesByPattern lists directories matching date pattern
func listDirectoriesByPattern(basePath, pattern string) ([]string, error) {
	// This is a simplified implementation
	// In production, you would use filepath.Walk or similar
	// to find directories matching the date pattern
	return nil, fmt.Errorf("not implemented")
}
