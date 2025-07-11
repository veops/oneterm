package model

import (
	"sync/atomic"
	"time"

	"gorm.io/plugin/soft_delete"
)

var (
	GlobalConfig atomic.Pointer[Config]
)

type SshConfig struct {
	Copy  bool `json:"copy" gorm:"column:copy"`
	Paste bool `json:"paste" gorm:"column:paste"`
}
type RdpConfig struct {
	Copy            bool   `json:"copy" gorm:"column:copy"`
	Paste           bool   `json:"paste" gorm:"column:paste"`
	EnableDrive     bool   `json:"enable_drive" gorm:"column:enable_drive"`
	DrivePath       string `json:"drive_path" gorm:"column:drive_path"`
	CreateDrivePath bool   `json:"create_drive_path" gorm:"column:create_drive_path"`
	DisableUpload   bool   `json:"disable_upload" gorm:"column:disable_upload"`
	DisableDownload bool   `json:"disable_download" gorm:"column:disable_download"`
}
type VncConfig struct {
	Copy  bool `json:"copy" gorm:"column:copy"`
	Paste bool `json:"paste" gorm:"column:paste"`
}

type Config struct {
	Id        int       `json:"id" gorm:"column:id;primarykey;autoIncrement"`
	Timeout   int       `json:"timeout" gorm:"column:timeout"`
	SshConfig SshConfig `json:"ssh_config" gorm:"embedded;embeddedPrefix:ssh_;column:ssh_config"`
	RdpConfig RdpConfig `json:"rdp_config" gorm:"embedded;embeddedPrefix:rdp_;column:rdp_config"`
	VncConfig VncConfig `json:"vnc_config" gorm:"embedded;embeddedPrefix:vnc_;column:vnc_config"`

	CreatorId int                   `json:"creator_id" gorm:"column:creator_id"`
	UpdaterId int                   `json:"updater_id" gorm:"column:updater_id"`
	CreatedAt time.Time             `json:"created_at" gorm:"column:created_at"`
	UpdatedAt time.Time             `json:"updated_at" gorm:"column:updated_at"`
	DeletedAt soft_delete.DeletedAt `json:"-" gorm:"column:deleted_at;uniqueIndex:deleted_at"`
}

func (m *Config) TableName() string {
	return "config"
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
