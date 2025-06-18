package service

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/veops/oneterm/internal/acl"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/repository"
	gsession "github.com/veops/oneterm/internal/session"
	"github.com/veops/oneterm/pkg/config"
	"github.com/veops/oneterm/pkg/logger"
	"gorm.io/gorm"
)

// SessionService handles session business logic
type SessionService struct {
	repo repository.SessionRepository
}

// NewSessionService creates a new session service
func NewSessionService() *SessionService {
	return &SessionService{
		repo: repository.NewSessionRepository(),
	}
}

// GetOnlineSessionByID retrieves an online session by ID
func (s *SessionService) GetOnlineSessionByID(ctx context.Context, sessionID string) (*gsession.Session, error) {
	return s.repo.GetOnlineSessionByID(ctx, sessionID)
}

// GetSshParserCommands retrieves SSH parser commands by IDs
func (s *SessionService) GetSshParserCommands(ctx context.Context, cmdIDs []int) ([]*model.Command, error) {
	return s.repo.GetSshParserCommands(ctx, cmdIDs)
}

// AttachCmdCounts attaches command counts to sessions
func (s *SessionService) AttachCmdCounts(ctx context.Context, sessions []*model.Session) error {
	if len(sessions) == 0 {
		return nil
	}

	// Get all session IDs
	sessionIds := lo.Map(sessions, func(s *model.Session, _ int) string { return s.SessionId })

	// Get command counts
	counts, err := s.repo.GetSessionCmdCounts(ctx, sessionIds)
	if err != nil {
		logger.L().Error("Failed to get session command counts", zap.Error(err))
		return err
	}

	// Attach counts to sessions
	for _, session := range sessions {
		session.CmdCount = counts[session.SessionId]
	}

	return nil
}

// CalculateDurations calculates session durations
func (s *SessionService) CalculateDurations(sessions []*model.Session) {
	now := time.Now()
	for _, session := range sessions {
		t := now
		if session.ClosedAt != nil {
			t = *session.ClosedAt
		}
		session.Duration = int64(t.Sub(session.CreatedAt).Seconds())
	}
}

// CreateSessionCmd creates a new session command
func (s *SessionService) CreateSessionCmd(ctx context.Context, cmd *model.SessionCmd) error {
	return s.repo.CreateSessionCmd(ctx, cmd)
}

// BuildQuery constructs a query for sessions
func (s *SessionService) BuildQuery(ctx *gin.Context) (*gorm.DB, error) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	isAdmin := acl.IsAdmin(currentUser)

	return s.repo.BuildQuery(ctx, isAdmin, currentUser.GetUid())
}

// BuildCmdQuery constructs a query for session commands
func (s *SessionService) BuildCmdQuery(ctx *gin.Context, sessionId string) *gorm.DB {
	return s.repo.BuildCmdQuery(ctx, sessionId)
}

// GetSessionOptionAssets retrieves session option assets
func (s *SessionService) GetSessionOptionAssets(ctx context.Context) ([]*model.SessionOptionAsset, error) {
	return s.repo.GetSessionOptionAssets(ctx)
}

// GetSessionOptionClientIps retrieves distinct client IPs
func (s *SessionService) GetSessionOptionClientIps(ctx context.Context) ([]string, error) {
	return s.repo.GetSessionOptionClientIps(ctx)
}

// CreateSessionReplay creates a session replay file
func (s *SessionService) CreateSessionReplay(ctx *gin.Context, sessionId string, file io.Reader) error {
	content, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	replayDir := config.Cfg.Session.ReplayDir
	if err := os.MkdirAll(replayDir, 0755); err != nil {
		return err
	}

	f, err := os.Create(filepath.Join(replayDir, fmt.Sprintf("%s.cast", sessionId)))
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(content)
	return err
}

// GetSessionReplayFilename gets the session replay filename
func (s *SessionService) GetSessionReplayFilename(ctx context.Context, sessionId string) (string, error) {
	session, err := s.repo.GetSession(ctx, sessionId)
	if err != nil {
		return "", err
	}

	filename := sessionId
	if !session.IsGuacd() {
		filename += ".cast"
	}

	return filename, nil
}

// GetSessionReplay gets session replay file reader
func (s *SessionService) GetSessionReplay(ctx context.Context, sessionId string) (io.ReadCloser, error) {
	return gsession.GetReplay(sessionId)
}
