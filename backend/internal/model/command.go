package model

import (
	"regexp"
	"time"

	"gorm.io/plugin/soft_delete"
)

// CommandCategory defines command categories
type CommandCategory string

const (
	CategorySecurity  CommandCategory = "security"  // Security related
	CategorySystem    CommandCategory = "system"    // System operations
	CategoryDatabase  CommandCategory = "database"  // Database operations
	CategoryNetwork   CommandCategory = "network"   // Network operations
	CategoryFile      CommandCategory = "file"      // File operations
	CategoryDeveloper CommandCategory = "developer" // Development related
	CategoryCustom    CommandCategory = "custom"    // Custom commands
)

// CommandRiskLevel defines risk levels
type CommandRiskLevel int

const (
	RiskLevelSafe     CommandRiskLevel = 0 // Safe commands
	RiskLevelWarning  CommandRiskLevel = 1 // Warning level
	RiskLevelDanger   CommandRiskLevel = 2 // Dangerous commands
	RiskLevelCritical CommandRiskLevel = 3 // Critical danger
)

type Command struct {
	Id     int            `json:"id" gorm:"column:id;primarykey;autoIncrement"`
	Name   string         `json:"name" gorm:"column:name;uniqueIndex:name_del;size:128"`
	Cmd    string         `json:"cmd" gorm:"column:cmd"`
	IsRe   bool           `json:"is_re" gorm:"column:is_re"`
	Enable bool           `json:"enable" gorm:"column:enable"`
	Re     *regexp.Regexp `json:"-" gorm:"-"`

	// Enhanced fields for better management
	Category    CommandCategory  `json:"category" gorm:"column:category;default:'custom'"`
	RiskLevel   CommandRiskLevel `json:"risk_level" gorm:"column:risk_level;default:0"`
	Description string           `json:"description" gorm:"column:description"`
	Tags        Slice[string]    `json:"tags" gorm:"column:tags;type:json"`
	IsGlobal    bool             `json:"is_global" gorm:"column:is_global;default:false"` // Global predefined command

	Permissions []string              `json:"permissions" gorm:"-"`
	ResourceId  int                   `json:"resource_id" gorm:"column:resource_id"`
	CreatorId   int                   `json:"creator_id" gorm:"column:creator_id"`
	UpdaterId   int                   `json:"updater_id" gorm:"column:updater_id"`
	CreatedAt   time.Time             `json:"created_at" gorm:"column:created_at"`
	UpdatedAt   time.Time             `json:"updated_at" gorm:"column:updated_at"`
	DeletedAt   soft_delete.DeletedAt `json:"-" gorm:"column:deleted_at;uniqueIndex:name_del"`
}

func (m *Command) TableName() string {
	return "command"
}
func (m *Command) SetId(id int) {
	m.Id = id
}
func (m *Command) SetCreatorId(creatorId int) {
	m.CreatorId = creatorId
}
func (m *Command) SetUpdaterId(updaterId int) {
	m.UpdaterId = updaterId
}
func (m *Command) SetResourceId(resourceId int) {
	m.ResourceId = resourceId
}
func (m *Command) GetResourceId() int {
	return m.ResourceId
}
func (m *Command) GetName() string {
	return m.Name
}
func (m *Command) GetId() int {
	return m.Id
}

func (m *Command) SetPerms(perms []string) {
	m.Permissions = perms
}

// GetRiskLevelText returns human readable risk level
func (m *Command) GetRiskLevelText() string {
	switch m.RiskLevel {
	case RiskLevelSafe:
		return "Safe"
	case RiskLevelWarning:
		return "Warning"
	case RiskLevelDanger:
		return "Danger"
	case RiskLevelCritical:
		return "Critical"
	default:
		return "Unknown"
	}
}

// CommandTemplate represents predefined command templates
type CommandTemplate struct {
	Id          int             `json:"id" gorm:"column:id;primarykey;autoIncrement"`
	Name        string          `json:"name" gorm:"column:name;size:128"`
	Description string          `json:"description" gorm:"column:description"`
	Category    CommandCategory `json:"category" gorm:"column:category"`
	CmdIds      Slice[int]      `json:"cmd_ids" gorm:"column:cmd_ids;type:json"`
	IsBuiltin   bool            `json:"is_builtin" gorm:"column:is_builtin;default:false"` // Built-in template
	ResourceId  int             `json:"resource_id" gorm:"column:resource_id"`
	CreatorId   int             `json:"creator_id" gorm:"column:creator_id"`
	UpdaterId   int             `json:"updater_id" gorm:"column:updater_id"`
	CreatedAt   time.Time       `json:"created_at" gorm:"column:created_at"`
	UpdatedAt   time.Time       `json:"updated_at" gorm:"column:updated_at"`
}

func (m *CommandTemplate) TableName() string {
	return "command_template"
}

func (m *CommandTemplate) GetName() string {
	return m.Name
}

func (m *CommandTemplate) GetId() int {
	return m.Id
}

func (m *CommandTemplate) SetId(id int) {
	m.Id = id
}

func (m *CommandTemplate) SetCreatorId(creatorId int) {
	m.CreatorId = creatorId
}

func (m *CommandTemplate) SetUpdaterId(updaterId int) {
	m.UpdaterId = updaterId
}

func (m *CommandTemplate) SetResourceId(resourceId int) {
	m.ResourceId = resourceId
}

func (m *CommandTemplate) GetResourceId() int {
	return m.ResourceId
}

func (m *CommandTemplate) SetPerms(perms []string) {
	// CommandTemplate doesn't have permissions field, but interface requires it
}
