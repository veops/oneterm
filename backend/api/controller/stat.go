package controller

import (
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"golang.org/x/sync/errgroup"

	"github.com/veops/oneterm/acl"
	redis "github.com/veops/oneterm/cache"
	mysql "github.com/veops/oneterm/db"
	"github.com/veops/oneterm/model"
	"github.com/veops/oneterm/util"
)

// StatAssetType godoc
//
//	@Tags		stat
//	@Success	200	{object}	HttpResponse{data=ListData{list=[]model.StatAssetType}}
//	@Router		/stat/assettype [get]
func (c *Controller) StatAssetType(ctx *gin.Context) {
	stat := make([]*model.StatAssetType, 0)
	key := "stat-assettype"
	if redis.Get(ctx, key, stat) == nil {
		ctx.JSON(http.StatusOK, NewHttpResponseWithData(toListData(stat)))
		return
	}

	m, err := nodeCountAsset()
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	if err = mysql.DB.
		Model(stat).
		Where("parent_id = 0").
		Find(&stat).
		Error; err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	for _, s := range stat {
		s.Count = m[s.Id]
	}

	redis.SetEx(ctx, key, stat, time.Minute)

	ctx.JSON(http.StatusOK, NewHttpResponseWithData(toListData(stat)))
}

// StatCount godoc
//
//	@Tags		stat
//	@Success	200	{object}	HttpResponse{data=model.StatCount}
//	@Router		/stat/count [get]
func (c *Controller) StatCount(ctx *gin.Context) {
	stat := &model.StatCount{}
	key := "stat-count"
	if redis.Get(ctx, key, stat) == nil {
		ctx.JSON(http.StatusOK, NewHttpResponseWithData(stat))
		return
	}

	eg := &errgroup.Group{}
	eg.Go(func() error {
		return mysql.DB.
			Model(model.DefaultSession).
			Select("COUNT(DISTINCT asset_id, account_id) as connect, COUNT(DISTINCT uid) as user, COUNT(DISTINCT gateway_id) as gateway, COUNT(*) as session").
			Where("status = 1").
			First(&stat).
			Error
	})
	eg.Go(func() error {
		return mysql.DB.Model(model.DefaultAsset).Count(&stat.TotalAsset).Error
	})
	eg.Go(func() error {
		return mysql.DB.Model(model.DefaultAsset).Where("connectable = 1").Count(&stat.Asset).Error
	})
	eg.Go(func() error {
		return mysql.DB.Model(model.DefaultGateway).Count(&stat.TotalGateway).Error
	})

	if err := eg.Wait(); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	stat.Gateway = lo.Ternary(stat.Gateway <= stat.TotalGateway, stat.Gateway, stat.TotalGateway)

	redis.SetEx(ctx, key, stat, time.Minute)

	ctx.JSON(http.StatusOK, NewHttpResponseWithData(stat))
}

// StatAccount godoc
//
//	@Tags		stat
//	@Param		type		query		string	true	"account name" Enums(day, week, month)
//	@Success	200	{object}	HttpResponse{data=ListData{list=[]model.StatAccount}}
//	@Router		/stat/account [get]
func (c *Controller) StatAccount(ctx *gin.Context) {
	start, end := time.Now(), time.Now()
	switch ctx.Query("type") {
	case "day":
		start = start.Add(-time.Hour * 24)
	case "week":
		start = start.Add(-time.Hour * 24 * 7)
	case "month":
		start = start.Add(-time.Hour * 24 * 30)
	default:
		ctx.AbortWithError(http.StatusBadRequest, fmt.Errorf("wrong time range %s", ctx.Query("type")))
		return
	}

	stat := make([]*model.StatAccount, 0)
	key := "stat-account-" + ctx.Query("type")
	if redis.Get(ctx, key, stat) == nil {
		ctx.JSON(http.StatusOK, NewHttpResponseWithData(toListData(stat)))
		return
	}

	err := mysql.DB.
		Model(&model.Account{}).
		Select("account.name, COUNT(*) AS count").
		Joins("LEFT JOIN session ON account.id = session.account_id").
		Group("account.id").
		Order("count DESC").
		Limit(10).
		Where("session.created_at >= ? AND session.created_at <= ?", start, end).
		Find(&stat).
		Error
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	redis.SetEx(ctx, key, stat, time.Minute)

	ctx.JSON(http.StatusOK, NewHttpResponseWithData(toListData(stat)))
}

// StatAsset godoc
//
//	@Tags		stat
//	@Param		type		query		string	true	"account name" Enums(day, week, month)
//	@Success	200	{object}	HttpResponse{data=ListData{list=[]model.StatAsset}}
//	@Router		/stat/asset [get]
func (c *Controller) StatAsset(ctx *gin.Context) {
	start, end := time.Now(), time.Now()
	interval := time.Hour * 24
	dateFmt := "%Y-%m-%d"
	timeFmt := time.DateOnly
	switch ctx.Query("type") {
	case "day":
		start = start.Add(-time.Hour * 24)
		interval = time.Hour
		dateFmt = "%Y-%m-%d %H:00:00"
		timeFmt = time.DateTime
	case "week":
		start = start.Add(-time.Hour * 24 * 7)
	case "month":
		start = start.Add(-time.Hour * 24 * 30)
	default:
		ctx.AbortWithError(http.StatusBadRequest, fmt.Errorf("wrong time range %s", ctx.Query("type")))
		return
	}

	stat := make([]*model.StatAsset, 0)
	key := "stat-asset-" + ctx.Query("type")
	if redis.Get(ctx, key, stat) == nil {
		ctx.JSON(http.StatusOK, NewHttpResponseWithData(toListData(stat)))
		return
	}
	err := mysql.DB.
		Model(model.DefaultSession).
		Select("COUNT(DISTINCT asset_id, uid) AS connect, COUNT(*) AS session, COUNT(DISTINCT asset_id) AS asset, COUNT(DISTINCT uid) AS user, DATE_FORMAT(created_at, ?) AS time", dateFmt).
		Where("session.created_at >= ? AND session.created_at <= ?", start, end).
		Group("time").
		Find(&stat).
		Error
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	for ; !start.After(end); start = start.Add(interval) {
		t := start.Truncate(interval).Format(timeFmt)
		if lo.ContainsBy(stat, func(s *model.StatAsset) bool { return t == s.Time }) {
			continue
		}
		stat = append(stat, &model.StatAsset{Time: t})
	}

	sort.Slice(stat, func(i, j int) bool { return stat[i].Time < stat[j].Time })

	redis.SetEx(ctx, key, stat, time.Minute)

	ctx.JSON(http.StatusOK, NewHttpResponseWithData(toListData(stat)))
}

// StatCountOfUser godoc
//
//	@Tags		stat
//	@Success	200	{object}	HttpResponse{data=model.StatCountOfUser}
//	@Router		/stat/count/ofuser [get]
func (c *Controller) StatCountOfUser(ctx *gin.Context) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	stat := &model.StatCountOfUser{}

	eg := &errgroup.Group{}
	eg.Go(func() error {
		return mysql.DB.
			Model(model.DefaultSession).
			Select("COUNT(DISTINCT asset_id, account_id) as connect, COUNT(DISTINCT asset_id) as asset, COUNT(*) as session").
			Where("status = 1").
			Where("uid = ?", currentUser.GetUid()).
			First(&stat).
			Error
	})
	eg.Go(func() error {
		isAdmin := acl.IsAdmin(currentUser)
		assets, err := util.GetAllFromCacheDb(ctx, model.DefaultAsset)
		if !isAdmin {
			assetIds, err := GetAssetIdsByAuthorization(ctx)
			if err != nil {
				return err
			}
			assets = lo.Filter(assets, func(a *model.Asset, _ int) bool { return lo.Contains(assetIds, a.Id) })
		}
		stat.TotalAsset = int64(len(assets))
		return err
	})

	if err := eg.Wait(); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, NewHttpResponseWithData(stat))
}

// StatRankOfUser godoc
//
//	@Tags		stat
//	@Success	200	{object}	HttpResponse{data=ListData{list=[]model.StatAsset}}
//	@Router		/stat/rank/ofuser [get]
func (c *Controller) StatRankOfUser(ctx *gin.Context) {
	stat := make([]*model.StatRankOfUser, 0)
	key := "stat-rank-user"
	if redis.Get(ctx, key, stat) == nil {
		ctx.JSON(http.StatusOK, NewHttpResponseWithData(toListData(stat)))
		return
	}

	if err := mysql.DB.
		Model(model.DefaultSession).
		Select("uid, COUNT(*) AS count, MAX(created_at) AS last_time").
		Group("uid").
		Order("count DESC").
		Limit(3).
		Find(&stat).
		Error; err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	redis.SetEx(ctx, key, stat, time.Minute)

	ctx.JSON(http.StatusOK, NewHttpResponseWithData(toListData(stat)))
}

func toListData[T any](data []T) *ListData {
	return &ListData{
		Count: int64(len(data)),
		List:  lo.Map(data, func(d T, _ int) any { return d }),
	}
}

func nodeCountAsset() (m map[int]int64, err error) {
	assets := make([]*model.AssetIdPid, 0)
	if err = mysql.DB.Model(model.DefaultAsset).Find(&assets).Error; err != nil {
		return
	}
	nodes := make([]*model.Node, 0)
	if err = mysql.DB.Model(model.DefaultNode).Find(&nodes).Error; err != nil {
		return
	}
	m = make(map[int]int64)
	for _, a := range assets {
		m[a.ParentId] += 1
	}
	g := make(map[int][]int)
	for _, n := range nodes {
		g[n.ParentId] = append(g[n.ParentId], n.Id)
	}
	var dfs func(int) int64
	dfs = func(x int) int64 {
		for _, y := range g[x] {
			m[x] += dfs(y)
		}
		return m[x]
	}
	dfs(0)

	return
}
