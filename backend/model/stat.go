package model

import (
	"time"
)

type StatAssetType struct {
	Id    int    `json:"id" gorm:"column:id"`
	Name  string `json:"name" gorm:"column:name"`
	Count int64  `json:"count" gorm:"column:count"`
}

func (m *StatAssetType) TableName() string {
	return TABLE_NAME_NODE
}

type StatCount struct {
	Connect    int64 `json:"connect" gorm:"column:connect"`
	Session    int64 `json:"session" gorm:"column:session"`
	Asset      int64 `json:"asset" gorm:"column:asset"`
	TotalAsset int64 `json:"total_asset" gorm:"column:total_asset"`
	User       int64 `json:"user" gorm:"column:user"`
	// TotalUser    int64 `json:"total_user"`
	Gateway      int64 `json:"gateway" gorm:"column:gateway"`
	TotalGateway int64 `json:"total_gateway" gorm:"column:total_gateway"`
}

type StatAccount struct {
	Name  string `json:"name" gorm:"column:name"`
	Count int    `json:"count" gorm:"column:count"`
}

type StatAsset struct {
	Connect int64  `json:"connect" gorm:"column:connect"`
	Session int64  `json:"session" gorm:"column:session"`
	Asset   int64  `json:"asset" gorm:"column:asset"`
	User    int64  `json:"user" gorm:"column:user"`
	Time    string `json:"time" gorm:"column:time"`
}

type StatCountOfUser struct {
	Connect    int64 `json:"connect" gorm:"column:connect"`
	Session    int64 `json:"session" gorm:"column:session"`
	Asset      int64 `json:"asset" gorm:"column:asset"`
	TotalAsset int64 `json:"total_asset" gorm:"column:total_asset"`
}

type StatRankOfUser struct {
	Uid      int       `json:"uid" gorm:"column:uid"`
	Count    int64     `json:"count" gorm:"column:count"`
	LastTime time.Time `json:"last_time" gorm:"column:last_time"`
}
