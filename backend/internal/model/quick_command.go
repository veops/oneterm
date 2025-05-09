package model

import (
	"time"

	"gorm.io/plugin/soft_delete"
)

// QuickCommand represents a quick command model
type QuickCommand struct {
	Id          int                   `json:"id" gorm:"primaryKey"`
	Name        string                `json:"name" gorm:"size:50;not null"`     // Command name
	Command     string                `json:"command" gorm:"size:500;not null"` // Actual command to execute
	Description string                `json:"description" gorm:"size:200"`      // Command description
	IsGlobal    bool                  `json:"is_global" gorm:"default:false"`   // Whether it's a global command
	CreatorId   int                   `json:"creator_id" gorm:"not null"`       // Creator ID
	CreatedAt   time.Time             `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time             `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt   soft_delete.DeletedAt `json:"-" gorm:"column:deleted_at"`
}

func (m *QuickCommand) TableName() string {
	return "quick_commands"
}
func (m *QuickCommand) SetId(id int) {
	m.Id = id
}
func (m *QuickCommand) SetCreatorId(creatorId int) {
	m.CreatorId = creatorId
}
func (m *QuickCommand) SetUpdaterId(updaterId int) {
}

func (m *QuickCommand) SetResourceId(resourceId int) {

}
func (m *QuickCommand) GetResourceId() int {
	return 0
}
func (m *QuickCommand) GetName() string {
	return m.Name
}
func (m *QuickCommand) GetId() int {
	return m.Id
}

func (m *QuickCommand) SetPerms(perms []string) {}
