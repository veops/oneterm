package handler

import (
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"go.uber.org/zap"

	"github.com/veops/oneterm/pkg/logger"
	"github.com/veops/oneterm/pkg/proto/ssh/config"
)

func (i *InteractiveHandler) PrintMessage(msg *i18n.Message, data any) {
	if config.SSHConfig.PlainMode {
		i.output("\r\n" + i.Message(msg, data))
	} else {
		i.MessageChan <- i.Message(msg, data)
	}
}

func (i *InteractiveHandler) PrintMessageV1(msg *i18n.Message, data any) {
	i.output(i.Message(msg, data))
}

func (i *InteractiveHandler) Message(msg *i18n.Message, data any) string {
	str, er := i.Localizer.Localize(&i18n.LocalizeConfig{
		DefaultMessage: msg,
		TemplateData:   data,
		PluralCount:    1,
	})
	if er != nil {
		logger.L.Warn(er.Error(), zap.String("module", "i18n"))
		return ""
	}
	return str
}
