package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nicksnyder/go-i18n/v2/i18n"

	myi18n "github.com/veops/oneterm/internal/i18n"
	"github.com/veops/oneterm/internal/model"
	dbpkg "github.com/veops/oneterm/pkg/db"
)

// GetHistories godoc
//
//	@Tags		history
//	@Param		page_index	query		int		true	"page_index"
//	@Param		page_size	query		int		true	"page_size"
//	@Param		type		query		string	false	"type"	Enums(account, asset, command, gateway, node, public_key)
//	@Param		target_id	query		int		false	"target_id"
//	@Param		uid			query		int		false	"uid"
//	@Param		action_type	query		int		false	"create=1 delete=2 update=3"
//	@Param		start		query		string	false	"start time, RFC3339"
//	@Param		end			query		string	false	"end time, RFC3339"
//	@Param		search		query		string	false	"search"
//	@Success	200			{object}	HttpResponse{data=ListData{list=[]model.History}}
//	@Router		/history [get]
func (c *Controller) GetHistories(ctx *gin.Context) {
	db := dbpkg.DB.Model(&model.History{})
	db = filterSearch(ctx, db, "old", "new")
	db, err := filterStartEnd(ctx, db)
	if err != nil {
		return
	}
	db = filterEqual(ctx, db, "type", "target_id", "action_type", "uid")

	doGet[*model.History](ctx, false, db, "")
}

// GetSessions godoc
//
//	@Tags		session
//	@Success	200	{object}	HttpResponse{data=map[string]string}
//	@Router		/history/type/mapping [get]
func (c *Controller) GetHistoryTypeMapping(ctx *gin.Context) {
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
		msg, _ := localizer.Localize(cfg)
		data[k] = msg
	}

	ctx.JSON(http.StatusOK, NewHttpResponseWithData(data))
}
