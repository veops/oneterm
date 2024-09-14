package model

import (
	"time"

	"gorm.io/plugin/soft_delete"
)

type Authorization struct {
	Id        int `json:"id" gorm:"column:id;primarykey;autoIncrement"`
	AssetId   int `json:"asset_id" gorm:"column:asset_id;uniqueIndex:asset_account_id_del"`
	AccountId int `json:"account_id" gorm:"column:account_id;uniqueIndex:asset_account_id_del"`

	ResourceId int                   `json:"resource_id" gorm:"column:resource_id"`
	CreatorId  int                   `json:"creator_id" gorm:"column:creator_id"`
	UpdaterId  int                   `json:"updater_id" gorm:"column:updater_id"`
	CreatedAt  time.Time             `json:"created_at" gorm:"column:created_at"`
	UpdatedAt  time.Time             `json:"updated_at" gorm:"column:updated_at"`
	DeletedAt  soft_delete.DeletedAt `json:"-" gorm:"column:deleted_at;uniqueIndex:asset_account_id_del"`
}

func (m *Authorization) TableName() string {
	return "authorization"
}

type InfoModel interface {
	GetId() int
}

type AuthorizationIds struct {
	AssetId   int `json:"asset_id" gorm:"column:asset_id"`
	AccountId int `json:"account_id" gorm:"column:account_id"`
}

func (m *AuthorizationIds) TableName() string {
	return "authorization"
}

type AssetInfo struct {
	Id            int           `json:"id" gorm:"column:id;primarykey;autoIncrement"`
	Name          string        `json:"name" gorm:"column:name"`
	Comment       string        `json:"comment" gorm:"column:comment"`
	ParentId      int           `json:"parent_id" gorm:"column:parent_id"`
	Ip            string        `json:"ip" gorm:"column:ip"`
	Protocols     Slice[string] `json:"protocols" gorm:"column:protocols"`
	Connectable   bool          `json:"connectable" gorm:"column:connectable"`
	NodeChain     string        `json:"node_chain" gorm:"-"`
	*AccessAuth   `json:"access_auth" gorm:"column:access_auth"`
	Authorization Map[int, Slice[int]] `json:"-" gorm:"column:authorization"`
	GatewayId     int                  `json:"-" gorm:"column:gateway_id"`
	Gateway       *GatewayInfo         `json:"gateway,omitempty" gorm:"-"`
	Accounts      []*AccountInfo       `json:"accounts" gorm:"-"`
	Commands      []*CmdInfo           `json:"commands" gorm:"-"`
}

func (m *AssetInfo) GetId() int {
	return m.Id
}

type AccountInfo struct {
	Id          int    `json:"id" gorm:"column:id;primarykey;autoIncrement"`
	Name        string `json:"name" gorm:"column:name"`
	Account     string `json:"account" gorm:"column:account"`
	AccountType int    `json:"account_type,omitempty" gorm:"column:account_type"`
	Password    string `json:"password,omitempty" gorm:"column:password"`
}

func (m *AccountInfo) GetId() int {
	return m.Id
}

type GatewayInfo struct {
	Id          int    `json:"id" gorm:"column:id;primarykey;autoIncrement"`
	Name        string `json:"name" gorm:"column:name"`
	Host        string `json:"host" gorm:"column:host"`
	Port        int    `json:"port" gorm:"column:port"`
	AccountType int    `json:"account_type" gorm:"column:account_type"`
	Account     string `json:"account" gorm:"column:account"`
	Password    string `json:"password" gorm:"column:password"`
}

func (m *GatewayInfo) GetId() int {
	return m.Id
}

type CmdInfo struct {
	Id     int           `json:"id" gorm:"column:id;primarykey;autoIncrement"`
	Name   string        `json:"name" gorm:"column:name"`
	Cmds   Slice[string] `json:"cmds" gorm:"column:cmds"`
	Enable int           `json:"enable" gorm:"column:enable"`
}

func (m *CmdInfo) GetId() int {
	return m.Id
}
