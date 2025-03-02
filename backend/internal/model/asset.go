package model

import (
	"time"

	"gorm.io/plugin/soft_delete"
)

const (
	TABLE_NAME_ASSET = "asset"
)

type Asset struct {
	Id            int                  `json:"id" gorm:"column:id;primarykey;autoIncrement"`
	Name          string               `json:"name" gorm:"column:name;uniqueIndex:name_del;size:128"`
	Comment       string               `json:"comment" gorm:"column:comment"`
	ParentId      int                  `json:"parent_id" gorm:"column:parent_id"`
	Ip            string               `json:"ip" gorm:"column:ip"`
	Protocols     Slice[string]        `json:"protocols" gorm:"column:protocols;type:text"`
	GatewayId     int                  `json:"gateway_id" gorm:"column:gateway_id"`
	Authorization Map[int, Slice[int]] `json:"authorization" gorm:"column:authorization;type:text"`
	AccessAuth    AccessAuth           `json:"access_auth" gorm:"embedded;column:access_auth"`
	Connectable   bool                 `json:"connectable" gorm:"column:connectable"`
	NodeChain     string               `json:"node_chain" gorm:"-"`

	Permissions []string              `json:"permissions" gorm:"-"`
	ResourceId  int                   `json:"resource_id" gorm:"column:resource_id"`
	CreatorId   int                   `json:"creator_id" gorm:"column:creator_id"`
	UpdaterId   int                   `json:"updater_id" gorm:"column:updater_id"`
	CreatedAt   time.Time             `json:"created_at" gorm:"column:created_at"`
	UpdatedAt   time.Time             `json:"updated_at" gorm:"column:updated_at"`
	DeletedAt   soft_delete.DeletedAt `json:"-" gorm:"column:deleted_at;uniqueIndex:name_del"`
}

type AccessAuth struct {
	Start  *time.Time   `json:"start,omitempty" gorm:"column:start"`
	End    *time.Time   `json:"end,omitempty" gorm:"column:end"`
	CmdIds Slice[int]   `json:"cmd_ids" gorm:"column:cmd_ids;type:text"`
	Ranges Slice[Range] `json:"ranges" gorm:"column:ranges;type:text"`
	Allow  bool         `json:"allow" gorm:"column:allow"`
}

type Range struct {
	Week  int           `json:"week" gorm:"column:week"`
	Times Slice[string] `json:"times" gorm:"column:times"`
}

func (m *Asset) TableName() string {
	return TABLE_NAME_ASSET
}
func (m *Asset) SetId(id int) {
	m.Id = id
}
func (m *Asset) SetCreatorId(creatorId int) {
	m.CreatorId = creatorId
}
func (m *Asset) SetUpdaterId(updaterId int) {
	m.UpdaterId = updaterId
}
func (m *Asset) SetResourceId(resourceId int) {
	m.ResourceId = resourceId
}
func (m *Asset) GetResourceId() int {
	return m.ResourceId
}
func (m *Asset) GetName() string {
	return m.Name
}
func (m *Asset) GetId() int {
	return m.Id
}

func (m *Asset) SetPerms(perms []string) {
	m.Permissions = perms
}

type AssetIdPid struct {
	Id       int `gorm:"column:id"`
	ParentId int `gorm:"column:parent_id"`
}

func (m *AssetIdPid) TableName() string {
	return TABLE_NAME_ASSET
}
