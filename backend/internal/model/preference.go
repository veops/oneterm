package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// JSON is a custom type for storing JSON data
type JSON map[string]interface{}

// Value implements the driver.Valuer interface
func (j JSON) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan implements the sql.Scanner interface
func (j *JSON) Scan(value interface{}) error {
	if value == nil {
		*j = make(JSON)
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	return json.Unmarshal(bytes, j)
}

// UserPreference stores user terminal preferences
type UserPreference struct {
	ID            int       `json:"id" gorm:"primaryKey"`
	UserID        int       `json:"user_id" gorm:"not null;uniqueIndex"` // User ID with unique index
	Theme         string    `json:"theme" gorm:"size:50"`                // Theme name
	FontFamily    string    `json:"font_family" gorm:"size:100"`         // Font family
	FontSize      int       `json:"font_size"`                           // Font size
	LineHeight    float64   `json:"line_height"`                         // Line height
	LetterSpacing float64   `json:"letter_spacing"`                      // Letter spacing
	CursorStyle   string    `json:"cursor_style" gorm:"size:20"`         // Cursor style (block, bar, underline)
	Settings      JSON      `json:"settings" gorm:"type:json"`           // Additional settings in JSON format
	CreatedAt     time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt     time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName specifies the table name for UserPreference
func (m *UserPreference) TableName() string {
	return "user_preferences"
}

// Implement Model interface
func (m *UserPreference) SetId(id int) {
	m.ID = id
}

func (m *UserPreference) SetCreatorId(creatorId int) {
	m.UserID = creatorId
}

func (m *UserPreference) SetUpdaterId(updaterId int) {
	// Not applicable
}

func (m *UserPreference) SetResourceId(resourceId int) {
	// Not applicable
}

func (m *UserPreference) GetResourceId() int {
	return 0
}

func (m *UserPreference) GetName() string {
	return "" // No name field
}

func (m *UserPreference) GetId() int {
	return m.ID
}

func (m *UserPreference) SetPerms(perms []string) {
	// Not applicable
}
