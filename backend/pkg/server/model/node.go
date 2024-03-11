package model

import (
	"time"

	"gorm.io/plugin/soft_delete"
)

const (
	TABLE_NAME_NODE = "node"
)

type Node struct {
	Id            int                  `json:"id" gorm:"column:id;primarykey"`
	Name          string               `json:"name" gorm:"column:name"`
	Comment       string               `json:"comment" gorm:"column:comment"`
	ParentId      int                  `json:"parent_id" gorm:"column:parent_id"`
	Authorization Map[int, Slice[int]] `json:"authorization" gorm:"column:authorization"`
	*AccessAuth   `json:"access_auth" gorm:"column:access_auth"`
	*Sync         `json:"sync" gorm:"column:sync"`
	Protocols     Slice[string] `json:"protocols" gorm:"column:protocols"`
	GatewayId     int           `json:"gateway_id" gorm:"column:gateway_id"`

	// ResourceId int       `json:"resource_id"`
	CreatorId int                   `json:"creator_id" gorm:"column:creator_id"`
	UpdaterId int                   `json:"updater_id" gorm:"column:updater_id"`
	CreatedAt time.Time             `json:"created_at" gorm:"column:created_at"`
	UpdatedAt time.Time             `json:"updated_at" gorm:"column:updated_at"`
	DeletedAt soft_delete.DeletedAt `json:"-" gorm:"column:deleted_at"`

	AssetCount int64 `json:"asset_count" gorm:"-"`
	HasChild   bool  `json:"has_child" gorm:"-"`
}

type Sync struct {
	TypeId    int                 `json:"type_id,omitempty" gorm:"column:type_id"`
	Mapping   Map[string, string] `json:"mapping" gorm:"column:mapping"`
	Filters   string              `json:"filters" gorm:"column:filters"`
	Enable    bool                `json:"enable" gorm:"column:enable"`
	Frequency float64             `json:"frequency" gorm:"column:frequency"`
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

}
func (m *Node) GetResourceId() int {
	return 0
}
func (m *Node) GetName() string {
	return m.Name
}
func (m *Node) GetId() int {
	return m.Id
}

type NodeIdPid struct {
	Id       int `gorm:"column:id"`
	ParentId int `gorm:"column:parent_id"`
}

func (m *NodeIdPid) TableName() string {
	return TABLE_NAME_NODE
}

type NodeIdPidName struct {
	Id       int    `gorm:"column:id"`
	ParentId int    `gorm:"column:parent_id"`
	Name     string `gorm:"column:name"`
}

func (m *NodeIdPidName) TableName() string {
	return TABLE_NAME_NODE
}
