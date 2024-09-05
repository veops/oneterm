package acl

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	WRITE  = "write"
	DELETE = "delete"
	READ   = "read"
	GRANT  = "grant"
)

var (
	AllPermissions = []string{WRITE, DELETE, READ, GRANT}
)

type Role struct {
	Permissions []string `json:"permissions"`
}

type ResourceResult struct {
	Groups    []any       `json:"groups"`
	Resources []*Resource `json:"resources"`
}
type Resource struct {
	AppID          int      `json:"app_id"`
	CreatedAt      string   `json:"created_at"`
	Deleted        bool     `json:"deleted"`
	DeletedAt      string   `json:"-"`
	ResourceId     int      `json:"id"`
	Name           string   `json:"name"`
	Permissions    []string `json:"permissions"`
	ResourceTypeID int      `json:"resource_type_id"`
	UID            int      `json:"uid"`
	UpdatedAt      string   `json:"updated_at"`
}

type Perm struct {
	Name string `json:"name"`
	Rid  int    `json:"rid"`
}

type ResourcePermissionsRespItem struct {
	Perms []*Perm `json:"perms"`
}

type Acl struct {
	Uid         int      `json:"uid"`
	UserName    string   `json:"userName"`
	Rid         int      `json:"rid"`
	RoleName    string   `json:"roleName"`
	ParentRoles []string `json:"parentRoles"`
	ChildRoles  []string `json:"childRoles"`
	NickName    string   `json:"nickName"`
}

type Session struct {
	Uid    int          `json:"uid"`
	Acl    Acl          `json:"acl"`
	Cookie *http.Cookie `json:"raw"`
}

func (s *Session) GetUid() int {
	return s.Uid
}

func (s *Session) GetRid() int {
	return s.Acl.Rid
}

func (s *Session) GetUserName() string {
	return s.Acl.UserName
}

func (a *Acl) GetUserName(ctx *gin.Context) string {
	res, exist := ctx.Get("session")
	if exist {
		if v, ok := res.(*Session); ok {
			return v.GetUserName()
		}
	}
	return ""
}

func (a *Acl) GetUserInfo(ctx *gin.Context) (any, error) {
	res, exist := ctx.Get("session")
	if exist {
		if v, ok := res.(*Session); ok {
			return v, nil
		}
	}
	return res, fmt.Errorf("no session")
}

type UserInfoResp struct {
	Result UserInfoRespResult `json:"result"`
}

type UserInfoRespResult struct {
	Avatar   string `json:"avatar"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Rid      int    `json:"rid"`
	Role     Role   `json:"role"`
	UID      int    `json:"uid"`
	Username string `json:"username"`
}

type AuthWithKeyResp struct {
	User AuthWithKeyResult `json:"user"`
}

type AuthWithKeyResult struct {
	Avatar      string   `json:"avatar"`
	Email       string   `json:"email"`
	Name        string   `json:"name"`
	Rid         int      `json:"rid"`
	Role        Role     `json:"role"`
	UID         int      `json:"uid"`
	Username    string   `json:"username"`
	ParentRoles []string `json:"parentRoles"`
}
