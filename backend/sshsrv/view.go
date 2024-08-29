package sshsrv

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gin-gonic/gin"
	"github.com/gliderlabs/ssh"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/veops/oneterm/acl"
	"github.com/veops/oneterm/api/controller"
	"github.com/veops/oneterm/conf"
	mysql "github.com/veops/oneterm/db"
	"github.com/veops/oneterm/logger"
	"github.com/veops/oneterm/model"
	"github.com/veops/oneterm/session"
)

const (
	prompt   = "> "
	hotPink  = lipgloss.Color("#FF06B7")
	darkGray = lipgloss.Color("#767676")
)

var (
	errStyle  = lipgloss.NewStyle().Foreground(hotPink)
	hintStyle = lipgloss.NewStyle().Foreground(darkGray)
)

type errMsg error

type keymap struct{}

func (k keymap) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "up")),
		key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "down")),
		// key.NewBinding(key.WithKeys("left"), key.WithHelp("←", "prev")),
		// key.NewBinding(key.WithKeys("right"), key.WithHelp("→", "next")),
		key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "complete")),
		key.NewBinding(key.WithKeys("esc", "ctrl+c"), key.WithHelp("esc/ctrl+c", "quit")),
	}
}
func (k keymap) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}

type view struct {
	Ctx        *gin.Context
	Sess       ssh.Session
	textinput  textinput.Model
	cmds       []string
	cmdsIdx    int
	combines   map[string][3]int
	connecting bool
	help       help.Model
	keys       keymap
}

func initialView(ctx *gin.Context, sess ssh.Session) *view {
	ti := textinput.New()
	ti.Placeholder = "ssh"
	ti.Focus()
	ti.Prompt = prompt
	ti.ShowSuggestions = true
	ti.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))
	v := view{
		Ctx:       ctx,
		Sess:      sess,
		textinput: ti,
		cmds:      []string{},
		help:      help.New(),
	}
	ti.KeyMap.NextSuggestion = key.NewBinding(key.WithKeys(""))
	ti.KeyMap.PrevSuggestion = key.NewBinding(key.WithKeys(""))
	v.refresh(ctx)

	return &v
}

func (m *view) Init() tea.Cmd {
	return tea.Println(banner())
}

func (m *view) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		hisCmd tea.Cmd
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
				return m, tea.Batch(tea.Printf(prompt))
			}
			hisCmd = tea.Printf("> %s", cmd)
			m.cmds = append(m.cmds, cmd)
			ln := len(m.cmds)
			if ln > 100 {
				m.cmds = m.cmds[ln-100 : ln]
			}
			m.cmdsIdx = len(m.cmds) - 1
			if cmd == "exit" {
				return m, tea.Sequence(hisCmd, tea.Quit)
			} else if strings.HasPrefix(cmd, "ssh") {
				pty, _, _ := m.Sess.Pty()
				m.Ctx.Request.URL.RawQuery = fmt.Sprintf("w=%d&h=%d", pty.Window.Width, pty.Window.Height)
				m.Ctx.Params = nil
				m.Ctx.Params = append(m.Ctx.Params, gin.Param{Key: "account_id", Value: cast.ToString(m.combines[cmd][0])})
				m.Ctx.Params = append(m.Ctx.Params, gin.Param{Key: "asset_id", Value: cast.ToString(m.combines[cmd][1])})
				m.Ctx.Params = append(m.Ctx.Params, gin.Param{Key: "protocol", Value: fmt.Sprintf("ssh:%d", m.combines[cmd][2])})
				m.Ctx = m.Ctx.Copy()
				m.connecting = true
				return m, tea.Sequence(hisCmd, tea.Exec(&connector{Ctx: m.Ctx, Sess: m.Sess}, func(err error) tea.Msg {
					m.connecting = false
					return err
				}), tea.Printf("%s", prompt))
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
			m.cmdsIdx++
			if m.cmdsIdx >= ln {
				m.cmdsIdx = ln - 1
				m.textinput.SetValue("")
			} else {
				m.textinput.SetValue(m.cmds[m.cmdsIdx])
			}
		}
	case errMsg:
		if msg != nil {
			return m, tea.Printf("  %s", errStyle.Render(msg.Error()))
		}
		return m, nil
	}
	m.textinput, tiCmd = m.textinput.Update(msg)

	return m, tea.Batch(hisCmd, tiCmd)
}

func (m *view) View() string {
	if m.connecting {
		return "\n\n"
	}
	return fmt.Sprintf(
		"%s\n  %s\n  %s",
		m.textinput.View(),
		m.help.View(m.keys),
		hintStyle.Render(m.possible()),
	) + "\n\n"
}

func (m *view) possible() string {
	cur := m.textinput
	res := lo.Filter(cur.AvailableSuggestions(), func(s string, _ int) bool {
		return cur.Value() != "" && strings.HasPrefix(strings.ToLower(s), strings.ToLower(cur.Value()))
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

	m.combines = make(map[string][3]int)
	for _, auth := range auths {
		asset, ok := assetMap[auth.AssetId]
		if !ok {
			continue
		}
		account, ok := accountMap[auth.AccountId]
		if !ok {
			continue
		}
		k := fmt.Sprintf("ssh %s@%s", account.Name, asset.Name)
		for _, p := range asset.Protocols {
			if strings.HasPrefix(p, "ssh") {
				ss := strings.Split(p, ":")
				port := cast.ToInt(ss[1])
				if len(ss) != 2 || port == 0 {
					continue
				}
				m.combines[lo.Ternary(port == 22, k, fmt.Sprintf("%s:%s", k, ss[1]))] = [3]int{account.Id, asset.Id, port}
			}
		}
	}

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

	r, w := io.Pipe()
	go func() {
		_, err := io.Copy(w, conn.stdin)
		gsess.Chans.ErrChan <- err
	}()

	gsess.CliRw = &session.CliRW{
		Reader: bufio.NewReader(r),
		Writer: conn.stdout,
	}

	_, ch, _ := conn.Sess.Pty()
	gsess.G.Go(func() error {
		defer r.Close()
		defer w.Close()
		for {
			select {
			case <-gsess.Gctx.Done():
				return nil
			case w := <-ch:
				gsess.Chans.WindowChan <- w
			}
		}
	})
	controller.HandleSsh(gsess)

	gsess.G.Wait()

	conn.stdout.Write([]byte("\n\n"))

	return nil
}
