package model

import (
	"sync/atomic"
	"time"

	"gorm.io/plugin/soft_delete"
)

var (
	GlobalConfig atomic.Pointer[Config]
)

// DefaultPermissions defines default permissions for authorization
type DefaultPermissions struct {
	Connect      bool `json:"connect" gorm:"column:connect"`
	FileUpload   bool `json:"file_upload" gorm:"column:file_upload"`
	FileDownload bool `json:"file_download" gorm:"column:file_download"`
	Copy         bool `json:"copy" gorm:"column:copy"`
	Paste        bool `json:"paste" gorm:"column:paste"`
	Share        bool `json:"share" gorm:"column:share"`
}

type Config struct {
	Id      int `json:"id" gorm:"column:id;primarykey;autoIncrement"`
	Timeout int `json:"timeout" gorm:"column:timeout"`

	// Default permissions for authorization creation
	DefaultPermissions DefaultPermissions `json:"default_permissions" gorm:"embedded;embeddedPrefix:default_"`

	CreatorId int                   `json:"creator_id" gorm:"column:creator_id"`
	UpdaterId int                   `json:"updater_id" gorm:"column:updater_id"`
	CreatedAt time.Time             `json:"created_at" gorm:"column:created_at"`
	UpdatedAt time.Time             `json:"updated_at" gorm:"column:updated_at"`
	DeletedAt soft_delete.DeletedAt `json:"-" gorm:"column:deleted_at;uniqueIndex:deleted_at"`
}

func (m *Config) TableName() string {
	return "config"
}

// GetDefaultPermissions returns the default permissions configuration
func (c *Config) GetDefaultPermissions() DefaultPermissions {
	return c.DefaultPermissions
}

// GetDefaultPermissionsAsAuthPermissions converts to AuthPermissions format
func (c *Config) GetDefaultPermissionsAsAuthPermissions() AuthPermissions {
	return AuthPermissions{
		Connect:      c.DefaultPermissions.Connect,
		FileUpload:   c.DefaultPermissions.FileUpload,
		FileDownload: c.DefaultPermissions.FileDownload,
		Copy:         c.DefaultPermissions.Copy,
		Paste:        c.DefaultPermissions.Paste,
		Share:        c.DefaultPermissions.Share,
	}
}

// ScheduleConfig defines configuration for scheduled tasks
type ScheduleConfig struct {
	ConnectableCheckInterval time.Duration `json:"connectable_check_interval" yaml:"connectable_check_interval" default:"30m"`
	ConfigUpdateInterval     time.Duration `json:"config_update_interval" yaml:"config_update_interval" default:"5m"`
	BatchSize                int           `json:"batch_size" yaml:"batch_size" default:"50"`
	ConcurrentWorkers        int           `json:"concurrent_workers" yaml:"concurrent_workers" default:"10"`
	ConnectTimeout           time.Duration `json:"connect_timeout" yaml:"connect_timeout" default:"3s"`
}

// GetDefaultScheduleConfig returns default schedule configuration
func GetDefaultScheduleConfig() *ScheduleConfig {
	return &ScheduleConfig{
		ConnectableCheckInterval: 30 * time.Minute, // Check connectivity every 30 minutes
		ConfigUpdateInterval:     5 * time.Minute,  // Update config every 5 minutes (reduced from 1 minute)
		BatchSize:                50,               // Process 50 assets per batch
		ConcurrentWorkers:        10,               // Use 10 concurrent workers
		ConnectTimeout:           3 * time.Second,  // 3 second timeout for connectivity tests
	}
}

// GetDefaultConfig returns a default configuration with reasonable defaults
func GetDefaultConfig() *Config {
	return &Config{
		Timeout: 1800, // 30 minutes
		DefaultPermissions: DefaultPermissions{
			Connect:      true,
			FileUpload:   true,
			FileDownload: true,
			Copy:         true,
			Paste:        true,
			Share:        false, // Share is disabled by default for security
		},
	}
}
