package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gorm.io/plugin/soft_delete"
)

type CustomTime struct {
	time.Time
}

func (ct *CustomTime) UnmarshalJSON(data []byte) error {
	str := string(data)
	str = strings.Trim(str, "\"")

	parsedTime, err := time.Parse("2006-01-02 15:04:05", str)
	if err != nil {
		return fmt.Errorf("failed to parse time %s: %v", str, err)
	}

	ct.Time = parsedTime
	return nil
}

func (ct CustomTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(ct.Time.Format("2006-01-02 15:04:05"))
}

func (ct *CustomTime) Scan(value interface{}) error {
	if value == nil {
		ct.Time = time.Time{}
		return nil
	}

	if t, ok := value.(time.Time); ok {
		ct.Time = t
		return nil
	}

	return fmt.Errorf("cannot scan %T into CustomTime", value)
}

func (ct CustomTime) Value() (driver.Value, error) {
	return ct.Time, nil
}

// SelectorType defines the type of target selector
type SelectorType string

const (
	SelectorTypeAll   SelectorType = "all"
	SelectorTypeIds   SelectorType = "ids"
	SelectorTypeRegex SelectorType = "regex"
	SelectorTypeTags  SelectorType = "tags"
)

// AuthAction defines the supported authorization actions
type AuthAction string

const (
	ActionConnect      AuthAction = "connect"
	ActionFileUpload   AuthAction = "file_upload"
	ActionFileDownload AuthAction = "file_download"
	ActionCopy         AuthAction = "copy"
	ActionPaste        AuthAction = "paste"
	ActionShare        AuthAction = "share"
)

// TimeRange defines time restrictions
type TimeRange struct {
	StartTime string     `json:"start_time" gorm:"column:start_time"`
	EndTime   string     `json:"end_time" gorm:"column:end_time"`
	Weekdays  Slice[int] `json:"weekdays" gorm:"column:weekdays"`
}

func (t *TimeRange) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	return json.Unmarshal(value.([]byte), t)
}

func (t TimeRange) Value() (driver.Value, error) {
	return json.Marshal(t)
}

// TimeRanges is a custom slice type for TimeRange
type TimeRanges []TimeRange

func (t *TimeRanges) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	return json.Unmarshal(value.([]byte), t)
}

func (t TimeRanges) Value() (driver.Value, error) {
	return json.Marshal(t)
}

// AccessTimeControl defines time-based access control (used by both Asset and AuthorizationV2)
type AccessTimeControl struct {
	Enabled    bool       `json:"enabled" gorm:"column:enabled"`
	TimeRanges TimeRanges `json:"time_ranges" gorm:"column:time_ranges"`
	Timezone   string     `json:"timezone" gorm:"column:timezone"` // e.g., "Asia/Shanghai"
	Comment    string     `json:"comment" gorm:"column:comment"`   // Description of the time restriction
}

func (a *AccessTimeControl) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	return json.Unmarshal(value.([]byte), a)
}

func (a AccessTimeControl) Value() (driver.Value, error) {
	return json.Marshal(a)
}

// AssetCommandControl defines command control restrictions for assets
type AssetCommandControl struct {
	Enabled     bool       `json:"enabled" gorm:"column:enabled"`
	CmdIds      Slice[int] `json:"cmd_ids" gorm:"column:cmd_ids"`           // Command IDs to control
	TemplateIds Slice[int] `json:"template_ids" gorm:"column:template_ids"` // Command template IDs
	Comment     string     `json:"comment" gorm:"column:comment"`           // Description of the command restriction
}

func (a *AssetCommandControl) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	return json.Unmarshal(value.([]byte), a)
}

func (a AssetCommandControl) Value() (driver.Value, error) {
	return json.Marshal(a)
}

// TargetSelector defines how to select targets (nodes, assets, accounts)
type TargetSelector struct {
	Type       SelectorType  `json:"type" gorm:"column:type"`
	Values     Slice[string] `json:"values" gorm:"column:values"`
	ExcludeIds Slice[int]    `json:"exclude_ids" gorm:"column:exclude_ids"`
}

func (t *TargetSelector) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	return json.Unmarshal(value.([]byte), t)
}

func (t TargetSelector) Value() (driver.Value, error) {
	return json.Marshal(t)
}

// AuthPermissions defines the permissions for different actions
type AuthPermissions struct {
	Connect      bool `json:"connect" gorm:"column:connect"`
	FileUpload   bool `json:"file_upload" gorm:"column:file_upload"`
	FileDownload bool `json:"file_download" gorm:"column:file_download"`
	Copy         bool `json:"copy" gorm:"column:copy"`
	Paste        bool `json:"paste" gorm:"column:paste"`
	Share        bool `json:"share" gorm:"column:share"`
}

func (p *AuthPermissions) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	return json.Unmarshal(value.([]byte), p)
}

func (p AuthPermissions) Value() (driver.Value, error) {
	return json.Marshal(p)
}

func (p *AuthPermissions) HasPermission(action AuthAction) bool {
	switch action {
	case ActionConnect:
		return p.Connect
	case ActionFileUpload:
		return p.FileUpload
	case ActionFileDownload:
		return p.FileDownload
	case ActionCopy:
		return p.Copy
	case ActionPaste:
		return p.Paste
	case ActionShare:
		return p.Share
	default:
		return false
	}
}

// CommandAction defines simple command actions
type CommandAction string

const (
	CommandActionAllow CommandAction = "allow"
	CommandActionDeny  CommandAction = "deny"
	CommandActionAudit CommandAction = "audit" // Log but allow
)

// AccessControl defines access control restrictions for authorization rules with time template support
type AccessControl struct {
	IPWhitelist Slice[string] `json:"ip_whitelist" gorm:"column:ip_whitelist"`

	// Time control options
	TimeTemplate     *TimeTemplateReference `json:"time_template" gorm:"column:time_template;type:json"`           // Reference to template
	CustomTimeRanges TimeRanges             `json:"custom_time_ranges" gorm:"column:custom_time_ranges;type:json"` // Direct definition
	Timezone         string                 `json:"timezone" gorm:"column:timezone;size:64"`

	// Command control
	CmdIds      Slice[int] `json:"cmd_ids" gorm:"column:cmd_ids"`
	TemplateIds Slice[int] `json:"template_ids" gorm:"column:template_ids"`

	MaxSessions    int `json:"max_sessions" gorm:"column:max_sessions"`
	SessionTimeout int `json:"session_timeout" gorm:"column:session_timeout"`
}

func (a *AccessControl) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	return json.Unmarshal(value.([]byte), a)
}

func (a AccessControl) Value() (driver.Value, error) {
	return json.Marshal(a)
}

// AuthorizationV2 is the new flexible authorization model
type AuthorizationV2 struct {
	Id          int    `json:"id" gorm:"column:id;primarykey;autoIncrement"`
	Name        string `json:"name" gorm:"column:name;uniqueIndex:name_del;size:128"`
	Description string `json:"description" gorm:"column:description"`
	Enabled     bool   `json:"enabled" gorm:"column:enabled;default:true"`

	// Rule validity period
	ValidFrom *CustomTime `json:"valid_from" gorm:"column:valid_from"`
	ValidTo   *CustomTime `json:"valid_to" gorm:"column:valid_to"`

	// Target selectors
	NodeSelector    TargetSelector `json:"node_selector" gorm:"column:node_selector;type:json"`
	AssetSelector   TargetSelector `json:"asset_selector" gorm:"column:asset_selector;type:json"`
	AccountSelector TargetSelector `json:"account_selector" gorm:"column:account_selector;type:json"`

	// Permissions configuration
	Permissions AuthPermissions `json:"permissions" gorm:"column:permissions;type:json"`

	// Access control with time template support
	AccessControl AccessControl `json:"access_control" gorm:"column:access_control;type:json"`

	// Role IDs for ACL integration
	Rids Slice[int] `json:"rids" gorm:"column:rids"`

	// Standard fields
	ResourceId int                   `json:"resource_id" gorm:"column:resource_id"`
	CreatorId  int                   `json:"creator_id" gorm:"column:creator_id"`
	UpdaterId  int                   `json:"updater_id" gorm:"column:updater_id"`
	CreatedAt  time.Time             `json:"created_at" gorm:"column:created_at"`
	UpdatedAt  time.Time             `json:"updated_at" gorm:"column:updated_at"`
	DeletedAt  soft_delete.DeletedAt `json:"-" gorm:"column:deleted_at;uniqueIndex:name_del"`
}

func (m *AuthorizationV2) TableName() string {
	return "authorization_v2"
}

func (m *AuthorizationV2) GetName() string {
	return m.Name
}

func (m *AuthorizationV2) GetId() int {
	return m.Id
}

func (m *AuthorizationV2) SetId(id int) {
	m.Id = id
}

func (m *AuthorizationV2) SetCreatorId(creatorId int) {
	m.CreatorId = creatorId
}

func (m *AuthorizationV2) SetUpdaterId(updaterId int) {
	m.UpdaterId = updaterId
}

func (m *AuthorizationV2) SetResourceId(resourceId int) {
	m.ResourceId = resourceId
}

func (m *AuthorizationV2) GetResourceId() int {
	return m.ResourceId
}

func (m *AuthorizationV2) SetPerms(perms []string) {}

// IsValid checks if the authorization rule is currently valid based on time period
func (m *AuthorizationV2) IsValid(checkTime time.Time) bool {
	if !m.Enabled {
		return false
	}

	// Check valid from time
	if m.ValidFrom != nil && checkTime.Before(m.ValidFrom.Time) {
		return false
	}

	// Check valid to time
	if m.ValidTo != nil && checkTime.After(m.ValidTo.Time) {
		return false
	}

	return true
}

// IsCurrentlyValid checks if the authorization rule is currently valid
func (m *AuthorizationV2) IsCurrentlyValid() bool {
	return m.IsValid(time.Now())
}

// AuthRequest represents an authorization request
type AuthRequest struct {
	UserId    int        `json:"user_id"`
	NodeId    int        `json:"node_id"`
	AssetId   int        `json:"asset_id"`
	AccountId int        `json:"account_id"`
	Action    AuthAction `json:"action"`
	ClientIP  string     `json:"client_ip"`
	UserAgent string     `json:"user_agent"`
	Timestamp time.Time  `json:"timestamp"`
}

// BatchAuthRequest represents a batch authorization request for multiple actions
type BatchAuthRequest struct {
	UserId    int          `json:"user_id"`
	NodeId    int          `json:"node_id"`
	AssetId   int          `json:"asset_id"`
	AccountId int          `json:"account_id"`
	Actions   []AuthAction `json:"actions"`
	ClientIP  string       `json:"client_ip"`
	UserAgent string       `json:"user_agent"`
	Timestamp time.Time    `json:"timestamp"`
}

// AuthResult represents the result of an authorization check
type AuthResult struct {
	Allowed      bool                   `json:"allowed"`
	Permissions  AuthPermissions        `json:"permissions"`
	Reason       string                 `json:"reason"`
	RuleId       int                    `json:"rule_id"`
	RuleName     string                 `json:"rule_name"`
	Restrictions map[string]interface{} `json:"restrictions"`
}

// BatchAuthResult represents the result of a batch authorization check
type BatchAuthResult struct {
	Results map[AuthAction]*AuthResult `json:"results"`
}

// IsAllowed checks if a specific action is allowed in the batch result
func (r *BatchAuthResult) IsAllowed(action AuthAction) bool {
	if result, exists := r.Results[action]; exists {
		return result.Allowed
	}
	return false
}

// GetResult returns the authorization result for a specific action
func (r *BatchAuthResult) GetResult(action AuthAction) *AuthResult {
	return r.Results[action]
}

// CommandCheckResult represents the result of a command permission check
type CommandCheckResult struct {
	Allowed bool          `json:"allowed"`
	Action  CommandAction `json:"action"`
	Reason  string        `json:"reason"`
}

// DefaultAuthorizationV2 for caching
var DefaultAuthorizationV2 = &AuthorizationV2{}
