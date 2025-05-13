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

	"github.com/veops/oneterm/internal/acl"
	"github.com/veops/oneterm/internal/api/controller"
	myConnector "github.com/veops/oneterm/internal/connector"
	"github.com/veops/oneterm/internal/connector/protocols"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/repository"
	"github.com/veops/oneterm/internal/service"
	"github.com/veops/oneterm/internal/session"
	"github.com/veops/oneterm/internal/sshsrv/textinput"
	"github.com/veops/oneterm/pkg/cache"
	"github.com/veops/oneterm/pkg/errors"
	"github.com/veops/oneterm/pkg/logger"
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

	p2p = map[string]int{
		"ssh":   22,
		"redis": 6379,
		"mysql": 3306,
	}
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
			} else if p, ok := lo.Find(lo.Keys(p2p), func(item string) bool { return strings.HasPrefix(cmd, item) }); ok {
				pty, _, _ := m.Sess.Pty()
				m.Ctx.Request.URL.RawQuery = fmt.Sprintf("w=%d&h=%d", pty.Window.Width, pty.Window.Height)
				m.Ctx.Params = nil
				m.Ctx.Params = append(m.Ctx.Params, gin.Param{Key: "account_id", Value: cast.ToString(m.combines[cmd][0])})
				m.Ctx.Params = append(m.Ctx.Params, gin.Param{Key: "asset_id", Value: cast.ToString(m.combines[cmd][1])})
				m.Ctx.Params = append(m.Ctx.Params, gin.Param{Key: "protocol", Value: fmt.Sprintf("%s:%d", p, m.combines[cmd][2])})
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
			if ae, ok := msg.(*errors.ApiError); ok {
				str = errors.Err2Msg[ae.Code].One
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
		assets, err := repository.GetAllFromCacheDb(m.gctx, model.DefaultAsset)
		if err != nil {
			return
		}
		accounts, err := repository.GetAllFromCacheDb(m.gctx, model.DefaultAccount)
		if err != nil {
			return
		}
		if !acl.IsAdmin(m.currentUser) {
			var assetIds, accountIds []int
			if _, assetIds, _, err = service.NewAssetService().GetAssetIdsByAuthorization(m.Ctx); err != nil {
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
			for accountId := range asset.Authorization {
				account, ok := accountMap[accountId]
				if !ok {
					continue
				}
				for _, p := range asset.Protocols {
					ss := strings.Split(p, ":")
					if len(ss) != 2 {
						continue
					}
					protocol := ss[0]
					defaultPort, ok := p2p[protocol]
					if !ok {
						continue
					}
					k := fmt.Sprintf("%s %s@%s", protocol, account.Name, asset.Name)
					port := cast.ToInt(ss[1])
					m.combines[lo.Ternary(port == defaultPort, k, fmt.Sprintf("%s:%s", k, ss[1]))] = [3]int{account.Id, asset.Id, port}
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
		m.cmds, err = cache.RC.LRange(m.Ctx, fmt.Sprintf(hisCmdsFmt, m.currentUser.GetUid()), -100, -1).Result()
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
	cache.RC.RPush(m.Ctx, k, m.cmds)
	cache.RC.LTrim(m.Ctx, k, -100, -1)
	cache.RC.Expire(m.Ctx, k, time.Hour*24*30)
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
	gsess, err := myConnector.DoConnect(conn.Ctx, nil)
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

	_, ch, ok := conn.Sess.Pty()
	if !ok {
		ch = make(<-chan ssh.Window)
	}
	gsess.G.Go(func() (err error) {
		defer r.Close()
		defer w.Close()
		for {
			select {
			case <-gsess.Chans.AwayChan:
				return
			case <-conn.gctx.Done():
				gsess.Once.Do(func() { close(gsess.Chans.AwayChan) })
				return
			case <-gsess.Gctx.Done():
				return
			case w := <-ch:
				gsess.Chans.WindowChan <- w
			}
		}
	})
	protocols.HandleTerm(gsess)

	if err = gsess.G.Wait(); err != nil {
		logger.L().Error("sshsrv run stopped", zap.String("sessionId", gsess.SessionId), zap.Error(err))
	}

	conn.stdout.Write([]byte("\n\n"))

	return nil
}
