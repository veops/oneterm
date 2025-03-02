// Package acl
package acl

import (
	"context"
	"fmt"
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

func CreateGrantAcl(ctx context.Context, session *Session, resourceType string, resourceName string) (resourceId int, err error) {
	resource, err := AddResource(ctx, session.GetUid(), resourceType, resourceName)
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
	resource, err := AddResource(ctx, session.GetUid(), resourceType, resourceName)
	if err != nil {
		return
	}

	resourceId = resource.ResourceId

	return
}
