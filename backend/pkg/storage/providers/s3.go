package providers

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/veops/oneterm/pkg/logger"
	"github.com/veops/oneterm/pkg/storage"
	"go.uber.org/zap"
)

// S3Config holds configuration for AWS S3 storage
type S3Config struct {
	Region          string                         `json:"region" mapstructure:"region"`
	AccessKeyID     string                         `json:"access_key_id" mapstructure:"access_key_id"`
	SecretAccessKey string                         `json:"secret_access_key" mapstructure:"secret_access_key"`
	BucketName      string                         `json:"bucket_name" mapstructure:"bucket_name"`
	Endpoint        string                         `json:"endpoint" mapstructure:"endpoint"` // For S3-compatible services
	UseSSL          bool                           `json:"use_ssl" mapstructure:"use_ssl"`
	PathStrategy    storage.PathStrategy           `json:"path_strategy" mapstructure:"path_strategy"`
	RetentionConfig storage.StorageRetentionConfig `json:"retention" mapstructure:"retention"`
}

// S3 implements the storage.Provider interface for AWS S3 storage
type S3 struct {
	client        *s3.S3
	uploader      *s3manager.Uploader
	downloader    *s3manager.Downloader
	config        S3Config
	pathGenerator *storage.PathGenerator
}

// NewS3 creates a new S3 storage provider
func NewS3(config S3Config) (storage.Provider, error) {
	// Set default path strategy if not specified
	if config.PathStrategy == "" {
		config.PathStrategy = storage.DateHierarchyStrategy
	}

	// Set default retention config if not specified
	if config.RetentionConfig.RetentionDays == 0 {
		config.RetentionConfig = storage.DefaultRetentionConfig()
	}

	// Set default region if not specified
	if config.Region == "" {
		config.Region = "us-east-1"
	}

	// Create AWS config
	awsConfig := &aws.Config{
		Region: aws.String(config.Region),
	}

	// Set credentials if provided
	if config.AccessKeyID != "" && config.SecretAccessKey != "" {
		awsConfig.Credentials = credentials.NewStaticCredentials(
			config.AccessKeyID,
			config.SecretAccessKey,
			"",
		)
	}

	// Set custom endpoint if provided (for S3-compatible services)
	if config.Endpoint != "" {
		awsConfig.Endpoint = aws.String(config.Endpoint)
		awsConfig.S3ForcePathStyle = aws.Bool(true)
	}

	// Set SSL configuration
	awsConfig.DisableSSL = aws.Bool(!config.UseSSL)

	// Create session
	sess, err := session.NewSession(awsConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session: %w", err)
	}

	// Create S3 client
	client := s3.New(sess)

	// Check if bucket exists and is accessible
	ctx := context.Background()
	_, err = client.HeadBucketWithContext(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(config.BucketName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to access bucket %s: %w", config.BucketName, err)
	}

	// Create uploader and downloader
	uploader := s3manager.NewUploaderWithClient(client)
	downloader := s3manager.NewDownloaderWithClient(client)

	// Use bucket name as virtual base path for path generator
	pathGenerator := storage.NewPathGenerator(config.PathStrategy, config.BucketName)

	return &S3{
		client:        client,
		uploader:      uploader,
		downloader:    downloader,
		config:        config,
		pathGenerator: pathGenerator,
	}, nil
}

// Upload uploads a file to S3 storage
func (s *S3) Upload(ctx context.Context, key string, reader io.Reader, size int64) error {
	objectKey := s.getObjectKey(key)

	_, err := s.uploader.UploadWithContext(ctx, &s3manager.UploadInput{
		Bucket:      aws.String(s.config.BucketName),
		Key:         aws.String(objectKey),
		Body:        reader,
		ContentType: aws.String("application/octet-stream"),
	})
	if err != nil {
		return fmt.Errorf("failed to upload object: %w", err)
	}

	return nil
}

// Download downloads a file from S3 storage
func (s *S3) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	objectKey := s.getObjectKey(key)

	// Try exact key first
	output, err := s.client.GetObjectWithContext(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.config.BucketName),
		Key:    aws.String(objectKey),
	})
	if err == nil {
		return output.Body, nil
	}

	// For date hierarchy strategy, search in date prefixes
	if s.config.PathStrategy == storage.DateHierarchyStrategy {
		return s.searchInDatePrefixes(ctx, key)
	}

	return nil, fmt.Errorf("object not found: %s", key)
}

// Delete deletes a file from S3 storage
func (s *S3) Delete(ctx context.Context, key string) error {
	objectKey := s.getObjectKey(key)

	_, err := s.client.DeleteObjectWithContext(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.config.BucketName),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	return nil
}

// Exists checks if a file exists in S3 storage
func (s *S3) Exists(ctx context.Context, key string) (bool, error) {
	objectKey := s.getObjectKey(key)

	// Try exact key first
	_, err := s.client.HeadObjectWithContext(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.config.BucketName),
		Key:    aws.String(objectKey),
	})
	if err == nil {
		return true, nil
	}

	// For date hierarchy strategy, search in date prefixes
	if s.config.PathStrategy == storage.DateHierarchyStrategy {
		_, err := s.searchInDatePrefixes(ctx, key)
		return err == nil, nil
	}

	return false, nil
}

// GetSize gets the size of a file in S3 storage
func (s *S3) GetSize(ctx context.Context, key string) (int64, error) {
	objectKey := s.getObjectKey(key)

	output, err := s.client.HeadObjectWithContext(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.config.BucketName),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		if s.config.PathStrategy == storage.DateHierarchyStrategy {
			return 0, fmt.Errorf("object not found: %s", key)
		}
		return 0, err
	}

	if output.ContentLength != nil {
		return *output.ContentLength, nil
	}

	return 0, nil
}

// Type returns the storage type
func (s *S3) Type() string {
	return "s3"
}

// HealthCheck performs a health check on S3 storage
func (s *S3) HealthCheck(ctx context.Context) error {
	// Check if bucket is accessible
	_, err := s.client.HeadBucketWithContext(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(s.config.BucketName),
	})
	if err != nil {
		return fmt.Errorf("S3 health check failed: %w", err)
	}

	return nil
}

// GetPathStrategy returns the path strategy
func (s *S3) GetPathStrategy() storage.PathStrategy {
	return s.config.PathStrategy
}

// GetRetentionConfig returns the retention configuration
func (s *S3) GetRetentionConfig() storage.StorageRetentionConfig {
	return s.config.RetentionConfig
}

// getObjectKey resolves the object key for a given storage key
func (s *S3) getObjectKey(key string) string {
	// Remove bucket prefix if present (path generator includes it)
	if strings.HasPrefix(key, s.config.BucketName+"/") {
		return strings.TrimPrefix(key, s.config.BucketName+"/")
	}
	return key
}

// searchInDatePrefixes searches for an object in date-based prefixes
func (s *S3) searchInDatePrefixes(ctx context.Context, key string) (io.ReadCloser, error) {
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
	output, err := s.client.ListObjectsV2WithContext(ctx, &s3.ListObjectsV2Input{
		Bucket:    aws.String(s.config.BucketName),
		Prefix:    aws.String(dir + "/"),
		Delimiter: aws.String("/"),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list objects: %w", err)
	}

	// Collect date prefixes from common prefixes
	var datePrefixes []string
	for _, prefix := range output.CommonPrefixes {
		if prefix.Prefix != nil {
			prefixStr := *prefix.Prefix
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
	}

	logger.L().Debug("Found date prefixes",
		zap.String("key", key),
		zap.Strings("prefixes", datePrefixes))

	// Search in date prefixes (newest first by sorting in reverse)
	for i := len(datePrefixes) - 1; i >= 0; i-- {
		possibleKey := datePrefixes[i] + filename
		logger.L().Debug("Trying date prefix",
			zap.String("possible_key", possibleKey))

		output, err := s.client.GetObjectWithContext(ctx, &s3.GetObjectInput{
			Bucket: aws.String(s.config.BucketName),
			Key:    aws.String(possibleKey),
		})
		if err == nil {
			logger.L().Debug("Found object in date prefix",
				zap.String("key", possibleKey))
			return output.Body, nil
		}
	}

	return nil, fmt.Errorf("object not found in any date prefix: %s (searched %d prefixes)", key, len(datePrefixes))
}

// ParseS3ConfigFromMap creates S3Config from string map (for database storage)
func ParseS3ConfigFromMap(configMap map[string]string) (S3Config, error) {
	config := S3Config{}

	config.Region = configMap["region"]
	config.AccessKeyID = configMap["access_key_id"]
	config.SecretAccessKey = configMap["secret_access_key"]
	config.BucketName = configMap["bucket_name"]
	config.Endpoint = configMap["endpoint"]

	// Parse boolean fields
	if useSSLStr, exists := configMap["use_ssl"]; exists {
		config.UseSSL = useSSLStr == "true"
	} else {
		config.UseSSL = true // Default to SSL
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
