package acl

import (
	"context"
	"strings"
	"time"

	"github.com/veops/oneterm/pkg/config"
	"github.com/veops/oneterm/pkg/remote"
)

// Resolve current roles without retaining a login cookie.
func PAMCurrentUser(ctx context.Context, uid int) (*Session, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	subject, err := GetPAMKeySubject(ctx, uid)
	if err != nil || subject.Blocked {
		return nil, ErrPAMKeyEvidence
	}
	token, err := remote.GetAclToken(ctx)
	if err != nil {
		return nil, ErrIdentityUnavailable
	}
	var data UserInfoResp
	response, err := identityClient.R().SetContext(ctx).SetHeader("App-Access-Token", token).
		SetQueryParams(map[string]string{"username": subject.Username, "app_id": config.Cfg.Auth.Acl.AppId}).SetResult(&data).
		Get(strings.TrimRight(config.Cfg.Auth.Acl.Url, "/") + "/acl/users/info")
	if err != nil || response.StatusCode() != 200 {
		return nil, ErrIdentityUnavailable
	}
	if data.Result.UID != uid || data.Result.Rid <= 0 {
		return nil, ErrPAMKeyEvidence
	}
	return &Session{Uid: uid, Acl: Acl{Uid: uid, Rid: data.Result.Rid, UserName: subject.Username, NickName: subject.Nickname, ParentRoles: data.Result.Role.Permissions}}, nil
}
