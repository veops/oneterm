package model

import (
	"time"

	"gorm.io/plugin/soft_delete"
)

type Config struct {
	Id      int `json:"id" gorm:"column:id;primarykey"`
	Timeout int `json:"timeout" gorm:"column:timeout"`

	CreatorId int                   `json:"creator_id" gorm:"column:creator_id"`
	UpdaterId int                   `json:"updater_id" gorm:"column:updater_id"`
	CreatedAt time.Time             `json:"created_at" gorm:"column:created_at"`
	UpdatedAt time.Time             `json:"updated_at" gorm:"column:updated_at"`
	DeletedAt soft_delete.DeletedAt `json:"-" gorm:"column:deleted_at"`
}

func (m *Config) TableName() string {
	return "config"
}
