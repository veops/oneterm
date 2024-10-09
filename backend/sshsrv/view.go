package sshsrv

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/gin-gonic/gin"
	"github.com/gliderlabs/ssh"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/veops/oneterm/acl"
	"github.com/veops/oneterm/api/controller"
	redis "github.com/veops/oneterm/cache"
	"github.com/veops/oneterm/logger"
	"github.com/veops/oneterm/model"
	"github.com/veops/oneterm/session"
	"github.com/veops/oneterm/sshsrv/textinput"
	"github.com/veops/oneterm/util"
)

const (
	prompt     = "> "
	hotPink    = lipgloss.Color("#FF06B7")
	darkGray   = lipgloss.Color("#767676")
	hisCmdsFmt = "hiscmds-%d"
)

var (
	errStyle     = lipgloss.NewStyle().Foreground(hotPink)
	hintStyle    = lipgloss.NewStyle().Foreground(darkGray)
	hiddenBorder = lipgloss.HiddenBorder()
)

func init() {
	hiddenBorder.Left = "  "
}

type errMsg error

type keymap struct{}

func (k keymap) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "up")),
		key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "down")),
		key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "complete")),
		key.NewBinding(key.WithKeys("f5"), key.WithHelp("F5", "refresh")),
		key.NewBinding(key.WithKeys("esc", "ctrl+c"), key.WithHelp("esc/ctrl+c", "quit")),
	}
}
func (k keymap) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}

type view struct {
	Ctx         *gin.Context
	Sess        ssh.Session
	currentUser *acl.Session
	textinput   textinput.Model
	cmds        []string
	cmdsIdx     int
	combines    map[string][3]int
	connecting  bool
	help        help.Model
	keys        keymap
	r           io.ReadCloser
	w           io.WriteCloser
	gctx        context.Context
}

func initialView(ctx *gin.Context, sess ssh.Session, r io.ReadCloser, w io.WriteCloser, gctx context.Context) *view {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	ti := textinput.New()
	ti.Placeholder = "ssh"
	ti.Focus()
	ti.Prompt = prompt
	ti.ShowSuggestions = true
	ti.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))
	v := view{
		Ctx:         ctx,
		Sess:        sess,
		currentUser: currentUser,
		textinput:   ti,
		cmds:        []string{},
		help:        help.New(),
		r:           r,
		w:           w,
		gctx:        gctx,
	}
	v.refresh()

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
			m.cmdsIdx = len(m.cmds)
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
				return m, tea.Sequence(hisCmd, tea.Exec(&connector{Ctx: m.Ctx, Sess: m.Sess, Vw: m, gctx: m.gctx}, func(err error) tea.Msg {
					m.connecting = false
					return err
				}), tea.Printf("%s", prompt), func() tea.Msg {
					m.textinput.ClearMatched()
					return nil
				}, m.magicn)
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
		case tea.KeyF5:
			m.refresh()
		}
	case errMsg:
		if msg != nil {
			str := msg.Error()
			if ae, ok := msg.(*controller.ApiError); ok {
				str = controller.Err2Msg[ae.Code].One
			}
			return m, tea.Printf("  [ERROR] %s\n\n", errStyle.Render(str))
		}
	}
	m.textinput, tiCmd = m.textinput.Update(msg)

	return m, tea.Batch(hisCmd, tiCmd)
}

func (m *view) View() string {
	if m.connecting {
		return "\n\n"
	}
	return fmt.Sprintf(
		"%s\n  %s\n%s",
		m.textinput.View(),
		m.help.View(m.keys),
		hintStyle.Render(m.possible()),
	) + "\n\n"
}

func (m *view) possible() string {
	ss := m.textinput.MatchedSuggestions()
	ln := len(ss)
	if ln <= 0 {
		return ""
	}
	ss = append(ss[:min(ln, 15)], lo.Ternary(ln > 15, fmt.Sprintf("%d more...", ln-15), ""))
	mw := 0
	for _, s := range ss {
		mw = max(mw, lipgloss.Width(s))
	}
	pty, _, _ := m.Sess.Pty()
	n := 1
	for i := 2; i*mw+(i+1)*1 < pty.Window.Width; i++ {
		n = i
	}
	tb := table.New().
		Border(hiddenBorder).
		StyleFunc(func(row, col int) lipgloss.Style { return hintStyle }).
		Rows(lo.Chunk(ss, n)...)
	return tb.Render()
}

func (m *view) refresh() {
	eg := &errgroup.Group{}
	eg.Go(func() (err error) {
		assets, err := util.GetAllFromCacheDb(m.gctx, model.DefaultAsset)
		if err != nil {
			return
		}
		accounts, err := util.GetAllFromCacheDb(m.gctx, model.DefaultAccount)
		if err != nil {
			return
		}
		if !acl.IsAdmin(m.currentUser) {
			var assetIds, accountIds []int
			if assetIds, err = controller.GetAssetIdsByAuthorization(m.Ctx); err != nil {
				return
			}
			assets = lo.Filter(assets, func(a *model.Asset, _ int) bool { return lo.Contains(assetIds, a.Id) })

			if accountIds, err = controller.GetAccountIdsByAuthorization(m.Ctx); err != nil {
				return
			}
			accounts = lo.Filter(accounts, func(a *model.Account, _ int) bool { return lo.Contains(accountIds, a.Id) })
		}

		accountMap := lo.SliceToMap(accounts, func(a *model.Account) (int, *model.Account) { return a.Id, a })

		m.combines = make(map[string][3]int)
		for _, asset := range assets {
			for accountId, _ := range asset.Authorization {
				account, ok := accountMap[accountId]
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
		}
		m.textinput.SetSuggestions(lo.Keys(m.combines))

		return
	})

	eg.Go(func() error {
		var err error
		if len(m.cmds) != 0 {
			return err
		}
		m.cmds, err = redis.RC.LRange(m.Ctx, fmt.Sprintf(hisCmdsFmt, m.currentUser.GetUid()), -100, -1).Result()
		m.cmdsIdx = len(m.cmds)
		return err
	})

	if err := eg.Wait(); err != nil {
		logger.L().Error("refresh failed", zap.Error(err))
		return
	}

}

func (m *view) magicn() tea.Msg {
	m.w.Write([]byte("\n"))
	return nil
}

func (m *view) RecordHisCmd() {
	k := fmt.Sprintf(hisCmdsFmt, m.currentUser.GetUid())
	redis.RC.RPush(m.Ctx, k, m.cmds)
	redis.RC.LTrim(m.Ctx, k, -100, -1)
	redis.RC.Expire(m.Ctx, k, time.Hour*24*30)
}

type connector struct {
	Ctx    *gin.Context
	Sess   ssh.Session
	Vw     *view
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
	gctx   context.Context
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
	gsess, err := controller.DoConnect(conn.Ctx, nil)
	if err != nil {
		return err
	}

	conn.Vw.magicn()

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
			case <-conn.gctx.Done():
				close(gsess.Chans.AwayChan)
				return nil
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
