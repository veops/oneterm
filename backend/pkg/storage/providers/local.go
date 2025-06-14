package providers

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/veops/oneterm/pkg/storage"
)

// LocalConfig holds configuration for local storage
type LocalConfig struct {
	BasePath        string                         `json:"base_path" mapstructure:"base_path"`
	PathStrategy    storage.PathStrategy           `json:"path_strategy" mapstructure:"path_strategy"`
	RetentionConfig storage.StorageRetentionConfig `json:"retention" mapstructure:"retention"`
}

// Local implements the storage.Provider interface for local filesystem
type Local struct {
	config        LocalConfig
	pathGenerator *storage.PathGenerator
}

// NewLocal creates a new local storage provider
func NewLocal(config LocalConfig) (storage.Provider, error) {
	// Set default path strategy if not specified
	if config.PathStrategy == "" {
		config.PathStrategy = storage.DateHierarchyStrategy
	}

	// Set default retention config if not specified
	if config.RetentionConfig.RetentionDays == 0 {
		config.RetentionConfig = storage.DefaultRetentionConfig()
	}

	// Ensure base directory exists
	if err := os.MkdirAll(config.BasePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	pathGenerator := storage.NewPathGenerator(config.PathStrategy, config.BasePath)

	return &Local{
		config:        config,
		pathGenerator: pathGenerator,
	}, nil
}

// Upload uploads a file to local storage
func (p *Local) Upload(ctx context.Context, key string, reader io.Reader, size int64) error {
	fmt.Println("Local provider Upload called:", key)

	// For backward compatibility, if key doesn't include timestamp info,
	// we'll use current time for path generation
	filePath := p.getFilePath(key)

	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(file, reader)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// Download downloads a file from local storage
func (p *Local) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	filePath := p.getFilePath(key)

	// Try exact path first
	if file, err := os.Open(filePath); err == nil {
		return file, nil
	}

	// For date hierarchy strategy, search in date directories if direct path fails
	if p.config.PathStrategy == storage.DateHierarchyStrategy {
		return p.searchInDateDirectories(key)
	}

	return nil, fmt.Errorf("file not found: %s", key)
}

// Delete deletes a file from local storage
func (p *Local) Delete(ctx context.Context, key string) error {
	filePath := p.getFilePath(key)
	return os.Remove(filePath)
}

// Exists checks if a file exists in local storage
func (p *Local) Exists(ctx context.Context, key string) (bool, error) {
	filePath := p.getFilePath(key)

	// Try exact path first
	if _, err := os.Stat(filePath); err == nil {
		return true, nil
	}

	// For date hierarchy strategy, search in date directories
	if p.config.PathStrategy == storage.DateHierarchyStrategy {
		_, err := p.searchInDateDirectories(key)
		return err == nil, nil
	}

	return false, nil
}

// GetSize gets the size of a file
func (p *Local) GetSize(ctx context.Context, key string) (int64, error) {
	filePath := p.getFilePath(key)

	stat, err := os.Stat(filePath)
	if err != nil {
		// Search in date directories for date hierarchy strategy
		if p.config.PathStrategy == storage.DateHierarchyStrategy {
			file, err := p.searchInDateDirectories(key)
			if err != nil {
				return 0, err
			}
			defer file.Close()

			// Get file info from opened file
			if f, ok := file.(*os.File); ok {
				stat, err = f.Stat()
				if err != nil {
					return 0, err
				}
				return stat.Size(), nil
			}
		}
		return 0, err
	}

	return stat.Size(), nil
}

// Type returns the storage type
func (p *Local) Type() string {
	return "local"
}

// HealthCheck performs a health check on the local storage
func (p *Local) HealthCheck(ctx context.Context) error {
	// Check if base directory is writable
	testFile := filepath.Join(p.config.BasePath, ".health_check")
	file, err := os.Create(testFile)
	if err != nil {
		return fmt.Errorf("local storage health check failed: %w", err)
	}
	file.Close()
	os.Remove(testFile)

	return nil
}

// getFilePath resolves the file path for a given key
func (p *Local) getFilePath(key string) string {
	// For new path generation with timestamps, we need additional context
	// For now, maintain backward compatibility with direct key-to-path mapping
	return filepath.Join(p.config.BasePath, key)
}

// GetFilePathWithTimestamp generates file path with timestamp for new uploads
func (p *Local) GetFilePathWithTimestamp(key string, timestamp time.Time) string {
	// Extract category and filename from key
	dir := filepath.Dir(key)
	filename := filepath.Base(key)

	switch dir {
	case "replays":
		// Extract session ID from filename (remove .cast extension)
		sessionID := filename[:len(filename)-len(filepath.Ext(filename))]
		return p.pathGenerator.GenerateReplayPath(sessionID, timestamp)
	default:
		// Fallback to basic path generation
		return p.pathGenerator.GenerateReplayPath(filename, timestamp)
	}
}

// searchInDateDirectories searches for a file in date directories
func (p *Local) searchInDateDirectories(key string) (io.ReadCloser, error) {
	dir := filepath.Dir(key)
	filename := filepath.Base(key)
	categoryPath := filepath.Join(p.config.BasePath, dir)

	// List all date directories
	entries, err := os.ReadDir(categoryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read category directory: %w", err)
	}

	// Search in date directories (newest first)
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		if entry.IsDir() {
			// Check if directory name looks like a date (YYYY-MM-DD)
			if len(entry.Name()) == 10 && entry.Name()[4] == '-' && entry.Name()[7] == '-' {
				possiblePath := filepath.Join(categoryPath, entry.Name(), filename)
				if file, err := os.Open(possiblePath); err == nil {
					return file, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("file not found in any date directory: %s", key)
}

// GetPathStrategy returns the path strategy
func (p *Local) GetPathStrategy() storage.PathStrategy {
	return p.config.PathStrategy
}

// GetRetentionConfig returns the retention configuration
func (p *Local) GetRetentionConfig() storage.StorageRetentionConfig {
	return p.config.RetentionConfig
}
