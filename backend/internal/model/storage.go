package model

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"gorm.io/plugin/soft_delete"
)

// StorageType represents the type of storage backend
type StorageType string

const (
	StorageTypeLocal StorageType = "local"
	StorageTypeS3    StorageType = "s3"
	StorageTypeMinio StorageType = "minio"
	StorageTypeOSS   StorageType = "oss"
	StorageTypeCOS   StorageType = "cos"
	StorageTypeAzure StorageType = "azure"
	StorageTypeOBS   StorageType = "obs"
	StorageTypeOOS   StorageType = "oos"
)

// StorageConfigMap represents the configuration parameters for a storage backend
type StorageConfigMap map[string]string

func (m *StorageConfigMap) Scan(value any) error {
	if value == nil {
		*m = make(map[string]string)
		return nil
	}
	return json.Unmarshal(value.([]byte), m)
}

func (m StorageConfigMap) Value() (driver.Value, error) {
	if m == nil {
		return "{}", nil
	}
	return json.Marshal(m)
}

// StorageConfig represents a storage configuration entry in the database
type StorageConfig struct {
	Id          int              `json:"id" gorm:"column:id;primarykey;autoIncrement"`
	Name        string           `json:"name" gorm:"column:name;uniqueIndex;size:64;not null"`
	Type        StorageType      `json:"type" gorm:"column:type;size:32;not null"`
	Enabled     bool             `json:"enabled" gorm:"column:enabled;default:true"`
	Priority    int              `json:"priority" gorm:"column:priority;default:10"`
	IsPrimary   bool             `json:"is_primary" gorm:"column:is_primary;default:false"`
	Config      StorageConfigMap `json:"config" gorm:"column:config;type:text"`
	Description string           `json:"description" gorm:"column:description;size:255"`

	// Standard fields
	CreatorId int                   `json:"creator_id" gorm:"column:creator_id"`
	UpdaterId int                   `json:"updater_id" gorm:"column:updater_id"`
	CreatedAt time.Time             `json:"created_at" gorm:"column:created_at"`
	UpdatedAt time.Time             `json:"updated_at" gorm:"column:updated_at"`
	DeletedAt soft_delete.DeletedAt `json:"-" gorm:"column:deleted_at"`
}

func (m *StorageConfig) TableName() string {
	return "storage_configs"
}

func (m *StorageConfig) SetId(id int) {
	m.Id = id
}

func (m *StorageConfig) SetCreatorId(id int) {
	m.CreatorId = id
}

func (m *StorageConfig) SetUpdaterId(id int) {
	m.UpdaterId = id
}

func (m *StorageConfig) SetResourceId(id int) {
	m.Id = id
}

func (m *StorageConfig) GetResourceId() int {
	return m.Id
}

func (m *StorageConfig) GetId() int {
	return m.Id
}

func (m *StorageConfig) GetName() string {
	return m.Name
}

func (m *StorageConfig) SetPerms(perms []string) {
	// Storage configs don't have permissions
}

// StorageMetrics represents storage usage metrics
type StorageMetrics struct {
	Id           int       `json:"id" gorm:"column:id;primarykey;autoIncrement"`
	StorageName  string    `json:"storage_name" gorm:"column:storage_name;size:64;not null"`
	FileCount    int64     `json:"file_count" gorm:"column:file_count;default:0"`
	TotalSize    int64     `json:"total_size" gorm:"column:total_size;default:0"`
	ReplayCount  int64     `json:"replay_count" gorm:"column:replay_count;default:0"`
	ReplaySize   int64     `json:"replay_size" gorm:"column:replay_size;default:0"`
	RdpFileCount int64     `json:"rdp_file_count" gorm:"column:rdp_file_count;default:0"`
	RdpFileSize  int64     `json:"rdp_file_size" gorm:"column:rdp_file_size;default:0"`
	LastUpdated  time.Time `json:"last_updated" gorm:"column:last_updated"`
	IsHealthy    bool      `json:"is_healthy" gorm:"column:is_healthy;default:true"`
	ErrorMessage string    `json:"error_message" gorm:"column:error_message;size:255"`

	CreatedAt time.Time             `json:"created_at" gorm:"column:created_at"`
	UpdatedAt time.Time             `json:"updated_at" gorm:"column:updated_at"`
	DeletedAt soft_delete.DeletedAt `json:"-" gorm:"column:deleted_at"`
}

func (m *StorageMetrics) TableName() string {
	return "storage_metrics"
}

// FileMetadata represents metadata for files stored in storage backends
type FileMetadata struct {
	Id          int         `json:"id" gorm:"column:id;primarykey;autoIncrement"`
	StorageKey  string      `json:"storage_key" gorm:"column:storage_key;uniqueIndex;size:255;not null"`
	FileName    string      `json:"file_name" gorm:"column:file_name;size:255;not null"`
	FileSize    int64       `json:"file_size" gorm:"column:file_size;default:0"`
	MimeType    string      `json:"mime_type" gorm:"column:mime_type;size:100"`
	Checksum    string      `json:"checksum" gorm:"column:checksum;size:64"`
	StorageType StorageType `json:"storage_type" gorm:"column:storage_type;size:32;not null"`
	StorageName string      `json:"storage_name" gorm:"column:storage_name;size:64;not null"`
	Category    string      `json:"category" gorm:"column:category;size:32"` // replay, rdp_file, etc.
	SessionId   string      `json:"session_id" gorm:"column:session_id;size:64"`
	AssetId     int         `json:"asset_id" gorm:"column:asset_id"`
	UserId      int         `json:"user_id" gorm:"column:user_id"`

	// Standard fields
	CreatorId int                   `json:"creator_id" gorm:"column:creator_id"`
	UpdaterId int                   `json:"updater_id" gorm:"column:updater_id"`
	CreatedAt time.Time             `json:"created_at" gorm:"column:created_at"`
	UpdatedAt time.Time             `json:"updated_at" gorm:"column:updated_at"`
	DeletedAt soft_delete.DeletedAt `json:"-" gorm:"column:deleted_at"`
}

func (m *FileMetadata) TableName() string {
	return "file_metadata"
}

func (m *FileMetadata) SetId(id int) {
	m.Id = id
}

func (m *FileMetadata) SetCreatorId(id int) {
	m.CreatorId = id
}

func (m *FileMetadata) SetUpdaterId(id int) {
	m.UpdaterId = id
}

func (m *FileMetadata) GetResourceId() int {
	return m.Id
}

func (m *FileMetadata) GetId() int {
	return m.Id
}

func (m *FileMetadata) GetName() string {
	return m.FileName
}
