// Package handler
/**
Copyright (c) The Authors.
* @Author: feng.xiang
* @Date: 2024/1/18 17:05
* @Desc:
*/
package handler

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	gossh "github.com/gliderlabs/ssh"
	"github.com/nicksnyder/go-i18n/v2/i18n"

	myi18n "github.com/veops/oneterm/pkg/i18n"
	"github.com/veops/oneterm/pkg/logger"
	"github.com/veops/oneterm/pkg/proto/ssh/client"
	"github.com/veops/oneterm/pkg/proto/ssh/config"
	"github.com/veops/oneterm/pkg/server/model"
)

func (i *InteractiveHandler) Proxy(line string, accountId int) (step int, err error) {
	step = 1
	if accountId > 0 {
		var accountName string
		defer func() {
			if err != nil {
				i.PrintMessage(myi18n.MsgSshAccountLoginError, map[string]any{"User": accountName})
			}
		}()
		if i.SelectedAsset != nil && accountId < 0 {
			accountName = line
		}
		host, ok, er := i.Check(i.SelectedAsset.Id, nil)
		if er != nil {
			err = er
			return
		}
		if !ok {
			err = fmt.Errorf("current time is not allowed to access")
			return
		}
		i.SelectedAsset = host
		if _, ok := i.SelectedAsset.Authorization[accountId]; !ok {
			err = fmt.Errorf("you donnot have permission")
			return
		}
		account, er := i.Sshd.Core.Auth.AccountInfo(i.Session.Context().Value("cookie").(string), accountId, accountName)
		if er != nil {
			err = er
			return
		}
		accountName = account.Name
		var gateway *model.Gateway
		if i.SelectedAsset.GatewayId != 0 {
			gateway, err = i.Sshd.Core.Asset.Gateway(i.Session.Context().Value("cookie").(string), i.SelectedAsset.GatewayId)
			if err != nil {
				return
			}
		}
		conn, er := i.NewSession(account, gateway)
		if er != nil {
			err = er
			return
		}
		er = i.UpsertSession(conn, model.SESSIONSTATUS_ONLINE)
		if er != nil {
			logger.L.Warn(er.Error())
		}
		config.TotalHostSession.Store(conn.SessionId, conn)

		if err := i.bind(i.Session, conn); err != nil {
			logger.L.Error(err.Error())
		}
		step = 1
	} else {
		if i.NeedAccount && i.SelectedAsset != nil {
			i.NeedAccount = false
			var account *model.Account
			accountIds := make([]int, 0)
			for aId := range i.SelectedAsset.Authorization {
				accountIds = append(accountIds, aId)
			}
			sort.Ints(accountIds)
			for _, aId := range accountIds {
				account := i.Accounts[aId]
				if config.SSHConfig.PlainMode {
					if account == nil || !strings.Contains(account.Name, line) {
						continue
					}
				} else {
					if account == nil || line != account.Name {
						continue
					}
				}
				return i.Proxy(line, account.Id)
			}
			if account == nil {
				i.PrintMessage(myi18n.MsgSshAccountLoginError, map[string]any{"User": line})
			}
			return
		} else if strings.TrimSpace(line) == "" {
			return
		}
		var (
			host *model.Asset
		)
		selectHosts, likeHosts, er := i.AcquireAndStoreAssets(line, 0)
		if er != nil {
			logger.L.Error(err.Error())
		}
		if len(selectHosts) == 0 && len(likeHosts) == 0 {
			i.PrintMessage(myi18n.MsgSshNoMatchingAsset, map[string]any{"Host": line})
			return
		}

		switch len(selectHosts) {
		case 0:
			switch len(likeHosts) {
			case 0:
				return
			case 1:
				host = likeHosts[0]
			default:
				i.showResult(likeHosts)
				return
			}
		case 1:
			host = selectHosts[0]
		default:
			i.showResult(selectHosts)
			return
		}
		i.SelectedAsset = host

		var sshPort string
		for _, v := range host.Protocols {
			if strings.HasPrefix(v, "ssh") {
				sshPort = getSshPort(v)
				break
			}
		}
		if sshPort == "" {
			i.PrintMessage(myi18n.MsgSshNoSshAccessMethod, map[string]any{"Host": line})
			return
		}
		i.SessionReq.Protocol = "ssh:" + sshPort

		var hostAccountIds []int
		if len(i.Accounts) == 0 {
			accounts, er := i.AcquireAccounts()
			if er != nil {
				logger.L.Info(er.Error())
			} else {
				i.Accounts = accounts
			}
		}
		for k := range host.Authorization {
			if v, ok := i.Accounts[k]; ok {
				hostAccountIds = append(hostAccountIds, v.Id)
			}
		}
		sort.Ints(hostAccountIds)
		switch len(hostAccountIds) {
		case 0:
			i.PrintMessage(myi18n.MsgSshNoSshAccountForAsset, map[string]any{"Host": line})
			return
		case 1:
			return i.Proxy(line, hostAccountIds[0])
		default:
			var accounts []string
			for _, aId := range hostAccountIds {
				if account, ok := i.Accounts[aId]; ok {
					i.AccountsForSelect = append(i.AccountsForSelect, account)
					accounts = append(accounts, account.Name)
				}
			}
			i.NeedAccount = true
			if config.SSHConfig.PlainMode {
				i.PrintMessage(myi18n.MsgSshMultiSshAccountForAsset, map[string]any{"Accounts": i.tableData(accounts)})
			}
			step = 2
		}
	}
	return
}

func getSshPort(protocol string) (sshPort string) {
	tmp := strings.Split(protocol, ":")
	if len(tmp) == 2 && tmp[0] == "ssh" {
		sshPort = tmp[1]
	}
	if strings.TrimSpace(sshPort) == "" {
		sshPort = "22"
	}
	return sshPort
}

func (i *InteractiveHandler) bind(userConn gossh.Session, hostConn *client.Connection) error {
	maxIdelTimeout := time.Hour * 2
	idleTimeout := time.Second * 60 * 5
	mConfig, _ := i.AcquireConfig()
	if mConfig != nil && mConfig.Timeout > 0 {
		idleTimeout = time.Second * time.Duration(mConfig.Timeout)
	}
	if idleTimeout > maxIdelTimeout {
		idleTimeout = maxIdelTimeout
	}

	targetInChan := make(chan []byte, 1)
	targetOutChan := make(chan []byte, 1)
	done := make(chan struct{})
	var (
		exit             bool
		accessUpdateStep = time.Minute
		waitRead         atomic.Bool
	)
	waitRead.Store(true)
	i.wrapJsonResponse(hostConn.SessionId, 0, "success")
	tk, tkAccess, readReset := time.NewTicker(idleTimeout), time.NewTicker(accessUpdateStep), time.NewTicker(time.Second)
	go func() {
		buffer := bytes.NewBuffer(make([]byte, 0, 1024*2))
		maxLen := 1024
		for {
			if !waitRead.Load() {
				if exit {
					return
				}
				time.Sleep(time.Millisecond * 100)
				continue
			}

			buf := make([]byte, maxLen)
			nr, err := userConn.Read(buf)
			waitRead.Store(config.SSHConfig.PlainMode)
			if err != nil {
				logger.L.Info(err.Error())
			}

			if nr > 0 {
				validBytes := buf[:nr]
				bufferLen := buffer.Len()
				if bufferLen > 0 || nr == maxLen {
					buffer.Write(buf[:nr])
					validBytes = validBytes[:0]
				}
				remainBytes := buffer.Bytes()
				for len(remainBytes) > 0 {
					r, size := utf8.DecodeRune(remainBytes)
					if r == utf8.RuneError {
						if len(remainBytes) <= 3 {
							break
						}
					}
					validBytes = append(validBytes, remainBytes[:size]...)
					remainBytes = remainBytes[size:]
				}
				buffer.Reset()
				if len(remainBytes) > 0 {
					buffer.Write(remainBytes)
				}
				select {
				case targetInChan <- validBytes:
				case <-done:
					break
				}
			}
			if exit {
				return
			}
			if err != nil {
				break
			}
		}
		close(targetInChan)
	}()
	go func() {
		defer func() {
			waitRead.Store(config.SSHConfig.PlainMode)
		}()
		for {
			buf := make([]byte, 1024)
			n, err := hostConn.Stdout.Read(buf)
			if err != nil {
				if err == io.EOF {
					close(done)
					exit = true
				} else {
					logger.L.Warn(err.Error())
				}
			}

			if exit {
				waitRead.Store(config.SSHConfig.PlainMode)
			} else {
				waitRead.Store(true)
			}

			select {
			case targetOutChan <- buf[:n]:
			case <-done:
				break
			}
			if exit || err != nil {
				break
			}
		}
	}()
	defer func() {
		exit = true

		waitRead.Store(config.SSHConfig.PlainMode)
	}()
	for {
		select {
		case p, ok := <-targetOutChan:
			if !ok {
				return nil
			}
			_, err := userConn.Write(p)
			if err != nil {
				logger.L.Info(err.Error())
			}

			err = i.HandleData("output", p, hostConn, targetOutChan)
			if err != nil {
				logger.L.Error(err.Error())
			}
			tk.Reset(idleTimeout)
		case p, ok := <-targetInChan:
			if !ok {
				return nil
			}
			tk.Reset(idleTimeout)
			//readL.WriteStdin(p)
			err := i.HandleData("input", p, hostConn, targetOutChan)
			if err != nil {
				logger.L.Error(err.Error())
			}
			readReset.Reset(time.Second)
		case <-done:
			break
		case <-tk.C:
			_, err := userConn.Write([]byte(i.Message(myi18n.MsgSShHostIdleTimeout, map[string]any{"Idle": idleTimeout})))

			if err != nil {
				logger.L.Warn(err.Error())
			}
			exit = true
		case <-readReset.C:
			waitRead.Store(true)
		case <-hostConn.Exit:
			exit = true
		case <-i.Session.Context().Done():
			exit = true
		case <-tkAccess.C:
			commands, er := i.AcquireCommands()
			if er == nil {
				i.Commands = commands
			}
			asset, er := i.AcquireAssets("", hostConn.AssetId)
			if er != nil || len(asset) <= 0 || !i.Sshd.Core.Asset.HasPermission(asset[0].AccessAuth) {
				_, err := userConn.Write([]byte(i.Message(myi18n.MsgSshAccessRefusedInTimespan, nil)))
				if err != nil {
					logger.L.Error(err.Error())
				}
				exit = true
				break
			}
			i.SelectedAsset = asset[0]
			if _, ok := asset[0].Authorization[hostConn.AccountId]; !ok {
				_, err := userConn.Write([]byte(i.Message(myi18n.MsgSshNoAssetPermission, map[string]any{"Host": i.SelectedAsset.Name})))
				if err != nil {
					logger.L.Warn(err.Error())
				}
				exit = true
				break
			}
			account, er := i.AcquireAccountInfo(hostConn.AccountId, "")
			if er != nil || account == nil {
				_, err := userConn.Write([]byte(i.Message(myi18n.MsgSshNoAssetPermission, map[string]any{"Host": i.SelectedAsset.Name})))
				if err != nil {
					logger.L.Warn(err.Error())
				}
				exit = true
				break
			}
		}
		if exit {
			i.Exits(hostConn)
			return nil
		}
	}
}

func (i *InteractiveHandler) CommandLevel(cmd string) int {
	// TODO
	return 0
}

func parseOutput(data []string) (output []string) {
	for _, line := range data {
		if strings.TrimSpace(line) != "" {
			output = append(output, line)
		}
	}
	return output
}

func (i *InteractiveHandler) Output() string {
	i.Parser.Output.Feed(i.Parser.OutputData)

	res := parseOutput(i.Parser.Output.Listener.Display())
	if len(res) == 0 {
		return ""
	}
	return res[len(res)-1]
}
func (i *InteractiveHandler) Command() string {
	s := i.Output()
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), strings.TrimSpace(i.Parser.Ps1)))
}

func (i *InteractiveHandler) HandleData(src string, data []byte, hostConn *client.Connection,
	targetOutputChan chan<- []byte) (err error) {
	switch src {
	case "input": // input from user
		if hostConn.Parser.State(data) {
			_, err = hostConn.Stdin.Write(data)
			if err != nil {
				logger.L.Error(err.Error())
			}
			return
		}
		var write bool

		//if bytes.LastIndex(data, []byte{13}) != 0 {
		//	fmt.Println("send....", data)
		//	_, err = hostConn.Stdin.Write(data)
		//	if err != nil {
		//		logger.L.Error(err.Error())
		//	}
		//
		//	write = true
		//}

		if bytes.LastIndex(data, []byte{0x0d}) == -1 {
			_, err = hostConn.Stdin.Write(data)
			if err != nil {
				logger.L.Error(err.Error())
			}

			write = true
		} else {
			if len(data) > 1 {
				var tmp []byte
				for _, d := range data {
					if d != 0x0d {
						tmp = append(tmp, d)
						continue
					}
					if len(tmp) > 0 {
						err = i.HandleData(src, tmp, hostConn, targetOutputChan)
						if err != nil {
							return err
						}

					}
					err = i.HandleData(src, []byte{0x0d}, hostConn, targetOutputChan)
					if err != nil {
						return err
					}
				}
				return
			}
		}

		if len(i.Parser.InputData) == 0 && i.Parser.Ps2 == "" {
			i.Parser.Ps1 = i.Output()
		}
		i.Parser.InputData = append(i.Parser.InputData, data...)
		if bytes.LastIndex(data, []byte{13}) == 0 {
			command := i.Command()
			i.Parser.Output.Listener.Reset()
			i.Parser.OutputData = nil
			i.Parser.InputData = nil
			i.Parser.Ps2 = ""

			if _, valid := i.CommandCheck(command); valid {
				if strings.TrimSpace(command) != "" {
					go i.Sshd.Core.Audit.AddCommand(model.SessionCmd{Cmd: command, Level: i.CommandLevel(command), SessionId: hostConn.SessionId})
				}
				if !write {
					_, err = hostConn.Stdin.Write(data)
					if err != nil {
						logger.L.Warn(err.Error())
					}
				}
			} else {
				tips, _ := i.Localizer.Localize(&i18n.LocalizeConfig{
					DefaultMessage: myi18n.MsgSshCommandRefused,
					TemplateData:   map[string]string{"Command": command},
					PluralCount:    1,
				})
				_, err = hostConn.Stdin.Write([]byte{0x15})
				if err != nil {
					logger.L.Warn(err.Error())
				}

				i.Parser.Ps2 = i.Parser.Ps1 + command
				targetOutputChan <- []byte("\r\n" + tips + i.Parser.Ps2)

				break
			}
		}
	case "output": // output from target
		if !hostConn.Parser.State(data) {
			i.Parser.OutputData = append(i.Parser.OutputData, data...)
		}
		err = hostConn.Record.Write(data)
		if err != nil {
			logger.L.Error(err.Error())
		}
		Monitor(hostConn.SessionId, data)
	}

	return
}

func (i *InteractiveHandler) Exits(conn *client.Connection) {
	if conn.GateWayCloseChan != nil {
		conn.GateWayCloseChan <- struct{}{}
	}
	_ = conn.Session.Close()
	conn.Record.Close()

	config.TotalHostSession.Delete(conn.SessionId)
	config.TotalMonitors.Delete(conn.SessionId)

	i.Locker.Lock()
	delete(i.SshSession, conn.SessionId)
	if len(i.SshSession) == 0 {
		_ = i.SshClient.Close()
		i.SshClient = nil
	}
	i.Locker.Unlock()
	err := i.UpsertSession(conn, model.SESSIONSTATUS_OFFLINE)
	if err != nil {
		logger.L.Error(err.Error())
	}
}

func (i *InteractiveHandler) CommandCheck(command string) (string, bool) {
	for _, id := range i.SelectedAsset.CmdIds {
		cmd, ok := i.Commands[id]
		if !ok || !cmd.Enable {
			continue
		}
		for _, c := range cmd.Cmds {
			p, err := regexp.Compile(c)
			if err == nil {
				if p.Match([]byte(command)) {
					return c, false
				}
			} else {
				if c == command {
					return c, false
				}
			}
		}
	}
	return "", true
}

//func (v *VirtualTermIn) Read(p []byte) (n int, err error) {
//	return v.InChan.Read(p)
//}
//
//func (v *VirtualTermIn) Close() error {
//	return nil
//}
//
//func (v *VirtualTermOut) Write(p []byte) (n int, err error) {
//	return v.OutChan.Write(p)
//}
