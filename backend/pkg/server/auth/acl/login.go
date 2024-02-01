package acl

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/veops/oneterm/pkg/conf"
	"github.com/veops/oneterm/pkg/server/remote"
)

func LoginByPassword(ctx context.Context, username string, password string) (cookie string, err error) {
	url := fmt.Sprintf("%s/acl/login", conf.Cfg.Auth.Acl.Url)
	data := make(map[string]any)
	resp, err := remote.RC.R().
		SetHeaders(map[string]string{
			"User-Agent": "oneterm",
		}).
		SetQueryParams(map[string]string{
			"channel": "ssh",
		}).
		SetResult(&data).
		SetBody(map[string]any{
			"username": username,
			// "password": fmt.Sprintf("%x", md5.Sum([]byte(password))),
			"password": password,
		}).
		Post(url)
	if err = remote.HandleErr(err, resp, func(dt map[string]any) bool { return true }); err != nil {
		return
	}

	cookie = resp.Header().Get("Set-Cookie")

	return
}

func LoginByPublicKey(ctx context.Context, username string) (cookie string, err error) {
	token, err := remote.GetAclToken(ctx)
	if err != nil {
		return
	}

	url := fmt.Sprintf("%s/acl/users/info", conf.Cfg.Auth.Acl.Url)
	data := &UserInfoResp{}
	resp, err := remote.RC.R().
		SetHeaders(map[string]string{
			"App-Access-Token": token,
			"User-Agent":       "oneterm",
		}).
		SetQueryParams(map[string]string{
			"channel": "ssh",
		}).
		SetQueryParam("username", username).
		SetResult(&data).
		Get(url)
	if err = remote.HandleErr(err, resp, func(dt map[string]any) bool { return true }); err != nil {
		return
	}
	sess := &Session{
		Uid: data.Result.UID,
		Acl: Acl{
			Uid:         data.Result.UID,
			UserName:    data.Result.Username,
			Rid:         data.Result.Rid,
			NickName:    data.Result.Name,
			ParentRoles: data.Result.Role.Permissions,
		},
	}

	bs, _ := json.Marshal(sess)

	s := NewSignature(conf.Cfg.SecretKey, "cookie-session", "", "hmac", nil, nil)
	buf := &bytes.Buffer{}
	zw := zlib.NewWriter(buf)
	_, _ = zw.Write(bs)
	_ = zw.Close()
	value := "." + base64.RawURLEncoding.EncodeToString(buf.Bytes())
	dk, _ := s.DeriveKey()
	sign := s.Algorithm.GetSignature(dk, value)
	vs := value + "." + base64.RawURLEncoding.EncodeToString(sign)

	cookie = "session=" + vs

	return
}
