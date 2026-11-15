package service

import (
	"context"
	"errors"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/veops/oneterm/internal/acl"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/repository"
	gsession "github.com/veops/oneterm/internal/session"
)

type pamGrantScope struct {
	UID       int
	Action    string
	Owner     model.CredentialOwner
	Policy    *model.PAMPolicy
	Binding   *model.PAMAssetAccountBinding
	Operation model.AuthAction
}

// A grant authorizes its recipient and pinned scope, never the approver's identity.
func findPAMGrant(tx *gorm.DB, scope pamGrantScope) (*model.PAMTemporaryAccess, error) {
	now := time.Now()
	query := tx.Model(&model.PAMAccessRequest{}).Where(
		"requester_uid = ? AND target_kind = ? AND target_id = ? AND action = ? AND state = ?",
		scope.UID, scope.Owner.CredentialOwnerKind(), scope.Owner.GetId(), scope.Action, model.PAMRequestApproved).
		Where("policy_id = ? AND policy_revision = ? AND grant_revoked_at IS NULL AND granted_at <= ? AND granted_until > ?",
			scope.Policy.Id, scope.Policy.Revision, now, now)
	if scope.Binding != nil {
		query = query.Where("binding_id = ? AND binding_revision = ? AND asset_id = ? AND account_id = ?",
			scope.Binding.ID, scope.Binding.Revision, scope.Binding.AssetID, scope.Binding.AccountID)
	}
	if scope.Action == acl.Retrieve {
		query = query.Where("credential_version_id = ?", passwordViewVersion(scope.Owner))
	}
	var candidates []model.PAMAccessRequest
	if err := query.Order("granted_until DESC, id").Find(&candidates).Error; err != nil {
		return nil, err
	}
	for _, candidate := range candidates {
		var current model.PAMAccessRequest
		err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Where("id = ?", candidate.ID).First(&current).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if !current.HasActiveAccess(time.Now()) || current.RequesterUID != scope.UID ||
			!requestScopeCurrent(tx.Statement.Context, tx, &current, scope.Owner, scope.Policy) {
			continue
		}
		grant := current.TemporaryAccess()
		if scope.Action == "connect" && (!grant.ConnectionPermissions.Connect || !grant.ConnectionPermissions.HasPermission(scope.Operation)) {
			continue
		}
		return grant, nil
	}
	return nil, nil
}

func checkPAMOwner(ctx context.Context, tx *gorm.DB, owner model.CredentialOwner, policy *model.PAMPolicy, source string) error {
	available, err := repository.CredentialOwnerAvailable(ctx, tx, owner)
	if err != nil {
		return ErrPAMUnavailable
	}
	if !available || !sourceAllowed(source, policy.SourceCIDRs) {
		return ErrPAMDenied
	}
	return nil
}

func authorizePAMConnection(ctx *gin.Context, database *gorm.DB, sess *gsession.Session, result *model.BatchAuthResult) (*model.BatchAuthResult, error) {
	var binding model.PAMAssetAccountBinding
	err := database.WithContext(ctx.Request.Context()).Where("asset_id = ? AND account_id = ?", sess.AssetId, sess.AccountId).First(&binding).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		var account model.Account
		if err := database.WithContext(ctx.Request.Context()).Select("id", "managed").First(&account, sess.AccountId).Error; err != nil {
			return nil, ErrPAMUnavailable
		}
		if account.Managed {
			return nil, repository.ErrPAMBindingUnavailable
		}
		return result, nil
	}
	if err != nil {
		return nil, ErrPAMUnavailable
	}
	operator, _ := acl.GetSessionFromCtx(ctx)
	if operator == nil || operator.Uid <= 0 {
		return nil, ErrPAMDenied
	}
	target := model.PAMCredentialTarget{Kind: model.PAMOwnerAccount, ID: binding.AccountID}
	err = database.WithContext(ctx.Request.Context()).Transaction(func(tx *gorm.DB) error {
		return repository.WithPAMPolicy(tx, target, "SHARE", func(tx *gorm.DB, owner model.CredentialOwner, policy *model.PAMPolicy) error {
			current, err := requestBinding(ctx.Request.Context(), tx, PAMRequestInput{Target: target, Action: "connect", AssetID: sess.AssetId, AccountID: sess.AccountId})
			if err != nil {
				return err
			}
			if err := checkPAMOwner(ctx.Request.Context(), tx, owner, policy, ctx.ClientIP()); err != nil {
				return err
			}
			permit := &model.PAMConnectionPermit{ShareID: sess.ShareId, OwnerID: target.ID, UID: operator.Uid, AssetID: sess.AssetId, AccountID: sess.AccountId,
				BindingID: current.ID, BindingRevision: current.Revision, PolicyID: policy.Id, PolicyRevision: policy.Revision, SourceIP: ctx.ClientIP()}
			if sess.ShareId != 0 && !sess.ShareEnd.IsZero() {
				deadline := sess.ShareEnd
				permit.Deadline = &deadline
			}
			grants := make(map[string]bool)
			standingConnection := result.IsAllowed(model.ActionConnect)
			approvalRequired := requiresPAMApproval(policy, "connect")
			if standingConnection && approvalRequired && sess.ShareId == 0 {
				gate, err := findPAMGrant(tx, pamGrantScope{UID: operator.Uid, Action: "connect", Owner: owner, Policy: policy, Binding: current, Operation: model.ActionConnect})
				if err != nil {
					return ErrPAMUnavailable
				}
				if gate != nil {
					permit.GrantIDs = []string{gate.ID}
					permit.StandingApproval = true
					if _, exists := result.Results[model.ActionShare]; exists {
						result.Results[model.ActionShare] = &model.AuthResult{Allowed: false, Reason: "pam_approval_is_personal"}
					}
					deadline := gate.ExpiresAt
					if permit.Deadline == nil || deadline.Before(*permit.Deadline) {
						permit.Deadline = &deadline
					}
					// Approval gates the connection; V2 remains the ceiling for every action.
					result.PAM = permit
					return nil
				}
			}
			if standingConnection && !approvalRequired {
				result.PAM = permit
				return nil
			}
			temporaryEligible := false
			if !standingConnection && policy.AllowRequests && sess.ShareId == 0 {
				var permissionErr error
				temporaryEligible, permissionErr = pamResourcePermission(ctx.Request.Context(), operator, owner, acl.RequestAccess, true)
				if permissionErr != nil {
					return ErrPAMUnavailable
				}
			}
			required := approvalRequired || !standingConnection
			for action, standing := range result.Results {
				if standing.Allowed && !required {
					continue
				}
				reason := "pam_authorization_required"
				if standingConnection && approvalRequired && sess.ShareId == 0 {
					reason = "pam_approval_required"
				}
				result.Results[action] = &model.AuthResult{Allowed: false, Reason: reason}
				// Shared access never borrows a requester's temporary authorization.
				if sess.ShareId != 0 || action == model.ActionShare || (standingConnection && approvalRequired) || (!standingConnection && !temporaryEligible) {
					continue
				}
				grant, err := findPAMGrant(tx, pamGrantScope{UID: operator.Uid, Action: "connect", Owner: owner,
					Policy: policy, Binding: current, Operation: action})
				if err != nil {
					return ErrPAMUnavailable
				}
				if grant != nil {
					if !grants[grant.ID] {
						permit.GrantIDs = append(permit.GrantIDs, grant.ID)
						grants[grant.ID] = true
					}
					if permit.Deadline == nil || grant.ExpiresAt.Before(*permit.Deadline) {
						deadline := grant.ExpiresAt
						permit.Deadline = &deadline
					}
					result.Results[action] = &model.AuthResult{Allowed: true, Reason: "pam_temporary_grant", Permissions: grant.ConnectionPermissions,
						Restrictions: map[string]any{"pam_grant_id": grant.ID, "expires_at": grant.ExpiresAt}}
				}
			}
			result.PAM = permit
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	if result.IsAllowed(model.ActionConnect) {
		subject, err := acl.GetPAMKeySubject(ctx.Request.Context(), operator.Uid)
		if err != nil || subject.Blocked {
			return nil, ErrPAMDenied
		}
	}
	return result, nil
}
