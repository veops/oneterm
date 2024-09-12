package model

import (
	"time"
)

type History struct {
	Id         int              `json:"id" gorm:"column:id;primarykey;autoIncrement"`
	RemoteIp   string           `json:"remote_ip" gorm:"column:remote_ip"`
	Type       string           `json:"type" gorm:"column:type"`
	TargetId   int              `json:"target_id" gorm:"column:target_id"`
	ActionType int              `json:"action_type" gorm:"column:action_type"`
	Old        Map[string, any] `json:"old" gorm:"column:old"`
	New        Map[string, any] `json:"new" gorm:"column:new"`

	CreatorId int       `json:"creator_id" gorm:"column:creator_id"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at"`
}

func (m *History) TableName() string {
	return "history"
}
