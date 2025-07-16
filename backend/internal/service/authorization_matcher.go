package service

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/spf13/cast"

	"github.com/veops/oneterm/internal/acl"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/repository"
)

// IAuthorizationMatcher defines the interface for authorization matching
type IAuthorizationMatcher interface {
	// Match checks if a request is authorized using all available rules
	Match(ctx *gin.Context, req *model.AuthRequest) (*model.AuthResult, error)

	// MatchWithScope checks if a request is authorized using only specified rule IDs
	MatchWithScope(ctx *gin.Context, req *model.AuthRequest, ruleIds []int) (*model.AuthResult, error)

	// MatchBatchWithScope checks if batch requests are authorized using only specified rule IDs
	MatchBatchWithScope(ctx *gin.Context, req *model.BatchAuthRequest, ruleIds []int) (*model.BatchAuthResult, error)

	GetTargetName(targetType string, targetId int) (string, error)
	GetTargetTags(targetType string, targetId int) ([]string, error)
}

// AuthorizationMatcher implements IAuthorizationMatcher
type AuthorizationMatcher struct {
	repo repository.IAuthorizationV2Repository
}

// NewAuthorizationMatcher creates a new authorization matcher
func NewAuthorizationMatcher(repo repository.IAuthorizationV2Repository) IAuthorizationMatcher {
	return &AuthorizationMatcher{
		repo: repo,
	}
}

// Match performs authorization matching against rules
func (m *AuthorizationMatcher) Match(ctx *gin.Context, req *model.AuthRequest) (*model.AuthResult, error) {
	// Get cache key for this request
	cacheKey := m.getCacheKey(req)

	// Try to get result from cache first
	if cached := m.getCachedResult(cacheKey); cached != nil {
		return cached, nil
	}

	// Get user's role IDs from context (assuming it's passed through ctx)
	userRids := m.getUserRoleIds(ctx, req.UserId)

	// Get user's authorization rules
	rules, err := m.repo.GetUserRules(ctx, userRids)
	if err != nil {
		return nil, fmt.Errorf("failed to get user rules: %w", err)
	}

	// Check all matching rules - if any rule allows the action, grant permission
	var matchedRules []*model.AuthorizationV2
	for _, rule := range rules {
		if m.matchRule(ctx, rule, req) {
			matchedRules = append(matchedRules, rule)
			// If this rule allows the requested action, grant permission immediately
			if rule.Permissions.HasPermission(req.Action) {
				result := &model.AuthResult{
					Allowed:     true,
					Permissions: rule.Permissions,
					Reason:      fmt.Sprintf("Allowed by rule: %s", rule.Name),
					RuleId:      rule.Id,
					RuleName:    rule.Name,
				}

				// Cache the result
				m.cacheResult(cacheKey, result)
				return result, nil
			}
		}
	}

	// If we found matching rules but none allowed the action, deny with specific reason
	if len(matchedRules) > 0 {
		result := &model.AuthResult{
			Allowed: false,
			Reason:  fmt.Sprintf("Action '%s' denied by %d matching rule(s)", req.Action, len(matchedRules)),
		}
		m.cacheResult(cacheKey, result)
		return result, nil
	}

	// No matching rule found
	result := &model.AuthResult{
		Allowed: false,
		Reason:  "No matching authorization rule found",
	}

	m.cacheResult(cacheKey, result)
	return result, nil
}

// getUserRoleIds extracts user role IDs from context or fetches them
func (m *AuthorizationMatcher) getUserRoleIds(ctx context.Context, userId int) []int {
	// Try to get from gin context if available
	if ginCtx, ok := ctx.(*gin.Context); ok {
		if currentUser, err := acl.GetSessionFromCtx(ginCtx); err == nil {
			return []int{currentUser.GetRid()}
		}
	}

	// Fallback: try to get role from ACL service
	// This is a simplified approach - in production you might want to implement
	// a more robust user role resolution mechanism
	return []int{} // Return empty for now - should be implemented based on your ACL system
}

// matchRule checks if a rule matches the request
func (m *AuthorizationMatcher) matchRule(ctx context.Context, rule *model.AuthorizationV2, req *model.AuthRequest) bool {
	// First check if the rule is currently valid (enabled and within validity period)
	if !rule.IsValid(req.Timestamp) {
		return false
	}

	// Check node selector
	if !m.matchSelector(ctx, rule.NodeSelector, "node", req.NodeId) {
		return false
	}

	// Check asset selector
	if !m.matchSelector(ctx, rule.AssetSelector, "asset", req.AssetId) {
		return false
	}

	// Check account selector
	if !m.matchSelector(ctx, rule.AccountSelector, "account", req.AccountId) {
		return false
	}

	// Check access control restrictions
	if !m.checkAccessControl(rule.AccessControl, req) {
		return false
	}

	return true
}

// matchSelector checks if a target selector matches the given target
func (m *AuthorizationMatcher) matchSelector(ctx context.Context, selector model.TargetSelector, targetType string, targetId int) bool {
	// Handle zero ID based on selector type
	if targetId == 0 {
		return selector.Type == model.SelectorTypeAll
	}

	// Check if target is in exclude list
	if lo.Contains(selector.ExcludeIds, targetId) {
		return false
	}

	switch selector.Type {
	case model.SelectorTypeAll:
		return true

	case model.SelectorTypeIds:
		targetIds := lo.Map(selector.Values, func(v string, _ int) int {
			return cast.ToInt(v)
		})
		return lo.Contains(targetIds, targetId)

	case model.SelectorTypeRegex:
		targetName, err := m.GetTargetName(targetType, targetId)
		if err != nil {
			return false
		}
		return m.matchRegexPatterns(selector.Values, targetName)

	case model.SelectorTypeTags:
		targetTags, err := m.GetTargetTags(targetType, targetId)
		if err != nil {
			return false
		}
		return len(lo.Intersect(selector.Values, targetTags)) > 0

	default:
		return false
	}
}

// matchRegexPatterns checks if any regex pattern matches the target name
func (m *AuthorizationMatcher) matchRegexPatterns(patterns []string, targetName string) bool {
	for _, pattern := range patterns {
		if matched, err := regexp.MatchString(pattern, targetName); err == nil && matched {
			return true
		}
	}
	return false
}

// checkAccessControl validates access control restrictions
func (m *AuthorizationMatcher) checkAccessControl(accessControl model.AccessControl, req *model.AuthRequest) bool {
	// Check IP whitelist
	if len(accessControl.IPWhitelist) > 0 && !m.checkIPWhitelist(accessControl.IPWhitelist, req.ClientIP) {
		return false
	}

	// Get asset for asset-level restrictions
	asset, err := m.getAssetById(req.AssetId)
	if err != nil {
		return false
	}

	// Check time restrictions with time template support
	if !m.checkUpdatedTimeRestrictions(asset.AccessTimeControl, &accessControl, req.Timestamp) {
		return false
	}

	// TODO: Check max sessions and session timeout (requires session management)

	return true
}

// checkUpdatedTimeRestrictions implements time restriction logic with template support
func (m *AuthorizationMatcher) checkUpdatedTimeRestrictions(assetTimeControl *model.AccessTimeControl, accessControl *model.AccessControl, timestamp time.Time) bool {
	if timestamp.IsZero() {
		timestamp = time.Now()
	}

	// 1. Check asset-level V2 time restrictions (base constraint)
	if assetTimeControl != nil && assetTimeControl.Enabled {
		if !m.checkAssetTimeRanges(assetTimeControl, timestamp) {
			return false // Asset-level restriction failed, deny access
		}
	}

	// 2. Check authorization rule's time template restrictions if configured
	if accessControl.TimeTemplate != nil {
		if !m.checkTimeTemplateAccess(accessControl.TimeTemplate, accessControl.Timezone, timestamp) {
			return false
		}
	}

	// 3. Check authorization rule's custom time ranges if configured
	if len(accessControl.CustomTimeRanges) > 0 {
		if !m.checkTimeRanges(accessControl.CustomTimeRanges, timestamp) {
			return false
		}
	}

	// 4. If no restrictions are configured or all pass, allow access
	return true
}

// checkTimeTemplateAccess checks if current time is within template's allowed ranges
func (m *AuthorizationMatcher) checkTimeTemplateAccess(templateRef *model.TimeTemplateReference, timezone string, timestamp time.Time) bool {
	// Get the time template service for validation
	timeTemplateService := NewTimeTemplateService()

	// Get the template
	ctx := context.Background()
	template, err := timeTemplateService.GetTimeTemplate(ctx, templateRef.TemplateId)
	if err != nil || template == nil {
		// If template cannot be found, deny access for safety
		return false
	}

	// Use the specified timezone or template's timezone
	checkTimezone := timezone
	if checkTimezone == "" {
		checkTimezone = template.Timezone
	}
	if checkTimezone == "" {
		checkTimezone = "Asia/Shanghai" // Default timezone
	}

	// Check if current time is within template ranges
	if m.isTimeInTemplateRanges(template.TimeRanges, checkTimezone, timestamp) {
		return true
	}

	// Check custom ranges in the template reference
	if len(templateRef.CustomRanges) > 0 {
		if m.isTimeInTemplateRanges(templateRef.CustomRanges, checkTimezone, timestamp) {
			return true
		}
	}

	return false
}

// isTimeInTemplateRanges checks if timestamp is within any of the template time ranges
func (m *AuthorizationMatcher) isTimeInTemplateRanges(ranges model.TimeRanges, timezone string, timestamp time.Time) bool {
	// Load timezone location
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		// Fall back to UTC if timezone is invalid
		loc = time.UTC
	}

	targetTime := timestamp.In(loc)
	currentWeekday := int(targetTime.Weekday())
	if currentWeekday == 0 {
		currentWeekday = 7 // Convert Sunday from 0 to 7
	}

	timeStr := targetTime.Format("15:04")

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
		if m.isTimeInRange(timeStr, timeRange.StartTime, timeRange.EndTime) {
			return true
		}
	}

	return false
}

// checkAssetTimeRanges validates asset-level time restrictions with timezone support
func (m *AuthorizationMatcher) checkAssetTimeRanges(assetTimeControl *model.AccessTimeControl, timestamp time.Time) bool {
	if len(assetTimeControl.TimeRanges) == 0 {
		return true // No time ranges specified, allow access
	}

	// Handle timezone conversion
	targetTime := timestamp
	if assetTimeControl.Timezone != "" {
		if loc, err := time.LoadLocation(assetTimeControl.Timezone); err == nil {
			targetTime = timestamp.In(loc)
		}
	}

	weekday := int(targetTime.Weekday())
	if weekday == 0 {
		weekday = 7 // Convert Sunday from 0 to 7
	}

	timeStr := targetTime.Format("15:04")

	for _, tr := range assetTimeControl.TimeRanges {
		// Check if current weekday is allowed
		if len(tr.Weekdays) > 0 && !lo.Contains(tr.Weekdays, weekday) {
			continue
		}

		// Check if current time is within allowed range
		if tr.StartTime != "" && tr.EndTime != "" {
			if m.isTimeInRange(timeStr, tr.StartTime, tr.EndTime) {
				return true
			}
		} else {
			return true // No time restriction
		}
	}

	return false
}

// getAssetById retrieves asset by ID (with caching)
func (m *AuthorizationMatcher) getAssetById(assetId int) (*model.Asset, error) {
	assets, err := repository.GetAllFromCacheDb(context.Background(), model.DefaultAsset)
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

// checkIPWhitelist validates if client IP is in whitelist
func (m *AuthorizationMatcher) checkIPWhitelist(whitelist []string, clientIP string) bool {
	if clientIP == "" {
		return false
	}

	clientIPNet := net.ParseIP(clientIP)
	if clientIPNet == nil {
		return false
	}

	for _, ipRange := range whitelist {
		if strings.Contains(ipRange, "/") {
			// CIDR notation
			_, cidr, err := net.ParseCIDR(ipRange)
			if err == nil && cidr.Contains(clientIPNet) {
				return true
			}
		} else {
			// Single IP
			if ipRange == clientIP {
				return true
			}
		}
	}

	return false
}

// checkTimeRanges validates if current time is within allowed ranges
func (m *AuthorizationMatcher) checkTimeRanges(timeRanges model.TimeRanges, timestamp time.Time) bool {
	if timestamp.IsZero() {
		timestamp = time.Now()
	}

	weekday := int(timestamp.Weekday())
	if weekday == 0 {
		weekday = 7 // Convert Sunday from 0 to 7
	}

	timeStr := timestamp.Format("15:04")

	for _, tr := range timeRanges {
		// Check if current weekday is allowed
		if len(tr.Weekdays) > 0 && !lo.Contains(tr.Weekdays, weekday) {
			continue
		}

		// Check if current time is within allowed range
		if tr.StartTime != "" && tr.EndTime != "" {
			if m.isTimeInRange(timeStr, tr.StartTime, tr.EndTime) {
				return true
			}
		} else {
			return true // No time restriction
		}
	}

	return len(timeRanges) == 0 // Allow if no time ranges specified
}

// isTimeInRange checks if time is within the specified range
func (m *AuthorizationMatcher) isTimeInRange(timeStr, startTime, endTime string) bool {
	return timeStr >= startTime && timeStr <= endTime
}

// GetTargetName retrieves the name of a target by type and ID
func (m *AuthorizationMatcher) GetTargetName(targetType string, targetId int) (string, error) {
	switch targetType {
	case "node":
		nodes, err := repository.GetAllFromCacheDb(context.Background(), model.DefaultNode)
		if err != nil {
			return "", err
		}
		for _, node := range nodes {
			if node.Id == targetId {
				return node.Name, nil
			}
		}
		return "", fmt.Errorf("node not found: %d", targetId)

	case "asset":
		assets, err := repository.GetAllFromCacheDb(context.Background(), model.DefaultAsset)
		if err != nil {
			return "", err
		}
		for _, asset := range assets {
			if asset.Id == targetId {
				return asset.Name, nil
			}
		}
		return "", fmt.Errorf("asset not found: %d", targetId)

	case "account":
		accounts, err := repository.GetAllFromCacheDb(context.Background(), model.DefaultAccount)
		if err != nil {
			return "", err
		}
		for _, account := range accounts {
			if account.Id == targetId {
				return account.Name, nil
			}
		}
		return "", fmt.Errorf("account not found: %d", targetId)

	default:
		return "", fmt.Errorf("unknown target type: %s", targetType)
	}
}

// GetTargetTags retrieves tags for a target (placeholder implementation)
func (m *AuthorizationMatcher) GetTargetTags(targetType string, targetId int) ([]string, error) {
	// TODO: Implement tag system for nodes, assets, and accounts
	// For now, return empty tags
	return []string{}, nil
}

// getCacheKey generates a cache key for the request
func (m *AuthorizationMatcher) getCacheKey(req *model.AuthRequest) string {
	return fmt.Sprintf("auth_v2:%d:%d:%d:%d:%s",
		req.UserId, req.NodeId, req.AssetId, req.AccountId, req.Action)
}

// getCachedResult retrieves cached authorization result
func (m *AuthorizationMatcher) getCachedResult(cacheKey string) *model.AuthResult {
	// TODO: Implement Redis caching
	return nil
}

// cacheResult caches the authorization result
func (m *AuthorizationMatcher) cacheResult(cacheKey string, result *model.AuthResult) {
	// TODO: Implement Redis caching with TTL
}

// MatchWithScope checks if a request is authorized using only specified rule IDs (like V1's AuthorizationIds filtering)
func (m *AuthorizationMatcher) MatchWithScope(ctx *gin.Context, req *model.AuthRequest, ruleIds []int) (*model.AuthResult, error) {
	if len(ruleIds) == 0 {
		return &model.AuthResult{
			Allowed: false,
			Reason:  "No rule IDs provided",
		}, nil
	}

	// Get only the rules that user has permission to access (already filtered by enabled=true)
	enabledRules, err := m.repo.GetByResourceIds(ctx, ruleIds)
	if err != nil {
		return &model.AuthResult{
			Allowed: false,
			Reason:  "Failed to load authorization rules",
		}, err
	}

	// Check each rule in the filtered scope
	for _, rule := range enabledRules {
		if m.matchRule(ctx, rule, req) {
			// If this rule allows the requested action, grant permission immediately
			if rule.Permissions.HasPermission(req.Action) {
				return &model.AuthResult{
					Allowed:     true,
					Permissions: rule.Permissions,
					Reason:      fmt.Sprintf("Allowed by rule: %s", rule.Name),
					RuleId:      rule.Id,
					RuleName:    rule.Name,
				}, nil
			}
		}
	}

	return &model.AuthResult{
		Allowed: false,
		Reason:  "No matching authorization rule found in scope",
	}, nil
}

// MatchBatchWithScope checks if batch requests are authorized using only specified rule IDs
func (m *AuthorizationMatcher) MatchBatchWithScope(ctx *gin.Context, req *model.BatchAuthRequest, ruleIds []int) (*model.BatchAuthResult, error) {
	results := make(map[model.AuthAction]*model.AuthResult)

	// Initialize all actions as denied
	for _, action := range req.Actions {
		results[action] = &model.AuthResult{
			Allowed: false,
			Reason:  "No matching authorization rule found in scope",
		}
	}

	if len(ruleIds) == 0 {
		for action := range results {
			results[action].Reason = "No rule IDs provided"
		}
		return &model.BatchAuthResult{Results: results}, nil
	}

	// Single database query for all rules
	enabledRules, err := m.repo.GetByResourceIds(ctx, ruleIds)
	if err != nil {
		for action := range results {
			results[action] = &model.AuthResult{
				Allowed: false,
				Reason:  "Failed to load authorization rules",
			}
		}
		return &model.BatchAuthResult{Results: results}, err
	}

	// Base request for rule matching
	baseReq := &model.AuthRequest{
		UserId:    req.UserId,
		NodeId:    req.NodeId,
		AssetId:   req.AssetId,
		AccountId: req.AccountId,
		ClientIP:  req.ClientIP,
		UserAgent: req.UserAgent,
		Timestamp: req.Timestamp,
	}

	// Check each rule once and validate all actions
	for _, rule := range enabledRules {
		if m.matchRule(ctx, rule, baseReq) {
			for _, action := range req.Actions {
				if rule.Permissions.HasPermission(action) && !results[action].Allowed {
					results[action] = &model.AuthResult{
						Allowed:     true,
						Permissions: rule.Permissions,
						Reason:      fmt.Sprintf("Allowed by rule: %s", rule.Name),
						RuleId:      rule.Id,
						RuleName:    rule.Name,
					}
				}
			}

			// Early exit if all actions are allowed
			allAllowed := true
			for _, action := range req.Actions {
				if !results[action].Allowed {
					allAllowed = false
					break
				}
			}
			if allAllowed {
				break
			}
		}
	}

	return &model.BatchAuthResult{Results: results}, nil
}
