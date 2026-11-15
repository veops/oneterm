package service

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/veops/oneterm/internal/acl"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/repository"
	"github.com/veops/oneterm/pkg/db"
	apiErrors "github.com/veops/oneterm/pkg/errors"
)

const PAMSessionCheckInterval = 10 * time.Second

func CheckPAMConnectionPermit(ctx context.Context, permit *model.PAMConnectionPermit) error {
	if permit == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if permit.Deadline != nil && !time.Now().Before(*permit.Deadline) {
		return ErrPAMDenied
	}
	if (len(permit.GrantIDs) == 0 || permit.StandingApproval) && permit.ShareID == 0 {
		if err := checkPAMStandingConnection(ctx, permit); err != nil {
			return err
		}
	} else {
		subject, err := acl.GetPAMKeySubject(ctx, permit.UID)
		if err != nil || subject.Blocked {
			return ErrPAMDenied
		}
	}
	return db.GetDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		target := model.PAMCredentialTarget{Kind: model.PAMOwnerAccount, ID: permit.OwnerID}
		return repository.WithPAMPolicy(tx, target, "SHARE", func(tx *gorm.DB, owner model.CredentialOwner, policy *model.PAMPolicy) error {
			if err := checkPAMConnectionScope(ctx, tx, permit, owner, policy); err != nil {
				return err
			}
			return checkPAMSessionLease(tx, permit)
		})
	})
}

func checkPAMConnectionScope(ctx context.Context, tx *gorm.DB, permit *model.PAMConnectionPermit, owner model.CredentialOwner, policy *model.PAMPolicy) error {
	if policy.Id != permit.PolicyID || policy.Revision != permit.PolicyRevision {
		return ErrPAMScopeChanged
	}
	if permit.Deadline != nil && !time.Now().Before(*permit.Deadline) {
		return ErrPAMDenied
	}
	if err := checkPAMOwner(ctx, tx, owner, policy, permit.SourceIP); err != nil {
		return err
	}
	target := model.PAMCredentialTarget{Kind: model.PAMOwnerAccount, ID: permit.OwnerID}
	binding, err := requestBinding(ctx, tx, PAMRequestInput{Target: target, Action: "connect", AssetID: permit.AssetID, AccountID: permit.AccountID})
	if err != nil {
		return err
	}
	if binding.ID != permit.BindingID || binding.Revision != permit.BindingRevision {
		return ErrPAMScopeChanged
	}
	if len(permit.GrantIDs) == 0 {
		if requiresPAMApproval(policy, "connect") {
			return ErrPAMDenied
		}
		if permit.ShareID != 0 {
			var share model.Share
			if err := tx.Where("id = ? AND asset_id = ? AND account_id = ?", permit.ShareID, permit.AssetID, permit.AccountID).First(&share).Error; err != nil {
				return ErrPAMDenied
			}
			now := time.Now()
			if now.Before(share.Start) || !now.Before(share.End) {
				return ErrPAMDenied
			}
		}
		return nil
	}
	var requests []model.PAMAccessRequest
	now := time.Now()
	if err := tx.Where("id IN ? AND requester_uid = ? AND target_kind = ? AND target_id = ? AND action = ? AND state = ?",
		permit.GrantIDs, permit.UID, target.Kind, target.ID, "connect", model.PAMRequestApproved).
		Where("binding_id = ? AND binding_revision = ? AND policy_id = ? AND policy_revision = ?", binding.ID, binding.Revision, policy.Id, policy.Revision).
		Where("grant_revoked_at IS NULL AND granted_at <= ? AND granted_until > ?", now, now).Find(&requests).Error; err != nil {
		return err
	}
	if len(requests) != len(permit.GrantIDs) {
		return ErrPAMDenied
	}
	return nil
}

// Only managed connections are monitored. No permission RPC or query runs per terminal input.
func WatchPAMConnection(ctx context.Context, permit *model.PAMConnectionPermit) error {
	if permit == nil {
		return nil
	}
	leaseExpiry := permit.LeaseExpiresAt
	for {
		checkDeadline := time.Now().Add(5 * time.Second)
		if permit.Deadline != nil && permit.Deadline.Before(checkDeadline) {
			checkDeadline = *permit.Deadline
		}
		if !leaseExpiry.IsZero() && leaseExpiry.Before(checkDeadline) {
			checkDeadline = leaseExpiry
		}
		checkCtx, cancel := context.WithDeadline(ctx, checkDeadline)
		err := CheckPAMConnectionPermit(checkCtx, permit)
		if err == nil {
			leaseExpiry, err = renewPAMSessionLease(checkCtx, permit)
		}
		cancel()
		if ctx.Err() != nil {
			return nil
		}
		if err != nil {
			return &apiErrors.ApiError{Code: apiErrors.ErrPAMSessionEnded}
		}
		nextCheck := time.Now().Add(PAMSessionCheckInterval)
		if permit.Deadline != nil && permit.Deadline.Before(nextCheck) {
			nextCheck = *permit.Deadline
		}
		if !leaseExpiry.IsZero() && leaseExpiry.Before(nextCheck) {
			nextCheck = leaseExpiry
		}
		timer := time.NewTimer(time.Until(nextCheck))
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
}
