package model

import "time"

const (
	PAMApprovalITSM       = "itsm"
	PAMRequestPending     = "pending"
	PAMRequestApproved    = "approved"
	PAMRequestRejected    = "rejected"
	PAMRequestCancelled   = "cancelled"
	PAMRequestExpired     = "expired"
	PAMRequestInvalidated = "invalidated"
	PAMPasswordViewPolicy = "password_view"
)

type PAMPolicy struct {
	Id                     int             `json:"id" gorm:"primaryKey"`
	TargetKind             string          `json:"target_kind" gorm:"size:24;not null;uniqueIndex:pam_policy_target"`
	TargetID               int             `json:"target_id" gorm:"not null;uniqueIndex:pam_policy_target"`
	Revision               uint64          `json:"revision" gorm:"not null;default:1"`
	AllowRequests          bool            `json:"allow_requests" gorm:"not null;default:false"`
	ApprovalProvider       string          `json:"approval_provider" gorm:"size:16;not null;default:itsm"`
	RequiredApprovals      int             `json:"required_approvals" gorm:"not null;default:1"`
	MaxDurationMinutes     int             `json:"max_duration_minutes" gorm:"not null;default:60"`
	RequestExpiryMinutes   int             `json:"request_expiry_minutes" gorm:"not null;default:1440"`
	RequireApprovalActions Slice[string]   `json:"require_approval_actions" gorm:"type:json;not null"`
	SourceCIDRs            Slice[string]   `json:"source_cidrs" gorm:"type:json;not null"`
	AllowApplications      bool            `json:"allow_applications" gorm:"not null"`
	ConnectionPermissions  AuthPermissions `json:"connection_permissions" gorm:"type:json;not null"`
	MaxSessions            int             `json:"max_sessions" gorm:"not null;default:0"`
	MaxSessionMinutes      int             `json:"max_session_minutes" gorm:"not null;default:0"`
	ITSMTemplateID         int             `json:"itsm_template_id" gorm:"not null;default:0"`
	ITSMTemplateRevision   string          `json:"itsm_template_revision" gorm:"size:64"`
	ITSMTemplateName       string          `json:"itsm_template_name" gorm:"size:255"`
	CreatorID              int             `json:"creator_id"`
	UpdaterID              int             `json:"updater_id"`
	CreatedAt              time.Time       `json:"created_at"`
	UpdatedAt              time.Time       `json:"updated_at"`
}

func (PAMPolicy) TableName() string { return "pam_policy" }

type PAMAccessRequest struct {
	ID                    string           `json:"id" gorm:"size:64;primaryKey"`
	RequesterUID          int              `json:"requester_uid" gorm:"not null;uniqueIndex:pam_request_idempotency;index"`
	IdempotencyKey        string           `json:"-" gorm:"size:64;not null;uniqueIndex:pam_request_idempotency"`
	IntentHash            string           `json:"-" gorm:"size:64;not null"`
	TargetKind            string           `json:"target_kind" gorm:"size:24;not null;index:pam_request_target"`
	TargetID              int              `json:"target_id" gorm:"not null;index:pam_request_target"`
	TargetName            string           `json:"target_name" gorm:"size:128"`
	TargetUsername        string           `json:"target_username" gorm:"size:128"`
	Action                string           `json:"action" gorm:"size:32;not null"`
	AssetID               int              `json:"asset_id" gorm:"not null;default:0"`
	AccountID             int              `json:"account_id" gorm:"not null;default:0"`
	AssetName             string           `json:"asset_name" gorm:"size:128"`
	TargetHost            string           `json:"target_host" gorm:"size:512"`
	BindingProtocols      Slice[string]    `json:"binding_protocols" gorm:"type:json"`
	BindingID             uint64           `json:"binding_id" gorm:"not null;default:0"`
	BindingRevision       uint64           `json:"binding_revision" gorm:"not null;default:0"`
	PolicyID              int              `json:"policy_id" gorm:"not null"`
	PolicyRevision        uint64           `json:"policy_revision" gorm:"not null"`
	CredentialVersionID   string           `json:"credential_version_id,omitempty" gorm:"size:64"`
	DurationMinutes       int              `json:"duration_minutes" gorm:"not null"`
	Reason                string           `json:"reason" gorm:"size:2048;not null"`
	State                 string           `json:"state" gorm:"size:24;not null;index"`
	Revision              uint64           `json:"revision" gorm:"not null;default:1"`
	ApprovalProvider      string           `json:"approval_provider" gorm:"size:16;not null;index:pam_request_sync"`
	RequiredApprovals     int              `json:"required_approvals" gorm:"not null"`
	ITSMTemplateID        int              `json:"itsm_template_id" gorm:"not null;default:0"`
	ITSMTemplateRevision  string           `json:"itsm_template_revision" gorm:"size:64"`
	ITSMRequestRevision   uint64           `json:"-" gorm:"not null;default:1"`
	ITSMNodeInfo          Map[string, any] `json:"-" gorm:"type:json"`
	TicketID              int              `json:"ticket_id" gorm:"not null;default:0;index"`
	ProviderRevision      uint64           `json:"provider_revision" gorm:"not null;default:0"`
	ProviderEventID       string           `json:"-" gorm:"size:64"`
	ProviderPayloadHash   string           `json:"-" gorm:"size:64"`
	ApproverUIDs          Slice[int]       `json:"approver_uids,omitempty" gorm:"type:json"`
	GrantedAt             *time.Time       `json:"granted_at,omitempty"`
	GrantRevokedAt        *time.Time       `json:"grant_revoked_at,omitempty"`
	GrantRevokerUID       int              `json:"grant_revoker_uid,omitempty" gorm:"not null;default:0"`
	GrantRevokeReason     string           `json:"grant_revoke_reason,omitempty" gorm:"size:1024"`
	SyncState             string           `json:"sync_state" gorm:"size:16;not null;default:none;index:pam_request_sync"`
	SyncAttempts          int              `json:"-" gorm:"not null;default:0"`
	SyncNextAt            *time.Time       `json:"-" gorm:"index:pam_request_sync"`
	SyncLeaseOwner        string           `json:"-" gorm:"size:64"`
	SyncLeaseUntil        *time.Time       `json:"-"`
	SyncErrorCode         string           `json:"sync_error_code,omitempty" gorm:"size:64"`
	ConnectionPermissions AuthPermissions  `json:"connection_permissions" gorm:"type:json;not null"`
	ExpiresAt             time.Time        `json:"expires_at" gorm:"not null;index"`
	GrantedUntil          *time.Time       `json:"granted_until,omitempty"`
	CreatedAt             time.Time        `json:"created_at" gorm:"index"`
	UpdatedAt             time.Time        `json:"updated_at"`
}

func (PAMAccessRequest) TableName() string { return "pam_access_request" }

// PAMTemporaryAccess is the approved request's authorization view, not another database record.
type PAMTemporaryAccess struct {
	ID                    string          `json:"id"`
	RequestID             string          `json:"request_id"`
	RecipientUID          int             `json:"recipient_uid"`
	TargetKind            string          `json:"target_kind"`
	TargetID              int             `json:"target_id"`
	Action                string          `json:"action"`
	AssetID               int             `json:"asset_id"`
	AccountID             int             `json:"account_id"`
	BindingID             uint64          `json:"binding_id"`
	BindingRevision       uint64          `json:"binding_revision"`
	PolicyID              int             `json:"policy_id"`
	PolicyRevision        uint64          `json:"policy_revision"`
	CredentialVersionID   string          `json:"credential_version_id,omitempty"`
	ConnectionPermissions AuthPermissions `json:"connection_permissions"`
	ValidFrom             time.Time       `json:"valid_from"`
	ExpiresAt             time.Time       `json:"expires_at"`
	RevokedAt             *time.Time      `json:"revoked_at,omitempty"`
	RevokerUID            int             `json:"revoker_uid"`
	RevokeReason          string          `json:"revoke_reason"`
}

func (r *PAMAccessRequest) TemporaryAccess() *PAMTemporaryAccess {
	if r.GrantedAt == nil || r.GrantedUntil == nil {
		return nil
	}
	return &PAMTemporaryAccess{ID: r.ID, RequestID: r.ID, RecipientUID: r.RequesterUID,
		TargetKind: r.TargetKind, TargetID: r.TargetID, Action: r.Action, AssetID: r.AssetID, AccountID: r.AccountID,
		BindingID: r.BindingID, BindingRevision: r.BindingRevision, PolicyID: r.PolicyID, PolicyRevision: r.PolicyRevision,
		CredentialVersionID: r.CredentialVersionID, ConnectionPermissions: r.ConnectionPermissions,
		ValidFrom: *r.GrantedAt, ExpiresAt: *r.GrantedUntil, RevokedAt: r.GrantRevokedAt,
		RevokerUID: r.GrantRevokerUID, RevokeReason: r.GrantRevokeReason}
}

func (r *PAMAccessRequest) HasActiveAccess(now time.Time) bool {
	return r.State == PAMRequestApproved && r.GrantedAt != nil && r.GrantedUntil != nil &&
		r.GrantRevokedAt == nil && !now.Before(*r.GrantedAt) && now.Before(*r.GrantedUntil)
}

// PAMConnectionPermit is server-side admission evidence, not a client-supplied token.
type PAMConnectionPermit struct {
	StandingApproval bool
	ShareID          int
	OwnerID          int
	UID              int
	AssetID          int
	AccountID        int
	BindingID        uint64
	BindingRevision  uint64
	PolicyID         int
	PolicyRevision   uint64
	GrantIDs         []string
	Deadline         *time.Time
	SourceIP         string
	LeaseID          string
	LeaseExpiresAt   time.Time
}

type PAMSessionLease struct {
	SessionID      string    `gorm:"size:128;primaryKey"`
	ID             string    `gorm:"size:64;not null;uniqueIndex"`
	OwnerID        int       `gorm:"not null;index:pam_session_capacity"`
	UID            int       `gorm:"not null"`
	AssetID        int       `gorm:"not null"`
	AccountID      int       `gorm:"not null"`
	State          string    `gorm:"size:16;not null;index:pam_session_capacity"`
	LeaseExpiresAt time.Time `gorm:"not null;index:pam_session_capacity"`
	Deadline       *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (PAMSessionLease) TableName() string { return "pam_session_lease" }
