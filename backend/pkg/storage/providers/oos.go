package providers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/veops/oneterm/pkg/logger"
	"github.com/veops/oneterm/pkg/storage"
	"go.uber.org/zap"
)

// OOSConfig holds configuration for China Telecom Cloud Object Storage Service
type OOSConfig struct {
	Endpoint        string                         `json:"endpoint" mapstructure:"endpoint"`
	AccessKeyID     string                         `json:"access_key_id" mapstructure:"access_key_id"`
	SecretAccessKey string                         `json:"secret_access_key" mapstructure:"secret_access_key"`
	BucketName      string                         `json:"bucket_name" mapstructure:"bucket_name"`
	Region          string                         `json:"region" mapstructure:"region"`
	PathStrategy    storage.PathStrategy           `json:"path_strategy" mapstructure:"path_strategy"`
	RetentionConfig storage.StorageRetentionConfig `json:"retention" mapstructure:"retention"`
}

// OOS implements the storage.Provider interface for China Telecom Cloud Object Storage Service
type OOS struct {
	config        OOSConfig
	httpClient    *http.Client
	pathGenerator *storage.PathGenerator
}

// NewOOS creates a new China Telecom Cloud Object Storage Service provider
func NewOOS(config OOSConfig) (storage.Provider, error) {
	// Set default path strategy if not specified
	if config.PathStrategy == "" {
		config.PathStrategy = storage.DateHierarchyStrategy
	}

	// Set default retention config if not specified
	if config.RetentionConfig.RetentionDays == 0 {
		config.RetentionConfig = storage.DefaultRetentionConfig()
	}

	// Validate required fields
	if config.Endpoint == "" || config.AccessKeyID == "" || config.SecretAccessKey == "" || config.BucketName == "" {
		return nil, fmt.Errorf("endpoint, access_key_id, secret_access_key, and bucket_name are required")
	}

	// Create HTTP client
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Use bucket name as virtual base path for path generator
	pathGenerator := storage.NewPathGenerator(config.PathStrategy, config.BucketName)

	oos := &OOS{
		config:        config,
		httpClient:    httpClient,
		pathGenerator: pathGenerator,
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := oos.HealthCheck(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect to OOS: %w", err)
	}

	return oos, nil
}

// Upload uploads a file to OOS storage
func (o *OOS) Upload(ctx context.Context, key string, reader io.Reader, size int64) error {
	objectKey := o.getObjectKey(key)

	// Read all data from reader
	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("failed to read data: %w", err)
	}

	// Create PUT request
	url := fmt.Sprintf("%s/%s/%s", strings.TrimRight(o.config.Endpoint, "/"), o.config.BucketName, objectKey)
	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Content-Length", strconv.FormatInt(int64(len(data)), 10))
	req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))

	// Sign request
	if err := o.signRequest(req, "PUT", objectKey, data); err != nil {
		return fmt.Errorf("failed to sign request: %w", err)
	}

	// Send request
	resp, err := o.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Download downloads a file from OOS storage
func (o *OOS) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	objectKey := o.getObjectKey(key)

	// Try exact key first
	reader, err := o.downloadObject(ctx, objectKey)
	if err == nil {
		return reader, nil
	}

	// For date hierarchy strategy, search in date prefixes
	if o.config.PathStrategy == storage.DateHierarchyStrategy {
		return o.searchInDatePrefixes(ctx, key)
	}

	return nil, fmt.Errorf("object not found: %s", key)
}

// Delete deletes a file from OOS storage
func (o *OOS) Delete(ctx context.Context, key string) error {
	objectKey := o.getObjectKey(key)

	// Create DELETE request
	url := fmt.Sprintf("%s/%s/%s", strings.TrimRight(o.config.Endpoint, "/"), o.config.BucketName, objectKey)
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))

	// Sign request
	if err := o.signRequest(req, "DELETE", objectKey, nil); err != nil {
		return fmt.Errorf("failed to sign request: %w", err)
	}

	// Send request
	resp, err := o.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Exists checks if a file exists in OOS storage
func (o *OOS) Exists(ctx context.Context, key string) (bool, error) {
	objectKey := o.getObjectKey(key)

	// Try exact key first
	exists, err := o.objectExists(ctx, objectKey)
	if err == nil && exists {
		return true, nil
	}

	// For date hierarchy strategy, search in date prefixes
	if o.config.PathStrategy == storage.DateHierarchyStrategy {
		_, err := o.searchInDatePrefixes(ctx, key)
		return err == nil, nil
	}

	return false, nil
}

// GetSize gets the size of a file in OOS storage
func (o *OOS) GetSize(ctx context.Context, key string) (int64, error) {
	objectKey := o.getObjectKey(key)

	// Create HEAD request
	url := fmt.Sprintf("%s/%s/%s", strings.TrimRight(o.config.Endpoint, "/"), o.config.BucketName, objectKey)
	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))

	// Sign request
	if err := o.signRequest(req, "HEAD", objectKey, nil); err != nil {
		return 0, fmt.Errorf("failed to sign request: %w", err)
	}

	// Send request
	resp, err := o.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("object not found: %s", key)
	}

	contentLength := resp.Header.Get("Content-Length")
	if contentLength == "" {
		return 0, nil
	}

	size, err := strconv.ParseInt(contentLength, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid content length: %s", contentLength)
	}

	return size, nil
}

// Type returns the storage type
func (o *OOS) Type() string {
	return "oos"
}

// HealthCheck performs a health check on OOS storage
func (o *OOS) HealthCheck(ctx context.Context) error {
	// Check if bucket is accessible by listing objects with limit 1
	url := fmt.Sprintf("%s/%s?max-keys=1", strings.TrimRight(o.config.Endpoint, "/"), o.config.BucketName)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))

	// Sign request
	if err := o.signRequest(req, "GET", "", nil); err != nil {
		return fmt.Errorf("failed to sign request: %w", err)
	}

	// Send request
	resp, err := o.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("health check failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetPathStrategy returns the path strategy
func (o *OOS) GetPathStrategy() storage.PathStrategy {
	return o.config.PathStrategy
}

// GetRetentionConfig returns the retention configuration
func (o *OOS) GetRetentionConfig() storage.StorageRetentionConfig {
	return o.config.RetentionConfig
}

// getObjectKey generates the object key based on the path strategy
func (o *OOS) getObjectKey(key string) string {
	if o.pathGenerator != nil {
		// For session replay files, use the replay path generator
		if strings.HasSuffix(key, ".cast") {
			sessionID := strings.TrimSuffix(key, ".cast")
			return o.pathGenerator.GenerateReplayPath(sessionID, time.Now())
		}

		// For other files, generate path based on strategy
		switch o.pathGenerator.Strategy {
		case storage.DateHierarchyStrategy:
			dateDir := time.Now().Format("2006/01/02")
			return filepath.Join(dateDir, key)
		case storage.FlatStrategy:
			fallthrough
		default:
			return key
		}
	}
	return key
}

// downloadObject downloads an object from OOS
func (o *OOS) downloadObject(ctx context.Context, objectKey string) (io.ReadCloser, error) {
	// Create GET request
	url := fmt.Sprintf("%s/%s/%s", strings.TrimRight(o.config.Endpoint, "/"), o.config.BucketName, objectKey)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))

	// Sign request
	if err := o.signRequest(req, "GET", objectKey, nil); err != nil {
		return nil, fmt.Errorf("failed to sign request: %w", err)
	}

	// Send request
	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	return resp.Body, nil
}

// objectExists checks if an object exists in OOS
func (o *OOS) objectExists(ctx context.Context, objectKey string) (bool, error) {
	// Create HEAD request
	url := fmt.Sprintf("%s/%s/%s", strings.TrimRight(o.config.Endpoint, "/"), o.config.BucketName, objectKey)
	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))

	// Sign request
	if err := o.signRequest(req, "HEAD", objectKey, nil); err != nil {
		return false, fmt.Errorf("failed to sign request: %w", err)
	}

	// Send request
	resp, err := o.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

// searchInDatePrefixes searches for a file in date-based prefixes
func (o *OOS) searchInDatePrefixes(ctx context.Context, key string) (io.ReadCloser, error) {
	// Generate possible date prefixes for the last 30 days
	now := time.Now()
	for i := 0; i < 30; i++ {
		date := now.AddDate(0, 0, -i)
		datePrefix := date.Format("2006/01/02")
		objectKey := filepath.Join(datePrefix, key)

		reader, err := o.downloadObject(ctx, objectKey)
		if err == nil {
			return reader, nil
		}

		// Log the attempt for debugging
		logger.L().Debug("Searched for object in date prefix",
			zap.String("date_prefix", datePrefix),
			zap.String("object_key", objectKey),
			zap.Error(err))
	}

	return nil, fmt.Errorf("object not found in any date prefix: %s", key)
}

// signRequest signs an HTTP request using OOS signature algorithm
func (o *OOS) signRequest(req *http.Request, method, objectKey string, body []byte) error {
	// Get date from header
	date := req.Header.Get("Date")
	if date == "" {
		date = time.Now().UTC().Format(http.TimeFormat)
		req.Header.Set("Date", date)
	}

	// Build string to sign
	stringToSign := o.buildStringToSign(method, objectKey, req.Header, date)

	// Calculate signature
	signature := o.calculateSignature(stringToSign)

	// Set authorization header
	auth := fmt.Sprintf("AWS %s:%s", o.config.AccessKeyID, signature)
	req.Header.Set("Authorization", auth)

	return nil
}

// buildStringToSign builds the string to sign for OOS authentication
func (o *OOS) buildStringToSign(method, objectKey string, headers http.Header, date string) string {
	var parts []string

	// HTTP method
	parts = append(parts, method)

	// Content-MD5 (usually empty)
	parts = append(parts, headers.Get("Content-MD5"))

	// Content-Type
	parts = append(parts, headers.Get("Content-Type"))

	// Date
	parts = append(parts, date)

	// Canonicalized OOS headers (x-oos-*)
	canonicalizedHeaders := o.getCanonicalizedHeaders(headers)
	if canonicalizedHeaders != "" {
		parts = append(parts, canonicalizedHeaders)
	}

	// Canonicalized resource
	resource := fmt.Sprintf("/%s", o.config.BucketName)
	if objectKey != "" {
		resource += "/" + objectKey
	}
	parts = append(parts, resource)

	return strings.Join(parts, "\n")
}

// getCanonicalizedHeaders gets canonicalized OOS headers
func (o *OOS) getCanonicalizedHeaders(headers http.Header) string {
	var oosHeaders []string

	for key, values := range headers {
		lowerKey := strings.ToLower(key)
		if strings.HasPrefix(lowerKey, "x-oos-") {
			for _, value := range values {
				oosHeaders = append(oosHeaders, fmt.Sprintf("%s:%s", lowerKey, strings.TrimSpace(value)))
			}
		}
	}

	if len(oosHeaders) == 0 {
		return ""
	}

	sort.Strings(oosHeaders)
	return strings.Join(oosHeaders, "\n") + "\n"
}

// calculateSignature calculates the HMAC-SHA1 signature
func (o *OOS) calculateSignature(stringToSign string) string {
	h := hmac.New(sha1.New, []byte(o.config.SecretAccessKey))
	h.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// ParseOOSConfigFromMap parses OOS configuration from a map
func ParseOOSConfigFromMap(configMap map[string]string) (OOSConfig, error) {
	config := OOSConfig{}

	// Required fields
	if endpoint, ok := configMap["endpoint"]; ok {
		config.Endpoint = endpoint
	} else {
		return config, fmt.Errorf("endpoint is required")
	}

	if accessKeyID, ok := configMap["access_key_id"]; ok {
		config.AccessKeyID = accessKeyID
	} else {
		return config, fmt.Errorf("access_key_id is required")
	}

	if secretAccessKey, ok := configMap["secret_access_key"]; ok {
		config.SecretAccessKey = secretAccessKey
	} else {
		return config, fmt.Errorf("secret_access_key is required")
	}

	if bucketName, ok := configMap["bucket_name"]; ok {
		config.BucketName = bucketName
	} else {
		return config, fmt.Errorf("bucket_name is required")
	}

	// Optional fields
	if region, ok := configMap["region"]; ok {
		config.Region = region
	}

	// Parse path strategy
	if pathStrategy, ok := configMap["path_strategy"]; ok {
		config.PathStrategy = storage.PathStrategy(pathStrategy)
	} else {
		config.PathStrategy = storage.DateHierarchyStrategy
	}

	// Parse retention config
	if retentionDaysStr, ok := configMap["retention_days"]; ok {
		if retentionDays, err := strconv.Atoi(retentionDaysStr); err == nil {
			config.RetentionConfig.RetentionDays = retentionDays
		}
	}

	if config.RetentionConfig.RetentionDays == 0 {
		config.RetentionConfig = storage.DefaultRetentionConfig()
	}

	return config, nil
}
