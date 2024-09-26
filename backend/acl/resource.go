package acl

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/veops/oneterm/conf"
	mysql "github.com/veops/oneterm/db"
	"github.com/veops/oneterm/logger"
	"github.com/veops/oneterm/model"
	"github.com/veops/oneterm/remote"
)

func init() {
	migrateNode()
}

type ResourceType struct {
	Name  string   `json:"name"`
	Perms []string `json:"perms"`
}

type ResourceTypeResp struct {
	Groups []*ResourceType `json:"groups"`
}

func migrateNode() {
	ctx := context.Background()

	rts, err := GetResourceTypes(ctx)
	if err != nil {
		logger.L().Fatal("get resource type failed", zap.Error(err))
	}

	if !lo.ContainsBy(rts, func(rt *ResourceType) bool { return rt.Name == "node" }) {
		if err = AddResourceTypes(ctx, &ResourceType{Name: "node", Perms: AllPermissions}); err != nil {
			logger.L().Fatal("add resource type failed", zap.Error(err))
		}
	}

	nodes := make([]*model.Node, 0)
	if err = mysql.DB.Model(&nodes).Where("resource_id=0").Find(&nodes).Error; err != nil {
		logger.L().Fatal("get nodes failed", zap.Error(err))
	}
	eg := errgroup.Group{}
	for _, n := range nodes {
		nd := n
		eg.Go(func() error {
			r, err := AddResource(ctx, nd.CreatorId, "node", cast.ToString(nd.Id))
			if err != nil {
				return err
			}
			if err := mysql.DB.Model(&nd).Where("id=?", nd.Id).Update("resource_id", r.ResourceId).Error; err != nil {
				return err
			}
			return nil
		})
	}
	if err = eg.Wait(); err != nil {
		logger.L().Fatal("add resource failed", zap.Error(err))
	}
}

func GetResourceTypes(ctx context.Context) (rt []*ResourceType, err error) {
	token, err := remote.GetAclToken(ctx)
	if err != nil {
		return
	}

	data := &ResourceTypeResp{}
	url := fmt.Sprintf("%s/acl/resource_types", conf.Cfg.Auth.Acl.Url)
	resp, err := remote.RC.R().
		SetHeaders(map[string]string{
			"App-Access-Token": token,
		}).
		SetQueryParams(map[string]string{
			"app_id":    "oneterm",
			"page_size": "100",
		}).
		SetResult(data).
		Get(url)
	err = remote.HandleErr(err, resp, func(dt map[string]any) bool { return true })

	rt = data.Groups

	return
}

func AddResourceTypes(ctx context.Context, rt *ResourceType) (err error) {
	token, err := remote.GetAclToken(ctx)
	if err != nil {
		return
	}

	url := fmt.Sprintf("%s/acl/resource_types", conf.Cfg.Auth.Acl.Url)
	resp, err := remote.RC.R().
		SetHeaders(map[string]string{
			"App-Access-Token": token,
		}).
		SetBody(rt).
		Post(url)
	err = remote.HandleErr(err, resp, func(dt map[string]any) bool { return true })

	return
}

func AddResource(ctx context.Context, uid int, resourceTypeId string, name string) (res *Resource, err error) {
	token, err := remote.GetAclToken(ctx)
	if err != nil {
		return
	}

	res = &Resource{}
	url := fmt.Sprintf("%s/acl/resources", conf.Cfg.Auth.Acl.Url)
	resp, err := remote.RC.R().
		SetHeaders(map[string]string{
			"App-Access-Token": token,
			"X-User-Id":        cast.ToString(uid),
			"Accept-Language":  getAcceptLanguage(ctx)}).
		SetBody(map[string]any{
			"type_id": resourceTypeId,
			"name":    name,
			"uid":     uid,
		}).
		SetResult(&res).
		Post(url)
	err = remote.HandleErr(err, resp, func(dt map[string]any) bool { return true })
	return
}

func DeleteResource(ctx context.Context, uid int, resourceId int) (err error) {
	token, err := remote.GetAclToken(ctx)
	if err != nil {
		return
	}

	url := fmt.Sprintf("%v/acl/resources/%v", conf.Cfg.Auth.Acl.Url, resourceId)
	resp, err := remote.RC.R().
		SetHeaders(map[string]string{
			"App-Access-Token": token,
			"X-User-Id":        cast.ToString(uid),
			"Accept-Language":  getAcceptLanguage(ctx)}).
		Delete(url)
	err = remote.HandleErr(err, resp, func(dt map[string]any) bool { return true })
	return
}

func UpdateResource(ctx context.Context, uid int, resourceId int, updates map[string]string) (err error) {
	token, err := remote.GetAclToken(ctx)
	if err != nil {
		return
	}

	url := fmt.Sprintf("%s/acl/resources/%d", conf.Cfg.Auth.Acl.Url, resourceId)
	resp, err := remote.RC.R().
		SetHeaders(map[string]string{
			"App-Access-Token": token,
			"X-User-Id":        cast.ToString(uid),
			"Accept-Language":  getAcceptLanguage(ctx)}).
		SetFormData(updates).
		Put(url)
	err = remote.HandleErr(err, resp, func(dt map[string]any) bool { return true })
	return
}

func GetResourcePermissions(ctx context.Context, resourceId int) (res map[string]*ResourcePermissionsRespItem, err error) {
	token, err := remote.GetAclToken(ctx)
	if err != nil {
		return
	}
	res = make(map[string]*ResourcePermissionsRespItem)
	url := fmt.Sprintf("%v/acl/resources/%v/permissions", conf.Cfg.Auth.Acl.Url, resourceId) //TODO conf
	resp, err := remote.RC.R().
		SetHeader("App-Access-Token", token).
		SetResult(&res).
		Get(url)
	err = remote.HandleErr(err, resp, func(dt map[string]any) bool { return true })
	return
}

func getAcceptLanguage(ctx context.Context) string {
	ginCtx, ok := ctx.(*gin.Context)
	if !ok {
		return ""
	}
	return ginCtx.GetHeader("Accept-Language")
}
