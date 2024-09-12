package controller

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"gorm.io/gorm"

	"github.com/veops/oneterm/acl"
	mysql "github.com/veops/oneterm/db"
	"github.com/veops/oneterm/model"
)

// CreateShare godoc
//
//	@Tags		share
//	@Param		share	body		model.Share	true	"share"
//	@Success	200		{object}	HttpResponse{data=ListData{list=[]string}}
//	@Router		/share [post]
func (c *Controller) CreateShare(ctx *gin.Context) {
	shares := make([]*model.Share, 0)
	if err := ctx.ShouldBindBodyWithJSON(&shares); err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &ApiError{Code: ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	}
	uuids := lo.Map(shares, func(s *model.Share, _ int) string {
		s.Uuid = uuid.New().String()
		return s.Uuid
	})
	if err := mysql.DB.Create(shares); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &ApiError{Code: ErrInternal, Data: map[string]any{"err": err}})
		return
	}
	ctx.JSON(http.StatusOK, toListData(uuids))
}

// DeleteAsset godoc
//
//	@Tags		share
//	@Param		id	path		int	true	"share id"
//	@Success	200	{object}	HttpResponse
//	@Router		/share/:id [delete]
func (c *Controller) DeleteShare(ctx *gin.Context) {
	doDelete(ctx, false, &model.Share{})
}

// GetAssets godoc
//
//	@Tags		share
//	@Param		page_index	query		int		true	"page_index"
//	@Param		page_size	query		int		true	"page_size"
//	@Param		search		query		string	false	"name or ip"
//	@Param		start		query		string	false	"start, RFC3339"
//	@Param		end			query		string	false	"end, RFC3339"
//	@Param		asset_id	query		string	false	"asset id"
//	@Param		account_id	query		string	false	"account id"
//	@Success	200			{object}	HttpResponse{data=ListData{list=[]model.Share}}
//	@Router		/share [get]
func (c *Controller) GetShare(ctx *gin.Context) {
	db := mysql.DB.Model(&model.Share{})
	db = filterSearch(ctx, db)
	db, err := filterStartEnd(ctx, db)
	if err != nil {
		return
	}
	db = filterEqual(ctx, db, "asset_id", "account_id")

	doGet[*model.Share](ctx, false, db, "")
}

// Connect godoc
//
//	@Tags		connect
//	@Success	200	{object}	HttpResponse
//	@Param		w	query		int	false	"width"
//	@Param		h	query		int	false	"height"
//	@Param		dpi	query		int	false	"dpi"
//	@Success	200	{object}	HttpResponse{}
//	@Router		/share/connect/:uuid [get]
func (c *Controller) ConnectShare(ctx *gin.Context) {
	share := &model.Share{}
	if err := mysql.DB.Transaction(func(tx *gorm.DB) (err error) {
		if err = tx.Where("uuid=?", ctx.Param("uuid")).First(share).Error; err != nil {
			return
		}
		now := time.Now()
		if now.Before(share.Start) || now.After(share.End) {
			ctx.AbortWithError(http.StatusBadRequest, &ApiError{Code: ErrBadRequest, Data: map[string]any{"err": "share expired or not started"}})
			return
		}
		if share.NoLimit {
			return
		}
		db := tx.Model(share).Where("uuid=? AND times>0", share.Uuid).Update("times", gorm.Expr("times-?", 1))
		if db.Error != nil {
			return
		}
		if db.RowsAffected != 1 {
			ctx.AbortWithError(http.StatusBadRequest, &ApiError{Code: ErrBadRequest, Data: map[string]any{"err": "no times left"}})
			return
		}
		return
	}); err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &ApiError{Code: ErrBadRequest, Data: map[string]any{"err": err}})
		return
	}

	ctx.Params = lo.Filter(ctx.Params, func(p gin.Param, _ int) bool {
		return !lo.Contains([]string{"account_id", "asset_id", "protocol"}, p.Key)
	})
	ctx.Params = append(ctx.Params, gin.Param{Key: "account_id", Value: cast.ToString(share.AccountId)})
	ctx.Params = append(ctx.Params, gin.Param{Key: "asset_id", Value: cast.ToString(share.AssetId)})
	ctx.Params = append(ctx.Params, gin.Param{Key: "protocol", Value: cast.ToString(share.Protocol)})
	ctx.Set("shareId", share.Id)
	ctx.Set("session", &acl.Session{})
	c.Connect(ctx)
}
