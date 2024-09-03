package acl

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"

	"github.com/veops/oneterm/conf"
	"github.com/veops/oneterm/remote"
)

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
