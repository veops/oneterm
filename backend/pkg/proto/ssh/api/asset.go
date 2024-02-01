package api

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"

	cfg "github.com/veops/oneterm/pkg/proto/ssh/config"
	"github.com/veops/oneterm/pkg/server/controller"
	"github.com/veops/oneterm/pkg/server/model"
	"github.com/veops/oneterm/pkg/util"
)

type AssetCore struct {
	Api    string
	XToken string
}

func NewAssetServer(Api, token string) *AssetCore {
	return &AssetCore{
		Api:    Api,
		XToken: token,
	}
}

func (a *AssetCore) Groups() (res any, err error) {
	return res, err
}

func (a *AssetCore) Lists(cookie, search string, id int) (res *controller.ListData, err error) {
	client := resty.New()

	var (
		data *controller.HttpResponse
	)

	params := map[string]string{
		"page_index": "1",
		"page_size":  "-1",
		"info":       "true",
	}
	if strings.TrimSpace(search) != "" {
		params["search"] = search
	}
	if id > 0 {
		params["id"] = fmt.Sprintf("%d", id)
	}
	resp, err := client.R().
		SetQueryParams(params).
		SetHeader("Cookie", cookie).
		SetHeader("X-Token", a.XToken).
		SetResult(&data).
		Get(strings.TrimSuffix(a.Api, "/") + assetUrl)
	if err != nil {
		return res, fmt.Errorf("api request error:%v", err.Error())
	}
	if resp.StatusCode() != 200 {
		return res, fmt.Errorf("auth code: %d %v", resp.StatusCode(), string(resp.Body()))
	}
	if data.Code != 0 {
		return res, fmt.Errorf(data.Message)
	}
	err = util.DecodeStruct(&res, data.Data)
	return
}

func (a *AssetCore) AllAssets() (res []*model.Asset, err error) {
	params := map[string]string{
		"page_index": "1",
		"page_size":  "-1",
	}
	resp, err := request(resty.MethodGet,
		a.Api+assetTotalUrl,
		map[string]string{"X-Token": a.XToken}, params, nil)
	if resp != nil {
		for _, v := range resp.List {
			var v1 model.Asset
			_ = util.DecodeStruct(&v1, v)
			res = append(res, &v1)
		}
	}
	return
}

func (a *AssetCore) HasPermission(data *model.AccessAuth) bool {
	now := time.Now()
	in := true
	if (data.Start != nil && now.Before(*data.Start)) || (data.End != nil && now.After(*data.End)) {
		in = false
	}
	if !in {
		return false
	}
	in = false
	week, hm := now.Weekday(), now.Format("15:04")
	for _, r := range data.Ranges {
		if (r.Week+1)%7 == int(week) {
			for _, str := range r.Times {
				ss := strings.Split(str, "~")
				in = in || (len(ss) >= 2 && hm >= ss[0] && hm <= ss[1])
			}
		}
	}
	return in == data.Allow
}

func (a *AssetCore) Gateway(cookie string, id int) (res *model.Gateway, err error) {
	params := map[string]string{
		"page_index": "1",
		"page_size":  "1",
		"info":       "true",
	}
	if id > 0 {
		params["id"] = fmt.Sprintf("%d", id)
	}

	data, err := request(resty.MethodGet,
		fmt.Sprintf("%s%s", strings.TrimSuffix(cfg.SSHConfig.Api, "/"), gatewayUrl),
		map[string]string{
			"Cookie":  cookie,
			"X-Token": cfg.SSHConfig.Token,
		}, params, nil)
	if err != nil {
		return
	}
	var r1 controller.ListData
	err = util.DecodeStruct(&r1, data)
	if err != nil {
		return
	}
	if len(r1.List) == 0 {
		err = fmt.Errorf("not found gateway for %d", id)
		return
	}
	err = util.DecodeStruct(&res, r1.List[0])
	return
}

func (a *AssetCore) Commands(cookie string) (res []*model.Command, err error) {
	params := map[string]string{
		"page_index": "1",
		"page_size":  "-1",
		"info":       "true",
	}

	data, err := request(resty.MethodGet,
		fmt.Sprintf("%s%s", strings.TrimSuffix(cfg.SSHConfig.Api, "/"), commandUrl),
		map[string]string{
			"Cookie":  cookie,
			"X-Token": cfg.SSHConfig.Token,
		}, params, nil)
	if err != nil {
		return
	}
	var r1 controller.ListData
	err = util.DecodeStruct(&r1, data)
	if err != nil {
		return
	}
	err = util.DecodeStruct(&res, r1.List)
	return
}

func (a *AssetCore) Config(cookie string) (res *model.Config, err error) {
	params := map[string]string{
		"info": "true",
	}
	data := &controller.HttpResponse{}
	_, err = resty.New().R().
		SetQueryParams(params).
		SetHeaders(map[string]string{
			"Cookie":  cookie,
			"X-Token": cfg.SSHConfig.Token,
		}).
		SetResult(data).
		Get(strings.TrimSuffix(a.Api, "/") + configUrl)
	if err != nil {
		return
	}

	err = util.DecodeStruct(&res, data.Data)
	return
}

func (a *AssetCore) ChangeState(data map[int]map[string]any) error {
	_, err := request(resty.MethodPut,
		strings.TrimSuffix(a.Api, "/")+assetUpdateState,
		map[string]string{
			"X-Token": cfg.SSHConfig.Token,
		}, nil, data)
	return err
}
