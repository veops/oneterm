package providers

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/veops/oneterm/pkg/logger"
	"github.com/veops/oneterm/pkg/storage"
	"go.uber.org/zap"
)

// AzureConfig holds configuration for Azure Blob Storage
type AzureConfig struct {
	AccountName     string                         `json:"account_name" mapstructure:"account_name"`
	AccountKey      string                         `json:"account_key" mapstructure:"account_key"`
	ContainerName   string                         `json:"container_name" mapstructure:"container_name"`
	EndpointSuffix  string                         `json:"endpoint_suffix" mapstructure:"endpoint_suffix"`
	PathStrategy    storage.PathStrategy           `json:"path_strategy" mapstructure:"path_strategy"`
	RetentionConfig storage.StorageRetentionConfig `json:"retention" mapstructure:"retention"`
}

// Azure implements the storage.Provider interface for Azure Blob Storage
type Azure struct {
	containerURL  azblob.ContainerURL
	config        AzureConfig
	pathGenerator *storage.PathGenerator
}

// NewAzure creates a new Azure Blob Storage provider
func NewAzure(config AzureConfig) (storage.Provider, error) {
	// Set default path strategy if not specified
	if config.PathStrategy == "" {
		config.PathStrategy = storage.DateHierarchyStrategy
	}

	// Set default retention config if not specified
	if config.RetentionConfig.RetentionDays == 0 {
		config.RetentionConfig = storage.DefaultRetentionConfig()
	}

	// Set default endpoint suffix if not specified
	if config.EndpointSuffix == "" {
		config.EndpointSuffix = "core.windows.net"
	}

	// Create credential
	credential, err := azblob.NewSharedKeyCredential(config.AccountName, config.AccountKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure credential: %w", err)
	}

	// Create pipeline
	pipeline := azblob.NewPipeline(credential, azblob.PipelineOptions{})

	// Create service URL
	serviceURL, err := url.Parse(fmt.Sprintf("https://%s.blob.%s", config.AccountName, config.EndpointSuffix))
	if err != nil {
		return nil, fmt.Errorf("failed to parse service URL: %w", err)
	}

	// Create container URL
	containerURL := azblob.NewContainerURL(*serviceURL, pipeline)

	// Check if container exists and is accessible
	ctx := context.Background()
	_, err = containerURL.GetProperties(ctx, azblob.LeaseAccessConditions{})
	if err != nil {
		return nil, fmt.Errorf("failed to access container %s: %w", config.ContainerName, err)
	}

	// Use container name as virtual base path for path generator
	pathGenerator := storage.NewPathGenerator(config.PathStrategy, config.ContainerName)

	return &Azure{
		containerURL:  containerURL,
		config:        config,
		pathGenerator: pathGenerator,
	}, nil
}

// Upload uploads a file to Azure Blob Storage
func (a *Azure) Upload(ctx context.Context, key string, reader io.Reader, size int64) error {
	blobKey := a.getBlobKey(key)

	blobURL := a.containerURL.NewBlockBlobURL(blobKey)

	_, err := azblob.UploadStreamToBlockBlob(ctx, reader, blobURL, azblob.UploadStreamToBlockBlobOptions{
		BufferSize: 4 * 1024 * 1024, // 4MB buffer
		MaxBuffers: 16,
		BlobHTTPHeaders: azblob.BlobHTTPHeaders{
			ContentType: "application/octet-stream",
		},
	})
	if err != nil {
		return fmt.Errorf("failed to upload blob: %w", err)
	}

	return nil
}

// Download downloads a file from Azure Blob Storage
func (a *Azure) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	blobKey := a.getBlobKey(key)

	// Try exact key first
	blobURL := a.containerURL.NewBlobURL(blobKey)
	response, err := blobURL.Download(ctx, 0, azblob.CountToEnd, azblob.BlobAccessConditions{}, false, azblob.ClientProvidedKeyOptions{})
	if err == nil {
		return response.Body(azblob.RetryReaderOptions{}), nil
	}

	// For date hierarchy strategy, search in date prefixes
	if a.config.PathStrategy == storage.DateHierarchyStrategy {
		return a.searchInDatePrefixes(ctx, key)
	}

	return nil, fmt.Errorf("blob not found: %s", key)
}

// Delete deletes a file from Azure Blob Storage
func (a *Azure) Delete(ctx context.Context, key string) error {
	blobKey := a.getBlobKey(key)

	blobURL := a.containerURL.NewBlobURL(blobKey)
	_, err := blobURL.Delete(ctx, azblob.DeleteSnapshotsOptionInclude, azblob.BlobAccessConditions{})
	if err != nil {
		return fmt.Errorf("failed to delete blob: %w", err)
	}

	return nil
}

// Exists checks if a file exists in Azure Blob Storage
func (a *Azure) Exists(ctx context.Context, key string) (bool, error) {
	blobKey := a.getBlobKey(key)

	// Try exact key first
	blobURL := a.containerURL.NewBlobURL(blobKey)
	_, err := blobURL.GetProperties(ctx, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
	if err == nil {
		return true, nil
	}

	// For date hierarchy strategy, search in date prefixes
	if a.config.PathStrategy == storage.DateHierarchyStrategy {
		_, err := a.searchInDatePrefixes(ctx, key)
		return err == nil, nil
	}

	return false, nil
}

// GetSize gets the size of a file in Azure Blob Storage
func (a *Azure) GetSize(ctx context.Context, key string) (int64, error) {
	blobKey := a.getBlobKey(key)

	blobURL := a.containerURL.NewBlobURL(blobKey)
	properties, err := blobURL.GetProperties(ctx, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
	if err != nil {
		if a.config.PathStrategy == storage.DateHierarchyStrategy {
			return 0, fmt.Errorf("blob not found: %s", key)
		}
		return 0, err
	}

	return properties.ContentLength(), nil
}

// Type returns the storage type
func (a *Azure) Type() string {
	return "azure"
}

// HealthCheck performs a health check on Azure Blob Storage
func (a *Azure) HealthCheck(ctx context.Context) error {
	// Check if container is accessible
	_, err := a.containerURL.GetProperties(ctx, azblob.LeaseAccessConditions{})
	if err != nil {
		return fmt.Errorf("Azure health check failed: %w", err)
	}

	return nil
}

// GetPathStrategy returns the path strategy
func (a *Azure) GetPathStrategy() storage.PathStrategy {
	return a.config.PathStrategy
}

// GetRetentionConfig returns the retention configuration
func (a *Azure) GetRetentionConfig() storage.StorageRetentionConfig {
	return a.config.RetentionConfig
}

// getBlobKey resolves the blob key for a given storage key
func (a *Azure) getBlobKey(key string) string {
	// Remove container prefix if present (path generator includes it)
	if strings.HasPrefix(key, a.config.ContainerName+"/") {
		return strings.TrimPrefix(key, a.config.ContainerName+"/")
	}
	return key
}

// searchInDatePrefixes searches for a blob in date-based prefixes
func (a *Azure) searchInDatePrefixes(ctx context.Context, key string) (io.ReadCloser, error) {
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

	// List blobs with category prefix to find date directories
	var datePrefixes []string
	for marker := (azblob.Marker{}); marker.NotDone(); {
		listResponse, err := a.containerURL.ListBlobsHierarchySegment(ctx, marker, "/", azblob.ListBlobsSegmentOptions{
			Prefix: dir + "/",
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list blobs: %w", err)
		}

		// Collect date prefixes from blob prefixes
		for _, prefix := range listResponse.Segment.BlobPrefixes {
			prefixStr := prefix.Name
			// Extract date directory from prefix
			parts := strings.Split(strings.TrimPrefix(prefixStr, dir+"/"), "/")
			if len(parts) >= 1 {
				dateStr := parts[0]
				// Check if it looks like a date (YYYY-MM-DD)
				if len(dateStr) == 10 && dateStr[4] == '-' && dateStr[7] == '-' {
					datePrefixes = append(datePrefixes, prefixStr)
				}
			}
		}

		marker = listResponse.NextMarker
	}

	logger.L().Debug("Found date prefixes",
		zap.String("key", key),
		zap.Strings("prefixes", datePrefixes))

	// Search in date prefixes (newest first by sorting in reverse)
	for i := len(datePrefixes) - 1; i >= 0; i-- {
		possibleKey := datePrefixes[i] + filename
		logger.L().Debug("Trying date prefix",
			zap.String("possible_key", possibleKey))

		blobURL := a.containerURL.NewBlobURL(possibleKey)
		response, err := blobURL.Download(ctx, 0, azblob.CountToEnd, azblob.BlobAccessConditions{}, false, azblob.ClientProvidedKeyOptions{})
		if err == nil {
			logger.L().Debug("Found blob in date prefix",
				zap.String("key", possibleKey))
			return response.Body(azblob.RetryReaderOptions{}), nil
		}
	}

	return nil, fmt.Errorf("blob not found in any date prefix: %s (searched %d prefixes)", key, len(datePrefixes))
}

// ParseAzureConfigFromMap creates AzureConfig from string map (for database storage)
func ParseAzureConfigFromMap(configMap map[string]string) (AzureConfig, error) {
	config := AzureConfig{}

	config.AccountName = configMap["account_name"]
	config.AccountKey = configMap["account_key"]
	config.ContainerName = configMap["container_name"]
	config.EndpointSuffix = configMap["endpoint_suffix"]

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
