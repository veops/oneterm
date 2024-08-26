package sshsrv

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gin-gonic/gin"
	"github.com/gliderlabs/ssh"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"github.com/veops/oneterm/acl"
	"github.com/veops/oneterm/api/controller"
	"github.com/veops/oneterm/conf"
	mysql "github.com/veops/oneterm/db"
	"github.com/veops/oneterm/logger"
	"github.com/veops/oneterm/model"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

const (
	hotPink  = lipgloss.Color("#FF06B7")
	darkGray = lipgloss.Color("#767676")
)

var (
	inputStyle = lipgloss.NewStyle().Foreground(hotPink)
	hintStyle  = lipgloss.NewStyle().Foreground(darkGray)
)

type view struct {
	Ctx       *gin.Context
	Sess      ssh.Session
	Pty       *ssh.Pty
	cmds      []string
	id        string
	textinput textinput.Model
	err       error
	cmdsIdx   int
	combines  map[string][2]int
}

func initialView(ctx *gin.Context, sess ssh.Session) view {
	pty, _, _ := sess.Pty()
	ti := textinput.New()
	ti.Placeholder = "ssh"
	ti.Focus()
	ti.Width = pty.Window.Width
	ti.Prompt = "> "
	ti.ShowSuggestions = true
	ti.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))
	v := view{
		Ctx:       ctx,
		Sess:      sess,
		textinput: ti,
		cmds:      []string{},
		err:       nil,
	}
	v.refresh(ctx)

	return v
}

func (m view) Init() tea.Cmd {
	return tea.Batch(tea.Println(banner()))
}

func (m view) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		// hisCmd tea.Cmd
		tiCmd  tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			cmd := m.textinput.Value()
			m.textinput.Reset()
			if cmd == "" {
				return m, tea.Batch(tea.Printf("> "))
			}
			// hisCmd = tea.Printf("> %s", cmd)
			m.cmds = append(m.cmds, cmd)
			m.cmdsIdx = len(m.cmds) - 1
			if strings.HasPrefix(cmd, "ssh") {
				k := strings.TrimSpace(strings.TrimPrefix(cmd, "ssh"))
				m.Ctx.Request.URL.RawQuery = fmt.Sprintf("w=%d&h=%d", m.Pty.Window.Width, m.Pty.Window.Height)
				m.Ctx.Params = nil
				m.Ctx.Params = append(m.Ctx.Params, gin.Param{Key: "account_id", Value: cast.ToString(m.combines[k][0])})
				m.Ctx.Params = append(m.Ctx.Params, gin.Param{Key: "account_id", Value: cast.ToString(m.combines[k][1])})
				m.Ctx = m.Ctx.Copy()
				return m, tea.Exec(&connector{Ctx: m.Ctx, Sess: m.Sess}, func(err error) tea.Msg { return err })
			}
		case tea.KeyUp:
			ln := len(m.cmds)
			if ln <= 0 {
				return m, nil
			}
			m.cmdsIdx = max(0, m.cmdsIdx-1)
			m.textinput.SetValue(m.cmds[m.cmdsIdx])
		case tea.KeyDown:
			ln := len(m.cmds)
			if ln <= 0 {
				return m, nil
			}
			m.cmdsIdx = min(ln-1, m.cmdsIdx+1)
			m.textinput.SetValue(m.cmds[m.cmdsIdx])
		}
	case error:
		return m, tea.Batch(tea.Printf(msg.Error()))
	}
	m.textinput, tiCmd = m.textinput.Update(msg)

	return m, tea.Batch(tiCmd)
}

func (m view) View() string {
	return fmt.Sprintf(
		"%s\n%s",
		m.textinput.View(),
		hintStyle.Render(m.possible()),
	) + "\n\n"
}

func (m *view) possible() string {
	cur := m.textinput
	res := lo.Filter(cur.AvailableSuggestions(), func(s string, _ int) bool {
		return strings.HasPrefix(strings.ToLower(s), strings.ToLower(cur.Value()))
	})
	return lo.Ternary(len(res) == 0, "", fmt.Sprintf("%s", res))
}

func (m *view) refresh(ctx *gin.Context) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	auths := make([]*model.Authorization, 0)
	assets := make([]*model.Asset, 0)
	accounts := make([]*model.Account, 0)
	dbAuth := mysql.DB.Model(auths)
	dbAsset := mysql.DB.Model(assets)
	dbAccount := mysql.DB.Model(accounts)

	if !acl.IsAdmin(currentUser) {
		rs, err := acl.GetRoleResources(ctx, currentUser.Acl.Rid, conf.GetResourceTypeName(conf.RESOURCE_AUTHORIZATION))
		if err != nil {
			logger.L().Error("auths", zap.Error(err))
			return
		}
		dbAuth = dbAuth.Where("resource_id IN ?", lo.Map(rs, func(r *acl.Resource, _ int) int { return r.ResourceId }))
	}
	if err := dbAuth.Find(&auths).Error; err != nil {
		logger.L().Error("auths", zap.Error(err))
		return
	}
	dbAccount = dbAccount.Where("id IN ?", lo.Map(auths, func(a *model.Authorization, _ int) int { return a.AccountId }))
	dbAsset = dbAsset.Where("id IN ?", lo.Map(auths, func(a *model.Authorization, _ int) int { return a.AssetId }))

	eg := &errgroup.Group{}
	eg.Go(func() error {
		return dbAsset.Find(&assets).Error
	})
	eg.Go(func() error {
		return dbAccount.Find(&accounts).Error
	})
	if err := eg.Wait(); err != nil {
		logger.L().Error("refresh failed", zap.Error(err))
		return
	}

	assetMap := lo.SliceToMap(assets, func(a *model.Asset) (int, *model.Asset) { return a.Id, a })
	accountMap := lo.SliceToMap(accounts, func(a *model.Account) (int, *model.Account) { return a.Id, a })

	combines := make(map[string][2]int)
	for _, auth := range auths {
		asset, ok := assetMap[auth.AssetId]
		if !ok || !lo.ContainsBy(asset.Protocols, func(p string) bool { return strings.HasPrefix(p, "ssh") }) {
			continue
		}
		account, ok := accountMap[auth.AccountId]
		if !ok {
			continue
		}
		combines[fmt.Sprintf("ssh %s@%s", account.Name, asset.Name)] = [2]int{account.Id, asset.Id}
	}
	m.combines = combines

	fmt.Println(lo.Keys(m.combines))

	m.textinput.SetSuggestions(lo.Keys(m.combines))
}

type connector struct {
	Ctx  *gin.Context
	Sess ssh.Session

	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

func (conn *connector) SetStdin(r io.Reader) {
	conn.stdin = r
}

func (conn *connector) SetStdout(w io.Writer) {
	conn.stdout = w
}

func (conn *connector) SetStderr(w io.Writer) {
	conn.stderr = w
}

func (conn *connector) Run() error {
	gsess, err := controller.DoConnect(conn.Ctx)
	if err != nil {
		return err
	}

	eg := errgroup.Group{}
	eg.Go(func() error {
		_, err := io.Copy(gsess.Chans.Win, conn.stdin)
		return err
	})
	eg.Go(func() error {
		_, ch, _ := conn.Sess.Pty()
		for {
			select {
			case <-conn.Ctx.Done():
				return nil
			case w := <-ch:
				gsess.Chans.WindowChan <- w
			}
		}
	})
	eg.Go(func() error {
		_, err := io.Copy(conn.stdout, gsess.Chans.Rout)
		return err
	})

	if err = eg.Wait(); err != nil {
		logger.L().Error("connector run failed", zap.Error(err))
	}

	return err
}
