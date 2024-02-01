// Package acl
package acl

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"

	"github.com/veops/oneterm/pkg/conf"
)

func GetSessionFromCtx(ctx *gin.Context) (res *Session, err error) {
	res, ok := ctx.Value("session").(*Session)
	if !ok || res == nil {
		err = fmt.Errorf("empty session")
	}
	return
}

func HasPerm(resourceId int, rid int, action string) bool {
	mapping, err := GetResourcePermissions(context.Background(), resourceId)
	if err != nil {
		return false
	}
	for _, v := range mapping {
		if lo.ContainsBy(v.Perms, func(p *Perm) bool { return p.Rid == rid && p.Name == action }) {
			return true
		}
	}
	return false
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
