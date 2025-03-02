package model

import (
	"database/sql/driver"
	"encoding/json"
)

const (
	ACTION_CREATE = iota + 1
	ACTION_DELETE
	ACTION_UPDATE
)

type Slice[T int | string | Range] []T

func (s *Slice[T]) Scan(value any) error {
	return json.Unmarshal(value.([]byte), s)
}

func (s Slice[T]) Value() (driver.Value, error) {
	return json.Marshal(s)
}

type Map[K comparable, V any] map[K]V

func (m *Map[K, V]) Scan(value any) error {
	return json.Unmarshal(value.([]byte), m)

}

func (m Map[K, V]) Value() (driver.Value, error) {
	return json.Marshal(m)
}

type Model interface {
	TableName() string
	SetId(int)
	SetCreatorId(int)
	SetUpdaterId(int)
	SetResourceId(int)
	GetResourceId() int
	GetId() int
	GetName() string
	SetPerms([]string)
}

type Pair[T1, T2 any] struct {
	First  T1
	Second T2
}
