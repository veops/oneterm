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
)
