package i18n

import (
	"fmt"

	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"go.uber.org/zap"
	"golang.org/x/text/language"

	"github.com/veops/oneterm/pkg/logger"
)

var (
	Bundle = i18n.NewBundle(language.English)
	langs  = []string{"en", "zh"}
)

func init() {
	Bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
	for _, lang := range langs {
		_, err := Bundle.LoadMessageFile(fmt.Sprintf("./locales/active.%s.toml", lang))
		if err != nil {
			logger.L().Error("load i18n message failed", zap.Error(err))
		}
	}
}

var (
	// errors
	MsgBadRequest = &i18n.Message{
		ID:    "MsgBadRequest",
		One:   "Bad Request: {{.err}}",
		Other: "Bad Request: {{.err}}",
	}
	MsgInvalidArguemnt = &i18n.Message{
		ID:    "MsgArgumentError",
		One:   "Bad Request: Argument is invalid, {{.err}}",
		Other: "Bad Request: Argument is invalid, {{.err}}",
	}
	MsgDupName = &i18n.Message{
		ID:    "MsgDupName",
		One:   "Bad Request: {{.name}} is duplicate",
		Other: "Bad Request: {{.name}} is duplicate",
	}
	MsgHasChild = &i18n.Message{
		ID:    "MsgHasChild",
		One:   "Bad Request: This folder has sub folder or assert, cannot be deleted",
		Other: "Bad Request: This folder has sub folder or assert, cannot be deleted",
	}
	MsgHasDepdency = &i18n.Message{
		ID:    "MsgHasDepdency",
		One:   "Bad Request: Asset {{.name}} dependens on this, cannot be deleted",
		Other: "Bad Request: Asset {{.name}} dependens on this, cannot be deleted",
	}
	MsgNoPerm = &i18n.Message{
		ID:    "MsgNoPerm",
		One:   "Bad Request: You do not have {{.perm}} permission",
		Other: "Bad Request: You do not have {{.perm}} permission",
	}
	MsgRemoteClient = &i18n.Message{
		ID:    "MsgRemote",
		One:   "Bad Request: {{.message}}",
		Other: "Bad Request: {{.message}}",
	}
	MsgWrongPvk = &i18n.Message{
		ID:    "MsgWrongPvk",
		One:   "Bad Request: Invalid SSH private key",
		Other: "Bad Request: Invalid SSH private key",
	}
	MsgWrongPk = &i18n.Message{
		ID:    "MsgWrongPk",
		One:   "Bad Request: Invalid SSH public key",
		Other: "Bad Request: Invalid SSH public key",
	}
	MsgWrongMac = &i18n.Message{
		ID:    "MsgWrongMac",
		One:   "Bad Request: Invalid Mac address",
		Other: "Bad Request: Invalid Mac address",
	}
	MsgInvalidSessionId = &i18n.Message{
		ID:    "MsgInvalidSessionId",
		One:   "Bad Request: Invalid session id {{.sessionId}}",
		Other: "Bad Request: Invalid session id {{.sessionId}}",
	}
	MsgSessionEnd = &i18n.Message{
		ID:    "MsgSessionEnd",
		One:   "\n----------Session {{.sessionId}} has been ended----------\n",
		Other: "\n----------Session {{.sessionId}} has been ended----------\n",
	}
	MsgLoginError = &i18n.Message{
		ID:    "MsgLoginError",
		One:   "Bad Request: Invalid account",
		Other: "Bad Request: Invalid account",
	}
	MsgAccessTime = &i18n.Message{
		ID:    "MsgAccessTime",
		One:   "Bad Request: current time is not allowed to access",
		Other: "Bad Request: current time is not allowed to access",
	}
	MsgIdleTimeout = &i18n.Message{
		ID:    "MsgIdleTimeout",
		One:   "Bad Request: idle timeout more than {{.second}} seconds",
		Other: "Bad Request: idle timeout more than {{.second}} seconds",
	}
	MsgUnauthorized = &i18n.Message{
		ID:    "MsgUnauthorized",
		One:   "Unauthorized",
		Other: "Unauthorized",
	}
	//
	MsgInternalError = &i18n.Message{
		ID:    "MsgInternalError",
		One:   "Server Error: {{.err}}",
		Other: "Server Error: {{.err}}",
	}
	MsgRemoteServer = &i18n.Message{
		ID:    "MsgRemoteServer",
		One:   "Server Error: {{.message}}",
		Other: "Server Error: {{.message}}",
	}
	MsgLoadSession = &i18n.Message{
		ID:    "MsgLoadSession",
		One:   "Load Session Faild",
		Other: "Load Session Faild",
	}
	MsgConnectServer = &i18n.Message{
		ID:    "MsgConnectServer",
		One:   "Connect Server Error",
		Other: "Connect Server Error",
	}
	MsgAdminClose = &i18n.Message{
		ID:    "MsgAdminClose",
		One:   "Sessoin has been closed by admin {{.admin}}",
		Other: "Sessoin has been closed by admin {{.admin}}",
	}

	// others
	MsgTypeMappingAccount = &i18n.Message{
		ID:    "MsgTypeMappingAccount",
		One:   "Account",
		Other: "Account",
	}
	MsgTypeMappingAsset = &i18n.Message{
		ID:    "MsgTypeMappingAsset",
		One:   "Asset",
		Other: "Asset",
	}
	MsgTypeMappingCommand = &i18n.Message{
		ID:    "MsgTypeMappingCommand",
		One:   "Command",
		Other: "Command",
	}
	MsgTypeMappingGateway = &i18n.Message{
		ID:    "MsgTypeMappingGateway",
		One:   "Gateway",
		Other: "Gateway",
	}
	MsgTypeMappingNode = &i18n.Message{
		ID:    "MsgTypeMappingNode",
		One:   "Node",
		Other: "Node",
	}
	MsgTypeMappingPublicKey = &i18n.Message{
		ID:    "MsgTypeMappingPublicKey",
		One:   "Public Key",
		Other: "Public Key",
	}

	// SSH
	MsgSshShowAssetResults = &i18n.Message{
		ID:    "MsgSshShowAssetResults",
		One:   "Total host count is:\033[0;32m {{.Count}} \033[0m \r\n{{.Msg}}\r\n",
		Other: "Total host count is:\033[0;32m {{.Count}} \033[0m \r\n{{.Msg}}\r\n",
	}
	MsgSshAccountLoginError = &i18n.Message{
		ID: "MsgSshAccountLoginError",
		One: "\x1b[1;30;32m failed login \x1b[0m \x1b[1;30;3m {{.User}}\x1b[0m\n" +
			"\x1b[0;33m you need to choose asset again \u001B[0m\n",
		Other: "\x1b[1;30;32m failed login \x1b[0m \x1b[1;30;3m {{.User}}\x1b[0m\n" +
			"\x1b[0;33m you need to choose asset again \u001B[0m\n",
	}
	MsgSshNoAssetPermission = &i18n.Message{
		ID:    "MsgSshNoAssetPermission",
		One:   "\r\n\u001B[0;33mNo permission for[0m:\033[0;31m {{.Host}} \033[0m\r\n",
		Other: "\r\n\u001B[0;33mNo permission for[0m:\033[0;31m {{.Host}} \033[0m\r\n",
	}
	MsgSshNoMatchingAsset = &i18n.Message{
		ID:    "MsgSshNoMatchingAsset",
		One:   "\x1b[0;33mNo matching asset for :\x1b[0m  \x1b[0;94m{{.Host}} \x1b[0m\r\n",
		Other: "\x1b[0;33mNo matching asset for :\x1b[0m  \x1b[0;94m{{.Host}} \x1b[0m\r\n",
	}
	MsgSshNoSshAccessMethod = &i18n.Message{
		ID:    "MsgSshNoSshAccessMethod",
		One:   "No ssh access method for :\033[0;31m {{.Host}} \033[0m\r\n",
		Other: "No ssh access method for :\033[0;31m {{.Host}} \033[0m\r\n",
	}
	MsgSshNoSshAccountForAsset = &i18n.Message{
		ID:    "MsgSshNoSshAccountForAsset",
		One:   "No ssh account for :\033[0;31m {{.Host}} \033[0m\r\n",
		Other: "No ssh account for :\033[0;31m {{.Host}} \033[0m\r\n",
	}
	MsgSshMultiSshAccountForAsset = &i18n.Message{
		ID:    "MsgSshMultiSshAccountForAsset",
		One:   "choose account: \n\033[0;31m {{.Accounts}} \033[0m\n",
		Other: "choose account: \n\033[0;31m {{.Accounts}} \033[0m\n",
	}
	MsgSshWelcome = &i18n.Message{
		ID: "MsgSshWelcomeMsg",
		One: "\x1b[0;47m Welcome: {{.User}} \x1b[0m\r\n" +
			" \x1b[1;30;32m /s \x1b[0m to switch language between english and 中文\r\n" +
			"\x1b[1;30;32m /* \x1b[0m to list all host which you have permission\r\n" +
			"\x1b[1;30;32m IP/hostname \x1b[0m to search and login if only one, eg. 192\r\n" +
			"\x1b[1;30;32m /q \x1b[0m to exit\r\n" +
			"\x1b[1;30;32m /? \x1b[0m for help\r\n",
		Other: "\x1b[0;47m Welcome: {{.User}} \x1b[0m\r\n" +
			" \x1b[1;30;32m /s \x1b[0m to switch language between english and 中文\r\n" +
			"\x1b[1;30;32m /* \x1b[0m to list all host which you have permission\r\n" +
			"\x1b[1;30;32m IP/hostname \x1b[0m to search and login if only one, eg. 192\r\n" +
			"\x1b[1;30;32m /q \x1b[0m to exit\r\n" +
			"\x1b[1;30;32m /? \x1b[0m for help\r\n",
	}
	MsgSshCommandRefused = &i18n.Message{
		ID:    "MsgSshCommandRefused",
		One:   "\x1b[0;31m you have no permission to execute command: \x1b[0m  \x1b[0;33m{{.Command}} \x1b[0m\r\n",
		Other: "\x1b[0;31m you have no permission to execute command: \x1b[0m  \x1b[0;33m{{.Command}} \x1b[0m\r\n",
	}
	MsgSShHostIdleTimeout = &i18n.Message{
		ID:    "MsgSShHostIdleTimeout",
		One:   "\r\n\x1b[0;31m disconnect since idle more than\x1b[0m \x1b[0;33m {{.Idle}} \x1b[0m\r\n",
		Other: "\r\n\x1b[0;31m disconnect since idle more than\x1b[0m \x1b[0;33m {{.Idle}} \x1b[0m\r\n",
	}

	MsgSshAccessRefusedInTimespan = &i18n.Message{
		ID:    "MsgSshAccessRefusedInTimespan",
		One:   "\r\n\x1b[0;31m disconnect since current time is not allowed \x1b[0m\r\n",
		Other: "\r\n\x1b[0;31m disconnect since current time is not allowed \x1b[0m\r\n",
	}

	MsgSShWelcomeForHelp = &i18n.Message{
		ID:    "MsgSShWelcomeForHelp",
		One:   "\x1b[31;47m Welcome: {{.User}}",
		Other: "\x1b[31;47m Welcome: {{.User}}",
	}

	// Predefined dangerous commands
	CmdDeleteRootDir = &i18n.Message{
		ID:    "CmdDeleteRootDir",
		One:   "Delete root directory",
		Other: "Delete root directory",
	}
	CmdDeleteRootDirDesc = &i18n.Message{
		ID:    "CmdDeleteRootDirDesc",
		One:   "Prohibit deletion of root directory, this will destroy the entire system",
		Other: "Prohibit deletion of root directory, this will destroy the entire system",
	}
	CmdDeleteSystemDirs = &i18n.Message{
		ID:    "CmdDeleteSystemDirs",
		One:   "Delete system directories",
		Other: "Delete system directories",
	}
	CmdDeleteSystemDirsDesc = &i18n.Message{
		ID:    "CmdDeleteSystemDirsDesc",
		One:   "Prohibit deletion of critical system directories",
		Other: "Prohibit deletion of critical system directories",
	}
	CmdDiskDestruction = &i18n.Message{
		ID:    "CmdDiskDestruction",
		One:   "Disk destruction operations",
		Other: "Disk destruction operations",
	}
	CmdDiskDestructionDesc = &i18n.Message{
		ID:    "CmdDiskDestructionDesc",
		One:   "Prohibit writing random data to disk devices, will destroy data",
		Other: "Prohibit writing random data to disk devices, will destroy data",
	}
	CmdFormatDisk = &i18n.Message{
		ID:    "CmdFormatDisk",
		One:   "Format disk",
		Other: "Format disk",
	}
	CmdFormatDiskDesc = &i18n.Message{
		ID:    "CmdFormatDiskDesc",
		One:   "Prohibit formatting disk partitions",
		Other: "Prohibit formatting disk partitions",
	}
	CmdForkBomb = &i18n.Message{
		ID:    "CmdForkBomb",
		One:   "Fork bomb",
		Other: "Fork bomb",
	}
	CmdForkBombDesc = &i18n.Message{
		ID:    "CmdForkBombDesc",
		One:   "Prohibit fork bomb attacks",
		Other: "Prohibit fork bomb attacks",
	}
	CmdSystemReboot = &i18n.Message{
		ID:    "CmdSystemReboot",
		One:   "System reboot shutdown",
		Other: "System reboot shutdown",
	}
	CmdSystemRebootDesc = &i18n.Message{
		ID:    "CmdSystemRebootDesc",
		One:   "Restrict system reboot and shutdown operations",
		Other: "Restrict system reboot and shutdown operations",
	}
	CmdModifySystemFiles = &i18n.Message{
		ID:    "CmdModifySystemFiles",
		One:   "Modify critical system files",
		Other: "Modify critical system files",
	}
	CmdModifySystemFilesDesc = &i18n.Message{
		ID:    "CmdModifySystemFilesDesc",
		One:   "Restrict direct modification of system configuration files",
		Other: "Restrict direct modification of system configuration files",
	}
	CmdDropDatabase = &i18n.Message{
		ID:    "CmdDropDatabase",
		One:   "Drop database",
		Other: "Drop database",
	}
	CmdDropDatabaseDesc = &i18n.Message{
		ID:    "CmdDropDatabaseDesc",
		One:   "Restrict dropping entire databases",
		Other: "Restrict dropping entire databases",
	}
	CmdTruncateTable = &i18n.Message{
		ID:    "CmdTruncateTable",
		One:   "Truncate table data",
		Other: "Truncate table data",
	}
	CmdTruncateTableDesc = &i18n.Message{
		ID:    "CmdTruncateTableDesc",
		One:   "Restrict clearing all table data",
		Other: "Restrict clearing all table data",
	}
	CmdModifyPermissions = &i18n.Message{
		ID:    "CmdModifyPermissions",
		One:   "Modify user permissions",
		Other: "Modify user permissions",
	}
	CmdModifyPermissionsDesc = &i18n.Message{
		ID:    "CmdModifyPermissionsDesc",
		One:   "Restrict modifying root directory permissions",
		Other: "Restrict modifying root directory permissions",
	}
	CmdDropTable = &i18n.Message{
		ID:    "CmdDropTable",
		One:   "Drop table",
		Other: "Drop table",
	}
	CmdDropTableDesc = &i18n.Message{
		ID:    "CmdDropTableDesc",
		One:   "Warning: Dropping database table",
		Other: "Warning: Dropping database table",
	}
	CmdServiceControl = &i18n.Message{
		ID:    "CmdServiceControl",
		One:   "Service control commands",
		Other: "Service control commands",
	}
	CmdServiceControlDesc = &i18n.Message{
		ID:    "CmdServiceControlDesc",
		One:   "Warning: Operating on system services",
		Other: "Warning: Operating on system services",
	}
	CmdNetworkConfig = &i18n.Message{
		ID:    "CmdNetworkConfig",
		One:   "Network configuration modification",
		Other: "Network configuration modification",
	}
	CmdNetworkConfigDesc = &i18n.Message{
		ID:    "CmdNetworkConfigDesc",
		One:   "Warning: Modifying network configuration",
		Other: "Warning: Modifying network configuration",
	}
	CmdUserManagement = &i18n.Message{
		ID:    "CmdUserManagement",
		One:   "User management",
		Other: "User management",
	}
	CmdUserManagementDesc = &i18n.Message{
		ID:    "CmdUserManagementDesc",
		One:   "Warning: Performing user management operations",
		Other: "Warning: Performing user management operations",
	}
	CmdKernelModule = &i18n.Message{
		ID:    "CmdKernelModule",
		One:   "Kernel module operations",
		Other: "Kernel module operations",
	}
	CmdKernelModuleDesc = &i18n.Message{
		ID:    "CmdKernelModuleDesc",
		One:   "Warning: Operating on kernel modules",
		Other: "Warning: Operating on kernel modules",
	}

	// Predefined command templates
	TmplBasicSecurity = &i18n.Message{
		ID:    "TmplBasicSecurity",
		One:   "Basic Security Protection",
		Other: "Basic Security Protection",
	}
	TmplBasicSecurityDesc = &i18n.Message{
		ID:    "TmplBasicSecurityDesc",
		One:   "Basic security command restrictions for all production environments",
		Other: "Basic security command restrictions for all production environments",
	}
	TmplDatabaseProtection = &i18n.Message{
		ID:    "TmplDatabaseProtection",
		One:   "Production Database Protection",
		Other: "Production Database Protection",
	}
	TmplDatabaseProtectionDesc = &i18n.Message{
		ID:    "TmplDatabaseProtectionDesc",
		One:   "Security policies to protect production databases",
		Other: "Security policies to protect production databases",
	}
	TmplServiceRestrictions = &i18n.Message{
		ID:    "TmplServiceRestrictions",
		One:   "System Service Control Restrictions",
		Other: "System Service Control Restrictions",
	}
	TmplServiceRestrictionsDesc = &i18n.Message{
		ID:    "TmplServiceRestrictionsDesc",
		One:   "Restrictions on critical system service operations",
		Other: "Restrictions on critical system service operations",
	}
	TmplNetworkSecurity = &i18n.Message{
		ID:    "TmplNetworkSecurity",
		One:   "Network Security Control",
		Other: "Network Security Control",
	}
	TmplNetworkSecurityDesc = &i18n.Message{
		ID:    "TmplNetworkSecurityDesc",
		One:   "Security controls for network configuration and user permissions",
		Other: "Security controls for network configuration and user permissions",
	}
	TmplDevEnvironment = &i18n.Message{
		ID:    "TmplDevEnvironment",
		One:   "Development Environment Basic Restrictions",
		Other: "Development Environment Basic Restrictions",
	}
	TmplDevEnvironmentDesc = &i18n.Message{
		ID:    "TmplDevEnvironmentDesc",
		One:   "Minimal security restrictions for development environments",
		Other: "Minimal security restrictions for development environments",
	}

	// Command tags
	TagDangerous = &i18n.Message{
		ID:    "TagDangerous",
		One:   "dangerous",
		Other: "dangerous",
	}
	TagDelete = &i18n.Message{
		ID:    "TagDelete",
		One:   "delete",
		Other: "delete",
	}
	TagSystem = &i18n.Message{
		ID:    "TagSystem",
		One:   "system",
		Other: "system",
	}
	TagSystemDirs = &i18n.Message{
		ID:    "TagSystemDirs",
		One:   "system-dirs",
		Other: "system-dirs",
	}
	TagDisk = &i18n.Message{
		ID:    "TagDisk",
		One:   "disk",
		Other: "disk",
	}
	TagDestruction = &i18n.Message{
		ID:    "TagDestruction",
		One:   "destruction",
		Other: "destruction",
	}
	TagFormat = &i18n.Message{
		ID:    "TagFormat",
		One:   "format",
		Other: "format",
	}
	TagAttack = &i18n.Message{
		ID:    "TagAttack",
		One:   "attack",
		Other: "attack",
	}
	TagResourceExhaustion = &i18n.Message{
		ID:    "TagResourceExhaustion",
		One:   "resource-exhaustion",
		Other: "resource-exhaustion",
	}
	TagReboot = &i18n.Message{
		ID:    "TagReboot",
		One:   "reboot",
		Other: "reboot",
	}
	TagShutdown = &i18n.Message{
		ID:    "TagShutdown",
		One:   "shutdown",
		Other: "shutdown",
	}
	TagEdit = &i18n.Message{
		ID:    "TagEdit",
		One:   "edit",
		Other: "edit",
	}
	TagSystemFiles = &i18n.Message{
		ID:    "TagSystemFiles",
		One:   "system-files",
		Other: "system-files",
	}
	TagConfig = &i18n.Message{
		ID:    "TagConfig",
		One:   "config",
		Other: "config",
	}
	TagDatabase = &i18n.Message{
		ID:    "TagDatabase",
		One:   "database",
		Other: "database",
	}
	TagDrop = &i18n.Message{
		ID:    "TagDrop",
		One:   "drop",
		Other: "drop",
	}
	TagClear = &i18n.Message{
		ID:    "TagClear",
		One:   "clear",
		Other: "clear",
	}
	TagTruncate = &i18n.Message{
		ID:    "TagTruncate",
		One:   "truncate",
		Other: "truncate",
	}
	TagPermissions = &i18n.Message{
		ID:    "TagPermissions",
		One:   "permissions",
		Other: "permissions",
	}
	TagChmod = &i18n.Message{
		ID:    "TagChmod",
		One:   "chmod",
		Other: "chmod",
	}
	TagSecurity = &i18n.Message{
		ID:    "TagSecurity",
		One:   "security",
		Other: "security",
	}
	TagTable = &i18n.Message{
		ID:    "TagTable",
		One:   "table",
		Other: "table",
	}
	TagService = &i18n.Message{
		ID:    "TagService",
		One:   "service",
		Other: "service",
	}
	TagSystemctl = &i18n.Message{
		ID:    "TagSystemctl",
		One:   "systemctl",
		Other: "systemctl",
	}
	TagControl = &i18n.Message{
		ID:    "TagControl",
		One:   "control",
		Other: "control",
	}
	TagNetwork = &i18n.Message{
		ID:    "TagNetwork",
		One:   "network",
		Other: "network",
	}
	TagFirewall = &i18n.Message{
		ID:    "TagFirewall",
		One:   "firewall",
		Other: "firewall",
	}
	TagRouting = &i18n.Message{
		ID:    "TagRouting",
		One:   "routing",
		Other: "routing",
	}
	TagUser = &i18n.Message{
		ID:    "TagUser",
		One:   "user",
		Other: "user",
	}
	TagManagement = &i18n.Message{
		ID:    "TagManagement",
		One:   "management",
		Other: "management",
	}
	TagKernel = &i18n.Message{
		ID:    "TagKernel",
		One:   "kernel",
		Other: "kernel",
	}
	TagModule = &i18n.Message{
		ID:    "TagModule",
		One:   "module",
		Other: "module",
	}
)
