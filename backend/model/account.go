package model

import (
	"time"

	"gorm.io/plugin/soft_delete"
)

type Account struct {
	Id          int    `json:"id" gorm:"column:id;primarykey;autoIncrement"`
	Name        string `json:"name" gorm:"column:name;uniqueIndex:name_del;size:128"`
	AccountType int    `json:"account_type" gorm:"column:account_type"`
	Account     string `json:"account" gorm:"column:account"`
	Password    string `json:"password" gorm:"column:password"`
	Pk          string `json:"pk" gorm:"column:pk"`
	Phrase      string `json:"phrase" gorm:"column:phrase"`

	Permissions []string              `json:"permissions" gorm:"-"`
	ResourceId  int                   `json:"resource_id" gorm:"column:resource_id"`
	CreatorId   int                   `json:"creator_id" gorm:"column:creator_id"`
	UpdaterId   int                   `json:"updater_id" gorm:"column:updater_id"`
	CreatedAt   time.Time             `json:"created_at" gorm:"column:created_at"`
	UpdatedAt   time.Time             `json:"updated_at" gorm:"column:updated_at"`
	DeletedAt   soft_delete.DeletedAt `json:"-" gorm:"column:deleted_at;uniqueIndex:name_del"`

	AssetCount int64 `json:"asset_count" gorm:"-"`
}

func (m *Account) TableName() string {
	return "account"
}
func (m *Account) SetId(id int) {
	m.Id = id
}
func (m *Account) SetCreatorId(creatorId int) {
	m.CreatorId = creatorId
}
func (m *Account) SetUpdaterId(updaterId int) {
	m.UpdaterId = updaterId
}
func (m *Account) SetResourceId(resourceId int) {
	m.ResourceId = resourceId
}
func (m *Account) GetResourceId() int {
	return m.ResourceId
}
func (m *Account) GetName() string {
	return m.Name
}
func (m *Account) GetId() int {
	return m.Id
}

func (m *Account) SetPerms(perms []string) {
	m.Permissions = perms
}

type AccountCount struct {
	Id    int   `json:"id" gorm:"id"`
	Count int64 `json:"count" gorm:"count"`
}
