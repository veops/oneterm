package model

import (
	"time"
)

const (
	FILE_ACTION_LS = iota + 1
	FILE_ACTION_MKDIR
	FILE_ACTION_UPLOAD
	FILE_ACTION_DOWNLOAD
)

type FileHistory struct {
	Id        int    `json:"id" gorm:"column:id;primarykey;autoIncrement"`
	Uid       int    `json:"uid" gorm:"column:uid"`
	UserName  string `json:"user_name" gorm:"column:user_name"`
	AssetId   int    `json:"asset_id" gorm:"column:asset_id"`
	AccountId int    `json:"account_id" gorm:"column:account_id"`
	ClientIp  string `json:"client_ip" gorm:"column:client_ip"`
	Action    int    `json:"action" gorm:"column:action"`
	Dir       string `json:"dir" gorm:"column:dir"`
	Filename  string `json:"filename" gorm:"column:filename"`

	CreatedAt time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt time.Time `json:"updated_at" gorm:"column:updated_at"`
}

func (m *FileHistory) TableName() string {
	return "file_history"
}
