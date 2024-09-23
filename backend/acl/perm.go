// Package acl
package acl

import (
	"context"
	"fmt"

	"github.com/veops/oneterm/conf"
)

func GetSessionFromCtx(ctx context.Context) (res *Session, err error) {
	res, ok := ctx.Value("session").(*Session)
	if !ok || res == nil {
		err = fmt.Errorf("empty session")
	}
	return
}

func IsAdmin(session *Session) bool {
	for _, pr := range session.Acl.ParentRoles {
		if pr == "admin" || pr == "acl_admin" || pr == "oneterm_admin" {
			return true
		}
	}
	return false
}

func GetResourceTypeName(resourceType string) string {
	names := conf.Cfg.Auth.Acl.ResourceNames
	for _, v := range names {
		if v.Key == resourceType {
			return v.Value
		}
	}
	return "NONE"
}

func CreateGrantAcl(ctx context.Context, session *Session, resourceType string, resourceName string) (resourceId int, err error) {
	resource, err := AddResource(ctx,
		session.GetUid(),
		GetResourceTypeName(resourceType),
		resourceName)
	if err != nil {
		return
	}

	if err = GrantRoleResource(ctx, session.GetUid(), session.Acl.Rid, resource.ResourceId, AllPermissions); err != nil {
		return
	}

	resourceId = resource.ResourceId

	return
}

func CreateAcl(ctx context.Context, session *Session, resourceType string, resourceName string) (resourceId int, err error) {
	resource, err := AddResource(ctx,
		session.GetUid(),
		GetResourceTypeName(resourceType),
		resourceName)
	if err != nil {
		return
	}

	resourceId = resource.ResourceId

	return
}
