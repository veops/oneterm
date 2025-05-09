package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"gorm.io/gorm"

	myi18n "github.com/veops/oneterm/internal/i18n"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/repository"
	dbpkg "github.com/veops/oneterm/pkg/db"
)

// HistoryService handles history business logic
type HistoryService struct {
	repo repository.HistoryRepository
}

// NewHistoryService creates a new history service
func NewHistoryService() *HistoryService {
	return &HistoryService{
		repo: repository.NewHistoryRepository(),
	}
}

// CreateHistory creates a new history record
func (s *HistoryService) CreateHistory(ctx context.Context, history *model.History) error {
	return s.repo.CreateHistory(ctx, history)
}

// CreateHistoryRecord creates a history record with common fields
func (s *HistoryService) CreateHistoryRecord(ctx context.Context, actionType int, modelObj model.Model, oldModel model.Model, userId int) *model.History {
	var clientIP string
	if ginCtx, ok := ctx.(*gin.Context); ok {
		clientIP = ginCtx.ClientIP()
	}

	return &model.History{
		RemoteIp:   clientIP,
		Type:       modelObj.TableName(),
		TargetId:   modelObj.GetId(),
		ActionType: actionType,
		Old:        toMap(oldModel),
		New:        toMap(modelObj),
		CreatorId:  userId,
		CreatedAt:  time.Now(),
	}
}

// CreateAndSaveHistory creates and saves a history record in one operation
func (s *HistoryService) CreateAndSaveHistory(ctx context.Context, actionType int, modelObj model.Model, oldModel model.Model, userId int) error {
	history := s.CreateHistoryRecord(ctx, actionType, modelObj, oldModel, userId)
	return s.CreateHistory(ctx, history)
}

// BuildQuery constructs history query with basic filters
func (s *HistoryService) BuildQuery(ctx *gin.Context) (*gorm.DB, error) {
	db := dbpkg.DB.Model(&model.History{})

	// Apply search filter
	db = dbpkg.FilterSearch(ctx, db, "old", "new")

	// Apply date range filter
	db, err := s.filterStartEnd(ctx, db)
	if err != nil {
		return nil, err
	}

	// Apply exact match filters
	db = dbpkg.FilterEqual(ctx, db, "type", "target_id", "action_type", "uid")

	return db, nil
}

// GetTypeMapping gets mapping between history types and localized strings
func (s *HistoryService) GetTypeMapping(ctx *gin.Context) (map[string]string, error) {
	lang := ctx.PostForm("lang")
	accept := ctx.GetHeader("Accept-Language")
	localizer := i18n.NewLocalizer(myi18n.Bundle, lang, accept)
	cfg := &i18n.LocalizeConfig{}

	key2msg := map[string]*i18n.Message{
		"account":    myi18n.MsgTypeMappingAccount,
		"asset":      myi18n.MsgTypeMappingAsset,
		"command":    myi18n.MsgTypeMappingCommand,
		"gateway":    myi18n.MsgTypeMappingGateway,
		"node":       myi18n.MsgTypeMappingNode,
		"public_key": myi18n.MsgTypeMappingPublicKey,
	}

	data := make(map[string]string)
	for k, v := range key2msg {
		cfg.DefaultMessage = v
		msg, err := localizer.Localize(cfg)
		if err != nil {
			return nil, err
		}
		data[k] = msg
	}

	return data, nil
}

// Helper methods for filtering
func (s *HistoryService) filterStartEnd(ctx *gin.Context, db *gorm.DB) (*gorm.DB, error) {
	if start, ok := ctx.GetQuery("start"); ok {
		db = db.Where("created_at >= ?", start)
	}
	if end, ok := ctx.GetQuery("end"); ok {
		db = db.Where("created_at <= ?", end)
	}
	return db, nil
}

// toMap converts an object to a map
func toMap(data any) model.Map[string, any] {
	if data == nil {
		return nil
	}
	bs, _ := json.Marshal(data)
	res := make(map[string]any)
	json.Unmarshal(bs, &res)
	return res
}
