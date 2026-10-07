package service

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/repository"
	gsession "github.com/veops/oneterm/internal/session"
	dbpkg "github.com/veops/oneterm/pkg/db"
	"github.com/veops/oneterm/pkg/logger"
)

// CommandAnalyzer handles command analysis for sessions
type CommandAnalyzer struct {
	authService IAuthorizationService
	matcher     *AuthorizationMatcher
	ctx         context.Context
}

// NewCommandAnalyzer creates a new command analyzer
func NewCommandAnalyzer() *CommandAnalyzer {
	return &CommandAnalyzer{
		authService: DefaultAuthService,
		matcher:     NewAuthorizationMatcher(repository.NewAuthorizationV2Repository(dbpkg.DB)).(*AuthorizationMatcher),
		ctx:         context.Background(),
	}
}

// AnalyzeSessionCommands analyzes and builds the final command list for a session
// This combines asset-level and authorization-level command controls
func (ca *CommandAnalyzer) AnalyzeSessionCommands(ctx *gin.Context, sess *gsession.Session) ([]*model.Command, error) {
	ca.ctx = ctx.Request.Context()
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
	assetCommands, err := ca.analyzeAssetCommands(sess.Session.Asset, enabledCommands)
	if err != nil {
		return nil, err
	}

	// Analyze authorization-level command control
	authCommands, err := ca.analyzeAuthorizationCommands(ctx, sess, enabledCommands)
	if err != nil {
		logger.L().Error("Failed to analyze authorization commands", zap.Error(err))
		return nil, err
	}

	// Merge and deduplicate command lists
	finalCommands := ca.mergeCommands(assetCommands, authCommands)

	// Compile regex patterns for performance
	for _, cmd := range finalCommands {
		if cmd.IsRe {
			if re, err := regexp.Compile(cmd.Cmd); err == nil {
				cmd.Re = re
			} else {
				return nil, fmt.Errorf("invalid command rule %d: %w", cmd.Id, err)
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
func (ca *CommandAnalyzer) analyzeAssetCommands(asset *model.Asset, allCommands []*model.Command) ([]*model.Command, error) {
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
			templateCommands, err := ca.expandCommandTemplates(asset.AssetCommandControl.TemplateIds, allCommands)
			if err != nil {
				return nil, err
			}
			v2Commands = append(v2Commands, templateCommands...)
		}

		// All configured commands are intercepted
		result = append(result, v2Commands...)

		logger.L().Debug("Asset V2 command control applied",
			zap.Int("assetId", asset.Id),
			zap.Int("cmdCount", len(v2Commands)))
	}

	return lo.UniqBy(result, func(cmd *model.Command) int { return cmd.Id }), nil
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

		matched, err := ca.ruleMatchesSession(rule, sess)
		if err != nil {
			return nil, err
		}
		if matched {
			ruleCommands, err := ca.extractCommandsFromRule(rule, allCommands)
			if err != nil {
				return nil, err
			}
			result = append(result, ruleCommands...)
		}
	}

	return lo.UniqBy(result, func(cmd *model.Command) int { return cmd.Id }), nil
}

func (ca *CommandAnalyzer) ruleMatchesSession(rule *model.AuthorizationV2, sess *gsession.Session) (bool, error) {
	if !rule.IsValid(time.Now()) {
		return false, nil
	}
	selectors := []model.TargetSelector{rule.NodeSelector, rule.AssetSelector, rule.AccountSelector}
	types := []string{"node", "asset", "account"}
	ids := []int{sess.Asset.ParentId, sess.AssetId, sess.AccountId}
	for i, selector := range selectors {
		if selector.Type == model.SelectorTypeRegex {
			for _, pattern := range selector.Values {
				if _, err := regexp.Compile(pattern); err != nil {
					return false, err
				}
			}
		}
		matched, err := ca.matcher.matchSelectorChecked(ca.ctx, selector, types[i], ids[i])
		if err != nil || !matched {
			return false, err
		}
	}
	return true, nil
}

// extractCommandsFromRule extracts commands from a V2 authorization rule
func (ca *CommandAnalyzer) extractCommandsFromRule(rule *model.AuthorizationV2, allCommands []*model.Command) ([]*model.Command, error) {
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
		templateCommands, err := ca.expandCommandTemplates(rule.AccessControl.TemplateIds, allCommands)
		if err != nil {
			return nil, err
		}
		result = append(result, templateCommands...)
	}

	// All configured commands are intercepted
	return result, nil
}

// expandCommandTemplates expands command template IDs to actual commands
func (ca *CommandAnalyzer) expandCommandTemplates(ids []int, commands []*model.Command) ([]*model.Command, error) {
	ctx, cancel := context.WithTimeout(ca.ctx, 5*time.Second)
	defer cancel()
	var templates []*model.CommandTemplate
	ids = lo.Uniq(ids)
	if err := dbpkg.DB.WithContext(ctx).Where("id IN ?", ids).Find(&templates).Error; err != nil {
		return nil, err
	}
	if len(templates) != len(ids) {
		return nil, fmt.Errorf("command template is unavailable")
	}
	selected := make(map[int]struct{})
	for _, template := range templates {
		for _, id := range template.CmdIds {
			selected[id] = struct{}{}
		}
	}
	return lo.Filter(commands, func(command *model.Command, _ int) bool { _, ok := selected[command.Id]; return ok }), nil
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
