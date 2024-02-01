package api

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"

	"github.com/veops/oneterm/pkg/server/auth/acl"
	"github.com/veops/oneterm/pkg/server/controller"
	"github.com/veops/oneterm/pkg/server/model"
	"github.com/veops/oneterm/pkg/util"
)

type Auth struct {
	Username  string
	Password  string
	PublicKey string

	Api       string
	XToken    string
	SecretKey string
}

func NewAuthServer(username, password, publicKey, Api, token, secretKey string) *Auth {
	return &Auth{
		Username:  username,
		Password:  password,
		PublicKey: publicKey,

		Api:       Api,
		XToken:    token,
		SecretKey: secretKey,
	}
}

func (a *Auth) Authenticate() (token string, err error) {
	client := resty.New()
	var (
		method int8
		data   *controller.HttpResponse
	)
	if a.Password != "" {
		method = 1
	} else if a.PublicKey != "" {
		method = 2
	} else {
		return "", fmt.Errorf("no password or publicKey")
	}

	resp, err := client.R().
		SetHeader("X-Token", a.XToken).
		SetHeader("Content-Type", "application/json").
		SetBody(map[string]interface{}{
			"method":   method,
			"password": a.Password,
			"pk":       a.PublicKey,
			"username": a.Username,
		}).
		SetResult(&data).
		Post(strings.TrimSuffix(a.Api, "/") + authUrl)
	if err != nil {
		return "", fmt.Errorf("api request error:%v", err.Error())
	}
	if resp.StatusCode() != 200 {
		return "", fmt.Errorf("%s", string(resp.Body()))
	}
	if data.Code != 0 {
		return "", fmt.Errorf(data.Message)
	}
	return data.Data.(map[string]any)["cookie"].(string), nil
}

func (a *Auth) AccountInfo(token string, uid int, name string) (account *model.Account, err error) {
	data := map[string]string{"info": "true"}
	if uid > 0 {
		data["id"] = fmt.Sprintf("%d", uid)
	}
	if name != "" {
		data["name"] = name
	}
	res, err := request(resty.MethodGet,
		fmt.Sprintf("%s%s", strings.TrimSuffix(a.Api, "/"), accountUrl),
		map[string]string{
			"Cookie":  token,
			"X-Token": a.XToken,
		}, data, nil)
	if err != nil {
		return
	}
	if res.Count == 0 {
		err = fmt.Errorf("no account found for %v", uid)
		return
	}
	err = util.DecodeStruct(&account, res.List[0])
	return
}

func (a *Auth) Accounts(token string) (account []*model.Account, err error) {
	res, err := request(resty.MethodGet,
		fmt.Sprintf("%s%s", strings.TrimSuffix(a.Api, "/"), accountUrl),
		map[string]string{
			"Cookie":  token,
			"X-Token": a.XToken,
		}, map[string]string{"info": "true"}, nil)
	if err != nil {
		return
	}
	if res.Count == 0 {
		err = fmt.Errorf("no account found")
		return
	}
	err = util.DecodeStruct(&account, res.List)
	return
}

func request(method, path string, headers map[string]string, param map[string]string,
	body any) (res *controller.ListData, err error) {
	client := resty.New().SetTimeout(time.Second * 15).R()
	if param != nil {
		client = client.SetQueryParams(param)
	}
	for k, v := range headers {
		client = client.SetHeader(k, v)
	}
	if body != nil {
		client = client.SetBody(body)
	}
	var response *controller.HttpResponse
	client = client.SetResult(&response)
	r, err := client.Execute(method, path)
	if err != nil {
		err = fmt.Errorf("api request error:%v", err.Error())
		return
	}

	if r.StatusCode() != 200 {
		err = fmt.Errorf("auth code: %d: %s", r.StatusCode(), r.String())
		return
	}
	if response.Code != 0 {
		err = fmt.Errorf(response.Message)
		return
	}
	err = util.DecodeStruct(&res, response.Data)
	return res, err
}

func (a *Auth) AclInfo(sess string) (aclInfo *acl.Acl, er error) {
	session := acl.Session{}
	for _, v := range strings.Split(sess, ";") {
		if strings.HasPrefix(strings.TrimSpace(v), "session=") {
			sess = strings.TrimPrefix(strings.TrimSpace(v), "session=")
		}
	}
	s := acl.NewSignature(a.SecretKey, "cookie-session", "", "hmac", nil, nil)
	content, err := s.Unsign(sess)
	if err != nil {
		er = err
		return
	}
	err = json.Unmarshal(content, &session)
	if err != nil {
		return aclInfo, err
	}
	return &session.Acl, nil
}
