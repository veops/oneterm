package service

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/veops/oneterm/internal/acl"
)

// Management discovery never supplies a connection grant or a credential value.
func accountManagementResources(ctx *gin.Context, grantOnly bool) ([]int, error) {
	actor, err := acl.GetSessionFromCtx(ctx)
	if err != nil || actor == nil || actor.Uid <= 0 {
		return nil, ErrPAMDenied
	}
	bounded, cancel := context.WithTimeout(ctx.Request.Context(), 10*time.Second)
	defer cancel()
	current, err := acl.PAMCurrentUser(bounded, actor.Uid)
	if err != nil || current == nil || current.GetRid() != actor.GetRid() {
		return nil, ErrPAMDenied
	}
	resources, err := acl.GetRoleResources(bounded, current.GetRid(), "account")
	if err != nil {
		return nil, ErrPAMUnavailable
	}
	ids := []int{}
	for _, resource := range resources {
		if resource == nil {
			continue
		}
		for _, action := range resource.Permissions {
			allowed := action == acl.GRANT
			if !grantOnly {
				switch action {
				case acl.READ, acl.WRITE, acl.DELETE, acl.GRANT, acl.Retrieve, acl.RequestAccess, acl.UpdateSecret, acl.VerifySecret, acl.RotateSecret, acl.RecoverSecret, acl.ManagePolicy, "request_retrieve":
					allowed = true
				}
			}
			if allowed {
				ids = append(ids, resource.ResourceId)
				break
			}
		}
	}
	return ids, nil
}
