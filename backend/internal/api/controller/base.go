package controller

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"

	"github.com/veops/oneterm/internal/acl"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/repository"
	"github.com/veops/oneterm/internal/service"
	"github.com/veops/oneterm/pkg/config"
	myErrors "github.com/veops/oneterm/pkg/errors"
	"github.com/veops/oneterm/pkg/remote"
)

var (
	defaultHttpResponse = &HttpResponse{
		Code:    0,
		Message: "ok",
		Data:    nil,
	}
)

type preHook[T any] func(*gin.Context, T)
type postHook[T any] func(*gin.Context, []T)
type deleteCheck func(*gin.Context, int)

type Controller struct {
	baseService    service.BaseService
	historyService *service.HistoryService
}

func NewController() *Controller {
	return &Controller{
		baseService:    service.NewBaseService(),
		historyService: service.NewHistoryService(),
	}
}

type HttpResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

type ListData struct {
	Count int64 `json:"count"`
	List  []any `json:"list"`
}

func NewHttpResponseWithData(data any) *HttpResponse {
	return &HttpResponse{
		Code:    0,
		Message: "ok",
		Data:    data,
	}
}

func doCreate[T model.Model](ctx *gin.Context, needAcl bool, md T, resourceType string, preHooks ...preHook[T]) (err error) {
	defer repository.DeleteAllFromCacheDb(ctx, md)

	currentUser, _ := acl.GetSessionFromCtx(ctx)
	baseService := service.NewBaseService()
	historyService := service.NewHistoryService()

	if err = ctx.ShouldBindBodyWithJSON(md); err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	}

	for _, hook := range preHooks {
		if hook == nil {
			continue
		}
		hook(ctx, md)
		if ctx.IsAborted() {
			return
		}
	}

	resourceId := 0
	if needAcl {
		_, ok := any(md).(*model.Node)
		resourceId, err = acl.CreateGrantAcl(ctx, currentUser, resourceType, md.GetName()+lo.Ternary(ok, time.Now().Format(time.RFC3339), ""))
		if err != nil {
			handleRemoteErr(ctx, err)
			return
		}
		md.SetResourceId(resourceId)
	}

	md.SetCreatorId(currentUser.Uid)
	md.SetUpdaterId(currentUser.Uid)

	if err = baseService.ExecuteInTransaction(ctx, func(tx *gorm.DB) (err error) {
		if err = tx.Model(md).Create(md).Error; err != nil {
			return
		}

		switch t := any(md).(type) {
		case *model.Asset:
			if err = handleAuthorization(ctx, tx, model.ACTION_CREATE, t, nil); err != nil {
				handleRemoteErr(ctx, err)
				return
			}
		case *model.Node:
			if err = acl.UpdateResource(ctx, currentUser.GetUid(), resourceId, map[string]string{"name": cast.ToString(md.GetId())}); err != nil {
				handleRemoteErr(ctx, err)
				return
			}
		}

		// Create history using history service
		err = historyService.CreateAndSaveHistory(ctx, model.ACTION_CREATE, md, nil, currentUser.Uid)
		return
	}); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrDuplicateName, Data: map[string]any{"err": err}})
			return
		}
		ctx.AbortWithError(http.StatusInternalServerError, &myErrors.ApiError{Code: myErrors.ErrInternal, Data: map[string]any{"err": err}})
		return
	}

	ctx.JSON(http.StatusOK, HttpResponse{
		Data: map[string]any{
			"id": md.GetId(),
		},
	})

	return
}

func doDelete[T model.Model](ctx *gin.Context, needAcl bool, md T, resourceType string, dcs ...deleteCheck) (err error) {
	defer repository.DeleteAllFromCacheDb(ctx, md)

	currentUser, _ := acl.GetSessionFromCtx(ctx)
	baseService := service.NewBaseService()
	historyService := service.NewHistoryService()

	id, err := cast.ToIntE(ctx.Param("id"))
	if err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	}

	// Use service to get model by ID
	if err = baseService.GetById(ctx, id, md); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.JSON(http.StatusOK, HttpResponse{
				Data: map[string]any{
					"id": md.GetId(),
				},
			})
			return
		}
		ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrInternal, Data: map[string]any{"err": err}})
		return
	}

	if needAcl && !hasPerm(ctx, md, resourceType, acl.DELETE) {
		ctx.AbortWithError(http.StatusForbidden, &myErrors.ApiError{Code: myErrors.ErrNoPerm, Data: map[string]any{"perm": acl.DELETE}})
		return
	}

	for _, dc := range dcs {
		if dc == nil {
			continue
		}
		dc(ctx, id)
		if ctx.IsAborted() {
			return
		}
	}

	if needAcl {
		if err = acl.DeleteResource(ctx, currentUser.GetUid(), md.GetResourceId()); err != nil {
			handleRemoteErr(ctx, err)
			return
		}
	}

	if err = baseService.ExecuteInTransaction(ctx, func(tx *gorm.DB) (err error) {
		switch t := any(md).(type) {
		case *model.Asset:
			if err = handleAuthorization(ctx, tx, model.ACTION_DELETE, t, nil, nil); err != nil {
				handleRemoteErr(ctx, err)
				return
			}
		}

		if err = tx.Delete(md, id).Error; err != nil {
			return
		}

		// Create history using history service
		err = historyService.CreateAndSaveHistory(ctx, model.ACTION_DELETE, md, nil, currentUser.Uid)
		return
	}); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrDuplicateName, Data: map[string]any{"err": err}})
			return
		}
		ctx.AbortWithError(http.StatusInternalServerError, &myErrors.ApiError{Code: myErrors.ErrInternal, Data: map[string]any{"err": err}})
		return
	}

	ctx.JSON(http.StatusOK, HttpResponse{
		Data: map[string]any{
			"id": md.GetId(),
		},
	})

	return
}

func doUpdate[T model.Model](ctx *gin.Context, needAcl bool, md T, resourceType string, preHooks ...preHook[T]) (err error) {
	defer repository.DeleteAllFromCacheDb(ctx, md)

	currentUser, _ := acl.GetSessionFromCtx(ctx)
	baseService := service.NewBaseService()
	historyService := service.NewHistoryService()

	id, err := cast.ToIntE(ctx.Param("id"))
	if err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	}

	if err = ctx.ShouldBindBodyWithJSON(md); err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	}
	md.SetUpdaterId(currentUser.Uid)

	for _, hook := range preHooks {
		if hook == nil {
			continue
		}
		hook(ctx, md)
		if ctx.IsAborted() {
			return
		}
	}

	old := getEmpty(md)
	if err = baseService.GetById(ctx, id, old); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.JSON(http.StatusOK, defaultHttpResponse)
			return
		}
		ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrInternal, Data: map[string]any{"err": err}})
		return
	}

	if needAcl {
		md.SetResourceId(old.GetResourceId())
		if !hasPerm(ctx, md, resourceType, acl.WRITE) {
			ctx.AbortWithError(http.StatusForbidden, &myErrors.ApiError{Code: myErrors.ErrNoPerm, Data: map[string]any{"perm": acl.WRITE}})
			return
		}

		_, ok := any(md).(*model.Node)
		if err = acl.UpdateResource(ctx, currentUser.GetUid(), old.GetResourceId(),
			map[string]string{"name": lo.Ternary(ok, cast.ToString(md.GetId()), md.GetName())}); err != nil {
			handleRemoteErr(ctx, err)
			return
		}
	}
	md.SetId(id)

	if err = baseService.ExecuteInTransaction(ctx, func(tx *gorm.DB) (err error) {
		omits := []string{"resource_id", "created_at", "deleted_at"}
		selects := []string{"*"}
		switch t := any(md).(type) {
		case *model.Asset:
			if err = handleAuthorization(ctx, tx, model.ACTION_UPDATE, t, nil); err != nil {
				handleRemoteErr(ctx, err)
				return
			}
			if cast.ToBool(ctx.Value("isAuthWithKey")) {
				selects = []string{"ip", "protocols", "authorization"}
			}
		case *model.Account:
			if cast.ToBool(ctx.Value("isAuthWithKey")) {
				selects = []string{"account", "password", "phrase", "pk", "account_type"}
			}
		}

		if err = tx.Select(selects).Omit(omits...).Save(md).Error; err != nil {
			return
		}

		// Create history using history service
		err = historyService.CreateAndSaveHistory(ctx, model.ACTION_UPDATE, md, old, currentUser.Uid)
		return
	}); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrDuplicateName, Data: map[string]any{"err": err}})
			return
		}
		ctx.AbortWithError(http.StatusInternalServerError, &myErrors.ApiError{Code: myErrors.ErrInternal, Data: map[string]any{"err": err}})
		return
	}

	ctx.JSON(http.StatusOK, HttpResponse{
		Data: map[string]any{
			"id": md.GetId(),
		},
	})

	return
}

func doGet[T any](ctx *gin.Context, needAcl bool, dbFind *gorm.DB, resourceType string, postHooks ...postHook[T]) (err error) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	if needAcl && !acl.IsAdmin(currentUser) {
		if dbFind, err = handleAcl[T](ctx, dbFind, resourceType); err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, &myErrors.ApiError{Code: myErrors.ErrInternal, Data: map[string]any{"err": err}})
			return
		}
	}

	dbCount := dbFind.Session(&gorm.Session{})
	dbFind = dbFind.Session(&gorm.Session{})

	count, list := int64(0), make([]T, 0)
	eg := &errgroup.Group{}
	eg.Go(func() error {
		return dbCount.Count(&count).
			Error
	})
	eg.Go(func() error {
		pi, ps := cast.ToInt(ctx.Query("page_index")), cast.ToInt(ctx.Query("page_size"))
		if _, ok := ctx.GetQuery("page_index"); ok {
			dbFind = dbFind.Offset((pi - 1) * ps)
		}
		if _, ok := ctx.GetQuery("page_size"); ok && ps != -1 {
			dbFind = dbFind.Limit(lo.Ternary(ps == 0, 20, ps))
		}
		return dbFind.
			Order("id DESC").
			Find(&list).
			Error
	})
	if err = eg.Wait(); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &myErrors.ApiError{Code: myErrors.ErrInternal, Data: map[string]any{"err": err}})
		return
	}

	for _, hook := range postHooks {
		if hook == nil {
			continue
		}
		hook(ctx, list)
		if ctx.IsAborted() {
			return
		}
	}

	if err = handlePermissions(ctx, list, resourceType); err != nil {
		return
	}

	res := &ListData{
		Count: count,
		List:  lo.Map(list, func(t T, _ int) any { return t }),
	}

	ctx.JSON(http.StatusOK, NewHttpResponseWithData(res))

	return
}

func handleRemoteErr(ctx *gin.Context, err error) {
	switch e := err.(type) {
	case *remote.RemoteError:
		if e.HttpCode == http.StatusBadRequest {
			ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrRemoteClient, Data: e.Resp})
			return
		}
		ctx.AbortWithError(http.StatusInternalServerError, &myErrors.ApiError{Code: myErrors.ErrRemoteServer, Data: e.Resp})
	default:
		ctx.AbortWithError(http.StatusInternalServerError, &myErrors.ApiError{Code: myErrors.ErrInternal, Data: map[string]any{"err": err}})
	}
}

func getEmpty[T model.Model](data T) T {
	t := reflect.TypeOf(data).Elem()
	v := reflect.New(t)
	return v.Interface().(T)
}

func hasPerm[T model.Model](ctx context.Context, md T, resourceTypeName, action string) bool {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	if acl.IsAdmin(currentUser) {
		return true
	}

	if ok, _ := acl.HasPermission(ctx, currentUser.GetRid(), resourceTypeName, md.GetResourceId(), action); ok {
		return true
	}

	pids := make([]int, 0)
	switch t := any(md).(type) {
	case *model.Asset:
		pids, _ = repository.HandleSelfParent(ctx, t.ParentId)
	case *model.Node:
		pids, _ = repository.HandleSelfParent(ctx, t.Id)
	}

	if len(pids) > 0 {
		res, _ := acl.GetRoleResources(ctx, currentUser.GetRid(), config.RESOURCE_NODE)
		resId2perms := lo.SliceToMap(res, func(r *acl.Resource) (int, []string) { return r.ResourceId, r.Permissions })
		resId2perms, _ = handleSelfChildPerms(ctx, resId2perms)
		nodes, _ := repository.GetAllFromCacheDb(ctx, model.DefaultNode)
		id2resId := lo.SliceToMap(nodes, func(n *model.Node) (int, int) { return n.Id, n.ResourceId })
		if lo.ContainsBy(pids, func(pid int) bool { return lo.Contains(resId2perms[id2resId[pid]], action) }) {
			return true
		}
	}

	return false
}

func handlePermissions[T any](ctx *gin.Context, data []T, resourceTypeName string) (err error) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	if !lo.Contains(config.PermResource, resourceTypeName) {
		return
	}

	res, err := acl.GetRoleResources(ctx, currentUser.GetRid(), resourceTypeName)
	if err != nil {
		handleRemoteErr(ctx, err)
		return
	}
	resId2perms := lo.SliceToMap(res, func(r *acl.Resource) (int, []string) { return r.ResourceId, r.Permissions })

	switch ds := any(data).(type) {
	case []*model.Node:
		resId2perms, err = handleSelfChildPerms(ctx, resId2perms)
		if err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, &myErrors.ApiError{Code: myErrors.ErrInternal, Data: map[string]any{"err": err}})
			return
		}
	case []*model.Asset:
		res, err = acl.GetRoleResources(ctx, currentUser.GetRid(), config.RESOURCE_NODE)
		if err != nil {
			handleRemoteErr(ctx, err)
			return
		}
		nodeResId2perms := lo.SliceToMap(res, func(r *acl.Resource) (int, []string) { return r.ResourceId, r.Permissions })
		if nodeResId2perms, err = handleSelfChildPerms(ctx, nodeResId2perms); err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, &myErrors.ApiError{Code: myErrors.ErrInternal, Data: map[string]any{"err": err}})
			return
		}
		var nodeId2ResId map[int]int
		nodeId2ResId, err = getNodeId2ResId(ctx)
		if err != nil {
			return
		}
		for _, d := range ds {
			resId2perms[d.GetResourceId()] = append(
				resId2perms[d.GetResourceId()],
				nodeResId2perms[nodeId2ResId[d.ParentId]]...,
			)
		}
	}

	ds := lo.Map(data, func(d T, _ int) model.Model {
		x, _ := any(d).(model.Model)
		return x
	})
	b := acl.IsAdmin(currentUser)
	for _, d := range ds {
		if b {
			d.SetPerms(acl.AllPermissions)
			continue
		}
		d.SetPerms(resId2perms[d.GetResourceId()])
	}

	return
}

func handleAcl[T any](ctx *gin.Context, dbFind *gorm.DB, resourceType string) (db *gorm.DB, err error) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	resIds, err := acl.GetRoleResourceIds(ctx, currentUser.Acl.Rid, resourceType)
	if err != nil {
		return
	}
	switch any(*new(T)).(type) {
	case *model.Node:
		db, err = repository.HandleNodeIds(ctx, dbFind, resIds)
	case *model.Asset:
		db, err = repository.HandleAssetIds(ctx, dbFind, resIds)
	case *model.Account:
		db, err = repository.HandleAccountIds(ctx, dbFind, resIds)
	case *model.Gateway:
		db = dbFind
	case *model.Command:
		db = dbFind
	default:
		db = dbFind.Where("resource_id IN ?", resIds)
	}

	return
}
