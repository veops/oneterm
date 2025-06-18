package providers

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/veops/oneterm/pkg/logger"
	"github.com/veops/oneterm/pkg/storage"
	"go.uber.org/zap"
)

// OSSConfig holds configuration for Alibaba Cloud OSS storage
type OSSConfig struct {
	Endpoint        string                         `json:"endpoint" mapstructure:"endpoint"`
	AccessKeyID     string                         `json:"access_key_id" mapstructure:"access_key_id"`
	AccessKeySecret string                         `json:"access_key_secret" mapstructure:"access_key_secret"`
	BucketName      string                         `json:"bucket_name" mapstructure:"bucket_name"`
	PathStrategy    storage.PathStrategy           `json:"path_strategy" mapstructure:"path_strategy"`
	RetentionConfig storage.StorageRetentionConfig `json:"retention" mapstructure:"retention"`
}

// OSS implements the storage.Provider interface for Alibaba Cloud OSS storage
type OSS struct {
	client        *oss.Client
	bucket        *oss.Bucket
	config        OSSConfig
	pathGenerator *storage.PathGenerator
}

// NewOSS creates a new Alibaba Cloud OSS storage provider
func NewOSS(config OSSConfig) (storage.Provider, error) {
	// Set default path strategy if not specified
	if config.PathStrategy == "" {
		config.PathStrategy = storage.DateHierarchyStrategy
	}

	// Set default retention config if not specified
	if config.RetentionConfig.RetentionDays == 0 {
		config.RetentionConfig = storage.DefaultRetentionConfig()
	}

	// Create OSS client
	client, err := oss.New(config.Endpoint, config.AccessKeyID, config.AccessKeySecret)
	if err != nil {
		return nil, fmt.Errorf("failed to create OSS client: %w", err)
	}

	// Get bucket
	bucket, err := client.Bucket(config.BucketName)
	if err != nil {
		return nil, fmt.Errorf("failed to get bucket %s: %w", config.BucketName, err)
	}

	// Check if bucket exists and is accessible
	exists, err := client.IsBucketExist(config.BucketName)
	if err != nil {
		return nil, fmt.Errorf("failed to check bucket existence: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("bucket %s does not exist", config.BucketName)
	}

	// Use bucket name as virtual base path for path generator
	pathGenerator := storage.NewPathGenerator(config.PathStrategy, config.BucketName)

	return &OSS{
		client:        client,
		bucket:        bucket,
		config:        config,
		pathGenerator: pathGenerator,
	}, nil
}

// Upload uploads a file to OSS storage
func (o *OSS) Upload(ctx context.Context, key string, reader io.Reader, size int64) error {
	objectKey := o.getObjectKey(key)

	err := o.bucket.PutObject(objectKey, reader, oss.ContentType("application/octet-stream"))
	if err != nil {
		return fmt.Errorf("failed to upload object: %w", err)
	}

	return nil
}

// Download downloads a file from OSS storage
func (o *OSS) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	objectKey := o.getObjectKey(key)

	// Try exact key first
	reader, err := o.bucket.GetObject(objectKey)
	if err == nil {
		return reader, nil
	}

	// For date hierarchy strategy, search in date prefixes
	if o.config.PathStrategy == storage.DateHierarchyStrategy {
		return o.searchInDatePrefixes(ctx, key)
	}

	return nil, fmt.Errorf("object not found: %s", key)
}

// Delete deletes a file from OSS storage
func (o *OSS) Delete(ctx context.Context, key string) error {
	objectKey := o.getObjectKey(key)

	err := o.bucket.DeleteObject(objectKey)
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	return nil
}

// Exists checks if a file exists in OSS storage
func (o *OSS) Exists(ctx context.Context, key string) (bool, error) {
	objectKey := o.getObjectKey(key)

	// Try exact key first
	exists, err := o.bucket.IsObjectExist(objectKey)
	if err != nil {
		return false, err
	}
	if exists {
		return true, nil
	}

	// For date hierarchy strategy, search in date prefixes
	if o.config.PathStrategy == storage.DateHierarchyStrategy {
		_, err := o.searchInDatePrefixes(ctx, key)
		return err == nil, nil
	}

	return false, nil
}

// GetSize gets the size of a file in OSS storage
func (o *OSS) GetSize(ctx context.Context, key string) (int64, error) {
	objectKey := o.getObjectKey(key)

	meta, err := o.bucket.GetObjectMeta(objectKey)
	if err != nil {
		if o.config.PathStrategy == storage.DateHierarchyStrategy {
			return 0, fmt.Errorf("object not found: %s", key)
		}
		return 0, err
	}

	contentLength := meta.Get("Content-Length")
	if contentLength == "" {
		return 0, nil
	}

	size, err := strconv.ParseInt(contentLength, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse content length: %w", err)
	}

	return size, nil
}

// Type returns the storage type
func (o *OSS) Type() string {
	return "oss"
}

// HealthCheck performs a health check on OSS storage
func (o *OSS) HealthCheck(ctx context.Context) error {
	// Check if bucket is accessible
	exists, err := o.client.IsBucketExist(o.config.BucketName)
	if err != nil {
		return fmt.Errorf("OSS health check failed: %w", err)
	}
	if !exists {
		return fmt.Errorf("bucket %s does not exist", o.config.BucketName)
	}

	return nil
}

// GetPathStrategy returns the path strategy
func (o *OSS) GetPathStrategy() storage.PathStrategy {
	return o.config.PathStrategy
}

// GetRetentionConfig returns the retention configuration
func (o *OSS) GetRetentionConfig() storage.StorageRetentionConfig {
	return o.config.RetentionConfig
}

// getObjectKey resolves the object key for a given storage key
func (o *OSS) getObjectKey(key string) string {
	// Remove bucket prefix if present (path generator includes it)
	if strings.HasPrefix(key, o.config.BucketName+"/") {
		return strings.TrimPrefix(key, o.config.BucketName+"/")
	}
	return key
}

// searchInDatePrefixes searches for an object in date-based prefixes
func (o *OSS) searchInDatePrefixes(ctx context.Context, key string) (io.ReadCloser, error) {
	dir := filepath.Dir(key)
	filename := filepath.Base(key)

	// For flat keys like "sessionID.cast", treat as replays category
	if dir == "." {
		dir = "replays"
		key = "replays/" + filename
	}

	logger.L().Debug("Searching in date prefixes",
		zap.String("original_key", key),
		zap.String("category", dir),
		zap.String("filename", filename))

	// List objects with category prefix to find date directories
	var datePrefixes []string
	marker := ""
	for {
		result, err := o.bucket.ListObjects(oss.Prefix(dir+"/"), oss.Delimiter("/"), oss.Marker(marker))
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}

		// Collect date prefixes from common prefixes
		for _, prefix := range result.CommonPrefixes {
			// Extract date directory from prefix
			parts := strings.Split(strings.TrimPrefix(prefix, dir+"/"), "/")
			if len(parts) >= 1 {
				dateStr := parts[0]
				// Check if it looks like a date (YYYY-MM-DD)
				if len(dateStr) == 10 && dateStr[4] == '-' && dateStr[7] == '-' {
					datePrefixes = append(datePrefixes, prefix)
				}
			}
		}

		if !result.IsTruncated {
			break
		}
		marker = result.NextMarker
	}

	logger.L().Debug("Found date prefixes",
		zap.String("key", key),
		zap.Strings("prefixes", datePrefixes))

	// Search in date prefixes (newest first by sorting in reverse)
	for i := len(datePrefixes) - 1; i >= 0; i-- {
		possibleKey := datePrefixes[i] + filename
		logger.L().Debug("Trying date prefix",
			zap.String("possible_key", possibleKey))

		reader, err := o.bucket.GetObject(possibleKey)
		if err == nil {
			logger.L().Debug("Found object in date prefix",
				zap.String("key", possibleKey))
			return reader, nil
		}
	}

	return nil, fmt.Errorf("object not found in any date prefix: %s (searched %d prefixes)", key, len(datePrefixes))
}

// ParseOSSConfigFromMap creates OSSConfig from string map (for database storage)
func ParseOSSConfigFromMap(configMap map[string]string) (OSSConfig, error) {
	config := OSSConfig{}

	config.Endpoint = configMap["endpoint"]
	config.AccessKeyID = configMap["access_key_id"]
	config.AccessKeySecret = configMap["access_key_secret"]
	config.BucketName = configMap["bucket_name"]

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
