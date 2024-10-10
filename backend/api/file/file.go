package file

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"github.com/veops/oneterm/util"
)

var (
	fm = &FileManager{
		sftps:    map[string]*sftp.Client{},
		lastTime: map[string]time.Time{},
		mtx:      sync.Mutex{},
	}
)

func init() {
	go func() {
		tk := time.NewTicker(time.Minute)
		for {
			<-tk.C
			func() {
				fm.mtx.Lock()
				defer fm.mtx.Unlock()
				for k, v := range fm.lastTime {
					if v.Before(time.Now().Add(time.Minute * 10)) {
						delete(fm.sftps, k)
						delete(fm.lastTime, k)
					}
				}
			}()
		}
	}()
}

type FileManager struct {
	sftps    map[string]*sftp.Client
	lastTime map[string]time.Time
	mtx      sync.Mutex
}

type FileInfo struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
	Mode  string `json:"mode"`
}

func GetFileManager() *FileManager {
	return fm
}

func (fm *FileManager) GetFileClient(assetId, accountId int) (cli *sftp.Client, err error) {
	fm.mtx.Lock()
	defer fm.mtx.Unlock()

	key := fmt.Sprintf("%d-%d", assetId, accountId)
	defer func() {
		fm.lastTime[key] = time.Now()
	}()

	cli, ok := fm.sftps[key]
	if ok {
		return
	}

	asset, account, gateway, err := util.GetAAG(assetId, accountId)
	if err != nil {
		return
	}

	ip, port, err := util.Proxy(uuid.New().String(), "sftp,ssh", asset, gateway)
	if err != nil {
		return
	}

	auth, err := util.GetAuth(account)
	if err != nil {
		return
	}

	sshCli, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", ip, port), &ssh.ClientConfig{
		User:            account.Account,
		Auth:            []ssh.AuthMethod{auth},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Second,
	})
	if err != nil {
		return
	}

	cli, err = sftp.NewClient(sshCli)
	fm.sftps[key] = cli

	return
}
