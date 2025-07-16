package service

import (
	"context"
	"regexp"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/repository"
	gsession "github.com/veops/oneterm/internal/session"
	"github.com/veops/oneterm/pkg/logger"
)

// CommandAnalyzer handles command analysis for sessions
type CommandAnalyzer struct {
	authService IAuthorizationService
}

// NewCommandAnalyzer creates a new command analyzer
func NewCommandAnalyzer() *CommandAnalyzer {
	return &CommandAnalyzer{
		authService: DefaultAuthService,
	}
}

// AnalyzeSessionCommands analyzes and builds the final command list for a session
// This combines asset-level and authorization-level command controls
func (ca *CommandAnalyzer) AnalyzeSessionCommands(ctx *gin.Context, sess *gsession.Session) ([]*model.Command, error) {
	// Get all available commands from cache
	allCommands, err := repository.GetAllFromCacheDb(ctx, model.DefaultCommand)
	if err != nil {
		logger.L().Error("Failed to get commands from cache", zap.Error(err))
		return nil, err
	}

	// Filter enabled commands
	enabledCommands := lo.Filter(allCommands, func(cmd *model.Command, _ int) bool {
		return cmd.Enable
	})

	// Analyze asset-level command control
	assetCommands := ca.analyzeAssetCommands(sess.Session.Asset, enabledCommands)

	// Analyze authorization-level command control
	authCommands, err := ca.analyzeAuthorizationCommands(ctx, sess, enabledCommands)
	if err != nil {
		logger.L().Error("Failed to analyze authorization commands", zap.Error(err))
		// Continue with asset-level commands only
		authCommands = []*model.Command{}
	}

	// Merge and deduplicate command lists
	finalCommands := ca.mergeCommands(assetCommands, authCommands)

	// Compile regex patterns for performance
	for _, cmd := range finalCommands {
		if cmd.IsRe {
			if re, err := regexp.Compile(cmd.Cmd); err == nil {
				cmd.Re = re
			} else {
				logger.L().Warn("Invalid regex pattern in command",
					zap.String("cmd", cmd.Cmd),
					zap.Int("id", cmd.Id),
					zap.Error(err))
			}
		}
	}

	logger.L().Info("Command analysis completed",
		zap.String("sessionId", sess.SessionId),
		zap.Int("assetCommands", len(assetCommands)),
		zap.Int("authCommands", len(authCommands)),
		zap.Int("finalCommands", len(finalCommands)))

	return finalCommands, nil
}

// analyzeAssetCommands analyzes asset-level command controls from V2 system
func (ca *CommandAnalyzer) analyzeAssetCommands(asset *model.Asset, allCommands []*model.Command) []*model.Command {
	var result []*model.Command

	// V2 asset command control
	if asset.AssetCommandControl != nil && asset.AssetCommandControl.Enabled {
		var v2Commands []*model.Command

		// Process direct command IDs
		if len(asset.AssetCommandControl.CmdIds) > 0 {
			cmdMap := make(map[int]*model.Command)
			for _, cmd := range allCommands {
				cmdMap[cmd.Id] = cmd
			}

			for _, cmdId := range asset.AssetCommandControl.CmdIds {
				if cmd, exists := cmdMap[cmdId]; exists {
					v2Commands = append(v2Commands, cmd)
				}
			}
		}

		// Process command template IDs
		if len(asset.AssetCommandControl.TemplateIds) > 0 {
			templateCommands := ca.expandCommandTemplates(asset.AssetCommandControl.TemplateIds, allCommands)
			v2Commands = append(v2Commands, templateCommands...)
		}

		// All configured commands are intercepted
		result = append(result, v2Commands...)

		logger.L().Debug("Asset V2 command control applied",
			zap.Int("assetId", asset.Id),
			zap.Int("cmdCount", len(v2Commands)))
	}

	return lo.UniqBy(result, func(cmd *model.Command) int { return cmd.Id })
}

// analyzeAuthorizationCommands analyzes authorization-level command controls from V2 rules
func (ca *CommandAnalyzer) analyzeAuthorizationCommands(ctx *gin.Context, sess *gsession.Session, allCommands []*model.Command) ([]*model.Command, error) {
	// Get user's authorized V2 rules
	authV2ResourceIds, err := ca.getAuthorizedV2ResourceIds(ctx)
	if err != nil {
		return nil, err
	}

	if len(authV2ResourceIds) == 0 {
		return []*model.Command{}, nil
	}

	// Get V2 rules that apply to this session
	authV2Service := NewAuthorizationV2Service()
	rules, err := authV2Service.repo.GetByResourceIds(ctx, authV2ResourceIds)
	if err != nil {
		return nil, err
	}

	var result []*model.Command

	// Analyze each applicable rule
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		// Check if rule matches this session (simplified matching)
		if ca.ruleMatchesSession(rule, sess) {
			ruleCommands := ca.extractCommandsFromRule(rule, allCommands)
			result = append(result, ruleCommands...)
		}
	}

	return lo.UniqBy(result, func(cmd *model.Command) int { return cmd.Id }), nil
}

// ruleMatchesSession checks if a V2 rule matches the current session (simplified)
func (ca *CommandAnalyzer) ruleMatchesSession(rule *model.AuthorizationV2, sess *gsession.Session) bool {
	// Quick check for asset selector
	if rule.AssetSelector.Type == model.SelectorTypeIds {
		assetIds := lo.FilterMap(rule.AssetSelector.Values, func(v string, _ int) (int, bool) {
			if id, err := strconv.Atoi(v); err == nil {
				return id, true
			}
			return 0, false
		})
		if !lo.Contains(assetIds, sess.AssetId) {
			return false
		}
	} else if rule.AssetSelector.Type != model.SelectorTypeAll {
		// For regex/tags selectors, we'd need more complex matching
		// For now, assume they match (could be optimized later)
	}

	// Quick check for account selector
	if rule.AccountSelector.Type == model.SelectorTypeIds {
		accountIds := lo.FilterMap(rule.AccountSelector.Values, func(v string, _ int) (int, bool) {
			if id, err := strconv.Atoi(v); err == nil {
				return id, true
			}
			return 0, false
		})
		if !lo.Contains(accountIds, sess.AccountId) {
			return false
		}
	}

	return true
}

// extractCommandsFromRule extracts commands from a V2 authorization rule
func (ca *CommandAnalyzer) extractCommandsFromRule(rule *model.AuthorizationV2, allCommands []*model.Command) []*model.Command {
	var result []*model.Command

	// Process direct command IDs
	if len(rule.AccessControl.CmdIds) > 0 {
		cmdMap := make(map[int]*model.Command)
		for _, cmd := range allCommands {
			cmdMap[cmd.Id] = cmd
		}

		for _, cmdId := range rule.AccessControl.CmdIds {
			if cmd, exists := cmdMap[cmdId]; exists {
				result = append(result, cmd)
			}
		}
	}

	// Process command template IDs
	if len(rule.AccessControl.TemplateIds) > 0 {
		templateCommands := ca.expandCommandTemplates(rule.AccessControl.TemplateIds, allCommands)
		result = append(result, templateCommands...)
	}

	// All configured commands are intercepted
	return result
}

// expandCommandTemplates expands command template IDs to actual commands
func (ca *CommandAnalyzer) expandCommandTemplates(templateIds []int, allCommands []*model.Command) []*model.Command {
	// Get command templates from database
	commandTemplateService := NewCommandTemplateService()
	ctx := context.Background()

	var expandedCommands []*model.Command

	for _, templateId := range templateIds {
		template, err := commandTemplateService.GetCommandTemplate(ctx, templateId)
		if err != nil {
			logger.L().Warn("Failed to get command template",
				zap.Int("templateId", templateId),
				zap.Error(err))
			continue
		}

		if template == nil {
			continue
		}

		// Get commands from template
		templateCmdIds := lo.Map(template.CmdIds, func(id int, _ int) int { return id })
		templateCommands := lo.Filter(allCommands, func(cmd *model.Command, _ int) bool {
			return lo.Contains(templateCmdIds, cmd.Id)
		})

		expandedCommands = append(expandedCommands, templateCommands...)
	}

	return expandedCommands
}

// mergeCommands merges and deduplicates command lists
func (ca *CommandAnalyzer) mergeCommands(assetCommands, authCommands []*model.Command) []*model.Command {
	// Combine all commands
	allCommands := append(assetCommands, authCommands...)

	// Deduplicate by ID
	return lo.UniqBy(allCommands, func(cmd *model.Command) int { return cmd.Id })
}

// getAuthorizedV2ResourceIds gets V2 authorization rule resource IDs that user has permission to
func (ca *CommandAnalyzer) getAuthorizedV2ResourceIds(ctx *gin.Context) ([]int, error) {
	return ca.authService.(*AuthorizationService).getAuthorizedV2ResourceIds(ctx)
}
