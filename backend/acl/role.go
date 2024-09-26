package acl

import (
	"context"
	"fmt"

	"github.com/samber/lo"
	"github.com/spf13/cast"
	"golang.org/x/sync/errgroup"

	"github.com/veops/oneterm/conf"
	"github.com/veops/oneterm/remote"
)

const (
	kFmtResources = "resource-%s-%d"
)

func GetRoleResources(ctx context.Context, rid int, resourceTypeId string) (res []*Resource, err error) {
	// k := fmt.Sprintf(kFmtResources, resourceTypeId, rid)
	// if err = redis.Get(ctx, k, &res); err == nil {
	// 	return
	// }

	token, err := remote.GetAclToken(ctx)
	if err != nil {
		return
	}

	data := &ResourceResult{}
	url := fmt.Sprintf("%s/acl/roles/%d/resources", conf.Cfg.Auth.Acl.Url, rid)
	resp, err := remote.RC.R().
		SetHeader("App-Access-Token", token).
		SetQueryParams(map[string]string{
			"app_id":           conf.Cfg.Auth.Acl.AppId,
			"resource_type_id": resourceTypeId,
		}).
		SetResult(data).
		Get(url)

	if err = remote.HandleErr(err, resp, func(dt map[string]any) bool { return true }); err != nil {
		return
	}

	res = data.Resources

	// redis.SetEx(ctx, k, res, time.Minute)

	return
}

func GetRoleResourceIds(ctx context.Context, rid int, resourceTypeId string) (ids []int, err error) {
	res, err := GetRoleResources(ctx, rid, resourceTypeId)
	if err != nil {
		return
	}

	ids = lo.Map(res, func(r *Resource, _ int) int { return r.ResourceId })
	return
}

func HasPermission(ctx context.Context, rid int, resourceTypeName string, resourceId int, permission string) (res bool, err error) {
	token, err := remote.GetAclToken(ctx)
	if err != nil {
		return false, err
	}

	data := make(map[string]any)
	url := fmt.Sprintf("%s/acl/roles/has_perm", conf.Cfg.Auth.Acl.Url)
	resp, err := remote.RC.R().
		SetHeader("App-Access-Token", token).
		SetQueryParams(map[string]string{
			"rid":                cast.ToString(rid),
			"resource_id":        cast.ToString(resourceId),
			"resource_type_name": resourceTypeName,
			"perm":               permission,
		}).
		SetResult(&data).
		Get(url)
	if err = remote.HandleErr(err, resp, func(dt map[string]any) bool { return true }); err != nil {
		return
	}

	if v, ok := data["result"]; ok {
		res = v.(bool)
	}

	return
}

func GrantRoleResource(ctx context.Context, uid int, roleId int, resourceId int, permissions []string) (err error) {
	token, err := remote.GetAclToken(ctx)
	if err != nil {
		return
	}

	url := fmt.Sprintf("%s/acl/roles/%d/resources/%d/grant", conf.Cfg.Auth.Acl.Url, roleId, resourceId)
	resp, err := remote.RC.R().
		SetHeaders(map[string]string{
			"App-Access-Token": token,
			"X-User-Id":        cast.ToString(uid)}).
		SetBody(map[string]any{
			"perms": permissions,
		}).
		Post(url)
	err = remote.HandleErr(err, resp, func(dt map[string]any) bool { return true })
	return
}

func RevokeRoleResource(ctx context.Context, uid int, roleId int, resourceId int, permissions []string) (err error) {
	token, err := remote.GetAclToken(ctx)
	if err != nil {
		return
	}

	url := fmt.Sprintf("%s/acl/roles/%d/resources/%d/revoke", conf.Cfg.Auth.Acl.Url, roleId, resourceId)
	resp, err := remote.RC.R().
		SetHeaders(map[string]string{
			"App-Access-Token": token,
			"X-User-Id":        cast.ToString(uid)}).
		SetBody(map[string]any{
			"perms": permissions,
		}).
		Post(url)
	err = remote.HandleErr(err, resp, func(dt map[string]any) bool { return true })
	return
}

func BatchGrantRoleResource(ctx context.Context, uid int, roleIds []int, resourceId int, permissions []string) (err error) {
	eg := &errgroup.Group{}
	for _, rid := range roleIds {
		localRid := rid
		eg.Go(func() error {
			return GrantRoleResource(ctx, uid, localRid, resourceId, permissions)
		})
	}
	err = eg.Wait()

	return
}

func BatchRevokeRoleResource(ctx context.Context, uid int, roleIds []int, resourceId int, permissions []string) (err error) {
	eg := &errgroup.Group{}
	for _, rid := range roleIds {
		localRid := rid
		eg.Go(func() error {
			return RevokeRoleResource(ctx, uid, localRid, resourceId, permissions)
		})
	}
	err = eg.Wait()

	return
}
