package providers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tencentyun/cos-go-sdk-v5"
	"github.com/veops/oneterm/pkg/logger"
	"github.com/veops/oneterm/pkg/storage"
	"go.uber.org/zap"
)

// COSConfig holds configuration for Tencent Cloud COS storage
type COSConfig struct {
	Region          string                         `json:"region" mapstructure:"region"`
	SecretID        string                         `json:"secret_id" mapstructure:"secret_id"`
	SecretKey       string                         `json:"secret_key" mapstructure:"secret_key"`
	BucketName      string                         `json:"bucket_name" mapstructure:"bucket_name"`
	AppID           string                         `json:"app_id" mapstructure:"app_id"`
	PathStrategy    storage.PathStrategy           `json:"path_strategy" mapstructure:"path_strategy"`
	RetentionConfig storage.StorageRetentionConfig `json:"retention" mapstructure:"retention"`
}

// COS implements the storage.Provider interface for Tencent Cloud COS storage
type COS struct {
	client        *cos.Client
	config        COSConfig
	pathGenerator *storage.PathGenerator
}

// NewCOS creates a new Tencent Cloud COS storage provider
func NewCOS(config COSConfig) (storage.Provider, error) {
	// Set default path strategy if not specified
	if config.PathStrategy == "" {
		config.PathStrategy = storage.DateHierarchyStrategy
	}

	// Set default retention config if not specified
	if config.RetentionConfig.RetentionDays == 0 {
		config.RetentionConfig = storage.DefaultRetentionConfig()
	}

	// Construct bucket URL
	bucketURL := fmt.Sprintf("https://%s-%s.cos.%s.myqcloud.com", config.BucketName, config.AppID, config.Region)
	u, err := url.Parse(bucketURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse bucket URL: %w", err)
	}

	// Create COS client
	client := cos.NewClient(&cos.BaseURL{BucketURL: u}, &http.Client{
		Transport: &cos.AuthorizationTransport{
			SecretID:  config.SecretID,
			SecretKey: config.SecretKey,
		},
	})

	// Check if bucket exists and is accessible
	ctx := context.Background()
	_, err = client.Bucket.Head(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to access bucket %s: %w", config.BucketName, err)
	}

	// Use bucket name as virtual base path for path generator
	pathGenerator := storage.NewPathGenerator(config.PathStrategy, config.BucketName)

	return &COS{
		client:        client,
		config:        config,
		pathGenerator: pathGenerator,
	}, nil
}

// Upload uploads a file to COS storage
func (c *COS) Upload(ctx context.Context, key string, reader io.Reader, size int64) error {
	objectKey := c.getObjectKey(key)

	_, err := c.client.Object.Put(ctx, objectKey, reader, &cos.ObjectPutOptions{
		ObjectPutHeaderOptions: &cos.ObjectPutHeaderOptions{
			ContentType: "application/octet-stream",
		},
	})
	if err != nil {
		return fmt.Errorf("failed to upload object: %w", err)
	}

	return nil
}

// Download downloads a file from COS storage
func (c *COS) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	objectKey := c.getObjectKey(key)

	// Try exact key first
	response, err := c.client.Object.Get(ctx, objectKey, nil)
	if err == nil {
		return response.Body, nil
	}

	// For date hierarchy strategy, search in date prefixes
	if c.config.PathStrategy == storage.DateHierarchyStrategy {
		return c.searchInDatePrefixes(ctx, key)
	}

	return nil, fmt.Errorf("object not found: %s", key)
}

// Delete deletes a file from COS storage
func (c *COS) Delete(ctx context.Context, key string) error {
	objectKey := c.getObjectKey(key)

	_, err := c.client.Object.Delete(ctx, objectKey)
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	return nil
}

// Exists checks if a file exists in COS storage
func (c *COS) Exists(ctx context.Context, key string) (bool, error) {
	objectKey := c.getObjectKey(key)

	// Try exact key first
	_, err := c.client.Object.Head(ctx, objectKey, nil)
	if err == nil {
		return true, nil
	}

	// For date hierarchy strategy, search in date prefixes
	if c.config.PathStrategy == storage.DateHierarchyStrategy {
		_, err := c.searchInDatePrefixes(ctx, key)
		return err == nil, nil
	}

	return false, nil
}

// GetSize gets the size of a file in COS storage
func (c *COS) GetSize(ctx context.Context, key string) (int64, error) {
	objectKey := c.getObjectKey(key)

	response, err := c.client.Object.Head(ctx, objectKey, nil)
	if err != nil {
		if c.config.PathStrategy == storage.DateHierarchyStrategy {
			return 0, fmt.Errorf("object not found: %s", key)
		}
		return 0, err
	}

	contentLength := response.Header.Get("Content-Length")
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
func (c *COS) Type() string {
	return "cos"
}

// HealthCheck performs a health check on COS storage
func (c *COS) HealthCheck(ctx context.Context) error {
	// Check if bucket is accessible
	_, err := c.client.Bucket.Head(ctx)
	if err != nil {
		return fmt.Errorf("COS health check failed: %w", err)
	}

	return nil
}

// GetPathStrategy returns the path strategy
func (c *COS) GetPathStrategy() storage.PathStrategy {
	return c.config.PathStrategy
}

// GetRetentionConfig returns the retention configuration
func (c *COS) GetRetentionConfig() storage.StorageRetentionConfig {
	return c.config.RetentionConfig
}

// getObjectKey resolves the object key for a given storage key
func (c *COS) getObjectKey(key string) string {
	// Remove bucket prefix if present (path generator includes it)
	if strings.HasPrefix(key, c.config.BucketName+"/") {
		return strings.TrimPrefix(key, c.config.BucketName+"/")
	}
	return key
}

// searchInDatePrefixes searches for an object in date-based prefixes
func (c *COS) searchInDatePrefixes(ctx context.Context, key string) (io.ReadCloser, error) {
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
		result, _, err := c.client.Bucket.Get(ctx, &cos.BucketGetOptions{
			Prefix:    dir + "/",
			Delimiter: "/",
			Marker:    marker,
		})
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

		response, err := c.client.Object.Get(ctx, possibleKey, nil)
		if err == nil {
			logger.L().Debug("Found object in date prefix",
				zap.String("key", possibleKey))
			return response.Body, nil
		}
	}

	return nil, fmt.Errorf("object not found in any date prefix: %s (searched %d prefixes)", key, len(datePrefixes))
}

// ParseCOSConfigFromMap creates COSConfig from string map (for database storage)
func ParseCOSConfigFromMap(configMap map[string]string) (COSConfig, error) {
	config := COSConfig{}

	config.Region = configMap["region"]
	config.SecretID = configMap["secret_id"]
	config.SecretKey = configMap["secret_key"]
	config.BucketName = configMap["bucket_name"]
	config.AppID = configMap["app_id"]

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
