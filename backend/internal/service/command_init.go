package service

import (
	"context"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/samber/lo"

	myi18n "github.com/veops/oneterm/internal/i18n"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/pkg/db"
	"github.com/veops/oneterm/pkg/logger"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// CommandDefinition defines a command with i18n message keys
type CommandDefinition struct {
	NameKey        *i18n.Message
	DescriptionKey *i18n.Message
	Cmd            string
	IsRe           bool
	RiskLevel      model.CommandRiskLevel
	Category       model.CommandCategory
	TagKeys        []*i18n.Message
}

// TemplateDefinition defines a template with i18n message keys
type TemplateDefinition struct {
	NameKey        *i18n.Message
	DescriptionKey *i18n.Message
	Category       model.CommandCategory
	CommandRefs    []string // References to command names by i18n key IDs
}

// getLocalizer creates a localizer for the given context (defaults to English)
func getLocalizer(ctx context.Context) *i18n.Localizer {
	// Default to English, as this runs during system startup without user context
	return i18n.NewLocalizer(myi18n.Bundle, "en")
}

// localizeMessage localizes a message using the given localizer
func localizeMessage(localizer *i18n.Localizer, msg *i18n.Message) string {
	result, _ := localizer.Localize(&i18n.LocalizeConfig{DefaultMessage: msg})
	return result
}

// localizeTags localizes an array of tag message keys
func localizeTags(localizer *i18n.Localizer, tagKeys []*i18n.Message) []string {
	return lo.Map(tagKeys, func(msg *i18n.Message, _ int) string {
		return localizeMessage(localizer, msg)
	})
}

// InitBuiltinCommands initializes predefined dangerous commands on system startup
func InitBuiltinCommands() error {
	ctx := context.Background()
	localizer := getLocalizer(ctx)

	logger.L().Info("Starting initialization of predefined dangerous commands")

	// Initialize commands
	commands := getPreDefinedCommands()
	for _, cmdDef := range commands {
		if err := createCommandIfNotExists(localizer, cmdDef); err != nil {
			logger.L().Error("Failed to create predefined command",
				zap.String("name_key", cmdDef.NameKey.ID),
				zap.Error(err))
			continue
		}
	}

	// Initialize templates
	templates := getPreDefinedTemplates()
	for _, tmplDef := range templates {
		if err := createTemplateIfNotExists(localizer, tmplDef); err != nil {
			logger.L().Error("Failed to create predefined template",
				zap.String("name_key", tmplDef.NameKey.ID),
				zap.Error(err))
			continue
		}
	}

	logger.L().Info("Predefined dangerous commands initialization completed successfully")
	return nil
}

// createCommandIfNotExists creates a command if it doesn't exist
func createCommandIfNotExists(localizer *i18n.Localizer, cmdDef CommandDefinition) error {
	name := localizeMessage(localizer, cmdDef.NameKey)

	var existingCmd model.Command
	err := db.DB.Where("name = ? AND is_global = ?", name, true).First(&existingCmd).Error
	if err == nil {
		logger.L().Debug("Predefined command already exists, skipping", zap.String("name", name))
		return nil
	}

	if err != gorm.ErrRecordNotFound {
		return err
	}

	// Create new command
	newCmd := &model.Command{
		Name:        name,
		Cmd:         cmdDef.Cmd,
		IsRe:        cmdDef.IsRe,
		Enable:      true,
		Category:    cmdDef.Category,
		RiskLevel:   cmdDef.RiskLevel,
		Description: localizeMessage(localizer, cmdDef.DescriptionKey),
		Tags:        localizeTags(localizer, cmdDef.TagKeys),
		IsGlobal:    true,
		CreatorId:   1, // System user
		UpdaterId:   1,
	}

	if err := db.DB.Create(newCmd).Error; err != nil {
		return err
	}

	logger.L().Info("Successfully created predefined command",
		zap.String("name", name),
		zap.Int("risk_level", int(cmdDef.RiskLevel)))

	return nil
}

// createTemplateIfNotExists creates a template if it doesn't exist
func createTemplateIfNotExists(localizer *i18n.Localizer, tmplDef TemplateDefinition) error {
	name := localizeMessage(localizer, tmplDef.NameKey)

	var existingTemplate model.CommandTemplate
	err := db.DB.Where("name = ? AND is_builtin = ?", name, true).First(&existingTemplate).Error
	if err == nil {
		logger.L().Debug("Predefined template already exists, skipping", zap.String("name", name))
		return nil
	}

	if err != gorm.ErrRecordNotFound {
		return err
	}

	// Get command IDs by localized names
	cmdIds, err := getCommandIdsByLocalizedNames(localizer, tmplDef.CommandRefs)
	if err != nil {
		logger.L().Warn("Some commands not found for template",
			zap.String("template", name),
			zap.Error(err))
	}

	// Create new template
	newTemplate := &model.CommandTemplate{
		Name:        name,
		Description: localizeMessage(localizer, tmplDef.DescriptionKey),
		Category:    tmplDef.Category,
		CmdIds:      cmdIds,
		IsBuiltin:   true,
		CreatorId:   1, // System user
	}

	if err := db.DB.Create(newTemplate).Error; err != nil {
		return err
	}

	logger.L().Info("Successfully created predefined template",
		zap.String("name", name),
		zap.Int("command_count", len(cmdIds)))

	return nil
}

// getCommandIdsByLocalizedNames retrieves command IDs by their localized names
func getCommandIdsByLocalizedNames(localizer *i18n.Localizer, nameKeys []string) (model.Slice[int], error) {
	// Convert i18n keys to localized names
	localizedNames := lo.Map(nameKeys, func(keyID string, _ int) string {
		// Find the message by ID in our predefined commands
		cmdDefs := getPreDefinedCommands()
		if cmdDef, found := lo.Find(cmdDefs, func(def CommandDefinition) bool {
			return def.NameKey.ID == keyID
		}); found {
			return localizeMessage(localizer, cmdDef.NameKey)
		}
		return keyID // fallback to key ID if not found
	})

	var commands []model.Command
	if err := db.DB.Where("name IN ? AND is_global = ?", localizedNames, true).Find(&commands).Error; err != nil {
		return nil, err
	}

	return lo.Map(commands, func(cmd model.Command, _ int) int {
		return cmd.Id
	}), nil
}

// getPreDefinedCommands returns command definitions with i18n message keys
func getPreDefinedCommands() []CommandDefinition {
	return []CommandDefinition{
		// Critical dangerous commands (RiskLevel: 3)
		{
			NameKey:        myi18n.CmdDeleteRootDir,
			DescriptionKey: myi18n.CmdDeleteRootDirDesc,
			Cmd:            "^rm\\s+(-rf?|--recursive\\s+--force?)\\s+/\\s*$",
			IsRe:           true,
			RiskLevel:      model.RiskLevelCritical,
			Category:       model.CategorySecurity,
			TagKeys:        []*i18n.Message{myi18n.TagDangerous, myi18n.TagDelete, myi18n.TagSystem},
		},
		{
			NameKey:        myi18n.CmdDeleteSystemDirs,
			DescriptionKey: myi18n.CmdDeleteSystemDirsDesc,
			Cmd:            "^rm\\s+(-rf?|--recursive\\s+--force?)\\s+/(bin|boot|dev|etc|lib|lib64|proc|root|sbin|sys|usr)(/.*)?$",
			IsRe:           true,
			RiskLevel:      model.RiskLevelCritical,
			Category:       model.CategorySecurity,
			TagKeys:        []*i18n.Message{myi18n.TagDangerous, myi18n.TagDelete, myi18n.TagSystemDirs},
		},
		{
			NameKey:        myi18n.CmdDiskDestruction,
			DescriptionKey: myi18n.CmdDiskDestructionDesc,
			Cmd:            "^dd\\s+if=/dev/(zero|random|urandom)\\s+of=/dev/[sh]d[a-z]+",
			IsRe:           true,
			RiskLevel:      model.RiskLevelCritical,
			Category:       model.CategorySecurity,
			TagKeys:        []*i18n.Message{myi18n.TagDangerous, myi18n.TagDisk, myi18n.TagDestruction},
		},
		{
			NameKey:        myi18n.CmdFormatDisk,
			DescriptionKey: myi18n.CmdFormatDiskDesc,
			Cmd:            "^(mkfs|fdisk|parted|gdisk)\\.(ext[234]|xfs|btrfs|ntfs|fat32)\\s+/dev/[sh]d[a-z]+",
			IsRe:           true,
			RiskLevel:      model.RiskLevelCritical,
			Category:       model.CategorySecurity,
			TagKeys:        []*i18n.Message{myi18n.TagDangerous, myi18n.TagFormat, myi18n.TagDisk},
		},
		{
			NameKey:        myi18n.CmdForkBomb,
			DescriptionKey: myi18n.CmdForkBombDesc,
			Cmd:            ":(\\)|\\{\\s*\\|\\s*:\\s*&\\s*\\};\\s*:",
			IsRe:           true,
			RiskLevel:      model.RiskLevelCritical,
			Category:       model.CategorySecurity,
			TagKeys:        []*i18n.Message{myi18n.TagDangerous, myi18n.TagAttack, myi18n.TagResourceExhaustion},
		},

		// Dangerous commands (RiskLevel: 2)
		{
			NameKey:        myi18n.CmdSystemReboot,
			DescriptionKey: myi18n.CmdSystemRebootDesc,
			Cmd:            "^(shutdown|reboot|halt|poweroff|init\\s+[06])\\b",
			IsRe:           true,
			RiskLevel:      model.RiskLevelDanger,
			Category:       model.CategorySystem,
			TagKeys:        []*i18n.Message{myi18n.TagReboot, myi18n.TagShutdown, myi18n.TagSystem},
		},
		{
			NameKey:        myi18n.CmdModifySystemFiles,
			DescriptionKey: myi18n.CmdModifySystemFilesDesc,
			Cmd:            "^(vi|vim|nano|emacs|cat\\s*>|echo\\s*>)\\s+/(etc|boot|sys)/.+",
			IsRe:           true,
			RiskLevel:      model.RiskLevelDanger,
			Category:       model.CategorySystem,
			TagKeys:        []*i18n.Message{myi18n.TagEdit, myi18n.TagSystemFiles, myi18n.TagConfig},
		},
		{
			NameKey:        myi18n.CmdDropDatabase,
			DescriptionKey: myi18n.CmdDropDatabaseDesc,
			Cmd:            "^(mysql|psql|mongo).*drop\\s+(database|schema|collection)\\s+\\w+",
			IsRe:           true,
			RiskLevel:      model.RiskLevelDanger,
			Category:       model.CategoryDatabase,
			TagKeys:        []*i18n.Message{myi18n.TagDatabase, myi18n.TagDelete, myi18n.TagDrop},
		},
		{
			NameKey:        myi18n.CmdTruncateTable,
			DescriptionKey: myi18n.CmdTruncateTableDesc,
			Cmd:            "^(mysql|psql).*\\b(truncate\\s+table|delete\\s+from\\s+\\w+\\s*;?)\\s*$",
			IsRe:           true,
			RiskLevel:      model.RiskLevelDanger,
			Category:       model.CategoryDatabase,
			TagKeys:        []*i18n.Message{myi18n.TagDatabase, myi18n.TagClear, myi18n.TagTruncate},
		},
		{
			NameKey:        myi18n.CmdModifyPermissions,
			DescriptionKey: myi18n.CmdModifyPermissionsDesc,
			Cmd:            "^(chmod|chown|chgrp)\\s+(777|666|755)\\s+/",
			IsRe:           true,
			RiskLevel:      model.RiskLevelDanger,
			Category:       model.CategorySecurity,
			TagKeys:        []*i18n.Message{myi18n.TagPermissions, myi18n.TagChmod, myi18n.TagSecurity},
		},

		// Warning level commands (RiskLevel: 1)
		{
			NameKey:        myi18n.CmdDropTable,
			DescriptionKey: myi18n.CmdDropTableDesc,
			Cmd:            "^(mysql|psql).*drop\\s+table\\s+\\w+",
			IsRe:           true,
			RiskLevel:      model.RiskLevelWarning,
			Category:       model.CategoryDatabase,
			TagKeys:        []*i18n.Message{myi18n.TagDatabase, myi18n.TagDelete, myi18n.TagTable},
		},
		{
			NameKey:        myi18n.CmdServiceControl,
			DescriptionKey: myi18n.CmdServiceControlDesc,
			Cmd:            "^(systemctl|service)\\s+(stop|restart|disable)\\s+\\w+",
			IsRe:           true,
			RiskLevel:      model.RiskLevelWarning,
			Category:       model.CategorySystem,
			TagKeys:        []*i18n.Message{myi18n.TagService, myi18n.TagSystemctl, myi18n.TagControl},
		},
		{
			NameKey:        myi18n.CmdNetworkConfig,
			DescriptionKey: myi18n.CmdNetworkConfigDesc,
			Cmd:            "^(iptables|ufw|firewall-cmd|ip\\s+route)\\s+",
			IsRe:           true,
			RiskLevel:      model.RiskLevelWarning,
			Category:       model.CategoryNetwork,
			TagKeys:        []*i18n.Message{myi18n.TagNetwork, myi18n.TagFirewall, myi18n.TagRouting},
		},
		{
			NameKey:        myi18n.CmdUserManagement,
			DescriptionKey: myi18n.CmdUserManagementDesc,
			Cmd:            "^(useradd|userdel|usermod|passwd)\\s+",
			IsRe:           true,
			RiskLevel:      model.RiskLevelWarning,
			Category:       model.CategorySecurity,
			TagKeys:        []*i18n.Message{myi18n.TagUser, myi18n.TagManagement, myi18n.TagSecurity},
		},
		{
			NameKey:        myi18n.CmdKernelModule,
			DescriptionKey: myi18n.CmdKernelModuleDesc,
			Cmd:            "^(modprobe|rmmod|insmod)\\s+",
			IsRe:           true,
			RiskLevel:      model.RiskLevelWarning,
			Category:       model.CategorySystem,
			TagKeys:        []*i18n.Message{myi18n.TagKernel, myi18n.TagModule, myi18n.TagSystem},
		},
	}
}

// getPreDefinedTemplates returns template definitions with i18n message keys
func getPreDefinedTemplates() []TemplateDefinition {
	return []TemplateDefinition{
		{
			NameKey:        myi18n.TmplBasicSecurity,
			DescriptionKey: myi18n.TmplBasicSecurityDesc,
			Category:       model.CategorySecurity,
			CommandRefs:    []string{"CmdDeleteRootDir", "CmdDeleteSystemDirs", "CmdDiskDestruction", "CmdFormatDisk", "CmdForkBomb"},
		},
		{
			NameKey:        myi18n.TmplDatabaseProtection,
			DescriptionKey: myi18n.TmplDatabaseProtectionDesc,
			Category:       model.CategoryDatabase,
			CommandRefs:    []string{"CmdDropDatabase", "CmdTruncateTable", "CmdDropTable"},
		},
		{
			NameKey:        myi18n.TmplServiceRestrictions,
			DescriptionKey: myi18n.TmplServiceRestrictionsDesc,
			Category:       model.CategorySystem,
			CommandRefs:    []string{"CmdSystemReboot", "CmdModifySystemFiles", "CmdServiceControl"},
		},
		{
			NameKey:        myi18n.TmplNetworkSecurity,
			DescriptionKey: myi18n.TmplNetworkSecurityDesc,
			Category:       model.CategorySecurity,
			CommandRefs:    []string{"CmdNetworkConfig", "CmdModifyPermissions", "CmdUserManagement"},
		},
		{
			NameKey:        myi18n.TmplDevEnvironment,
			DescriptionKey: myi18n.TmplDevEnvironmentDesc,
			Category:       model.CategorySecurity,
			CommandRefs:    []string{"CmdDeleteRootDir", "CmdFormatDisk", "CmdForkBomb"},
		},
	}
}
