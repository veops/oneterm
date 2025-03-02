package model

import (
	"time"

	"gorm.io/plugin/soft_delete"
)

const (
	TABLE_NAME_SHARE = "share"
)

type Share struct {
	Id        int       `json:"id" gorm:"column:id;primarykey;autoIncrement"`
	Uuid      string    `json:"uuid" gorm:"column:uuid;uniqueIndex:uuid;size:128"`
	AssetId   int       `json:"asset_id" gorm:"column:asset_id"`
	AccountId int       `json:"account_id" gorm:"column:account_id"`
	Protocol  string    `json:"protocol" gorm:"column:protocol"`
	NoLimit   bool      `json:"no_limit" gorm:"column:no_limit"`
	Times     int       `json:"times" gorm:"column:times"`
	Start     time.Time `json:"start" gorm:"column:start"`
	End       time.Time `json:"end" gorm:"column:end"`

	CreatorId int                   `json:"creator_id" gorm:"column:creator_id"`
	UpdaterId int                   `json:"updater_id" gorm:"column:updater_id"`
	CreatedAt time.Time             `json:"created_at" gorm:"column:created_at"`
	UpdatedAt time.Time             `json:"updated_at" gorm:"column:updated_at"`
	DeletedAt soft_delete.DeletedAt `json:"-" gorm:"column:deleted_at"`
}

func (m *Share) TableName() string {
	return TABLE_NAME_SHARE
}
func (m *Share) SetId(id int) {
	m.Id = id
}
func (m *Share) SetCreatorId(creatorId int) {
	m.CreatorId = creatorId
}
func (m *Share) SetUpdaterId(updaterId int) {
	m.UpdaterId = updaterId
}
func (m *Share) SetResourceId(resourceId int) {

}
func (m *Share) GetResourceId() int {
	return 0
}
func (m *Share) GetName() string {
	return ""
}
func (m *Share) GetId() int {
	return m.Id
}

func (m *Share) SetPerms(perms []string){}