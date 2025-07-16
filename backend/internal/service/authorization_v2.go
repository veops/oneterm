package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/veops/oneterm/internal/acl"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/repository"
	"github.com/veops/oneterm/pkg/config"
	dbpkg "github.com/veops/oneterm/pkg/db"
	"github.com/veops/oneterm/pkg/logger"
)

// AuthorizationV2Service handles business logic for authorization V2
type AuthorizationV2Service struct {
	repo                repository.IAuthorizationV2Repository
	timeTemplateService *TimeTemplateService
	matcher             IAuthorizationMatcher
}

// NewAuthorizationV2Service creates a new authorization V2 service
func NewAuthorizationV2Service() *AuthorizationV2Service {
	repo := repository.NewAuthorizationV2Repository(dbpkg.DB)
	return &AuthorizationV2Service{
		repo:                repo,
		timeTemplateService: NewTimeTemplateService(),
		matcher:             NewAuthorizationMatcher(repo),
	}
}

// BuildQuery builds the base query for authorization V2 rules
func (s *AuthorizationV2Service) BuildQuery(ctx context.Context) (*gorm.DB, error) {
	db := dbpkg.DB.Model(model.DefaultAuthorizationV2)

	// Add any additional filters here if needed
	// For example, filtering by enabled status, etc.

	return db, nil
}

// ValidateRule validates an authorization rule
func (s *AuthorizationV2Service) ValidateRule(ctx context.Context, rule *model.AuthorizationV2) error {
	// Validate selector types
	if !s.isValidSelectorType(rule.NodeSelector.Type) {
		return errors.New("invalid node selector type")
	}
	if !s.isValidSelectorType(rule.AssetSelector.Type) {
		return errors.New("invalid asset selector type")
	}
	if !s.isValidSelectorType(rule.AccountSelector.Type) {
		return errors.New("invalid account selector type")
	}
	// Note: UserSelector is handled via Rids field for ACL integration

	// Validate regex patterns if type is regex
	if rule.NodeSelector.Type == model.SelectorTypeRegex {
		if err := s.validateRegexPatterns(rule.NodeSelector.Values); err != nil {
			return fmt.Errorf("invalid node selector regex: %w", err)
		}
	}
	if rule.AssetSelector.Type == model.SelectorTypeRegex {
		if err := s.validateRegexPatterns(rule.AssetSelector.Values); err != nil {
			return fmt.Errorf("invalid asset selector regex: %w", err)
		}
	}
	if rule.AccountSelector.Type == model.SelectorTypeRegex {
		if err := s.validateRegexPatterns(rule.AccountSelector.Values); err != nil {
			return fmt.Errorf("invalid account selector regex: %w", err)
		}
	}
	// Note: User selection is handled via Rids field, no regex validation needed

	// Validate time template reference if present
	if rule.AccessControl.TimeTemplate != nil {
		if err := s.validateTimeTemplateReference(ctx, rule.AccessControl.TimeTemplate); err != nil {
			return fmt.Errorf("invalid time template reference: %w", err)
		}
	}

	// Validate custom time ranges if present
	if len(rule.AccessControl.CustomTimeRanges) > 0 {
		if err := s.validateTimeRanges(rule.AccessControl.CustomTimeRanges); err != nil {
			return fmt.Errorf("invalid custom time ranges: %w", err)
		}
	}

	return nil
}

// validateTimeTemplateReference validates a time template reference
func (s *AuthorizationV2Service) validateTimeTemplateReference(ctx context.Context, ref *model.TimeTemplateReference) error {
	if ref.TemplateId <= 0 {
		return errors.New("template_id must be positive")
	}

	// Check if template exists
	template, err := s.timeTemplateService.GetTimeTemplate(ctx, ref.TemplateId)
	if err != nil {
		return fmt.Errorf("failed to get time template: %w", err)
	}
	if template == nil {
		return errors.New("time template not found")
	}

	// Validate custom ranges if present
	if len(ref.CustomRanges) > 0 {
		if err := s.validateTimeRanges(ref.CustomRanges); err != nil {
			return fmt.Errorf("invalid custom ranges in template reference: %w", err)
		}
	}

	return nil
}

// validateTimeRanges validates time ranges
func (s *AuthorizationV2Service) validateTimeRanges(ranges model.TimeRanges) error {
	for i, timeRange := range ranges {
		if err := s.validateTimeRange(timeRange); err != nil {
			return fmt.Errorf("invalid time range %d: %w", i+1, err)
		}
	}
	return nil
}

// validateTimeRange validates a single time range
func (s *AuthorizationV2Service) validateTimeRange(timeRange model.TimeRange) error {
	// Validate time format (HH:MM)
	timePattern := regexp.MustCompile(`^([0-1]?[0-9]|2[0-3]):[0-5][0-9]$`)

	if !timePattern.MatchString(timeRange.StartTime) {
		return fmt.Errorf("invalid start time format: %s (expected HH:MM)", timeRange.StartTime)
	}

	if !timePattern.MatchString(timeRange.EndTime) {
		return fmt.Errorf("invalid end time format: %s (expected HH:MM)", timeRange.EndTime)
	}

	// Parse and validate time logic
	startMinutes, err := s.parseTimeToMinutes(timeRange.StartTime)
	if err != nil {
		return fmt.Errorf("invalid start time: %w", err)
	}

	endMinutes, err := s.parseTimeToMinutes(timeRange.EndTime)
	if err != nil {
		return fmt.Errorf("invalid end time: %w", err)
	}

	if startMinutes >= endMinutes {
		return errors.New("start time must be before end time")
	}

	// Validate weekdays
	if len(timeRange.Weekdays) == 0 {
		return errors.New("at least one weekday must be specified")
	}

	for _, day := range timeRange.Weekdays {
		if day < 1 || day > 7 {
			return fmt.Errorf("invalid weekday: %d (must be 1-7, where 1=Monday, 7=Sunday)", day)
		}
	}

	return nil
}

// parseTimeToMinutes converts HH:MM format to minutes since midnight
func (s *AuthorizationV2Service) parseTimeToMinutes(timeStr string) (int, error) {
	parts := strings.Split(timeStr, ":")
	if len(parts) != 2 {
		return 0, errors.New("invalid time format")
	}

	hour, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, err
	}

	minute, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, err
	}

	return hour*60 + minute, nil
}

// CheckTimeAccess checks if current time allows access according to the access control
func (s *AuthorizationV2Service) CheckTimeAccess(ctx context.Context, accessControl *model.AccessControl, timezone string) (bool, error) {
	// If no time restrictions are configured, allow access
	if accessControl.TimeTemplate == nil && len(accessControl.CustomTimeRanges) == 0 {
		return true, nil
	}

	// Check time template if configured
	if accessControl.TimeTemplate != nil {
		// Get the template
		template, err := s.timeTemplateService.GetTimeTemplate(ctx, accessControl.TimeTemplate.TemplateId)
		if err != nil {
			return false, fmt.Errorf("failed to get time template: %w", err)
		}
		if template == nil {
			return false, errors.New("time template not found")
		}

		// Use template's timezone if not specified in access control
		templateTimezone := timezone
		if templateTimezone == "" {
			templateTimezone = accessControl.Timezone
		}
		if templateTimezone == "" {
			templateTimezone = template.Timezone
		}

		// Check if current time is within template ranges
		if s.timeTemplateService.IsTimeInTemplate(template, templateTimezone) {
			return true, nil
		}

		// Check custom ranges in the template reference
		if len(accessControl.TimeTemplate.CustomRanges) > 0 {
			if s.isTimeInRanges(accessControl.TimeTemplate.CustomRanges, templateTimezone) {
				return true, nil
			}
		}
	}

	// Check custom time ranges if configured
	if len(accessControl.CustomTimeRanges) > 0 {
		checkTimezone := timezone
		if checkTimezone == "" {
			checkTimezone = accessControl.Timezone
		}
		if checkTimezone == "" {
			checkTimezone = "Asia/Shanghai" // Default timezone
		}

		if s.isTimeInRanges(accessControl.CustomTimeRanges, checkTimezone) {
			return true, nil
		}
	}

	return false, nil
}

// isTimeInRanges checks if current time is within any of the specified time ranges
func (s *AuthorizationV2Service) isTimeInRanges(ranges model.TimeRanges, timezone string) bool {
	// Load timezone location
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		// Fall back to UTC if timezone is invalid
		loc = time.UTC
	}

	now := time.Now().In(loc)
	currentWeekday := int(now.Weekday())
	if currentWeekday == 0 {
		currentWeekday = 7 // Convert Sunday from 0 to 7
	}

	currentMinutes := now.Hour()*60 + now.Minute()

	// Check each time range
	for _, timeRange := range ranges {
		// Check if current weekday is in the allowed weekdays
		weekdayMatch := false
		for _, day := range timeRange.Weekdays {
			if day == currentWeekday {
				weekdayMatch = true
				break
			}
		}
		if !weekdayMatch {
			continue
		}

		// Check if current time is in the allowed time range
		startMinutes, err := s.parseTimeToMinutes(timeRange.StartTime)
		if err != nil {
			continue
		}

		endMinutes, err := s.parseTimeToMinutes(timeRange.EndTime)
		if err != nil {
			continue
		}

		if currentMinutes >= startMinutes && currentMinutes <= endMinutes {
			return true
		}
	}

	return false
}

// isValidSelectorType checks if selector type is valid
func (s *AuthorizationV2Service) isValidSelectorType(selectorType model.SelectorType) bool {
	switch selectorType {
	case model.SelectorTypeAll, model.SelectorTypeIds, model.SelectorTypeRegex, model.SelectorTypeTags:
		return true
	default:
		return false
	}
}

// validateRegexPatterns validates regex patterns
func (s *AuthorizationV2Service) validateRegexPatterns(patterns []string) error {
	for _, pattern := range patterns {
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("invalid regex pattern '%s': %w", pattern, err)
		}
	}
	return nil
}

// GetAuthorizedAssetIds returns asset IDs that the user has permission to access using V2 authorization
func (s *AuthorizationV2Service) GetAuthorizedAssetIds(ctx *gin.Context, action model.AuthAction) ([]int, error) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	// Administrators have access to all assets
	if acl.IsAdmin(currentUser) {
		assets, err := repository.GetAllFromCacheDb(ctx, model.DefaultAsset)
		if err != nil {
			return nil, err
		}
		return lo.Map(assets, func(a *model.Asset, _ int) int { return a.Id }), nil
	}

	// Get all assets from cache
	assets, err := repository.GetAllFromCacheDb(ctx, model.DefaultAsset)
	if err != nil {
		return nil, err
	}

	// Check permission for each asset
	authorizedAssetIds := make([]int, 0)
	for _, asset := range assets {
		// Create authorization request
		req := &model.AuthRequest{
			UserId:    currentUser.GetUid(),
			NodeId:    asset.ParentId,
			AssetId:   asset.Id,
			AccountId: 0, // Check asset-level permission (any account)
			Action:    action,
			ClientIP:  s.getClientIP(ctx),
			Timestamp: time.Now(),
		}

		// Use V2 matcher
		result, err := s.matcher.Match(ctx, req)
		if err != nil {
			logger.L().Error("Failed to check asset permission", zap.Int("assetId", asset.Id), zap.Error(err))
			continue
		}

		if result.Allowed {
			authorizedAssetIds = append(authorizedAssetIds, asset.Id)
		}
	}

	return authorizedAssetIds, nil
}

// GetAuthorizedAccountIds returns account IDs that the user has permission to access for given assets
func (s *AuthorizationV2Service) GetAuthorizedAccountIds(ctx *gin.Context, assetIds []int, action model.AuthAction) ([]int, error) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	// Administrators have access to all accounts
	if acl.IsAdmin(currentUser) {
		accounts, err := repository.GetAllFromCacheDb(ctx, model.DefaultAccount)
		if err != nil {
			return nil, err
		}
		return lo.Map(accounts, func(a *model.Account, _ int) int { return a.Id }), nil
	}

	// Get all accounts from cache
	accounts, err := repository.GetAllFromCacheDb(ctx, model.DefaultAccount)
	if err != nil {
		return nil, err
	}

	// Get assets for the given asset IDs
	assets, err := repository.GetAllFromCacheDb(ctx, model.DefaultAsset)
	if err != nil {
		return nil, err
	}
	assetMap := lo.SliceToMap(assets, func(a *model.Asset) (int, *model.Asset) { return a.Id, a })

	authorizedAccountIds := make([]int, 0)

	// Check permission for each account against each asset
	for _, account := range accounts {
		hasPermission := false
		for _, assetId := range assetIds {
			asset, exists := assetMap[assetId]
			if !exists {
				continue
			}

			// Create authorization request
			req := &model.AuthRequest{
				UserId:    currentUser.GetUid(),
				NodeId:    asset.ParentId,
				AssetId:   assetId,
				AccountId: account.Id,
				Action:    action,
				ClientIP:  s.getClientIP(ctx),
				Timestamp: time.Now(),
			}

			// Use V2 matcher
			result, err := s.matcher.Match(ctx, req)
			if err != nil {
				logger.L().Error("Failed to check account permission",
					zap.Int("assetId", assetId),
					zap.Int("accountId", account.Id),
					zap.Error(err))
				continue
			}

			if result.Allowed {
				hasPermission = true
				break
			}
		}

		if hasPermission {
			authorizedAccountIds = append(authorizedAccountIds, account.Id)
		}
	}

	return lo.Uniq(authorizedAccountIds), nil
}

// GetAuthorizedNodeIds returns node IDs that the user has permission to access
func (s *AuthorizationV2Service) GetAuthorizedNodeIds(ctx *gin.Context, action model.AuthAction) ([]int, error) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	// Administrators have access to all nodes
	if acl.IsAdmin(currentUser) {
		nodes, err := repository.GetAllFromCacheDb(ctx, model.DefaultNode)
		if err != nil {
			return nil, err
		}
		return lo.Map(nodes, func(n *model.Node, _ int) int { return n.Id }), nil
	}

	// Get all nodes from cache
	nodes, err := repository.GetAllFromCacheDb(ctx, model.DefaultNode)
	if err != nil {
		return nil, err
	}

	authorizedNodeIds := make([]int, 0)
	for _, node := range nodes {
		// Create authorization request
		req := &model.AuthRequest{
			UserId:    currentUser.GetUid(),
			NodeId:    node.Id,
			AssetId:   0, // Check node-level permission (any asset)
			AccountId: 0, // Check node-level permission (any account)
			Action:    action,
			ClientIP:  s.getClientIP(ctx),
			Timestamp: time.Now(),
		}

		// Use V2 matcher
		result, err := s.matcher.Match(ctx, req)
		if err != nil {
			logger.L().Error("Failed to check node permission", zap.Int("nodeId", node.Id), zap.Error(err))
			continue
		}

		if result.Allowed {
			authorizedNodeIds = append(authorizedNodeIds, node.Id)
		}
	}

	return authorizedNodeIds, nil
}

// ApplyAssetPermissionFilter filters assets based on V2 authorization
func (s *AuthorizationV2Service) ApplyAssetPermissionFilter(ctx *gin.Context, assets []*model.Asset, action model.AuthAction) []*model.Asset {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	// Administrators have access to all assets
	if acl.IsAdmin(currentUser) {
		return assets
	}

	filteredAssets := make([]*model.Asset, 0)
	for _, asset := range assets {
		// Create authorization request
		req := &model.AuthRequest{
			UserId:    currentUser.GetUid(),
			NodeId:    asset.ParentId,
			AssetId:   asset.Id,
			AccountId: 0, // Check asset-level permission
			Action:    action,
			ClientIP:  s.getClientIP(ctx),
			Timestamp: time.Now(),
		}

		// Use V2 matcher
		result, err := s.matcher.Match(ctx, req)
		if err != nil {
			logger.L().Error("Failed to check asset permission", zap.Int("assetId", asset.Id), zap.Error(err))
			continue
		}

		if result.Allowed {
			filteredAssets = append(filteredAssets, asset)
		}
	}

	return filteredAssets
}

// getClientIP extracts client IP from gin context
func (s *AuthorizationV2Service) getClientIP(ctx *gin.Context) string {
	// Try to get real IP from headers first
	clientIP := ctx.GetHeader("X-Forwarded-For")
	if clientIP == "" {
		clientIP = ctx.GetHeader("X-Real-IP")
	}
	if clientIP == "" {
		clientIP = ctx.ClientIP()
	}

	// Parse and validate IP
	if ip := net.ParseIP(clientIP); ip != nil {
		return clientIP
	}

	return ""
}

// GetAssetPermissions returns all permissions for a user on a specific asset
func (s *AuthorizationV2Service) GetAssetPermissions(ctx *gin.Context, assetId int, accountId int) (*model.BatchAuthResult, error) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	// Helper function to create batch result for all actions
	allActions := []model.AuthAction{
		model.ActionConnect,
		model.ActionFileUpload,
		model.ActionFileDownload,
		model.ActionCopy,
		model.ActionPaste,
		model.ActionShare,
	}

	createBatchResult := func(allowed bool, reason string, permissions *model.AuthPermissions) *model.BatchAuthResult {
		results := make(map[model.AuthAction]*model.AuthResult)
		for _, action := range allActions {
			result := &model.AuthResult{
				Allowed: allowed,
				Reason:  reason,
			}
			// Only set permissions if they are provided
			if permissions != nil {
				result.Permissions = *permissions
			}
			results[action] = result
		}
		return &model.BatchAuthResult{
			Results: results,
		}
	}

	// Administrators have access to all resources
	if acl.IsAdmin(currentUser) {
		// For administrators, permissions reflect their actual capabilities
		adminPermissions := &model.AuthPermissions{
			Connect:      true,
			FileUpload:   true,
			FileDownload: true,
			Copy:         true,
			Paste:        true,
			Share:        true,
		}
		return createBatchResult(true, "Administrator access", adminPermissions), nil
	}

	// Get the asset information
	asset, err := s.getAssetById(ctx, assetId)
	if err != nil {
		return createBatchResult(false, "Asset not found", nil), err
	}

	// Get user's authorized V2 rule IDs from ACL
	authV2ResourceIds, err := s.getAuthorizedV2ResourceIds(ctx)
	if err != nil {
		return createBatchResult(false, "Failed to get authorized rules", nil), err
	}

	if len(authV2ResourceIds) == 0 {
		return createBatchResult(false, "No authorization rules available", nil), nil
	}

	// Create batch authorization request for all actions
	clientIP := s.getClientIP(ctx)
	batchReq := &model.BatchAuthRequest{
		UserId:    currentUser.GetUid(),
		NodeId:    asset.ParentId,
		AssetId:   assetId,
		AccountId: accountId,
		Actions:   allActions,
		ClientIP:  clientIP,
		Timestamp: time.Now(),
	}

	// Use V2 matcher with filtered rule scope
	return s.matcher.MatchBatchWithScope(ctx, batchReq, authV2ResourceIds)
}

// getAssetById retrieves asset by ID from cache
func (s *AuthorizationV2Service) getAssetById(ctx context.Context, assetId int) (*model.Asset, error) {
	assets, err := repository.GetAllFromCacheDb(ctx, model.DefaultAsset)
	if err != nil {
		return nil, err
	}

	for _, asset := range assets {
		if asset.Id == assetId {
			return asset, nil
		}
	}

	return nil, fmt.Errorf("asset not found: %d", assetId)
}

// getAuthorizedV2ResourceIds gets V2 authorization rule resource IDs that user has permission to
func (s *AuthorizationV2Service) getAuthorizedV2ResourceIds(ctx *gin.Context) ([]int, error) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	// Get ACL resources for authorization_v2 that this user's role has access to
	res, err := acl.GetRoleResources(ctx, currentUser.GetRid(), config.RESOURCE_AUTHORIZATION)
	if err != nil {
		return nil, err
	}

	// Extract resource IDs (these are the V2 rule IDs user can access)
	resourceIds := lo.Map(res, func(r *acl.Resource, _ int) int { return r.ResourceId })
	return resourceIds, nil
}

// GetAuthorizationScopeByACL efficiently gets authorization scope using ACL + V2 rules
func (s *AuthorizationV2Service) GetAuthorizationScopeByACL(ctx *gin.Context) (nodeIds []int, assetIds []int, accountIds []int, err error) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	// Administrators have access to all resources
	if acl.IsAdmin(currentUser) {
		nodes, err := repository.GetAllFromCacheDb(ctx, model.DefaultNode)
		if err != nil {
			return nil, nil, nil, err
		}
		nodeIds = lo.Map(nodes, func(n *model.Node, _ int) int { return n.Id })

		assets, err := repository.GetAllFromCacheDb(ctx, model.DefaultAsset)
		if err != nil {
			return nil, nil, nil, err
		}
		assetIds = lo.Map(assets, func(a *model.Asset, _ int) int { return a.Id })

		accounts, err := repository.GetAllFromCacheDb(ctx, model.DefaultAccount)
		if err != nil {
			return nil, nil, nil, err
		}
		accountIds = lo.Map(accounts, func(a *model.Account, _ int) int { return a.Id })

		return nodeIds, assetIds, accountIds, nil
	}

	// Get authorized resource IDs from ACL (this handles role inheritance)
	authResourceIds, err := acl.GetRoleResourceIds(ctx, currentUser.GetRid(), config.RESOURCE_AUTHORIZATION)
	if err != nil {
		return nil, nil, nil, err
	}

	// No authorized resources means no access
	if len(authResourceIds) == 0 {
		return []int{}, []int{}, []int{}, nil
	}

	// Query V2 rules by resource IDs (much more efficient than querying all rules)
	rules, err := s.repo.GetByResourceIds(ctx, authResourceIds)
	if err != nil {
		return nil, nil, nil, err
	}

	// Filter enabled rules
	enabledRules := lo.Filter(rules, func(rule *model.AuthorizationV2, _ int) bool {
		return rule.Enabled
	})

	// Extract IDs from the user's authorized rules
	nodeIds = s.extractNodeIdsEfficient(ctx, enabledRules)
	assetIds = s.extractAssetIdsEfficient(ctx, enabledRules)
	accountIds = s.extractAccountIdsEfficient(ctx, enabledRules)

	return lo.Uniq(nodeIds), lo.Uniq(assetIds), lo.Uniq(accountIds), nil
}

// extractNodeIdsEfficient efficiently extracts node IDs from rules
func (s *AuthorizationV2Service) extractNodeIdsEfficient(ctx context.Context, rules []*model.AuthorizationV2) []int {
	var nodeIds []int
	nodeCache := make(map[int]*model.Node) // Cache to avoid repeated queries

	for _, rule := range rules {
		switch rule.NodeSelector.Type {
		case model.SelectorTypeAll:
			// Add all node IDs
			if len(nodeCache) == 0 {
				nodes, err := repository.GetAllFromCacheDb(ctx, model.DefaultNode)
				if err != nil {
					logger.L().Error("Failed to get nodes from cache", zap.Error(err))
					continue
				}
				for _, node := range nodes {
					nodeCache[node.Id] = node
				}
			}
			for nodeId := range nodeCache {
				nodeIds = append(nodeIds, nodeId)
			}

		case model.SelectorTypeIds:
			// Add specific node IDs
			for _, value := range rule.NodeSelector.Values {
				if id, err := strconv.Atoi(value); err == nil {
					nodeIds = append(nodeIds, id)
				}
			}

		case model.SelectorTypeRegex:
			// Match nodes by regex patterns (load cache if needed)
			if len(nodeCache) == 0 {
				nodes, err := repository.GetAllFromCacheDb(ctx, model.DefaultNode)
				if err != nil {
					logger.L().Error("Failed to get nodes from cache", zap.Error(err))
					continue
				}
				for _, node := range nodes {
					nodeCache[node.Id] = node
				}
			}
			for _, node := range nodeCache {
				if s.matchRegexPatterns(node.Name, rule.NodeSelector.Values) {
					nodeIds = append(nodeIds, node.Id)
				}
			}

		case model.SelectorTypeTags:
			// Tags selector not supported for nodes (no tags field)
			logger.L().Warn("Tags selector not supported for nodes")
		}
	}

	return nodeIds
}

// extractAssetIdsEfficient efficiently extracts asset IDs from rules
func (s *AuthorizationV2Service) extractAssetIdsEfficient(ctx context.Context, rules []*model.AuthorizationV2) []int {
	var assetIds []int
	assetCache := make(map[int]*model.Asset) // Cache to avoid repeated queries

	for _, rule := range rules {
		switch rule.AssetSelector.Type {
		case model.SelectorTypeAll:
			// Add all asset IDs
			if len(assetCache) == 0 {
				assets, err := repository.GetAllFromCacheDb(ctx, model.DefaultAsset)
				if err != nil {
					logger.L().Error("Failed to get assets from cache", zap.Error(err))
					continue
				}
				for _, asset := range assets {
					assetCache[asset.Id] = asset
				}
			}
			for assetId := range assetCache {
				assetIds = append(assetIds, assetId)
			}

		case model.SelectorTypeIds:
			// Add specific asset IDs
			for _, value := range rule.AssetSelector.Values {
				if id, err := strconv.Atoi(value); err == nil {
					assetIds = append(assetIds, id)
				}
			}

		case model.SelectorTypeRegex:
			// Match assets by regex patterns (load cache if needed)
			if len(assetCache) == 0 {
				assets, err := repository.GetAllFromCacheDb(ctx, model.DefaultAsset)
				if err != nil {
					logger.L().Error("Failed to get assets from cache", zap.Error(err))
					continue
				}
				for _, asset := range assets {
					assetCache[asset.Id] = asset
				}
			}
			for _, asset := range assetCache {
				if s.matchRegexPatterns(asset.Name, rule.AssetSelector.Values) || s.matchRegexPatterns(asset.Ip, rule.AssetSelector.Values) {
					assetIds = append(assetIds, asset.Id)
				}
			}

		case model.SelectorTypeTags:
			// Tags selector not supported for assets (no tags field)
			logger.L().Warn("Tags selector not supported for assets")
		}
	}

	return assetIds
}

// extractAccountIdsEfficient efficiently extracts account IDs from rules
func (s *AuthorizationV2Service) extractAccountIdsEfficient(ctx context.Context, rules []*model.AuthorizationV2) []int {
	var accountIds []int
	accountCache := make(map[int]*model.Account) // Cache to avoid repeated queries

	for _, rule := range rules {
		switch rule.AccountSelector.Type {
		case model.SelectorTypeAll:
			// Add all account IDs
			if len(accountCache) == 0 {
				accounts, err := repository.GetAllFromCacheDb(ctx, model.DefaultAccount)
				if err != nil {
					logger.L().Error("Failed to get accounts from cache", zap.Error(err))
					continue
				}
				for _, account := range accounts {
					accountCache[account.Id] = account
				}
			}
			for accountId := range accountCache {
				accountIds = append(accountIds, accountId)
			}

		case model.SelectorTypeIds:
			// Add specific account IDs
			for _, value := range rule.AccountSelector.Values {
				if id, err := strconv.Atoi(value); err == nil {
					accountIds = append(accountIds, id)
				}
			}

		case model.SelectorTypeRegex:
			// Match accounts by regex patterns (load cache if needed)
			if len(accountCache) == 0 {
				accounts, err := repository.GetAllFromCacheDb(ctx, model.DefaultAccount)
				if err != nil {
					logger.L().Error("Failed to get accounts from cache", zap.Error(err))
					continue
				}
				for _, account := range accounts {
					accountCache[account.Id] = account
				}
			}
			for _, account := range accountCache {
				if s.matchRegexPatterns(account.Name, rule.AccountSelector.Values) || s.matchRegexPatterns(account.Account, rule.AccountSelector.Values) {
					accountIds = append(accountIds, account.Id)
				}
			}

		case model.SelectorTypeTags:
			// Tags selector not supported for accounts (no tags field)
			logger.L().Warn("Tags selector not supported for accounts")
		}
	}

	return accountIds
}

// matchRegexPatterns checks if a string matches any of the regex patterns
func (s *AuthorizationV2Service) matchRegexPatterns(str string, patterns []string) bool {
	for _, pattern := range patterns {
		if matched, err := regexp.MatchString(pattern, str); err == nil && matched {
			return true
		}
	}
	return false
}

// CloneRule clones an existing authorization rule with ACL handling
func (s *AuthorizationV2Service) CloneRule(ctx context.Context, sourceId int, newName string) (*model.AuthorizationV2, error) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	if currentUser == nil {
		return nil, errors.New("user not found in context")
	}

	// Get the source rule
	sourceRule, err := s.repo.GetById(ctx, sourceId)
	if err != nil {
		return nil, fmt.Errorf("failed to get source rule: %w", err)
	}
	if sourceRule == nil {
		return nil, errors.New("source rule not found")
	}

	// Create a copy of the rule
	clonedRule := &model.AuthorizationV2{
		Name:        newName,
		Description: fmt.Sprintf("Clone of: %s", sourceRule.Description),
		Enabled:     false, // Start disabled by default
		ValidFrom:   sourceRule.ValidFrom,
		ValidTo:     sourceRule.ValidTo,

		// Copy selectors
		NodeSelector:    sourceRule.NodeSelector,
		AssetSelector:   sourceRule.AssetSelector,
		AccountSelector: sourceRule.AccountSelector,

		// Copy permissions and access control
		Permissions:   sourceRule.Permissions,
		AccessControl: sourceRule.AccessControl,

		// Copy role IDs
		Rids: sourceRule.Rids,

		// Set metadata for new rule
		CreatorId: currentUser.GetUid(),
		UpdaterId: currentUser.GetUid(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Validate the cloned rule
	if err := s.ValidateRule(ctx, clonedRule); err != nil {
		return nil, fmt.Errorf("cloned rule validation failed: %w", err)
	}

	// Use transaction to ensure consistency
	var result *model.AuthorizationV2
	err = dbpkg.DB.Transaction(func(tx *gorm.DB) error {
		// Create ACL resource for the cloned rule
		resourceId, err := acl.CreateAcl(ctx, currentUser, config.RESOURCE_AUTHORIZATION, clonedRule.Name)
		if err != nil {
			return fmt.Errorf("failed to create ACL resource: %w", err)
		}
		clonedRule.ResourceId = resourceId

		// Create the cloned rule in database
		if err := tx.Create(clonedRule).Error; err != nil {
			return fmt.Errorf("failed to create cloned rule: %w", err)
		}

		// Grant permissions to roles if specified
		if len(clonedRule.Rids) > 0 {
			if err := acl.BatchGrantRoleResource(ctx, currentUser.GetUid(), clonedRule.Rids, resourceId, []string{acl.READ}); err != nil {
				return fmt.Errorf("failed to grant role permissions: %w", err)
			}
		}

		result = clonedRule
		return nil
	})

	if err != nil {
		// Clean up ACL resource if transaction failed
		if clonedRule.ResourceId > 0 {
			acl.DeleteResource(ctx, currentUser.GetUid(), clonedRule.ResourceId)
		}
		return nil, err
	}

	logger.L().Info("Authorization rule cloned successfully",
		zap.Int("source_id", sourceId),
		zap.Int("cloned_id", result.Id),
		zap.String("cloned_name", newName),
		zap.Int("user_id", currentUser.GetUid()))

	return result, nil
}

// CreateRule creates a new authorization rule with ACL handling
func (s *AuthorizationV2Service) CreateRule(ctx context.Context, rule *model.AuthorizationV2) error {
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	if currentUser == nil {
		return errors.New("user not found in context")
	}

	// Validate the rule
	if err := s.ValidateRule(ctx, rule); err != nil {
		return fmt.Errorf("rule validation failed: %w", err)
	}

	// Set metadata
	rule.CreatorId = currentUser.GetUid()
	rule.UpdaterId = currentUser.GetUid()
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()

	// Use transaction to ensure consistency
	return dbpkg.DB.Transaction(func(tx *gorm.DB) error {
		// Create ACL resource
		resourceId, err := acl.CreateAcl(ctx, currentUser, config.RESOURCE_AUTHORIZATION, rule.Name)
		if err != nil {
			return fmt.Errorf("failed to create ACL resource: %w", err)
		}
		rule.ResourceId = resourceId

		// Create the rule in database
		if err := tx.Create(rule).Error; err != nil {
			return fmt.Errorf("failed to create rule: %w", err)
		}

		// Grant permissions to roles if specified
		if len(rule.Rids) > 0 {
			if err := acl.BatchGrantRoleResource(ctx, currentUser.GetUid(), rule.Rids, resourceId, []string{acl.READ}); err != nil {
				return fmt.Errorf("failed to grant role permissions: %w", err)
			}
		}

		return nil
	})
}

// UpdateRule updates an existing authorization rule with ACL handling
func (s *AuthorizationV2Service) UpdateRule(ctx context.Context, rule *model.AuthorizationV2) error {
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	if currentUser == nil {
		return errors.New("user not found in context")
	}

	// Get existing rule for comparison
	existingRule, err := s.repo.GetById(ctx, rule.Id)
	if err != nil {
		return fmt.Errorf("failed to get existing rule: %w", err)
	}
	if existingRule == nil {
		return errors.New("rule not found")
	}

	// Validate the updated rule
	if err := s.ValidateRule(ctx, rule); err != nil {
		return fmt.Errorf("rule validation failed: %w", err)
	}

	// Preserve some fields
	rule.ResourceId = existingRule.ResourceId
	rule.CreatorId = existingRule.CreatorId
	rule.CreatedAt = existingRule.CreatedAt
	rule.UpdaterId = currentUser.GetUid()
	rule.UpdatedAt = time.Now()

	// Use transaction to ensure consistency
	return dbpkg.DB.Transaction(func(tx *gorm.DB) error {
		// Update role permissions if Rids changed
		if !reflect.DeepEqual(rule.Rids, existingRule.Rids) {
			// Revoke permissions from removed roles
			removedRids := lo.Without(existingRule.Rids, rule.Rids...)
			if len(removedRids) > 0 {
				if err := acl.BatchRevokeRoleResource(ctx, currentUser.GetUid(), removedRids, rule.ResourceId, []string{acl.READ}); err != nil {
					return fmt.Errorf("failed to revoke role permissions: %w", err)
				}
			}

			// Grant permissions to new roles
			newRids := lo.Without(rule.Rids, existingRule.Rids...)
			if len(newRids) > 0 {
				if err := acl.BatchGrantRoleResource(ctx, currentUser.GetUid(), newRids, rule.ResourceId, []string{acl.READ}); err != nil {
					return fmt.Errorf("failed to grant role permissions: %w", err)
				}
			}
		}

		// Update the rule in database
		if err := tx.Save(rule).Error; err != nil {
			return fmt.Errorf("failed to update rule: %w", err)
		}

		return nil
	})
}

// DeleteRule deletes an authorization rule with ACL cleanup
func (s *AuthorizationV2Service) DeleteRule(ctx context.Context, id int) error {
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	if currentUser == nil {
		return errors.New("user not found in context")
	}

	// Get the rule to delete
	rule, err := s.repo.GetById(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get rule: %w", err)
	}
	if rule == nil {
		return errors.New("rule not found")
	}

	// Use transaction to ensure consistency
	return dbpkg.DB.Transaction(func(tx *gorm.DB) error {
		// Delete ACL resource
		if err := acl.DeleteResource(ctx, currentUser.GetUid(), rule.ResourceId); err != nil {
			return fmt.Errorf("failed to delete ACL resource: %w", err)
		}

		// Delete the rule from database
		if err := tx.Delete(rule).Error; err != nil {
			return fmt.Errorf("failed to delete rule: %w", err)
		}

		return nil
	})
}

// GetRuleById retrieves a rule by ID
func (s *AuthorizationV2Service) GetRuleById(ctx context.Context, id int) (*model.AuthorizationV2, error) {
	return s.repo.GetById(ctx, id)
}

// generateCloneName generates a unique name for a cloned rule
func (s *AuthorizationV2Service) generateCloneName(ctx context.Context, baseName string) (string, error) {
	// Try "Copy of {baseName}" first
	candidateName := fmt.Sprintf("Copy of %s", baseName)

	// Check if this name exists
	if exists, err := s.checkNameExists(ctx, candidateName); err != nil {
		return "", err
	} else if !exists {
		return candidateName, nil
	}

	// Try "Copy of {baseName} (2)", "Copy of {baseName} (3)", etc.
	for i := 2; i <= 999; i++ {
		candidateName = fmt.Sprintf("Copy of %s (%d)", baseName, i)
		if exists, err := s.checkNameExists(ctx, candidateName); err != nil {
			return "", err
		} else if !exists {
			return candidateName, nil
		}
	}

	return "", errors.New("unable to generate unique clone name")
}

// checkNameExists checks if a rule name already exists
func (s *AuthorizationV2Service) checkNameExists(ctx context.Context, name string) (bool, error) {
	var count int64
	err := dbpkg.DB.Model(&model.AuthorizationV2{}).Where("name = ?", name).Count(&count).Error
	return count > 0, err
}
