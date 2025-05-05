package repository

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/veops/oneterm/internal/model"
	dbpkg "github.com/veops/oneterm/pkg/db"
	"gorm.io/gorm"
)

// SessionRepository defines the interface for session repository
type SessionRepository interface {
	GetSession(ctx context.Context, sessionId string) (*model.Session, error)
	BuildQuery(ctx *gin.Context, isAdmin bool, uid int) (*gorm.DB, error)
	BuildCmdQuery(ctx *gin.Context, sessionId string) *gorm.DB
	GetSessionOptionAssets(ctx context.Context) ([]*model.SessionOptionAsset, error)
	GetSessionOptionClientIps(ctx context.Context) ([]string, error)
	CreateSessionCmd(ctx context.Context, cmd *model.SessionCmd) error
	GetSessionCmdCounts(ctx context.Context, sessionIds []string) (map[string]int64, error)
}

type sessionRepository struct{}

// NewSessionRepository creates a new session repository
func NewSessionRepository() SessionRepository {
	return &sessionRepository{}
}

// GetSession retrieves a session by session ID
func (r *sessionRepository) GetSession(ctx context.Context, sessionId string) (*model.Session, error) {
	session := &model.Session{}
	if err := dbpkg.DB.Where("session_id = ?", sessionId).First(session).Error; err != nil {
		return nil, err
	}
	return session, nil
}

// BuildQuery constructs a query for sessions with filters
func (r *sessionRepository) BuildQuery(ctx *gin.Context, isAdmin bool, uid int) (*gorm.DB, error) {
	db := dbpkg.DB.Model(model.DefaultSession)

	// Apply user filter if not admin
	if !isAdmin {
		db = db.Where("uid = ?", uid)
	}

	// Apply text search
	if q, ok := ctx.GetQuery("search"); ok && q != "" {
		db = db.Where("user_name LIKE ? OR asset_info LIKE ? OR gateway_info LIKE ? OR account_info LIKE ?",
			"%"+q+"%", "%"+q+"%", "%"+q+"%", "%"+q+"%")
	}

	// Apply date range filters
	if start, ok := ctx.GetQuery("start"); ok {
		t, err := time.Parse(time.RFC3339, start)
		if err != nil {
			return nil, err
		}
		db = db.Where("created_at >= ?", t)
	}

	if end, ok := ctx.GetQuery("end"); ok {
		t, err := time.Parse(time.RFC3339, end)
		if err != nil {
			return nil, err
		}
		db = db.Where("created_at <= ?", t)
	}

	// Apply exact match filters
	for _, field := range []string{"status", "uid", "asset_id", "client_ip"} {
		if q, ok := ctx.GetQuery(field); ok && q != "" {
			db = db.Where(field+" = ?", q)
		}
	}

	return db, nil
}

// BuildCmdQuery constructs a query for session commands
func (r *sessionRepository) BuildCmdQuery(ctx *gin.Context, sessionId string) *gorm.DB {
	db := dbpkg.DB.Model(&model.SessionCmd{})
	db = db.Where("session_id = ?", sessionId)

	// Apply text search
	if q, ok := ctx.GetQuery("search"); ok && q != "" {
		db = db.Where("cmd LIKE ? OR result LIKE ?", "%"+q+"%", "%"+q+"%")
	}

	return db
}

// GetSessionOptionAssets retrieves session option assets
func (r *sessionRepository) GetSessionOptionAssets(ctx context.Context) ([]*model.SessionOptionAsset, error) {
	opts := make([]*model.SessionOptionAsset, 0)
	if err := dbpkg.DB.
		Model(model.DefaultAsset).
		Select("id, name").
		Find(&opts).
		Error; err != nil {
		return nil, err
	}
	return opts, nil
}

// GetSessionOptionClientIps retrieves distinct client IPs
func (r *sessionRepository) GetSessionOptionClientIps(ctx context.Context) ([]string, error) {
	opts := make([]string, 0)
	if err := dbpkg.DB.
		Model(model.DefaultSession).
		Distinct("client_ip").
		Find(&opts).
		Error; err != nil {
		return nil, err
	}
	return opts, nil
}

// CreateSessionCmd creates a new session command
func (r *sessionRepository) CreateSessionCmd(ctx context.Context, cmd *model.SessionCmd) error {
	return dbpkg.DB.Create(cmd).Error
}

// GetSessionCmdCounts retrieves command counts for sessions
func (r *sessionRepository) GetSessionCmdCounts(ctx context.Context, sessionIds []string) (map[string]int64, error) {
	if len(sessionIds) <= 0 {
		return map[string]int64{}, nil
	}

	post := make([]*model.CmdCount, 0)
	if err := dbpkg.DB.
		Model(&model.SessionCmd{}).
		Select("session_id, COUNT(*) AS count").
		Where("session_id IN ?", sessionIds).
		Group("session_id").
		Find(&post).
		Error; err != nil {
		return nil, err
	}

	// Convert to map
	result := make(map[string]int64)
	for _, p := range post {
		result[p.SessionId] = p.Count
	}

	return result, nil
}
