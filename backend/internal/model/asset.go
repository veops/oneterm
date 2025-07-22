package model

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"gorm.io/plugin/soft_delete"
)

const (
	TABLE_NAME_ASSET = "asset"
)

// AccountAuthorization represents authorization info for a specific account
type AccountAuthorization struct {
	Rids        Slice[int]       `json:"rids"`        // Role IDs for ACL system
	Permissions *AuthPermissions `json:"permissions"` // V2 permissions (connect, file_upload, etc.)
	RuleId      int              `json:"rule_id"`     // V2 authorization rule ID for tracking
}

// AuthorizationMap is a custom type that handles V1 to V2 authorization format conversion
type AuthorizationMap map[int]AccountAuthorization

// Scan implements the driver.Scanner interface for database deserialization
func (am *AuthorizationMap) Scan(value interface{}) error {
	if value == nil {
		*am = make(AuthorizationMap)
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		*am = make(AuthorizationMap)
		return nil
	}

	// Try to unmarshal as V2 format first
	var v2Auth map[int]AccountAuthorization
	if err := json.Unmarshal(bytes, &v2Auth); err == nil {
		// Check if this is actually V2 format (has Permissions field)
		for _, auth := range v2Auth {
			if auth.Permissions != nil {
				*am = AuthorizationMap(v2Auth)
				return nil
			}
		}
	}

	// Try to unmarshal as V1 format
	var v1Auth map[int][]int
	if err := json.Unmarshal(bytes, &v1Auth); err == nil {
		// Successfully parsed as V1 format, convert to V2
		defaultPermissions := getDefaultPermissionsForAsset()
		v2Auth = make(map[int]AccountAuthorization)
		for accountId, roleIds := range v1Auth {
			v2Auth[accountId] = AccountAuthorization{
				Rids:        roleIds,
				Permissions: &defaultPermissions,
			}
		}
		*am = AuthorizationMap(v2Auth)
		return nil
	}

	// Cannot parse as either format, set to empty map
	*am = make(AuthorizationMap)
	return nil
}

// Value implements the driver.Valuer interface for database serialization
func (am AuthorizationMap) Value() (driver.Value, error) {
	if am == nil {
		return "{}", nil
	}
	return json.Marshal(am)
}

// UnmarshalJSON implements JSON unmarshaling for API requests
func (am *AuthorizationMap) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as V2 format first
	var v2Auth map[int]AccountAuthorization
	if err := json.Unmarshal(data, &v2Auth); err == nil {
		*am = AuthorizationMap(v2Auth)
		return nil
	}

	// Try to unmarshal as V1 format
	var v1Auth map[int][]int
	if err := json.Unmarshal(data, &v1Auth); err == nil {
		// Successfully parsed as V1 format, convert to V2
		defaultPermissions := getDefaultPermissionsForAsset()
		v2Auth = make(map[int]AccountAuthorization)
		for accountId, roleIds := range v1Auth {
			v2Auth[accountId] = AccountAuthorization{
				Rids:        roleIds,
				Permissions: &defaultPermissions,
			}
		}
		*am = AuthorizationMap(v2Auth)
		return nil
	}

	// Cannot parse as either format, set to empty map
	*am = make(AuthorizationMap)
	return nil
}

// MarshalJSON implements JSON marshaling
func (am AuthorizationMap) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[int]AccountAuthorization(am))
}

type Asset struct {
	Id            int              `json:"id" gorm:"column:id;primarykey;autoIncrement"`
	Name          string           `json:"name" gorm:"column:name;uniqueIndex:name_del;size:128"`
	Comment       string           `json:"comment" gorm:"column:comment"`
	ParentId      int              `json:"parent_id" gorm:"column:parent_id"`
	Ip            string           `json:"ip" gorm:"column:ip"`
	Protocols     Slice[string]    `json:"protocols" gorm:"column:protocols;type:text"`
	GatewayId     int              `json:"gateway_id" gorm:"column:gateway_id"`
	Authorization AuthorizationMap `json:"authorization" gorm:"column:authorization;type:text"`
	AccessAuth    AccessAuth       `json:"access_auth" gorm:"embedded;column:access_auth"` // Deprecated: Use V2 fields below
	Connectable   bool             `json:"connectable" gorm:"column:connectable"`
	NodeChain     string           `json:"node_chain" gorm:"-"`

	// V2 Access Control (replaces AccessAuth)
	AccessTimeControl   *AccessTimeControl   `json:"access_time_control,omitempty" gorm:"column:access_time_control;type:json"`
	AssetCommandControl *AssetCommandControl `json:"asset_command_control,omitempty" gorm:"column:asset_command_control;type:json"`

	Permissions []string              `json:"permissions" gorm:"-"`
	ResourceId  int                   `json:"resource_id" gorm:"column:resource_id"`
	CreatorId   int                   `json:"creator_id" gorm:"column:creator_id"`
	UpdaterId   int                   `json:"updater_id" gorm:"column:updater_id"`
	CreatedAt   time.Time             `json:"created_at" gorm:"column:created_at"`
	UpdatedAt   time.Time             `json:"updated_at" gorm:"column:updated_at"`
	DeletedAt   soft_delete.DeletedAt `json:"-" gorm:"column:deleted_at;uniqueIndex:name_del"`
}

// getDefaultPermissionsForAsset returns default permissions for asset authorization conversion
func getDefaultPermissionsForAsset() AuthPermissions {
	// Try to get from global config first
	if config := GlobalConfig.Load(); config != nil {
		return config.GetDefaultPermissionsAsAuthPermissions()
	}

	// Fallback to connect-only permissions for security
	return AuthPermissions{
		Connect:      true,
		FileUpload:   false,
		FileDownload: false,
		Copy:         false,
		Paste:        false,
		Share:        false,
	}
}

type AccessAuth struct {
	Start  *time.Time   `json:"start,omitempty" gorm:"column:start"`
	End    *time.Time   `json:"end,omitempty" gorm:"column:end"`
	CmdIds Slice[int]   `json:"cmd_ids" gorm:"column:cmd_ids;type:text"`
	Ranges Slice[Range] `json:"ranges" gorm:"column:ranges;type:text"`
	Allow  bool         `json:"allow" gorm:"column:allow"`
}

// AccessTimeControl and AssetCommandControl are defined in authorization_v2.go

type Range struct {
	Week  int           `json:"week" gorm:"column:week"`
	Times Slice[string] `json:"times" gorm:"column:times"`
}

func (m *Asset) TableName() string {
	return TABLE_NAME_ASSET
}
func (m *Asset) SetId(id int) {
	m.Id = id
}
func (m *Asset) SetCreatorId(creatorId int) {
	m.CreatorId = creatorId
}
func (m *Asset) SetUpdaterId(updaterId int) {
	m.UpdaterId = updaterId
}
func (m *Asset) SetResourceId(resourceId int) {
	m.ResourceId = resourceId
}
func (m *Asset) GetResourceId() int {
	return m.ResourceId
}
func (m *Asset) GetName() string {
	return m.Name
}
func (m *Asset) GetId() int {
	return m.Id
}

func (m *Asset) SetPerms(perms []string) {
	m.Permissions = perms
}

type AssetIdPid struct {
	Id       int `gorm:"column:id"`
	ParentId int `gorm:"column:parent_id"`
}

func (m *AssetIdPid) TableName() string {
	return TABLE_NAME_ASSET
}
