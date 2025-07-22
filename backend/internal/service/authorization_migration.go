package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/repository"
	dbpkg "github.com/veops/oneterm/pkg/db"
	"github.com/veops/oneterm/pkg/logger"
)

// AuthorizationMigrationService handles V1 to V2 authorization migration
type AuthorizationMigrationService struct {
	db     *gorm.DB
	v1Repo repository.IAuthorizationRepository
	v2Repo repository.IAuthorizationV2Repository
}

// NewAuthorizationMigrationService creates a new migration service
func NewAuthorizationMigrationService(
	db *gorm.DB,
	v1Repo repository.IAuthorizationRepository,
	v2Repo repository.IAuthorizationV2Repository,
) *AuthorizationMigrationService {
	return &AuthorizationMigrationService{
		db:     db,
		v1Repo: v1Repo,
		v2Repo: v2Repo,
	}
}

// MigrateV1ToV2 performs the complete migration from V1 to V2
func (s *AuthorizationMigrationService) MigrateV1ToV2(ctx context.Context) error {
	logger.L().Info("Starting V1 to V2 authorization migration")

	// Check if migration is already completed
	if completed, err := s.IsMigrationCompleted(ctx); err != nil {
		return fmt.Errorf("failed to check migration status: %w", err)
	} else if completed {
		logger.L().Info("Migration already completed, skipping")
		return nil
	}

	// Get all V1 authorization rules
	v1Rules, err := s.getAllV1Rules(ctx)
	if err != nil {
		return fmt.Errorf("failed to get V1 rules: %w", err)
	}

	if len(v1Rules) == 0 {
		logger.L().Info("No V1 authorization rules found, marking migration as completed")
		return s.markMigrationCompleted(ctx, 0)
	}

	logger.L().Info("Found V1 authorization rules", zap.Int("count", len(v1Rules)))

	// Mark migration as running
	if err := s.markMigrationRunning(ctx); err != nil {
		return fmt.Errorf("failed to mark migration as running: %w", err)
	}

	// Start transaction
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			s.markMigrationFailed(ctx, fmt.Sprintf("panic during migration: %v", r))
		}
	}()

	// Migrate each V1 rule to V2
	migratedCount := 0
	for _, v1Rule := range v1Rules {
		v2Rule := s.convertV1ToV2(v1Rule)
		if err := s.v2Repo.Create(ctx, v2Rule); err != nil {
			tx.Rollback()
			s.markMigrationFailed(ctx, fmt.Sprintf("failed to create V2 rule for V1 rule %d: %v", v1Rule.Id, err))
			return fmt.Errorf("failed to create V2 rule for V1 rule %d: %w", v1Rule.Id, err)
		}
		migratedCount++
		logger.L().Debug("Migrated V1 rule", zap.Int("v1_id", v1Rule.Id), zap.Int("v2_id", v2Rule.Id))
	}

	// Also migrate asset authorization fields from V1 to V2 format
	if err := s.migrateAssetAuthorizationFields(ctx, tx); err != nil {
		tx.Rollback()
		s.markMigrationFailed(ctx, fmt.Sprintf("failed to migrate asset authorization fields: %v", err))
		return fmt.Errorf("failed to migrate asset authorization fields: %w", err)
	}

	// Mark migration as completed
	if err := s.markMigrationCompleted(ctx, migratedCount); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to mark migration as completed: %w", err)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit migration transaction: %w", err)
	}

	logger.L().Info("V1 to V2 migration completed successfully",
		zap.Int("migrated_count", migratedCount))
	return nil
}

// IsMigrationCompleted checks if the migration has been completed
func (s *AuthorizationMigrationService) IsMigrationCompleted(ctx context.Context) (bool, error) {
	var record model.MigrationRecord
	err := s.db.Where("migration_name = ?", model.MigrationAuthV1ToV2).First(&record).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil
		}
		return false, err
	}

	return record.Status == model.MigrationStatusCompleted, nil
}

// markMigrationCompleted marks the migration as completed
func (s *AuthorizationMigrationService) markMigrationCompleted(ctx context.Context, recordsCount int) error {
	now := time.Now()

	// Try to update existing record, or create new one
	var record model.MigrationRecord
	err := s.db.Where("migration_name = ?", model.MigrationAuthV1ToV2).First(&record).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}

	if err == gorm.ErrRecordNotFound {
		// Create new record
		record = model.MigrationRecord{
			MigrationName: model.MigrationAuthV1ToV2,
			Status:        model.MigrationStatusCompleted,
			StartedAt:     &now,
			CompletedAt:   &now,
			RecordsCount:  recordsCount,
		}
		return s.db.Create(&record).Error
	}

	// Update existing record
	return s.db.Model(&record).Updates(map[string]interface{}{
		"status":        model.MigrationStatusCompleted,
		"completed_at":  &now,
		"error_message": "",
		"records_count": recordsCount,
	}).Error
}

// getAllV1Rules retrieves all V1 authorization rules
func (s *AuthorizationMigrationService) getAllV1Rules(ctx context.Context) ([]*model.Authorization, error) {
	var rules []*model.Authorization
	err := s.db.Find(&rules).Error
	return rules, err
}

// convertV1ToV2 converts a V1 authorization rule to V2 format
func (s *AuthorizationMigrationService) convertV1ToV2(v1 *model.Authorization) *model.AuthorizationV2 {
	v2 := &model.AuthorizationV2{
		Name:        s.generateRuleName(v1),
		Description: s.generateRuleDescription(v1),
		Enabled:     true,
		Rids:        v1.Rids,

		// Copy standard fields
		ResourceId: v1.ResourceId,
		CreatorId:  v1.CreatorId,
		UpdaterId:  v1.UpdaterId,
		CreatedAt:  v1.CreatedAt,
		UpdatedAt:  v1.UpdatedAt,
	}

	// Convert specific IDs to selectors
	if v1.NodeId > 0 {
		v2.NodeSelector = model.TargetSelector{
			Type:   model.SelectorTypeIds,
			Values: []string{fmt.Sprintf("%d", v1.NodeId)},
		}
	} else {
		v2.NodeSelector = model.TargetSelector{
			Type: model.SelectorTypeAll,
		}
	}

	if v1.AssetId > 0 {
		v2.AssetSelector = model.TargetSelector{
			Type:   model.SelectorTypeIds,
			Values: []string{fmt.Sprintf("%d", v1.AssetId)},
		}
	} else {
		v2.AssetSelector = model.TargetSelector{
			Type: model.SelectorTypeAll,
		}
	}

	if v1.AccountId > 0 {
		v2.AccountSelector = model.TargetSelector{
			Type:   model.SelectorTypeIds,
			Values: []string{fmt.Sprintf("%d", v1.AccountId)},
		}
	} else {
		v2.AccountSelector = model.TargetSelector{
			Type: model.SelectorTypeAll,
		}
	}

	// Set default permissions - V1 only had connect permission
	v2.Permissions = model.AuthPermissions{
		Connect:      true,
		FileUpload:   s.getDefaultFileUploadPermission(),
		FileDownload: s.getDefaultFileDownloadPermission(),
		Copy:         s.getDefaultCopyPermission(),
		Paste:        s.getDefaultPastePermission(),
		Share:        false, // Default to false for security
	}

	// Set default access control
	v2.AccessControl = model.AccessControl{
		IPWhitelist:    []string{}, // No IP restrictions by default
		MaxSessions:    0,          // No session limit by default
		SessionTimeout: 0,          // Use system default
	}

	return v2
}

// generateRuleName generates a name for the migrated rule
func (s *AuthorizationMigrationService) generateRuleName(v1 *model.Authorization) string {
	parts := []string{"Migrated"}

	if v1.NodeId > 0 {
		parts = append(parts, fmt.Sprintf("Node-%d", v1.NodeId))
	}
	if v1.AssetId > 0 {
		parts = append(parts, fmt.Sprintf("Asset-%d", v1.AssetId))
	}
	if v1.AccountId > 0 {
		parts = append(parts, fmt.Sprintf("Account-%d", v1.AccountId))
	}

	return fmt.Sprintf("%s-Rule-V1-%d", joinParts(parts), v1.Id)
}

// generateRuleDescription generates a description for the migrated rule
func (s *AuthorizationMigrationService) generateRuleDescription(v1 *model.Authorization) string {
	return fmt.Sprintf("Automatically migrated from V1 authorization rule (ID: %d). "+
		"Node: %d, Asset: %d, Account: %d, Roles: %v",
		v1.Id, v1.NodeId, v1.AssetId, v1.AccountId, v1.Rids)
}

// Helper functions to get default permissions from system config
func (s *AuthorizationMigrationService) getDefaultFileUploadPermission() bool {
	// In a real implementation, you might check system configuration
	// For now, return true as a safe default
	return true
}

func (s *AuthorizationMigrationService) getDefaultFileDownloadPermission() bool {
	return true
}

func (s *AuthorizationMigrationService) getDefaultCopyPermission() bool {
	return true
}

func (s *AuthorizationMigrationService) getDefaultPastePermission() bool {
	return true
}

// joinParts joins string parts with "-"
func joinParts(parts []string) string {
	result := ""
	for i, part := range parts {
		if i > 0 {
			result += "-"
		}
		result += part
	}
	return result
}

// markMigrationRunning marks the migration as running
func (s *AuthorizationMigrationService) markMigrationRunning(ctx context.Context) error {
	now := time.Now()

	// Try to update existing record, or create new one
	var record model.MigrationRecord
	err := s.db.Where("migration_name = ?", model.MigrationAuthV1ToV2).First(&record).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}

	if err == gorm.ErrRecordNotFound {
		// Create new record
		record = model.MigrationRecord{
			MigrationName: model.MigrationAuthV1ToV2,
			Status:        model.MigrationStatusRunning,
			StartedAt:     &now,
		}
		return s.db.Create(&record).Error
	}

	// Update existing record
	return s.db.Model(&record).Updates(map[string]interface{}{
		"status":        model.MigrationStatusRunning,
		"started_at":    &now,
		"completed_at":  nil,
		"error_message": "",
	}).Error
}

// markMigrationFailed marks the migration as failed with error message
func (s *AuthorizationMigrationService) markMigrationFailed(ctx context.Context, errorMsg string) error {
	var record model.MigrationRecord
	err := s.db.Where("migration_name = ?", model.MigrationAuthV1ToV2).First(&record).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// Create new record with failed status
			record = model.MigrationRecord{
				MigrationName: model.MigrationAuthV1ToV2,
				Status:        model.MigrationStatusFailed,
				ErrorMessage:  errorMsg,
			}
			return s.db.Create(&record).Error
		}
		return err
	}

	// Update existing record
	return s.db.Model(&record).Updates(map[string]interface{}{
		"status":        model.MigrationStatusFailed,
		"error_message": errorMsg,
	}).Error
}

// migrateAssetAuthorizationFields migrates asset authorization fields from V1 to V2 format
func (s *AuthorizationMigrationService) migrateAssetAuthorizationFields(ctx context.Context, tx *gorm.DB) error {
	logger.L().Info("Starting asset authorization field migration")

	// Query assets with potential V1 authorization data
	var assets []*model.Asset
	if err := tx.Select("id", "authorization").Find(&assets).Error; err != nil {
		logger.L().Error("Failed to get assets for authorization field migration", zap.Error(err))
		return err
	}

	migrationCount := 0
	for _, asset := range assets {
		if asset.Id == 0 {
			continue
		}

		// Check if authorization field needs migration
		needsMigration, err := s.checkAssetAuthorizationNeedsMigration(tx, asset.Id)
		if err != nil {
			logger.L().Error("Failed to check asset authorization migration need", zap.Int("assetId", asset.Id), zap.Error(err))
			continue
		}

		if !needsMigration {
			continue
		}

		// Migrate this asset
		if err := s.migrateAssetAuthorizationField(ctx, tx, asset.Id); err != nil {
			logger.L().Error("Failed to migrate asset authorization field", zap.Int("assetId", asset.Id), zap.Error(err))
			continue
		}

		migrationCount++
	}

	logger.L().Info("Asset authorization field migration completed", zap.Int("migrated", migrationCount))
	return nil
}

// MigrateV1AuthorizationData migrates V1 authorization data format to V2
func MigrateV1AuthorizationData(ctx context.Context) error {
	logger.L().Info("Starting V1 to V2 authorization data migration")

	// Query assets with potential V1 authorization data
	var assets []*model.Asset
	if err := dbpkg.DB.Select("id", "authorization").Find(&assets).Error; err != nil {
		logger.L().Error("Failed to get assets for migration", zap.Error(err))
		return err
	}

	migrationCount := 0
	for _, asset := range assets {
		if asset.Id == 0 {
			continue
		}

		// Check if authorization field needs migration
		needsMigration, err := checkNeedsMigration(asset.Id)
		if err != nil {
			logger.L().Error("Failed to check migration need", zap.Int("assetId", asset.Id), zap.Error(err))
			continue
		}

		if !needsMigration {
			continue
		}

		// Migrate this asset
		if err := migrateAssetAuthorization(ctx, asset.Id); err != nil {
			logger.L().Error("Failed to migrate asset authorization", zap.Int("assetId", asset.Id), zap.Error(err))
			continue
		}

		migrationCount++
	}

	logger.L().Info("V1 to V2 authorization migration completed", zap.Int("migrated", migrationCount))
	return nil
}

// checkNeedsMigration checks if an asset's authorization data needs migration
func checkNeedsMigration(assetId int) (bool, error) {
	var rawAuth json.RawMessage
	if err := dbpkg.DB.Model(&model.Asset{}).
		Where("id = ?", assetId).
		Select("authorization").
		Scan(&rawAuth).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil
		}
		return false, err
	}

	if len(rawAuth) == 0 {
		return false, nil
	}

	// Try to parse as V2 format first
	var v2Auth map[int]model.AccountAuthorization
	if err := json.Unmarshal(rawAuth, &v2Auth); err == nil {
		// Successfully parsed as V2, no migration needed
		return false, nil
	}

	// Try to parse as V1 format
	var v1Auth map[int][]int
	if err := json.Unmarshal(rawAuth, &v1Auth); err == nil {
		// Successfully parsed as V1, needs migration
		return true, nil
	}

	// Cannot parse as either format, skip
	return false, nil
}

// migrateAssetAuthorization migrates a single asset's authorization data
func migrateAssetAuthorization(ctx context.Context, assetId int) error {
	// Get raw authorization data
	var rawAuth json.RawMessage
	if err := dbpkg.DB.Model(&model.Asset{}).
		Where("id = ?", assetId).
		Select("authorization").
		Scan(&rawAuth).Error; err != nil {
		return err
	}

	if len(rawAuth) == 0 {
		return nil
	}

	// Parse as V1 format
	var v1Auth map[int][]int
	if err := json.Unmarshal(rawAuth, &v1Auth); err != nil {
		return fmt.Errorf("failed to parse V1 authorization data: %w", err)
	}

	// Get default permissions from config
	defaultPermissions := getDefaultAuthPermissions()

	// Start transaction for atomic operation
	return dbpkg.DB.Transaction(func(tx *gorm.DB) error {
		// Convert V1 to V2 format AND create V2 authorization rules
		v2Auth := make(map[int]model.AccountAuthorization)
		for accountId, roleIds := range v1Auth {
			// Create V2 authorization rule for this asset-account combination
			rule := &model.AuthorizationV2{
				Name:        fmt.Sprintf("Asset-%d-Account-%d-Migrated", assetId, accountId),
				Description: fmt.Sprintf("Migrated rule for asset %d and account %d", assetId, accountId),
				Enabled:     true,

				// Target selectors - specific asset and account
				AssetSelector: model.TargetSelector{
					Type:       model.SelectorTypeIds,
					Values:     []string{fmt.Sprintf("%d", assetId)},
					ExcludeIds: []int{},
				},
				AccountSelector: model.TargetSelector{
					Type:       model.SelectorTypeIds,
					Values:     []string{fmt.Sprintf("%d", accountId)},
					ExcludeIds: []int{},
				},

				// Use default permissions
				Permissions: defaultPermissions,

				// Role IDs from V1 data
				Rids: roleIds,

				// Standard fields (use system user for migration)
				CreatorId: 1, // System user
				UpdaterId: 1, // System user
			}

			// Create ACL resource for this rule
			resourceId, err := createBasicACLResourceForMigration(rule.Name)
			if err != nil {
				return fmt.Errorf("failed to create ACL resource: %w", err)
			}
			rule.ResourceId = resourceId

			// Create the V2 rule
			if err := tx.Create(rule).Error; err != nil {
				return fmt.Errorf("failed to create V2 authorization rule: %w", err)
			}

			// Convert to V2 format with RuleId
			v2Auth[accountId] = model.AccountAuthorization{
				Rids:        roleIds,
				Permissions: &defaultPermissions,
				RuleId:      rule.Id, // Set the rule ID for tracking
			}
		}

		// Update the database with V2 format including RuleIds
		if err := tx.Model(&model.Asset{}).
			Where("id = ?", assetId).
			Update("authorization", v2Auth).Error; err != nil {
			return fmt.Errorf("failed to update authorization data: %w", err)
		}

		logger.L().Debug("Migrated asset authorization data with V2 rules",
			zap.Int("assetId", assetId),
			zap.Int("accounts", len(v1Auth)))

		return nil
	})
}

// getDefaultAuthPermissions returns default permissions for migration
func getDefaultAuthPermissions() model.AuthPermissions {
	// Get from config if available
	if config := model.GlobalConfig.Load(); config != nil {
		return config.GetDefaultPermissionsAsAuthPermissions()
	}

	// Fallback to connect-only permissions
	return model.AuthPermissions{
		Connect:      true,
		FileUpload:   false,
		FileDownload: false,
		Copy:         false,
		Paste:        false,
		Share:        false,
	}
}

// checkAssetAuthorizationNeedsMigration checks if an asset's authorization data needs migration
func (s *AuthorizationMigrationService) checkAssetAuthorizationNeedsMigration(tx *gorm.DB, assetId int) (bool, error) {
	var rawAuth json.RawMessage
	if err := tx.Model(&model.Asset{}).
		Where("id = ?", assetId).
		Select("authorization").
		Scan(&rawAuth).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil
		}
		return false, err
	}

	if len(rawAuth) == 0 {
		return false, nil
	}

	// Try to parse as V2 format first
	var v2Auth map[int]model.AccountAuthorization
	if err := json.Unmarshal(rawAuth, &v2Auth); err == nil {
		// Successfully parsed as V2, no migration needed
		return false, nil
	}

	// Try to parse as V1 format
	var v1Auth map[int][]int
	if err := json.Unmarshal(rawAuth, &v1Auth); err == nil {
		// Successfully parsed as V1, needs migration
		return true, nil
	}

	// Cannot parse as either format, skip
	return false, nil
}

// migrateAssetAuthorizationField migrates a single asset's authorization data
func (s *AuthorizationMigrationService) migrateAssetAuthorizationField(ctx context.Context, tx *gorm.DB, assetId int) error {
	// Get raw authorization data
	var rawAuth json.RawMessage
	if err := tx.Model(&model.Asset{}).
		Where("id = ?", assetId).
		Select("authorization").
		Scan(&rawAuth).Error; err != nil {
		return err
	}

	if len(rawAuth) == 0 {
		return nil
	}

	// Parse as V1 format
	var v1Auth map[int][]int
	if err := json.Unmarshal(rawAuth, &v1Auth); err != nil {
		return fmt.Errorf("failed to parse V1 authorization data: %w", err)
	}

	// Get default permissions
	defaultPermissions := s.getDefaultAuthPermissions()

	// Convert V1 to V2 format AND create V2 authorization rules
	v2Auth := make(map[int]model.AccountAuthorization)
	for accountId, roleIds := range v1Auth {
		// Create V2 authorization rule for this asset-account combination
		rule := &model.AuthorizationV2{
			Name:        fmt.Sprintf("Asset-%d-Account-%d-Migrated", assetId, accountId),
			Description: fmt.Sprintf("Migrated rule for asset %d and account %d", assetId, accountId),
			Enabled:     true,

			// Target selectors - specific asset and account
			AssetSelector: model.TargetSelector{
				Type:       model.SelectorTypeIds,
				Values:     []string{fmt.Sprintf("%d", assetId)},
				ExcludeIds: []int{},
			},
			AccountSelector: model.TargetSelector{
				Type:       model.SelectorTypeIds,
				Values:     []string{fmt.Sprintf("%d", accountId)},
				ExcludeIds: []int{},
			},

			// Use default permissions
			Permissions: defaultPermissions,

			// Role IDs from V1 data
			Rids: roleIds,

			// Standard fields (use system user for migration)
			CreatorId: 1, // System user
			UpdaterId: 1, // System user
		}

		// Create ACL resource for this rule
		// Note: For migration, we'll create a basic ACL resource without user context
		resourceId, err := s.createBasicACLResource(rule.Name)
		if err != nil {
			return fmt.Errorf("failed to create ACL resource: %w", err)
		}
		rule.ResourceId = resourceId

		// Create the V2 rule
		if err := tx.Create(rule).Error; err != nil {
			return fmt.Errorf("failed to create V2 authorization rule: %w", err)
		}

		// Convert to V2 format with RuleId
		v2Auth[accountId] = model.AccountAuthorization{
			Rids:        roleIds,
			Permissions: &defaultPermissions,
			RuleId:      rule.Id, // Set the rule ID for tracking
		}
	}

	// Update the database with V2 format including RuleIds
	if err := tx.Model(&model.Asset{}).
		Where("id = ?", assetId).
		Update("authorization", v2Auth).Error; err != nil {
		return fmt.Errorf("failed to update authorization data: %w", err)
	}

	logger.L().Debug("Migrated asset authorization data with V2 rules",
		zap.Int("assetId", assetId),
		zap.Int("accounts", len(v1Auth)))

	return nil
}

// createBasicACLResource creates a basic ACL resource for migration purposes
func (s *AuthorizationMigrationService) createBasicACLResource(name string) (int, error) {
	// For migration, we'll create a simplified ACL resource
	// This is a basic implementation - in a real system, you might need to
	// integrate with the actual ACL system more thoroughly

	// Note: This is a placeholder implementation
	// In the actual system, you would need to use the proper ACL creation methods
	// For now, we'll create a unique resource ID (this should be replaced with actual ACL integration)

	// Generate a simple resource ID based on current time
	// In real implementation, this should use the actual ACL system
	resourceId := int(time.Now().UnixNano() % 1000000)

	logger.L().Debug("Created basic ACL resource for migration",
		zap.String("name", name),
		zap.Int("resourceId", resourceId))

	return resourceId, nil
}

// getDefaultAuthPermissions returns default permissions for migration service
func (s *AuthorizationMigrationService) getDefaultAuthPermissions() model.AuthPermissions {
	// Get from config if available
	if config := model.GlobalConfig.Load(); config != nil {
		return config.GetDefaultPermissionsAsAuthPermissions()
	}

	// Fallback to connect-only permissions
	return model.AuthPermissions{
		Connect:      true,
		FileUpload:   false,
		FileDownload: false,
		Copy:         false,
		Paste:        false,
		Share:        false,
	}
}

// createBasicACLResourceForMigration creates a basic ACL resource for migration purposes (standalone function)
func createBasicACLResourceForMigration(name string) (int, error) {
	// For migration, we'll create a simplified ACL resource
	// This is a basic implementation - in a real system, you might need to
	// integrate with the actual ACL system more thoroughly

	// Generate a simple resource ID based on current time
	// In real implementation, this should use the actual ACL system
	resourceId := int(time.Now().UnixNano() % 1000000)

	logger.L().Debug("Created basic ACL resource for migration",
		zap.String("name", name),
		zap.Int("resourceId", resourceId))

	return resourceId, nil
}

// MigrateNodeAuthorization migrates node authorization from V1 to V2 format
func MigrateNodeAuthorization() error {
	logger.L().Info("Starting node authorization V1 to V2 migration")

	// Get all nodes that need migration
	var nodes []*model.Node
	if err := dbpkg.DB.Find(&nodes).Error; err != nil {
		return fmt.Errorf("failed to fetch nodes: %w", err)
	}

	logger.L().Info("Found nodes for migration", zap.Int("count", len(nodes)))

	migratedCount := 0
	for _, node := range nodes {
		if len(node.Authorization) == 0 {
			continue // Skip nodes without authorization
		}

		// Check if node needs migration (has V1 format)
		needsMigration := false
		for _, authData := range node.Authorization {
			if authData.Permissions == nil {
				needsMigration = true
				break
			}
		}

		if !needsMigration {
			continue // Skip nodes that are already in V2 format
		}

		ctx := context.Background()
		if err := migrateNodeAuthorizationField(ctx, dbpkg.DB, node.Id); err != nil {
			logger.L().Error("Failed to migrate node authorization",
				zap.Int("nodeId", node.Id),
				zap.String("nodeName", node.Name),
				zap.Error(err))
			continue // Continue with other nodes
		}

		migratedCount++
		logger.L().Info("Successfully migrated node authorization",
			zap.Int("nodeId", node.Id),
			zap.String("nodeName", node.Name))
	}

	logger.L().Info("Node authorization migration completed",
		zap.Int("migratedCount", migratedCount),
		zap.Int("totalNodes", len(nodes)))

	return nil
}

// migrateNodeAuthorizationField migrates a specific node's authorization field from V1 to V2
func migrateNodeAuthorizationField(ctx context.Context, tx *gorm.DB, nodeId int) error {
	// Get current node data
	var node model.Node
	if err := tx.Where("id = ?", nodeId).First(&node).Error; err != nil {
		return fmt.Errorf("failed to get node: %w", err)
	}

	if len(node.Authorization) == 0 {
		return nil // No authorization to migrate
	}

	// Convert V1 authorization to V2 format and create actual V2 rules
	defaultPermissions := getDefaultAuthPermissions()

	for accountId, authData := range node.Authorization {
		// Skip if already V2 format
		if authData.Permissions != nil {
			continue
		}

		// Create V2 authorization rule
		rule := &model.AuthorizationV2{
			Name:        fmt.Sprintf("Node-%d-Account-%d", nodeId, accountId),
			Description: fmt.Sprintf("Migrated rule for node %s and account %d", node.Name, accountId),
			Enabled:     true,

			// Target selectors - specific node and account
			NodeSelector: model.TargetSelector{
				Type:       model.SelectorTypeIds,
				Values:     []string{fmt.Sprintf("%d", nodeId)},
				ExcludeIds: []int{},
			},
			AccountSelector: model.TargetSelector{
				Type:       model.SelectorTypeIds,
				Values:     []string{fmt.Sprintf("%d", accountId)},
				ExcludeIds: []int{},
			},

			// Use default permissions for migration
			Permissions: defaultPermissions,

			// Use existing role IDs
			Rids: authData.Rids,

			// Standard fields
			CreatorId: 1, // System migration
			UpdaterId: 1, // System migration
		}

		// Create placeholder ACL resource for migration
		resourceId, err := createBasicACLResourceForMigration(rule.Name)
		if err != nil {
			logger.L().Error("Failed to create ACL resource during migration",
				zap.String("ruleName", rule.Name),
				zap.Error(err))
			continue
		}
		rule.ResourceId = resourceId

		// Create the V2 rule
		if err := tx.Create(rule).Error; err != nil {
			logger.L().Error("Failed to create V2 authorization rule during migration",
				zap.String("ruleName", rule.Name),
				zap.Error(err))
			continue
		}

		// Update the node.Authorization field with V2 format including RuleId
		v2AuthData := model.AccountAuthorization{
			Rids:        authData.Rids,
			Permissions: &defaultPermissions,
			RuleId:      rule.Id, // Important: Link to the created rule
		}
		node.Authorization[accountId] = v2AuthData
	}

	// Update the node's authorization field in database
	if err := tx.Model(&node).Update("authorization", node.Authorization).Error; err != nil {
		return fmt.Errorf("failed to update node authorization: %w", err)
	}

	return nil
}
