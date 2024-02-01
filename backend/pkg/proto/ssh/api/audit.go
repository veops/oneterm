package api

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/go-resty/resty/v2"
	"go.uber.org/zap"

	"github.com/veops/oneterm/pkg/logger"
	cfg "github.com/veops/oneterm/pkg/proto/ssh/config"
	"github.com/veops/oneterm/pkg/server/controller"
	"github.com/veops/oneterm/pkg/util"
)

type AuditCore struct {
	Api    string
	XToken string
}

func NewAuditServer(Api, token string) *Auth {
	return &Auth{
		Api:    Api,
		XToken: token,
	}
}

type CommandLevel int

const (
	CommandLevelNormal = iota + 1
	CommandLevelReject
)

func (a *AuditCore) NewSession(data any) error {
	_, err := request(resty.MethodPost,
		fmt.Sprintf("%s%s", strings.TrimSuffix(cfg.SSHConfig.Api, "/"), sessionUrl),
		map[string]string{
			"X-Token": cfg.SSHConfig.Token,
		}, nil, data)

	return err
}

func (a *AuditCore) AddCommand(data any) {
	_, err := request(resty.MethodPost,
		fmt.Sprintf("%s%s", strings.TrimSuffix(cfg.SSHConfig.Api, "/"), sessionCmdUrl),
		map[string]string{
			"X-Token":      cfg.SSHConfig.Token,
			"Content-Type": "application/json",
		}, nil, data)
	if err != nil {
		logger.L.Error(err.Error())
	}
}

func AddReplay(sessionId string, data any) error {
	_, err := request(resty.MethodPost,
		fmt.Sprintf("%s%s/%s", strings.TrimSuffix(cfg.SSHConfig.Api, "/"), replayUrl, sessionId),
		map[string]string{
			"X-Token": cfg.SSHConfig.Token,
		}, nil, data)

	return err
}

func AddReplayFile(sessionId, filePath string) (err error) {
	var response *controller.HttpResponse
	r, er := resty.New().R().SetFile("replay.cast", filePath).
		SetHeader("X-Token", cfg.SSHConfig.Token).
		SetHeader("Content-Type", "application/json").
		SetBody(map[string]any{
			"session_id": sessionId,
			"body":       "",
		}).SetResult(&response).Post(fmt.Sprintf("%s%s/%s", strings.TrimSuffix(cfg.SSHConfig.Api, "/"), replayFileUrl, sessionId))
	if er != nil {
		err = er
		return err
	}
	if r.StatusCode() != 200 {
		err = fmt.Errorf("auth code: %d: %s", r.StatusCode(), r.String())
		return
	}
	if response.Code != 0 {
		err = fmt.Errorf(response.Message)
		return
	}
	err = util.DecodeStruct(&response, response.Data)
	return err
}

func GetCommandLevel(command string, commands []string) CommandLevel {
	for _, v := range commands {
		pattern, err := regexp.Compile(v)
		if err != nil {
			logger.L.Warn(err.Error(), zap.String("module", ""))
			continue
		}
		if pattern.MatchString(command) {
			return CommandLevelReject
		}
	}
	return CommandLevelNormal
}
