package service

import (
	"fmt"

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

// BuildQuery constructs history query with basic filters
func (s *HistoryService) BuildQuery(ctx *gin.Context) (*gorm.DB, error) {
	db := dbpkg.DB.Model(&model.History{})

	// Apply search filter
	db = s.filterSearch(ctx, db, "old", "new")

	// Apply date range filter
	db, err := s.filterStartEnd(ctx, db)
	if err != nil {
		return nil, err
	}

	// Apply exact match filters
	db = s.filterEqual(ctx, db, "type", "target_id", "action_type", "uid")

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
func (s *HistoryService) filterEqual(ctx *gin.Context, db *gorm.DB, fields ...string) *gorm.DB {
	for _, f := range fields {
		if q, ok := ctx.GetQuery(f); ok {
			db = db.Where(fmt.Sprintf("%s = ?", f), q)
		}
	}
	return db
}

func (s *HistoryService) filterSearch(ctx *gin.Context, db *gorm.DB, fields ...string) *gorm.DB {
	q, ok := ctx.GetQuery("search")
	if !ok || len(fields) <= 0 {
		return db
	}

	d := dbpkg.DB
	for _, f := range fields {
		d = d.Or(fmt.Sprintf("%s LIKE ?", f), fmt.Sprintf("%%%s%%", q))
	}

	db = db.Where(d)
	return db
}

func (s *HistoryService) filterStartEnd(ctx *gin.Context, db *gorm.DB) (*gorm.DB, error) {
	if start, ok := ctx.GetQuery("start"); ok {
		db = db.Where("created_at >= ?", start)
	}
	if end, ok := ctx.GetQuery("end"); ok {
		db = db.Where("created_at <= ?", end)
	}
	return db, nil
}
