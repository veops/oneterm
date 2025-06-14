package storage

import (
	"context"
	"io"
)

// Provider defines the interface for storage operations
type Provider interface {
	// Upload uploads a file to storage
	Upload(ctx context.Context, key string, reader io.Reader, size int64) error

	// Download downloads a file from storage
	Download(ctx context.Context, key string) (io.ReadCloser, error)

	// Delete deletes a file from storage
	Delete(ctx context.Context, key string) error

	// Exists checks if a file exists
	Exists(ctx context.Context, key string) (bool, error)

	// GetSize gets the size of a file
	GetSize(ctx context.Context, key string) (int64, error)

	// Type returns the storage provider type
	Type() string

	// HealthCheck performs a health check
	HealthCheck(ctx context.Context) error
}

// Config represents storage configuration
type Config struct {
	Type       string            `json:"type"`
	Name       string            `json:"name"`
	Parameters map[string]string `json:"parameters"`
}

// ProviderType represents the type of storage provider
type ProviderType string

const (
	TypeLocal ProviderType = "local"
	TypeS3    ProviderType = "s3"
	TypeMinio ProviderType = "minio"
)
