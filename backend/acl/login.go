package acl

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/samber/lo"
	"github.com/veops/oneterm/conf"
	mysql "github.com/veops/oneterm/db"
	"github.com/veops/oneterm/logger"
	"github.com/veops/oneterm/model"
	"github.com/veops/oneterm/remote"
	"github.com/veops/oneterm/util"
	"go.uber.org/zap"
)

func LoginByPassword(ctx context.Context, username string, password string) (sess *Session, err error) {
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
			"password": fmt.Sprintf("%x", md5.Sum([]byte(password))),
			// "password": password,
		}).
		Post(url)
	if err = remote.HandleErr(err, resp, func(dt map[string]any) bool { return true }); err != nil {
		return
	}

	cookie, ok := lo.Find(resp.Cookies(), func(c *http.Cookie) bool { return c.Name == "session" })
	if !ok {
		err = errors.New("empty cookie")
		return
	}
	return ParseCookie(cookie.Value)
}

func LoginByPublicKey(ctx context.Context, username string, pk string) (sess *Session, err error) {
	pk = strings.TrimSpace(pk)
	enc := util.EncryptAES(pk)
	cnt := int64(0)
	if err = mysql.DB.Model(&model.PublicKey{}).Where("username = ? AND pk = ?", username, enc).Count(&cnt).Error; err != nil || cnt == 0 {
		err = fmt.Errorf("%w", err)
		logger.L().Warn("find pk failed", zap.Int64("cnt", cnt), zap.Error(err))
		return
	}

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
	sess = &Session{
		Uid: data.Result.UID,
		Acl: Acl{
			Uid:         data.Result.UID,
			UserName:    data.Result.Username,
			Rid:         data.Result.Rid,
			NickName:    data.Result.Name,
			ParentRoles: data.Result.Role.Permissions,
		},
	}

	return

	// bs, _ := json.Marshal(sess)

	// s := NewSignature(conf.Cfg.SecretKey, "cookie-session", "", "hmac", nil, nil)
	// buf := &bytes.Buffer{}
	// zw := zlib.NewWriter(buf)
	// _, _ = zw.Write(bs)
	// _ = zw.Close()
	// value := "." + base64.RawURLEncoding.EncodeToString(buf.Bytes())
	// dk, _ := s.DeriveKey()
	// sign := s.Algorithm.GetSignature(dk, value)
	// vs := value + "." + base64.RawURLEncoding.EncodeToString(sign)

	// cookie = "session=" + vs

	// return
}

func ParseCookie(cookie string) (sess *Session, err error) {
	s := NewSignature(conf.Cfg.SecretKey, "cookie-session", "", "hmac", nil, nil)
	content, err := s.Unsign(cookie)
	if err != nil {
		logger.L().Error("cannot unsign", zap.Error(err))
		return
	}
	sess = &Session{}
	err = json.Unmarshal(content, &sess)
	if err != nil {
		logger.L().Error("cannot unmarshal to session", zap.Error(err))
		return
	}

	return
}
