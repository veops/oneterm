// Package handler
/**
Copyright (c) The Authors.
* @Author: feng.xiang
* @Date: 2023/12/13 09:50
* @Desc:
*/
package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/c-bata/go-prompt"
	"github.com/chzyer/readline"
	gossh "github.com/gliderlabs/ssh"
	"github.com/mattn/go-runewidth"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/olekukonko/tablewriter"
	"github.com/patrickmn/go-cache"
	"github.com/spf13/cast"
	"github.com/veops/go-ansiterm"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"
	"golang.org/x/text/language"

	myi18n "github.com/veops/oneterm/pkg/i18n"
	"github.com/veops/oneterm/pkg/logger"
	"github.com/veops/oneterm/pkg/proto/ssh/client"
	"github.com/veops/oneterm/pkg/proto/ssh/config"
	gsession "github.com/veops/oneterm/pkg/server/global/session"
	"github.com/veops/oneterm/pkg/server/model"
	"github.com/veops/oneterm/pkg/util"
)

type InteractiveHandler struct {
	Locker *sync.RWMutex

	Session gossh.Session
	//Term      *term.Terminal
	Term      *readline.Instance
	Prompt    *prompt.Prompt
	Localizer *i18n.Localizer
	SshType   int
	pty       *gossh.Pty

	Sshd     *sshdServer
	Pty      gossh.Pty
	Language int

	Assets       []*model.Asset
	Accounts     map[int]*model.Account
	Commands     map[int]*model.Command
	HistoryInput []string

	SshClient        *ssh.Client
	SshSession       map[string]*client.Connection
	GatewayCloseChan chan struct{}

	SelectedAsset *model.Asset
	SessionReq    *gsession.SshReq

	AccountInfo *model.Account
	NeedAccount bool

	Parser *Parser

	GatewayListener   net.Listener
	MessageChan       chan string
	AccountsForSelect []*model.Account
	Cache             *cache.Cache
}

type Parser struct {
	Input      *ansiterm.ByteStream
	Output     *ansiterm.ByteStream
	InputData  []byte
	OutputData []byte
	Ps1        string
	Ps2        string
}

var (
	Bundle       = i18n.NewBundle(language.Chinese)
	TotalSession = map[string]*client.Connection{}
)

func I18nInit(path string) {
	Bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
	files, err := util.ListFiles(path)
	if err != nil {
		logger.L.Error(err.Error(), zap.String("module", "i18n"))
	}
	for _, f := range files {
		_, err = Bundle.LoadMessageFile(f)
		if err != nil {
			logger.L.Warn(err.Error(), zap.String("module", "i18n"))
		}
	}
}

func NewInteractiveHandler(s gossh.Session, ss *sshdServer, pty gossh.Pty) *InteractiveHandler {
	//t := term.NewTerminal(s, "> ")

	t, err := readline.NewEx(&readline.Config{
		Stdin:  s,
		Stdout: s,
		Prompt: ">",
	})
	if err != nil {
		logger.L.Error(err.Error())
	}

	ih := &InteractiveHandler{
		Locker:  new(sync.RWMutex),
		Term:    t,
		Session: s,
		Sshd:    ss,

		SessionReq:  &gsession.SshReq{},
		SshSession:  map[string]*client.Connection{},
		Pty:         pty,
		MessageChan: make(chan string, 128),
		Cache:       cache.New(time.Minute, time.Minute*5),
	}
	ih.Language = 1
	ih.Localizer = i18n.NewLocalizer(Bundle)
	width := 120
	height := 40
	if pty.Window.Width != 0 {
		width = pty.Window.Width
	}
	if pty.Window.Height != 0 {
		height = pty.Window.Height
	}
	ih.Parser = &Parser{
		Input:  NewParser(width, height),
		Output: NewParser(width, height),
	}

	return ih
}

func completer(d prompt.Document) []prompt.Suggest {
	// 这里可以根据用户的实时输入来动态生成建议
	suggestions := []prompt.Suggest{
		{Text: "users", Description: "Store the username"},
		{Text: "articles", Description: "Store the article text posted by user"},
	}

	// 只有当用户输入为空，或者以 'u' 开始时，才显示建议
	if d.TextBeforeCursor() == "" || d.GetWordBeforeCursorWithSpace() == "u" {
		return prompt.FilterHasPrefix(suggestions, d.GetWordBeforeCursor(), true)
	}

	// 其他情况不显示任何建议
	return []prompt.Suggest{}
}

func NewParser(width, height int) *ansiterm.ByteStream {
	screen := ansiterm.NewScreen(width, height)
	stream := ansiterm.InitByteStream(screen, false)
	stream.Attach(screen)
	return stream
}

func (i *InteractiveHandler) WatchWinSize(winChan <-chan gossh.Window) {
	for {
		select {
		case <-i.Session.Context().Done():
			return
		case win, ok := <-winChan:
			if !ok {
				return
			}
			for _, v := range i.SshSession {
				client.ResizeSshClient(v.Session, win.Height, win.Width)
				_ = v.Record.Resize(win.Height, win.Width)
			}
		}
	}
}

func (i *InteractiveHandler) SwitchLanguage(lang string) {
	languages := []string{"zh", "en"}

	switch len(lang) {
	case 0:
		length := len(languages)
		if length <= 1 {
			return
		}
		if i.Language >= length {
			i.Language = 1
		} else {
			i.Language += 1
		}
	default:
		for index, v := range languages {
			if v == lang {
				i.Language = index + 1
			}
		}
	}
	i.Localizer = i18n.NewLocalizer(Bundle, languages[i.Language-1])

}

func (i *InteractiveHandler) SwitchLang(lang string) {
	languages := []string{"zh", "en"}

	switch len(lang) {
	case 0:
		length := len(languages)
		if length <= 1 {
			return
		}
		if i.Language >= length {
			i.Language = 1
		} else {
			i.Language += 1
		}
	default:
		for index, v := range languages {
			if v == lang {
				i.Language = index + 1
			}
		}
	}
	i.Localizer = i18n.NewLocalizer(Bundle, languages[i.Language-1])
	i.PrintMessage(myi18n.MsgSshWelcome, map[string]any{"User": i.Session.User()})
}

func (i *InteractiveHandler) output(msg string) {
	_, _ = io.WriteString(i.Session, msg)
}

func (i *InteractiveHandler) HostInfo(id int) (asset *model.Asset, err error) {
	if id < 0 {
		return
	}

	cookie, ok := i.Session.Context().Value("cookie").(string)
	if !ok {
		err = fmt.Errorf("no cookie")
		return
	}
	res, er := i.Sshd.Core.Asset.Lists(cookie, "", id)
	if er != nil {
		err = er
		return
	}
	if res.Count != 1 {
		er = fmt.Errorf("found %d hosts: not unique", res.Count)
		return
	}
	bs, er := json.Marshal(res.List[0])
	if er != nil {
		err = er
		return
	}
	err = json.Unmarshal(bs, &asset)
	if err != nil {
		return
	}
	return asset, nil
}

func (i *InteractiveHandler) Check(id int, host *model.Asset) (asset *model.Asset, state bool, err error) {
	assets, er := i.AcquireAssets("", id)
	if er != nil {
		err = er
		return
	}
	if len(assets) == 0 {
		return
	}
	asset = assets[0]
	state = i.Sshd.Core.Asset.HasPermission(asset.AccessAuth)
	return
}

func (i *InteractiveHandler) generateSessionRecord(conn *client.Connection, status int) (res *model.Session, err error) {
	res = &model.Session{
		SessionType: cast.ToInt(i.Session.Context().Value("sshType")),
	}
	if i.SessionReq != nil && i.SessionReq.Uid != 0 {
		err = util.DecodeStruct(&res, i.SessionReq)
		if err != nil {
			return
		}
		res.Uid = i.SessionReq.Uid
	} else {
		res.ClientIp = i.Session.RemoteAddr().String()
	}

	res.UserName = i.Session.Context().User()
	res.AccountInfo = fmt.Sprintf("%s(%s)", i.AccountInfo.Name, i.AccountInfo.Account)

	s, er := i.Sshd.Core.Auth.AclInfo(i.Session.Context().Value("cookie").(string))
	if er != nil {
		logger.L.Warn(er.Error(), zap.String("session", "add"))
	} else if s != nil {
		res.Uid = s.Uid
		res.UserName = s.UserName
	}
	res.Status = status
	res.AssetInfo = fmt.Sprintf("%s(%s)", i.SelectedAsset.Name, i.SelectedAsset.Ip)
	res.SessionId = conn.SessionId
	res.GatewayId = i.SelectedAsset.GatewayId
	if conn.Gateway != nil {
		res.GatewayInfo = fmt.Sprintf("%s:%d", conn.Gateway.Host, conn.Gateway.Port)
	}
	res.AssetId = i.SelectedAsset.Id
	res.AccountId = i.AccountInfo.Id
	if status == model.SESSIONSTATUS_OFFLINE {
		t := time.Now()
		res.ClosedAt = &t
	}
	return
}
func readLine(s gossh.Session) string {
	buf := make([]byte, 1)
	var in []byte
	for {
		_, _ = s.Read(buf)
		switch buf[0] {
		case []byte("\r")[0], []byte("\r\n")[0]:
			return string(in)
		default:
			in = append(in, buf[0])
		}
	}
}

func (i *InteractiveHandler) Schedule(pty *gossh.Pty) {
	i.pty = pty
	var err error
	var line string
	if st, ok := i.Session.Context().Value("sshType").(int); ok && st == model.SESSIONTYPE_WEB {
		//line, err = i.Term.ReadLine()
		line = readLine(i.Session)
		if err != nil {
			logger.L.Debug("connection closed", zap.String("msg", err.Error()))
			return
		}
		var r *gsession.SshReq
		err = json.Unmarshal([]byte(line), &r)
		if err != nil {
			logger.L.Warn(err.Error())
			return
		}
		// "Accept-Language")
		//i.Localizer = i18n.NewLocalizer(conf.Bundle, lang, accept)
		i.Session.Context().SetValue("cookie", r.Cookie)
		i.SessionReq = r

		// monitor
		{
			if i.SessionReq.SessionId != "" {
				switch i.SessionReq.Action {
				case model.SESSIONACTION_MONITOR:
					i.wrapJsonResponse(i.SessionReq.SessionId, 0, "success")
					RegisterMonitorSession(i.SessionReq.SessionId, i.Session)
					return
				case model.SESSIONACTION_CLOSE:
					if v, ok := config.TotalHostSession.Load(i.SessionReq.SessionId); ok {
						err = v.(*client.Connection).Session.Close()
						if err != nil {
							logger.L.Warn(err.Error())
							i.wrapJsonResponse(i.SessionReq.SessionId, 1, "failed")
							return
						}
						close(v.(*client.Connection).Exit)
					}
					i.wrapJsonResponse(i.SessionReq.SessionId, 0, "success")
					return
				}
			}
		}

		host, ok, err := i.Check(r.AssetId, nil)
		if err != nil {
			logger.L.Warn(err.Error())
			i.wrapJsonResponse("", 1, err.Error())
			return
		}

		if !ok {
			i.wrapJsonResponse("", 1, fmt.Sprintf("invalid status for %v", r.AssetId))
			return
		}
		i.SelectedAsset = host

		commands, er := i.AcquireCommands()
		if er != nil {
			return
		}
		i.Commands = commands
		_, err = i.Proxy(host.Ip, r.AccountId)
		if err != nil {
			logger.L.Error(err.Error(), zap.String("module", "proxy"))
			i.wrapJsonResponse("", 1, err.Error())
		}
		return
	} else {
		if config.SSHConfig.PlainMode {
			i.SwitchLang("zh")
			for {
				//line, err = i.Term.ReadLine()
				line, err = i.Term.Readline()

				if err != nil {
					logger.L.Debug("connection closed", zap.String("msg", err.Error()))
					break
				}
				if strings.TrimSpace(line) == "" {
					continue
				}
				if i.HandleInput(strings.TrimSpace(line)) {
					break
				}
			}
		} else {
			tm := InitAndRunTerm(i)
			_, err := tm.Run()
			if err != nil {
				logger.L.Error(err.Error(), zap.String("module", "schedule"))
			}
		}
	}
}

func (i *InteractiveHandler) HandleInput(line string) (exit bool) {

	switch strings.TrimSpace(line) {
	case "/*":
		i.SelectedAsset = nil
		assets, er := i.AcquireAssets("", 0)
		if er != nil {
			return
		}
		accounts, er := i.AcquireAccounts()
		if er != nil {
			return
		}
		commands, er := i.AcquireCommands()
		if er != nil {
			return
		}
		i.Locker.Lock()
		i.Assets = assets
		i.Accounts = accounts
		i.Commands = commands
		i.Locker.Unlock()

		i.showResult(assets)
		return
	case "/?", "/？":
		i.PrintMessage(myi18n.MsgSshWelcome, map[string]any{"User": i.Session.User()})
		return
	case "/s":
		i.SwitchLang("")
		return
	case "/q":
		i.Session.Close()
		return
	default:
		switch {
		case line == "exit":
			logger.L.Info("exit", zap.String("user", i.Session.User()), zap.String("input", line))
			i.Session.Close()
			return
		}
	}
	_, er := i.Proxy(line, -1)
	if er != nil {
		logger.L.Info(er.Error())
	}
	if st, ok := i.Session.Context().Value("sshType").(int); ok && st == model.SESSIONTYPE_WEB {
		exit = true
	}
	return
}

func (i *InteractiveHandler) AcquireAndStoreAssets(search string, id int) (selectedHosts, likeHosts []*model.Asset, err error) {
	i.Locker.RLock()
	count := len(i.Assets)
	i.Locker.RUnlock()
	var find = func(assets []*model.Asset) (selectedHosts, likeHosts []*model.Asset) {
		if search == "" {
			return
		}
		for _, v := range assets {
			if v.Ip == search || v.Name == search {
				selectedHosts = append(selectedHosts, v)
			} else if strings.Contains(v.Ip, search) || strings.Contains(v.Name, search) {
				likeHosts = append(likeHosts, v)
			}
		}
		return
	}
	if count == 0 {
		res, er := i.AcquireAssets(search, id)
		if er != nil {
			err = er
			return
		}
		selectedHosts, likeHosts = find(res)
		i.Locker.Lock()
		i.Assets = res
		i.Locker.Unlock()
		return
	} else {
		i.Locker.Lock()
		selectedHosts, likeHosts = find(i.Assets)
		i.Locker.Unlock()
		return
	}
}

func (i *InteractiveHandler) AcquireAssets(search string, id int) (assets []*model.Asset, err error) {
	if search == "" && id <= 0 {
		if v, ok := i.Cache.Get("assets"); ok {
			return v.([]*model.Asset), nil
		} else {
			defer func() {
				if err == nil {
					i.Cache.Set("assets", assets, 0)
				}
			}()
		}
	}
	if totalAssets, ok := i.Cache.Get("assets"); ok {
		for _, v := range totalAssets.([]*model.Asset) {
			if id > 0 {
				if id == v.Id {
					assets = append(assets, v)
					return
				} else {
					continue
				}
			} else {
				if strings.Contains(v.Name, search) {
					assets = append(assets, v)
				}
			}
		}
	} else {
		cookie, ok := i.Session.Context().Value("cookie").(string)
		if ok {
			res, er := i.Sshd.Core.Asset.Lists(cookie, search, id)
			if er != nil {
				err = er
				return
			}
			if res != nil {
				for _, v := range res.List {
					var v1 model.Asset
					_ = util.DecodeStruct(&v1, v)
					bs, _ := json.Marshal(v.(map[string]interface{})["authorization"])
					er = json.Unmarshal(bs, &v1.Authorization)
					if er != nil {
						logger.L.Warn(er.Error())
					}
					assets = append(assets, &v1)
				}
			}
		} else {
			err = fmt.Errorf("no cookies")
		}
	}
	return
}

func (i *InteractiveHandler) AcquireAccounts() (accounts map[int]*model.Account, err error) {
	accounts = map[int]*model.Account{}
	cookie, ok := i.Session.Context().Value("cookie").(string)
	if ok {
		res, er := i.Sshd.Core.Auth.Accounts(cookie)
		if er != nil {
			err = er
			return
		}
		for _, v := range res {
			var v1 model.Account
			_ = util.DecodeStruct(&v1, v)
			accounts[v1.Id] = &v1
		}
	} else {
		err = fmt.Errorf("no cookies")
	}
	return
}

func (i *InteractiveHandler) AcquireAccountInfo(id int, name string) (res *model.Account, err error) {
	cookie, ok := i.Session.Context().Value("cookie").(string)
	if ok {
		return i.Sshd.Core.Auth.AccountInfo(cookie, id, name)
	} else {
		err = fmt.Errorf("no cookies")
	}
	return
}

func (i *InteractiveHandler) AcquireCommands() (commands map[int]*model.Command, err error) {
	commands = map[int]*model.Command{}
	cookie, ok := i.Session.Context().Value("cookie").(string)
	if ok {
		res, er := i.Sshd.Core.Asset.Commands(cookie)
		if er != nil {
			err = er
			return
		}
		for _, v := range res {
			var v1 model.Command
			_ = util.DecodeStruct(&v1, v)
			commands[v1.Id] = &v1
		}
	} else {
		err = fmt.Errorf("no cookies")
	}
	return
}

func (i *InteractiveHandler) AcquireConfig() (config *model.Config, err error) {
	config = &model.Config{}
	cookie, ok := i.Session.Context().Value("cookie").(string)
	if ok {
		res, er := i.Sshd.Core.Asset.Config(cookie)
		if er != nil {
			err = er
			return
		}
		config = res
	} else {
		err = fmt.Errorf("no cookies")
	}
	return
}

func (i *InteractiveHandler) showResult(data []*model.Asset) {
	i.Term.SetPrompt("host> ")
	var hosts []string
	for _, d := range data {
		hosts = append(hosts, d.Name)
	}

	var templateData = map[string]interface{}{
		"Count": len(data),
		"Msg":   "",
	}

	if data != nil {
		templateData["Msg"] = i.tableData(hosts)
	}
	i.PrintMessage(myi18n.MsgSshShowAssetResults, templateData)
}

func (i *InteractiveHandler) tableData(data []string) string {
	chunkData := i.chunkData(data)
	buf := &bytes.Buffer{}
	tw := tablewriter.NewWriter(buf)
	tw.SetAutoWrapText(false)
	tw.SetColumnSeparator(" ")
	tw.SetNoWhiteSpace(false)
	tw.SetBorder(false)
	tw.SetAlignment(tablewriter.ALIGN_LEFT)
	tw.AppendBulk(chunkData)
	tw.Render()
	return buf.String()
}

func (i *InteractiveHandler) chunkData(data []string) (res [][]string) {
	width := 80
	if i.pty != nil {
		width = i.pty.Window.Width
	}
	n := len(data)
	chunk := n
	for ; chunk >= 1; chunk -= 1 {
		ok := true
		for i := 0; i < n && ok; i += chunk {
			w := chunk*3 + 4
			r := i + chunk
			if r > n {
				r = n
			}
			for _, s := range data[i:r] {
				w += runewidth.StringWidth(s)
			}
			ok = ok && w <= width
		}
		if ok {
			t := i.getChunk(data, chunk)
			maxLen := make(map[int]int)
			for _, c := range t {
				for i, v := range c {
					l := runewidth.StringWidth(v)
					if l > maxLen[i] {
						maxLen[i] = l
					}
				}
			}
			for _, row := range t {
				w := chunk*3 + 4
				for i := range row {
					w += maxLen[i]
				}
				ok = ok && w <= width
			}

		}
		if ok {
			break
		}
	}
	if chunk < 1 {
		chunk = 1
	}
	res = i.getChunk(data, chunk)

	return
}

func (i *InteractiveHandler) getChunk(data []string, chunk int) (res [][]string) {
	n := len(data)
	for i := 0; i < n; i += chunk {
		r := i + chunk
		if r > n {
			r = n
		}
		res = append(res, data[i:r])
	}
	return
}

func (i *InteractiveHandler) wrapJsonResponse(sessionId string, code int, message string) {
	if st, ok := i.Session.Context().Value("sshType").(int); ok && st != model.SESSIONTYPE_WEB {
		return
	}
	res, er := json.Marshal(gsession.ServerResp{
		Code:      code,
		Message:   message,
		SessionId: sessionId,
		Uid:       i.SessionReq.Uid,
		UserName:  i.SessionReq.UserName,
	})

	if er != nil {
		logger.L.Error(er.Error())
	}
	i.output(string(append(res, []byte("\r")...)))
}

func (i *InteractiveHandler) NewSession(account *model.Account, gateway *model.Gateway) (conn *client.Connection, err error) {
	i.Locker.Lock()
	defer i.Locker.Unlock()
	if i.SshClient == nil {
		protocol := i.SessionReq.Protocol
		if strings.HasPrefix(i.SessionReq.Protocol, "ssh:") {
			protocol = "ssh:" + getSshPort(i.SessionReq.Protocol)
		}
		con, ch, er := client.NewSShClient(strings.ReplaceAll(protocol, "ssh", i.SelectedAsset.Ip), account, gateway)
		if er != nil {
			err = er
			return
		}
		i.SshClient = con
		i.GatewayCloseChan = ch
	}
	i.AccountInfo = account

	conn, err = client.NewSShSession(i.SshClient, i.Pty, i.GatewayCloseChan)

	if err != nil {
		return
	}
	conn.AssetId = i.SelectedAsset.Id
	conn.AccountId = account.Id
	conn.Gateway = gateway
	i.SshSession[conn.SessionId] = conn

	return
}

func (i *InteractiveHandler) UpsertSession(conn *client.Connection, status int) error {
	resp, err := i.generateSessionRecord(conn, status)
	if err != nil {
		return err
	}

	return i.Sshd.Core.Audit.NewSession(resp)
}
