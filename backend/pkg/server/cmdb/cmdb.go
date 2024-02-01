package cmdb

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/samber/lo"
	"github.com/spf13/cast"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/veops/oneterm/pkg/conf"
	"github.com/veops/oneterm/pkg/logger"
	"github.com/veops/oneterm/pkg/server/auth/acl"
	"github.com/veops/oneterm/pkg/server/controller"
	"github.com/veops/oneterm/pkg/server/model"
	"github.com/veops/oneterm/pkg/server/remote"
	"github.com/veops/oneterm/pkg/server/storage/db/mysql"
)

var (
	ctx, cancel = context.WithCancel(context.Background())
)

func Run() (err error) {
	currentUser := &acl.Session{
		Uid: conf.Cfg.Worker.Uid,
		Acl: acl.Acl{
			Rid: conf.Cfg.Worker.Rid,
		},
	}
	tk := time.NewTicker(time.Minute)
	last := make(map[int]time.Time)
	for {
		select {
		case <-tk.C:
			nodes, err := getNodes()
			if err != nil {
				logger.L.Error("get nodes faild", zap.Error(err))
				continue
			}
			now := time.Now()
			for _, n := range nodes {
				d, _ := time.ParseDuration(fmt.Sprintf("%fh", n.Sync.Frequency))
				if last[n.Id].Add(d).After(now) {
					continue
				}
				if n.Sync.TypeId <= 0 {
					continue
				}
				cis, err := getCis(n.Sync.TypeId, n.Sync.Filters)
				if err != nil {
					logger.L.Error("get cmdb failed", zap.Error(err))
					continue
				}
				for _, ci := range cis {
					a := &model.Asset{
						Ciid:          cast.ToInt(ci["_id"]),
						ParentId:      n.Id,
						UpdaterId:     conf.Cfg.Worker.Uid,
						Ip:            cast.ToString(ci[n.Sync.Mapping["ip"]]),
						Name:          fmt.Sprintf("%s@%v", cast.ToString(ci[n.Sync.Mapping["name"]]), time.Now().Format(time.RFC3339)),
						Protocols:     n.Protocols,
						Authorization: n.Authorization,
						AccessAuth:    n.AccessAuth,
					}
					resourceIds := make([]int, 0)
					if err := mysql.DB.Model(a).Select("resource_id").Where("ci_id = ?", a.Ciid).Find(&resourceIds).Error; err != nil {
						logger.L.Error("insert ci failed", zap.Error(err))
						continue
					}
					if !errors.Is(mysql.DB.Model(a).Where("ci_id = ?", a.Ciid).Where("parent_id = ?", a.ParentId).First(map[string]any{}).Error, gorm.ErrRecordNotFound) {
						continue
					}
					a.CreatorId = conf.Cfg.Worker.Uid

					a.ResourceId, err = acl.CreateAcl(ctx, currentUser, acl.GetResourceTypeName(conf.RESOURCE_ASSET), a.Name)
					if err != nil {
						continue
					}
					if err = mysql.DB.Transaction(func(tx *gorm.DB) (err error) {
						if err = tx.Create(a).Error; err != nil {
							return
						}
						err = controller.HandleAuthorization(currentUser, tx, model.ACTION_CREATE, nil, a)
						return
					}); err != nil {
						continue
					}
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

func Stop(err error) {
	defer cancel()
}

func getNodes() (res map[int]*model.Node, err error) {
	data := make([]*model.Node, 0)
	err = mysql.DB.
		Model(&model.Node{}).
		Where("enable = ?", 1).
		Find(&data).
		Error
	if err != nil {
		return
	}

	res = lo.SliceToMap(data, func(d *model.Node) (int, *model.Node) { return d.Id, d })

	return
}

func getCis(typeId int, filters string) (res []map[string]any, err error) {
	url := fmt.Sprintf("%s/ci/s", conf.Cfg.Cmdb.Url)
	params := map[string]any{
		"q": fmt.Sprintf("_type:(%d),%s", typeId, filters),
	}
	params["_secret"] = buildAPIKey(url, params)
	params["_key"] = conf.Cfg.Worker.Key
	ps := make(map[string]string)
	for k, v := range params {
		ps[k] = cast.ToString(v)
	}

	data := &GetCIResult{}
	resp, err := remote.RC.R().
		SetQueryParams(ps).
		SetResult(data).
		Get(url)

	if err = remote.HandleErr(err, resp, func(dt map[string]any) bool { return true }); err != nil {
		return
	}

	res = data.Result

	return
}

type GetCIResult struct {
	Counter  map[string]int   `json:"counter"`
	Facet    map[string]any   `json:"facet"`
	Numfound int              `json:"numfound"`
	Page     int              `json:"page"`
	Result   []map[string]any `json:"result"`
	Total    int              `json:"total"`
}

func buildAPIKey(u string, params map[string]any) string {
	pu, _ := url.Parse(u)
	keys := lo.Keys(params)
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	vals := strings.Join(
		lo.Map(keys, func(k string, _ int) string {
			return lo.Ternary(strings.HasPrefix(k, "_"), "", cast.ToString(params[k]))
		}),
		"")
	sha := sha1.New()
	sha.Write([]byte(strings.Join([]string{pu.Path, conf.Cfg.Worker.Secret, vals}, "")))
	return hex.EncodeToString(sha.Sum(nil))
}
