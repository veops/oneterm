package model

import (
	"time"

	"gorm.io/plugin/soft_delete"
)

// SystemConfig stores sensitive system-level configurations
// This model is for internal use only and should never be exposed via API
type SystemConfig struct {
	Id    int    `json:"id" gorm:"column:id;primarykey;autoIncrement"`
	Key   string `json:"key" gorm:"column:config_key;size:191;uniqueIndex;not null"`
	Value string `json:"value" gorm:"column:value;type:text"`

	CreatorId int                   `json:"creator_id" gorm:"column:creator_id"`
	UpdaterId int                   `json:"updater_id" gorm:"column:updater_id"`
	CreatedAt time.Time             `json:"created_at" gorm:"column:created_at"`
	UpdatedAt time.Time             `json:"updated_at" gorm:"column:updated_at"`
	DeletedAt soft_delete.DeletedAt `json:"-" gorm:"column:deleted_at"`
}

func (m *SystemConfig) TableName() string {
	return "system_config"
}

// System config key constants
const (
	SysConfigSSHPrivateKey = "ssh_private_key"
)
