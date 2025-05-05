package service

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/repository"
	"github.com/veops/oneterm/internal/tunneling"
	dbpkg "github.com/veops/oneterm/pkg/db"
)

var (
	fm = &FileManager{
		sftps:    map[string]*sftp.Client{},
		lastTime: map[string]time.Time{},
		mtx:      sync.Mutex{},
	}

	// Global file service instance
	DefaultFileService IFileService
)

// InitFileService initializes the global file service
func InitFileService() {
	repo := repository.NewFileRepository(dbpkg.DB)
	DefaultFileService = NewFileService(repo)
}

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

	asset, account, gateway, err := GetAAG(assetId, accountId)
	if err != nil {
		return
	}

	ip, port, err := tunneling.Proxy(false, uuid.New().String(), "sftp,ssh", asset, gateway)
	if err != nil {
		return
	}

	auth, err := GetAuth(account)
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

// File service interface
type IFileService interface {
	ReadDir(ctx context.Context, assetId, accountId int, dir string) ([]fs.FileInfo, error)
	MkdirAll(ctx context.Context, assetId, accountId int, dir string) error
	Create(ctx context.Context, assetId, accountId int, path string) (io.WriteCloser, error)
	Open(ctx context.Context, assetId, accountId int, path string) (io.ReadCloser, error)
	AddFileHistory(ctx context.Context, history *model.FileHistory) error
	GetFileHistory(ctx context.Context, filters map[string]interface{}) ([]*model.FileHistory, int64, error)
}

// File service implementation
type FileService struct {
	repo repository.IFileRepository
}

func NewFileService(repo repository.IFileRepository) IFileService {
	return &FileService{
		repo: repo,
	}
}

// ReadDir gets directory listing
func (s *FileService) ReadDir(ctx context.Context, assetId, accountId int, dir string) ([]fs.FileInfo, error) {
	cli, err := GetFileManager().GetFileClient(assetId, accountId)
	if err != nil {
		return nil, err
	}
	return cli.ReadDir(dir)
}

// MkdirAll creates a directory
func (s *FileService) MkdirAll(ctx context.Context, assetId, accountId int, dir string) error {
	cli, err := GetFileManager().GetFileClient(assetId, accountId)
	if err != nil {
		return err
	}
	return cli.MkdirAll(dir)
}

// Create creates a file
func (s *FileService) Create(ctx context.Context, assetId, accountId int, path string) (io.WriteCloser, error) {
	cli, err := GetFileManager().GetFileClient(assetId, accountId)
	if err != nil {
		return nil, err
	}
	return cli.Create(path)
}

// Open opens a file
func (s *FileService) Open(ctx context.Context, assetId, accountId int, path string) (io.ReadCloser, error) {
	cli, err := GetFileManager().GetFileClient(assetId, accountId)
	if err != nil {
		return nil, err
	}
	return cli.Open(path)
}

// AddFileHistory adds a file history record
func (s *FileService) AddFileHistory(ctx context.Context, history *model.FileHistory) error {
	return s.repo.AddFileHistory(ctx, history)
}

// GetFileHistory gets file history records
func (s *FileService) GetFileHistory(ctx context.Context, filters map[string]interface{}) ([]*model.FileHistory, int64, error) {
	return s.repo.GetFileHistory(ctx, filters)
}
