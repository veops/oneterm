package providers

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/veops/oneterm/pkg/storage"
)

// MinioConfig holds configuration for Minio storage
type MinioConfig struct {
	Endpoint        string                         `json:"endpoint" mapstructure:"endpoint"`
	AccessKeyID     string                         `json:"access_key_id" mapstructure:"access_key_id"`
	SecretAccessKey string                         `json:"secret_access_key" mapstructure:"secret_access_key"`
	UseSSL          bool                           `json:"use_ssl" mapstructure:"use_ssl"`
	BucketName      string                         `json:"bucket_name" mapstructure:"bucket_name"`
	PathStrategy    storage.PathStrategy           `json:"path_strategy" mapstructure:"path_strategy"`
	RetentionConfig storage.StorageRetentionConfig `json:"retention" mapstructure:"retention"`
}

// Minio implements the storage.Provider interface for Minio object storage
type Minio struct {
	client        *minio.Client
	config        MinioConfig
	pathGenerator *storage.PathGenerator
}

// NewMinio creates a new Minio storage provider
func NewMinio(config MinioConfig) (storage.Provider, error) {
	// Set default path strategy if not specified
	if config.PathStrategy == "" {
		config.PathStrategy = storage.DateHierarchyStrategy
	}

	// Set default retention config if not specified
	if config.RetentionConfig.RetentionDays == 0 {
		config.RetentionConfig = storage.DefaultRetentionConfig()
	}

	// Initialize Minio client
	client, err := minio.New(config.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.AccessKeyID, config.SecretAccessKey, ""),
		Secure: config.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Minio client: %w", err)
	}

	// Create bucket if it doesn't exist
	ctx := context.Background()
	exists, err := client.BucketExists(ctx, config.BucketName)
	if err != nil {
		return nil, fmt.Errorf("failed to check if bucket exists: %w", err)
	}
	if !exists {
		err = client.MakeBucket(ctx, config.BucketName, minio.MakeBucketOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to create bucket: %w", err)
		}
	}

	// Use bucket name as virtual base path for path generator
	pathGenerator := storage.NewPathGenerator(config.PathStrategy, config.BucketName)

	return &Minio{
		client:        client,
		config:        config,
		pathGenerator: pathGenerator,
	}, nil
}

// Upload uploads a file to Minio storage
func (m *Minio) Upload(ctx context.Context, key string, reader io.Reader, size int64) error {
	objectKey := m.getObjectKey(key)

	_, err := m.client.PutObject(ctx, m.config.BucketName, objectKey, reader, size, minio.PutObjectOptions{
		ContentType: "application/octet-stream",
	})
	if err != nil {
		return fmt.Errorf("failed to upload object: %w", err)
	}

	return nil
}

// Download downloads a file from Minio storage
func (m *Minio) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	objectKey := m.getObjectKey(key)

	// Try exact key first
	object, err := m.client.GetObject(ctx, m.config.BucketName, objectKey, minio.GetObjectOptions{})
	if err == nil {
		// Verify object exists by reading stat
		_, err = object.Stat()
		if err == nil {
			return object, nil
		}
		object.Close()
	}

	// For date hierarchy strategy, search in date prefixes
	if m.config.PathStrategy == storage.DateHierarchyStrategy {
		return m.searchInDatePrefixes(ctx, key)
	}

	return nil, fmt.Errorf("object not found: %s", key)
}

// Delete deletes a file from Minio storage
func (m *Minio) Delete(ctx context.Context, key string) error {
	objectKey := m.getObjectKey(key)
	return m.client.RemoveObject(ctx, m.config.BucketName, objectKey, minio.RemoveObjectOptions{})
}

// Exists checks if a file exists in Minio storage
func (m *Minio) Exists(ctx context.Context, key string) (bool, error) {
	objectKey := m.getObjectKey(key)

	// Try exact key first
	_, err := m.client.StatObject(ctx, m.config.BucketName, objectKey, minio.StatObjectOptions{})
	if err == nil {
		return true, nil
	}

	// For date hierarchy strategy, search in date prefixes
	if m.config.PathStrategy == storage.DateHierarchyStrategy {
		_, err := m.searchInDatePrefixes(ctx, key)
		return err == nil, nil
	}

	return false, nil
}

// GetSize gets the size of a file in Minio storage
func (m *Minio) GetSize(ctx context.Context, key string) (int64, error) {
	objectKey := m.getObjectKey(key)

	stat, err := m.client.StatObject(ctx, m.config.BucketName, objectKey, minio.StatObjectOptions{})
	if err != nil {
		// For date hierarchy strategy, we can't easily get size from search
		// Return 0 as fallback since we can't stat the object in date prefixes efficiently
		if m.config.PathStrategy == storage.DateHierarchyStrategy {
			return 0, fmt.Errorf("object not found: %s", key)
		}
		return 0, err
	}

	return stat.Size, nil
}

// Type returns the storage type
func (m *Minio) Type() string {
	return "minio"
}

// HealthCheck performs a health check on Minio storage
func (m *Minio) HealthCheck(ctx context.Context) error {
	// Check if bucket is accessible
	_, err := m.client.BucketExists(ctx, m.config.BucketName)
	if err != nil {
		return fmt.Errorf("Minio health check failed: %w", err)
	}

	return nil
}

// GetPathStrategy returns the path strategy
func (m *Minio) GetPathStrategy() storage.PathStrategy {
	return m.config.PathStrategy
}

// GetRetentionConfig returns the retention configuration
func (m *Minio) GetRetentionConfig() storage.StorageRetentionConfig {
	return m.config.RetentionConfig
}

// getObjectKey resolves the object key for a given storage key
func (m *Minio) getObjectKey(key string) string {
	// Remove bucket prefix if present (path generator includes it)
	if strings.HasPrefix(key, m.config.BucketName+"/") {
		return strings.TrimPrefix(key, m.config.BucketName+"/")
	}
	return key
}

// GetObjectKeyWithTimestamp generates object key with timestamp for new uploads
func (m *Minio) GetObjectKeyWithTimestamp(key string, timestamp time.Time) string {
	// Extract category and filename from key
	dir := filepath.Dir(key)
	filename := filepath.Base(key)

	switch dir {
	case "replays":
		// Generate date-based path for replays
		if m.config.PathStrategy == storage.DateHierarchyStrategy {
			dateDir := timestamp.Format("2006-01-02")
			return "replays/" + dateDir + "/" + filename
		}
	}

	// Fallback to direct key
	return key
}

// searchInDatePrefixes searches for an object in date-based prefixes
func (m *Minio) searchInDatePrefixes(ctx context.Context, key string) (io.ReadCloser, error) {
	dir := filepath.Dir(key)
	filename := filepath.Base(key)

	// List objects with category prefix to find date directories
	objectCh := m.client.ListObjects(ctx, m.config.BucketName, minio.ListObjectsOptions{
		Prefix:    dir + "/",
		Recursive: false,
	})

	// Collect date prefixes
	var datePrefixes []string
	for object := range objectCh {
		if object.Err != nil {
			continue
		}

		// Extract date directory from object key
		parts := strings.Split(strings.TrimPrefix(object.Key, dir+"/"), "/")
		if len(parts) >= 1 {
			dateStr := parts[0]
			// Check if it looks like a date (YYYY-MM-DD)
			if len(dateStr) == 10 && dateStr[4] == '-' && dateStr[7] == '-' {
				prefix := dir + "/" + dateStr + "/"
				// Avoid duplicates
				found := false
				for _, p := range datePrefixes {
					if p == prefix {
						found = true
						break
					}
				}
				if !found {
					datePrefixes = append(datePrefixes, prefix)
				}
			}
		}
	}

	// Search in date prefixes (newest first by sorting in reverse)
	for i := len(datePrefixes) - 1; i >= 0; i-- {
		possibleKey := datePrefixes[i] + filename
		object, err := m.client.GetObject(ctx, m.config.BucketName, possibleKey, minio.GetObjectOptions{})
		if err == nil {
			// Verify object exists
			_, err = object.Stat()
			if err == nil {
				return object, nil
			}
			object.Close()
		}
	}

	return nil, fmt.Errorf("object not found in any date prefix: %s", key)
}

// ParseMinioConfigFromMap creates MinioConfig from string map (for database storage)
func ParseMinioConfigFromMap(configMap map[string]string) (MinioConfig, error) {
	config := MinioConfig{}

	config.Endpoint = configMap["endpoint"]
	config.AccessKeyID = configMap["access_key_id"]
	config.SecretAccessKey = configMap["secret_access_key"]
	config.BucketName = configMap["bucket_name"]

	// Parse boolean fields
	if useSSLStr, exists := configMap["use_ssl"]; exists {
		config.UseSSL = useSSLStr == "true"
	}

	// Parse path strategy
	if strategyStr, exists := configMap["path_strategy"]; exists {
		config.PathStrategy = storage.PathStrategy(strategyStr)
	} else {
		config.PathStrategy = storage.DateHierarchyStrategy
	}

	// Parse retention configuration
	retentionConfig := storage.DefaultRetentionConfig()
	if retentionDaysStr, exists := configMap["retention_days"]; exists {
		if days, err := strconv.Atoi(retentionDaysStr); err == nil {
			retentionConfig.RetentionDays = days
		}
	}
	if archiveDaysStr, exists := configMap["archive_days"]; exists {
		if days, err := strconv.Atoi(archiveDaysStr); err == nil {
			retentionConfig.ArchiveDays = days
		}
	}
	if cleanupStr, exists := configMap["cleanup_enabled"]; exists {
		retentionConfig.CleanupEnabled = cleanupStr == "true"
	}
	if archiveStr, exists := configMap["archive_enabled"]; exists {
		retentionConfig.ArchiveEnabled = archiveStr == "true"
	}
	config.RetentionConfig = retentionConfig

	return config, nil
}
