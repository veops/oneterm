package model

import (
	"time"

	"gorm.io/plugin/soft_delete"
)

const (
	TABLE_NAME_NODE = "node"
)

type Node struct {
	Id            int                  `json:"id" gorm:"column:id;primarykey;autoIncrement"`
	Name          string               `json:"name" gorm:"column:name"`
	Comment       string               `json:"comment" gorm:"column:comment"`
	ParentId      int                  `json:"parent_id" gorm:"column:parent_id"`
	Authorization Map[int, Slice[int]] `json:"authorization" gorm:"column:authorization;type:text"`
	AccessAuth    AccessAuth           `json:"access_auth" gorm:"embedded;column:access_auth"`
	Protocols     Slice[string]        `json:"protocols" gorm:"column:protocols;type:text"`
	GatewayId     int                  `json:"gateway_id" gorm:"column:gateway_id"`

	Permissions []string              `json:"permissions" gorm:"-"`
	ResourceId  int                   `json:"resource_id" gorm:"column:resource_id"`
	CreatorId   int                   `json:"creator_id" gorm:"column:creator_id"`
	UpdaterId   int                   `json:"updater_id" gorm:"column:updater_id"`
	CreatedAt   time.Time             `json:"created_at" gorm:"column:created_at"`
	UpdatedAt   time.Time             `json:"updated_at" gorm:"column:updated_at"`
	DeletedAt   soft_delete.DeletedAt `json:"-" gorm:"column:deleted_at"`

	AssetCount int64 `json:"asset_count" gorm:"-"`
	HasChild   bool  `json:"has_child" gorm:"-"`
}

func (m *Node) TableName() string {
	return TABLE_NAME_NODE
}
func (m *Node) SetId(id int) {
	m.Id = id
}
func (m *Node) SetCreatorId(creatorId int) {
	m.CreatorId = creatorId
}
func (m *Node) SetUpdaterId(updaterId int) {
	m.UpdaterId = updaterId
}
func (m *Node) SetResourceId(resourceId int) {
	m.ResourceId = resourceId
}
func (m *Node) GetResourceId() int {
	return m.ResourceId
}
func (m *Node) GetName() string {
	return m.Name
}
func (m *Node) GetId() int {
	return m.Id
}

func (m *Node) SetPerms(perms []string) {
	m.Permissions = perms
}
