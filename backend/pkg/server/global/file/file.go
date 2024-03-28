package file

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/sftp"
	"github.com/spf13/cast"
	"golang.org/x/crypto/ssh"

	ggateway "github.com/veops/oneterm/pkg/server/global/gateway"
	"github.com/veops/oneterm/pkg/server/model"
	"github.com/veops/oneterm/pkg/server/storage/db/mysql"
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
	asset, account, gateway := &model.Asset{}, &model.Account{}, &model.Gateway{}
	if err = mysql.DB.Model(asset).Where("id = ?", assetId).First(asset).Error; err != nil {
		return
	}
	if err = mysql.DB.Model(account).Where("id = ?", accountId).First(account).Error; err != nil {
		return
	}
	if asset.GatewayId != 0 {
		if err = mysql.DB.Model(gateway).Where("id = ?", asset.GatewayId).First(gateway).Error; err != nil {
			return
		}
	}
	ip, port := asset.Ip, 22
	for _, p := range asset.Protocols {
		if strings.HasPrefix(p, "sftp") {
			port = cast.ToInt(strings.Split(p, ":")[1])
			break
		}
	}
	if asset.GatewayId != 0 {
		sid, _ := uuid.NewUUID()
		var g *ggateway.GatewayTunnel
		g, err = ggateway.GetGatewayManager().Open(sid.String(), ip, port, gateway)
		if err != nil {
			return
		}
		ip, port = g.LocalIp, g.LocalPort
	}
	auth, err := fm.getAuth(account)
	if err != nil {
		return
	}
	sshCli, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", ip, port), &ssh.ClientConfig{
		User:            gateway.Account,
		Auth:            []ssh.AuthMethod{auth},
		Timeout:         time.Second * 3,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	})
	if err != nil {
		return
	}

	cli, err = sftp.NewClient(sshCli)
	fm.sftps[key] = cli

	return
}

func (fm *FileManager) getAuth(account *model.Account) (ssh.AuthMethod, error) {
	switch account.AccountType {
	case model.AUTHMETHOD_PASSWORD:
		return ssh.Password(account.Password), nil
	case model.AUTHMETHOD_PUBLICKEY:
		if account.Phrase == "" {
			pk, err := ssh.ParsePrivateKey([]byte(account.Pk))
			if err != nil {
				return nil, err
			}
			return ssh.PublicKeys(pk), nil
		} else {
			pk, err := ssh.ParsePrivateKeyWithPassphrase([]byte(account.Pk), []byte(account.Phrase))
			if err != nil {
				return nil, err
			}
			return ssh.PublicKeys(pk), nil
		}
	default:
		return nil, fmt.Errorf("invalid authmethod %d", account.AccountType)
	}
}
