package providers

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/huaweicloud/huaweicloud-sdk-go-obs/obs"
	"github.com/veops/oneterm/pkg/logger"
	"github.com/veops/oneterm/pkg/storage"
	"go.uber.org/zap"
)

// OBSConfig holds configuration for Huawei Cloud OBS storage
type OBSConfig struct {
	Endpoint        string                         `json:"endpoint" mapstructure:"endpoint"`
	AccessKeyID     string                         `json:"access_key_id" mapstructure:"access_key_id"`
	SecretAccessKey string                         `json:"secret_access_key" mapstructure:"secret_access_key"`
	BucketName      string                         `json:"bucket_name" mapstructure:"bucket_name"`
	PathStrategy    storage.PathStrategy           `json:"path_strategy" mapstructure:"path_strategy"`
	RetentionConfig storage.StorageRetentionConfig `json:"retention" mapstructure:"retention"`
}

// OBS implements the storage.Provider interface for Huawei Cloud OBS storage
type OBS struct {
	client        *obs.ObsClient
	config        OBSConfig
	pathGenerator *storage.PathGenerator
}

// NewOBS creates a new Huawei Cloud OBS storage provider
func NewOBS(config OBSConfig) (storage.Provider, error) {
	// Set default path strategy if not specified
	if config.PathStrategy == "" {
		config.PathStrategy = storage.DateHierarchyStrategy
	}

	// Set default retention config if not specified
	if config.RetentionConfig.RetentionDays == 0 {
		config.RetentionConfig = storage.DefaultRetentionConfig()
	}

	// Create OBS client
	client, err := obs.New(config.AccessKeyID, config.SecretAccessKey, config.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to create OBS client: %w", err)
	}

	// Check if bucket exists and is accessible
	_, err = client.HeadBucket(config.BucketName)
	if err != nil {
		return nil, fmt.Errorf("failed to access bucket %s: %w", config.BucketName, err)
	}

	// Use bucket name as virtual base path for path generator
	pathGenerator := storage.NewPathGenerator(config.PathStrategy, config.BucketName)

	return &OBS{
		client:        client,
		config:        config,
		pathGenerator: pathGenerator,
	}, nil
}

// Upload uploads a file to OBS storage
func (o *OBS) Upload(ctx context.Context, key string, reader io.Reader, size int64) error {
	objectKey := o.getObjectKey(key)

	input := &obs.PutObjectInput{}
	input.Bucket = o.config.BucketName
	input.Key = objectKey
	input.Body = reader
	input.ContentType = "application/octet-stream"

	_, err := o.client.PutObject(input)
	if err != nil {
		return fmt.Errorf("failed to upload object: %w", err)
	}

	return nil
}

// Download downloads a file from OBS storage
func (o *OBS) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	objectKey := o.getObjectKey(key)

	// Try exact key first
	input := &obs.GetObjectInput{}
	input.Bucket = o.config.BucketName
	input.Key = objectKey

	output, err := o.client.GetObject(input)
	if err == nil {
		return output.Body, nil
	}

	// For date hierarchy strategy, search in date prefixes
	if o.config.PathStrategy == storage.DateHierarchyStrategy {
		return o.searchInDatePrefixes(ctx, key)
	}

	return nil, fmt.Errorf("object not found: %s", key)
}

// Delete deletes a file from OBS storage
func (o *OBS) Delete(ctx context.Context, key string) error {
	objectKey := o.getObjectKey(key)

	input := &obs.DeleteObjectInput{}
	input.Bucket = o.config.BucketName
	input.Key = objectKey

	_, err := o.client.DeleteObject(input)
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	return nil
}

// Exists checks if a file exists in OBS storage
func (o *OBS) Exists(ctx context.Context, key string) (bool, error) {
	objectKey := o.getObjectKey(key)

	// Try exact key first
	input := &obs.GetObjectMetadataInput{}
	input.Bucket = o.config.BucketName
	input.Key = objectKey

	_, err := o.client.GetObjectMetadata(input)
	if err == nil {
		return true, nil
	}

	// For date hierarchy strategy, search in date prefixes
	if o.config.PathStrategy == storage.DateHierarchyStrategy {
		_, err := o.searchInDatePrefixes(ctx, key)
		return err == nil, nil
	}

	return false, nil
}

// GetSize gets the size of a file in OBS storage
func (o *OBS) GetSize(ctx context.Context, key string) (int64, error) {
	objectKey := o.getObjectKey(key)

	input := &obs.GetObjectMetadataInput{}
	input.Bucket = o.config.BucketName
	input.Key = objectKey

	output, err := o.client.GetObjectMetadata(input)
	if err != nil {
		if o.config.PathStrategy == storage.DateHierarchyStrategy {
			return 0, fmt.Errorf("object not found: %s", key)
		}
		return 0, err
	}

	return output.ContentLength, nil
}

// Type returns the storage type
func (o *OBS) Type() string {
	return "obs"
}

// HealthCheck performs a health check on OBS storage
func (o *OBS) HealthCheck(ctx context.Context) error {
	// Check if bucket is accessible
	_, err := o.client.HeadBucket(o.config.BucketName)
	if err != nil {
		return fmt.Errorf("OBS health check failed: %w", err)
	}

	return nil
}

// GetPathStrategy returns the path strategy
func (o *OBS) GetPathStrategy() storage.PathStrategy {
	return o.config.PathStrategy
}

// GetRetentionConfig returns the retention configuration
func (o *OBS) GetRetentionConfig() storage.StorageRetentionConfig {
	return o.config.RetentionConfig
}

// getObjectKey resolves the object key for a given storage key
func (o *OBS) getObjectKey(key string) string {
	// Remove bucket prefix if present (path generator includes it)
	if strings.HasPrefix(key, o.config.BucketName+"/") {
		return strings.TrimPrefix(key, o.config.BucketName+"/")
	}
	return key
}

// searchInDatePrefixes searches for an object in date-based prefixes
func (o *OBS) searchInDatePrefixes(ctx context.Context, key string) (io.ReadCloser, error) {
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
	input := &obs.ListObjectsInput{}
	input.Bucket = o.config.BucketName
	input.Prefix = dir + "/"
	input.Delimiter = "/"

	for {
		output, err := o.client.ListObjects(input)
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}

		// Collect date prefixes from common prefixes
		for _, prefix := range output.CommonPrefixes {
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

		if !output.IsTruncated {
			break
		}
		input.Marker = output.NextMarker
	}

	logger.L().Debug("Found date prefixes",
		zap.String("key", key),
		zap.Strings("prefixes", datePrefixes))

	// Search in date prefixes (newest first by sorting in reverse)
	for i := len(datePrefixes) - 1; i >= 0; i-- {
		possibleKey := datePrefixes[i] + filename
		logger.L().Debug("Trying date prefix",
			zap.String("possible_key", possibleKey))

		getInput := &obs.GetObjectInput{}
		getInput.Bucket = o.config.BucketName
		getInput.Key = possibleKey

		output, err := o.client.GetObject(getInput)
		if err == nil {
			logger.L().Debug("Found object in date prefix",
				zap.String("key", possibleKey))
			return output.Body, nil
		}
	}

	return nil, fmt.Errorf("object not found in any date prefix: %s (searched %d prefixes)", key, len(datePrefixes))
}

// ParseOBSConfigFromMap creates OBSConfig from string map (for database storage)
func ParseOBSConfigFromMap(configMap map[string]string) (OBSConfig, error) {
	config := OBSConfig{}

	config.Endpoint = configMap["endpoint"]
	config.AccessKeyID = configMap["access_key_id"]
	config.SecretAccessKey = configMap["secret_access_key"]
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
