package model

import (
	"time"

	"gorm.io/plugin/soft_delete"
)

type Command struct {
	Id     int           `json:"id" gorm:"column:id;primarykey;autoIncrement"`
	Name   string        `json:"name" gorm:"column:name;uniqueIndex:name_del;size:128"`
	Cmds   Slice[string] `json:"cmds" gorm:"column:cmds"`
	Enable bool          `json:"enable" gorm:"column:enable"`

	ResourceId int                   `json:"resource_id" gorm:"column:resource_id"`
	CreatorId  int                   `json:"creator_id" gorm:"column:creator_id"`
	UpdaterId  int                   `json:"updater_id" gorm:"column:updater_id"`
	CreatedAt  time.Time             `json:"created_at" gorm:"column:created_at"`
	UpdatedAt  time.Time             `json:"updated_at" gorm:"column:updated_at"`
	DeletedAt  soft_delete.DeletedAt `json:"-" gorm:"column:deleted_at;uniqueIndex:name_del"`
}

func (m *Command) TableName() string {
	return "command"
}
func (m *Command) SetId(id int) {
	m.Id = id
}
func (m *Command) SetCreatorId(creatorId int) {
	m.CreatorId = creatorId
}
func (m *Command) SetUpdaterId(updaterId int) {
	m.UpdaterId = updaterId
}
func (m *Command) SetResourceId(resourceId int) {
	m.ResourceId = resourceId
}
func (m *Command) GetResourceId() int {
	return m.ResourceId
}
func (m *Command) GetName() string {
	return m.Name
}
func (m *Command) GetId() int {
	return m.Id
}
