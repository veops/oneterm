package model

import (
	"time"

	"gorm.io/plugin/soft_delete"
)

const (
	AUTHMETHOD_PASSWORD  = 1
	AUTHMETHOD_PUBLICKEY = 2
)

type PublicKey struct {
	Id       int    `json:"id" gorm:"column:id;primarykey"`
	Uid      int    `json:"uid" gorm:"column:uid"`
	UserName string `json:"username" gorm:"column:username"`
	Name     string `json:"name" gorm:"column:name"`
	Mac      string `json:"mac" gorm:"column:mac"`
	Pk       string `json:"pk" gorm:"column:pk"`

	// ResourceId int       `json:"resource_id"`
	CreatorId int                   `json:"creator_id" gorm:"column:creator_id"`
	UpdaterId int                   `json:"updater_id" gorm:"column:updater_id"`
	CreatedAt time.Time             `json:"created_at" gorm:"column:created_at"`
	UpdatedAt time.Time             `json:"updated_at" gorm:"column:updated_at"`
	DeletedAt soft_delete.DeletedAt `json:"-" gorm:"column:deleted_at"`
}

func (m *PublicKey) TableName() string {
	return "public_key"
}
func (m *PublicKey) SetId(id int) {
	m.Id = id
}
func (m *PublicKey) SetCreatorId(creatorId int) {
	m.CreatorId = creatorId
}
func (m *PublicKey) SetUpdaterId(updaterId int) {
	m.UpdaterId = updaterId
}
func (m *PublicKey) SetResourceId(resourceId int) {

}
func (m *PublicKey) GetResourceId() int {
	return 0
}
func (m *PublicKey) GetName() string {
	return m.Name
}
func (m *PublicKey) GetId() int {
	return m.Id
}

type ReqAuth struct {
	Method   int    `json:"method"`
	UserName string `json:"username"`
	Password string `json:"password"`
	Pk       string `json:"pk"`
}

type UserInfoResp struct {
	Result UserInfoRespResult `json:"result"`
}

type UserInfoRespResult struct {
	Uid int `json:"uid"`
	Rid int `json:"rid"`
}
