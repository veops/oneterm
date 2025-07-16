package model

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"gorm.io/plugin/soft_delete"
)

// TimeTemplate defines predefined time access templates
type TimeTemplate struct {
	Id          int    `json:"id" gorm:"column:id;primarykey;autoIncrement"`
	Name        string `json:"name" gorm:"column:name;uniqueIndex:name_del;size:128;not null"`
	Description string `json:"description" gorm:"column:description"`
	Category    string `json:"category" gorm:"column:category;size:64"` // work, maintenance, emergency, etc.

	// Time configuration
	TimeRanges TimeRanges `json:"time_ranges" gorm:"column:time_ranges;type:json"`
	Timezone   string     `json:"timezone" gorm:"column:timezone;size:64;default:'Asia/Shanghai'"`

	// Status and metadata
	IsBuiltIn bool `json:"is_builtin" gorm:"column:is_builtin;default:false"` // System built-in templates
	IsActive  bool `json:"is_active" gorm:"column:is_active;default:true"`

	// Usage statistics
	UsageCount int `json:"usage_count" gorm:"column:usage_count;default:0"`

	// Standard fields
	ResourceId int                   `json:"resource_id" gorm:"column:resource_id"`
	CreatorId  int                   `json:"creator_id" gorm:"column:creator_id"`
	UpdaterId  int                   `json:"updater_id" gorm:"column:updater_id"`
	CreatedAt  time.Time             `json:"created_at" gorm:"column:created_at"`
	UpdatedAt  time.Time             `json:"updated_at" gorm:"column:updated_at"`
	DeletedAt  soft_delete.DeletedAt `json:"-" gorm:"column:deleted_at;uniqueIndex:name_del"`
}

func (m *TimeTemplate) TableName() string {
	return "time_template"
}

func (m *TimeTemplate) GetName() string {
	return m.Name
}

func (m *TimeTemplate) GetId() int {
	return m.Id
}

func (m *TimeTemplate) SetId(id int) {
	m.Id = id
}

func (m *TimeTemplate) SetCreatorId(creatorId int) {
	m.CreatorId = creatorId
}

func (m *TimeTemplate) SetUpdaterId(updaterId int) {
	m.UpdaterId = updaterId
}

func (m *TimeTemplate) SetResourceId(resourceId int) {
	m.ResourceId = resourceId
}

func (m *TimeTemplate) GetResourceId() int {
	return m.ResourceId
}

func (m *TimeTemplate) SetPerms(perms []string) {}

// TimeTemplateReference defines how authorization rules reference time templates
type TimeTemplateReference struct {
	TemplateId   int        `json:"template_id" gorm:"column:template_id"`
	TemplateName string     `json:"template_name" gorm:"column:template_name"`           // For display
	CustomRanges TimeRanges `json:"custom_ranges" gorm:"column:custom_ranges;type:json"` // Additional custom time ranges
}

func (t *TimeTemplateReference) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	return json.Unmarshal(value.([]byte), t)
}

func (t TimeTemplateReference) Value() (driver.Value, error) {
	return json.Marshal(t)
}

// BuiltInTimeTemplates defines system built-in time templates
var BuiltInTimeTemplates = []TimeTemplate{
	{
		Name:        "Business Hours",
		Description: "Standard business hours: Monday to Friday 9:00-18:00",
		Category:    "work",
		TimeRanges: TimeRanges{
			{
				StartTime: "09:00",
				EndTime:   "18:00",
				Weekdays:  Slice[int]{1, 2, 3, 4, 5}, // Monday to Friday
			},
		},
		IsBuiltIn: true,
		IsActive:  true,
	},
	{
		Name:        "Weekend Duty",
		Description: "Weekend duty hours: Saturday and Sunday 10:00-16:00",
		Category:    "duty",
		TimeRanges: TimeRanges{
			{
				StartTime: "10:00",
				EndTime:   "16:00",
				Weekdays:  Slice[int]{6, 7}, // Saturday and Sunday
			},
		},
		IsBuiltIn: true,
		IsActive:  true,
	},
	{
		Name:        "Maintenance Window",
		Description: "System maintenance window: Sunday 2:00-6:00",
		Category:    "maintenance",
		TimeRanges: TimeRanges{
			{
				StartTime: "02:00",
				EndTime:   "06:00",
				Weekdays:  Slice[int]{7}, // Sunday
			},
		},
		IsBuiltIn: true,
		IsActive:  true,
	},
	{
		Name:        "24x7 Access",
		Description: "24x7 around the clock access",
		Category:    "always",
		TimeRanges: TimeRanges{
			{
				StartTime: "00:00",
				EndTime:   "23:59",
				Weekdays:  Slice[int]{1, 2, 3, 4, 5, 6, 7}, // All days
			},
		},
		IsBuiltIn: true,
		IsActive:  true,
	},
	{
		Name:        "Emergency Response",
		Description: "Emergency response hours: weekdays 18:00-22:00",
		Category:    "emergency",
		TimeRanges: TimeRanges{
			{
				StartTime: "18:00",
				EndTime:   "22:00",
				Weekdays:  Slice[int]{1, 2, 3, 4, 5}, // Monday to Friday
			},
		},
		IsBuiltIn: true,
		IsActive:  true,
	},
}

// DefaultTimeTemplate for caching
var DefaultTimeTemplate = &TimeTemplate{}
