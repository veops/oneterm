package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"github.com/veops/oneterm/internal/acl"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/pkg/config"
	dbpkg "github.com/veops/oneterm/pkg/db"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

// AccountRepository interface for account data access
type AccountRepository interface {
	AttachAssetCount(ctx context.Context, accounts []*model.Account) error
	CheckAssetDependencies(ctx context.Context, id int) (string, error)
	BuildQuery(ctx *gin.Context) *gorm.DB
	GetAccountIdsByAuthorization(ctx context.Context, assetIds []int, authorizationIds []int) ([]int, error)
}

// accountRepository implements AccountRepository
type accountRepository struct{}

// NewAccountRepository creates a new account repository
func NewAccountRepository() AccountRepository {
	return &accountRepository{}
}

// BuildQuery builds the base query for accounts with filters
func (r *accountRepository) BuildQuery(ctx *gin.Context) *gorm.DB {
	db := dbpkg.DB.Model(&model.Account{})

	// Apply filters
	db = dbpkg.FilterEqual(ctx, db, "id", "type")
	db = dbpkg.FilterLike(ctx, db, "name")
	db = dbpkg.FilterSearch(ctx, db, "name", "account")

	// Handle IDs parameter
	if q, ok := ctx.GetQuery("ids"); ok {
		db = db.Where("id IN ?", lo.Map(strings.Split(q, ","),
			func(s string, _ int) int { return cast.ToInt(s) }))
	}

	// Sort by name
	db = db.Order("name")

	return db
}

// AttachAssetCount attaches asset count to accounts using V2 authorization system
func (r *accountRepository) AttachAssetCount(ctx context.Context, accounts []*model.Account) error {
	// Get account IDs to filter
	accountIds := lo.Map(accounts, func(account *model.Account, _ int) int { return account.Id })

	// Get all V2 authorization rules where both account and asset selectors are 'ids' type
	// and account selector contains any of the target account IDs
	var rules []*model.AuthorizationV2
	if err := dbpkg.DB.Model(&model.AuthorizationV2{}).
		Where("enabled = ? AND JSON_EXTRACT(account_selector, '$.type') = ? AND JSON_EXTRACT(asset_selector, '$.type') = ?",
			true, "ids", "ids").
		Find(&rules).Error; err != nil {
		return err
	}

	// Count assets for each account
	accountAssetCounts := make(map[int]int64)

	// Filter rules that contain any of the target account IDs
	filteredRules := lo.Filter(rules, func(rule *model.AuthorizationV2, _ int) bool {
		ruleAccountIds := lo.Map(rule.AccountSelector.Values, func(value string, _ int) int {
			if id, err := strconv.Atoi(value); err == nil {
				return id
			}
			return -1
		})
		// Check if any account ID in the rule matches our target account IDs
		for _, ruleAccountId := range ruleAccountIds {
			if lo.Contains(accountIds, ruleAccountId) {
				return true
			}
		}
		return false
	})

	for _, rule := range filteredRules {
		// Extract account IDs from account selector
		ruleAccountIds := lo.FilterMap(rule.AccountSelector.Values, func(value string, _ int) (int, bool) {
			if id, err := strconv.Atoi(value); err == nil {
				return id, true
			}
			return 0, false
		})

		// Extract asset IDs from asset selector
		ruleAssetIds := lo.FilterMap(rule.AssetSelector.Values, func(value string, _ int) (int, bool) {
			if id, err := strconv.Atoi(value); err == nil {
				return id, true
			}
			return 0, false
		})

		// Count assets for each account in this rule
		for _, accountId := range ruleAccountIds {
			accountAssetCounts[accountId] += int64(len(ruleAssetIds))
		}
	}

	// Apply counts to accounts
	for _, account := range accounts {
		account.AssetCount = accountAssetCounts[account.Id]
	}

	return nil
}

// CheckAssetDependencies checks if account has dependent assets using V2 authorization system
func (r *accountRepository) CheckAssetDependencies(ctx context.Context, id int) (string, error) {
	// Get all V2 authorization rules where both account and asset selectors are 'ids' type
	var rules []*model.AuthorizationV2
	if err := dbpkg.DB.Model(&model.AuthorizationV2{}).
		Where("enabled = ? AND JSON_EXTRACT(account_selector, '$.type') = ? AND JSON_EXTRACT(asset_selector, '$.type') = ?",
			true, "ids", "ids").
		Find(&rules).Error; err != nil {
		return "", err
	}

	// Check if any rule contains this account ID
	for _, rule := range rules {
		// Extract account IDs from account selector
		ruleAccountIds := lo.FilterMap(rule.AccountSelector.Values, func(value string, _ int) (int, bool) {
			if accountId, err := strconv.Atoi(value); err == nil {
				return accountId, true
			}
			return 0, false
		})

		// Check if this account ID is in the rule
		if lo.Contains(ruleAccountIds, id) {
			// Extract asset IDs from asset selector
			ruleAssetIds := lo.FilterMap(rule.AssetSelector.Values, func(value string, _ int) (int, bool) {
				if assetId, err := strconv.Atoi(value); err == nil {
					return assetId, true
				}
				return 0, false
			})

			// If there are assets in this rule, return the first asset name
			if len(ruleAssetIds) > 0 {
				var assetName string
				err := dbpkg.DB.
					Model(model.DefaultAsset).
					Select("name").
					Where("id = ?", ruleAssetIds[0]).
					First(&assetName).
					Error

				if err == nil {
					return assetName, errors.New("account has dependent assets")
				}
			}
		}
	}

	return "", nil
}

// GetAccountIdsByAuthorization gets account IDs by authorization and asset IDs
func (r *accountRepository) GetAccountIdsByAuthorization(ctx context.Context, assetIds []int, authorizationIds []int) ([]int, error) {
	// Get account IDs from assets' authorization lists
	ss := make([]model.Slice[string], 0)
	if err := dbpkg.DB.Model(model.DefaultAsset).Where("id IN ?", assetIds).Pluck("JSON_KEYS(authorization)", &ss).Error; err != nil {
		return nil, err
	}

	// Process account IDs obtained from assets
	accountIds := lo.Uniq(lo.Map(lo.Flatten(ss), func(s string, _ int) int { return cast.ToInt(s) }))

	// Merge with account IDs from authorizations
	return lo.Uniq(append(accountIds, authorizationIds...)), nil
}

// HandleAccountIds filters account queries based on resource IDs
func HandleAccountIds(ctx context.Context, dbFind *gorm.DB, resIds []int) (db *gorm.DB, err error) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	assetResIds, err := acl.GetRoleResourceIds(ctx, currentUser.GetRid(), config.RESOURCE_ASSET)
	if err != nil {
		return
	}
	t, _ := HandleAssetIds(ctx, dbpkg.DB.Model(model.DefaultAsset), assetResIds)
	ss := make([]model.Slice[string], 0)
	if err = t.Pluck("JSON_KEYS(authorization)", &ss).Error; err != nil {
		return
	}
	ids := lo.Uniq(lo.Map(lo.Flatten(ss), func(s string, _ int) int { return cast.ToInt(s) }))

	d := dbpkg.DB.Where("resource_id IN ?", resIds).Or("id IN ?", ids)

	db = dbFind.Where(d)

	return
}

// GetAuth creates SSH authentication method from account credentials
func GetAuth(account *model.Account) (ssh.AuthMethod, error) {
	switch account.AccountType {
	case model.AUTHMETHOD_PASSWORD:
		return ssh.Password(account.Password), nil
	case model.AUTHMETHOD_PUBLICKEY:
		if account.Phrase == "" {
			pk, err := ssh.ParsePrivateKey([]byte(account.Pk))
			if err != nil {
				return nil, err
			}
			return ssh.PublicKeys(pk), nil
		} else {
			pk, err := ssh.ParsePrivateKeyWithPassphrase([]byte(account.Pk), []byte(account.Phrase))
			if err != nil {
				return nil, err
			}
			return ssh.PublicKeys(pk), nil
		}
	default:
		return nil, fmt.Errorf("invalid authmethod %d", account.AccountType)
	}
}
