package model

import (
	"time"
)

// MigrationRecord tracks the status of different migrations
type MigrationRecord struct {
	Id            int        `json:"id" gorm:"column:id;primarykey;autoIncrement"`
	MigrationName string     `json:"migration_name" gorm:"column:migration_name;uniqueIndex;size:128;not null"`
	Status        string     `json:"status" gorm:"column:status;size:32;not null"` // pending, running, completed, failed
	StartedAt     *time.Time `json:"started_at" gorm:"column:started_at"`
	CompletedAt   *time.Time `json:"completed_at" gorm:"column:completed_at"`
	ErrorMessage  string     `json:"error_message" gorm:"column:error_message;type:text"`
	RecordsCount  int        `json:"records_count" gorm:"column:records_count;default:0"` // Number of records migrated

	CreatedAt time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt time.Time `json:"updated_at" gorm:"column:updated_at"`
}

func (m *MigrationRecord) TableName() string {
	return "migration_records"
}

// Migration constants
const (
	MigrationAuthV1ToV2 = "auth_v1_to_v2"
)

// Migration status constants
const (
	MigrationStatusPending   = "pending"
	MigrationStatusRunning   = "running"
	MigrationStatusCompleted = "completed"
	MigrationStatusFailed    = "failed"
)
