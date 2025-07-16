package acl

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/pkg/config"
	dbpkg "github.com/veops/oneterm/pkg/db"
	"github.com/veops/oneterm/pkg/logger"
	"github.com/veops/oneterm/pkg/remote"
)

type ResourceType struct {
	Name  string   `json:"name"`
	Perms []string `json:"perms"`
}

type ResourceTypeResp struct {
	Groups []*ResourceType `json:"groups"`
}

func MigrateNode() {
	ctx := context.Background()

	rts, err := GetResourceTypes(ctx)
	if err != nil {
		logger.L().Fatal("get resource type failed", zap.Error(err))
	}

	if !lo.ContainsBy(rts, func(rt *ResourceType) bool { return rt.Name == "node" }) {
		if err = AddResourceTypes(ctx, &ResourceType{Name: "node", Perms: AllPermissions}); err != nil {
			logger.L().Fatal("add resource type failed", zap.Error(err))
		}
	}

	nodes := make([]*model.Node, 0)
	if err = dbpkg.DB.Model(&nodes).Where("resource_id = 0").Or("resource_id IS NULL").Find(&nodes).Error; err != nil {
		logger.L().Fatal("get nodes failed", zap.Error(err))
	}
	eg := errgroup.Group{}
	for _, n := range nodes {
		nd := n
		eg.Go(func() error {
			r, err := AddResource(ctx, nd.CreatorId, "node", cast.ToString(nd.Id))
			if err != nil {
				return err
			}
			if err := dbpkg.DB.Model(&nd).Where("id=?", nd.Id).Update("resource_id", r.ResourceId).Error; err != nil {
				return err
			}
			return nil
		})
	}
	if err = eg.Wait(); err != nil {
		logger.L().Fatal("add resource failed", zap.Error(err))
	}
}

func MigrateCommand() {
	ctx := context.Background()

	rts, err := GetResourceTypes(ctx)
	if err != nil {
		logger.L().Fatal("get resource type failed", zap.Error(err))
	}

	// Ensure command resource type exists
	if !lo.ContainsBy(rts, func(rt *ResourceType) bool { return rt.Name == "command" }) {
		if err = AddResourceTypes(ctx, &ResourceType{Name: "command", Perms: AllPermissions}); err != nil {
			logger.L().Fatal("add command resource type failed", zap.Error(err))
		}
	}

	// Ensure command_template resource type exists
	if !lo.ContainsBy(rts, func(rt *ResourceType) bool { return rt.Name == "command_template" }) {
		if err = AddResourceTypes(ctx, &ResourceType{Name: "command_template", Perms: AllPermissions}); err != nil {
			logger.L().Fatal("add command_template resource type failed", zap.Error(err))
		}
	}

	// Migrate Commands
	commands := make([]*model.Command, 0)
	if err = dbpkg.DB.Model(&commands).Where("resource_id = 0").Or("resource_id IS NULL").Find(&commands).Error; err != nil {
		logger.L().Fatal("get commands failed", zap.Error(err))
	}

	// Get existing command resources to avoid duplicates
	existingCommandResources, err := GetCommandResources(ctx)
	if err != nil {
		logger.L().Fatal("get existing command resources failed", zap.Error(err))
	}
	commandNameToResourceId := lo.SliceToMap(existingCommandResources, func(r *Resource) (string, int) {
		return r.Name, r.ResourceId
	})

	eg := errgroup.Group{}
	for _, c := range commands {
		cmd := c
		eg.Go(func() error {
			var resourceId int

			// Check if resource already exists
			if existingResourceId, exists := commandNameToResourceId[cmd.Name]; exists {
				resourceId = existingResourceId
				logger.L().Info("Using existing resource for command",
					zap.String("command", cmd.Name),
					zap.Int("resource_id", resourceId))
			} else {
				// Create new resource
				r, err := AddResource(ctx, cmd.CreatorId, "command", cmd.Name)
				if err != nil {
					return err
				}
				resourceId = r.ResourceId
				logger.L().Info("Created new resource for command",
					zap.String("command", cmd.Name),
					zap.Int("resource_id", resourceId))
			}

			// Update command with resource_id
			if err := dbpkg.DB.Model(&cmd).Where("id=?", cmd.Id).Update("resource_id", resourceId).Error; err != nil {
				return err
			}
			return nil
		})
	}

	// Migrate CommandTemplates
	templates := make([]*model.CommandTemplate, 0)
	if err = dbpkg.DB.Model(&templates).Where("resource_id = 0").Or("resource_id IS NULL").Find(&templates).Error; err != nil {
		logger.L().Fatal("get command templates failed", zap.Error(err))
	}

	// Get existing command_template resources to avoid duplicates
	existingTemplateResources, err := GetCommandTemplateResources(ctx)
	if err != nil {
		logger.L().Fatal("get existing command template resources failed", zap.Error(err))
	}
	templateNameToResourceId := lo.SliceToMap(existingTemplateResources, func(r *Resource) (string, int) {
		return r.Name, r.ResourceId
	})

	for _, t := range templates {
		tmpl := t
		eg.Go(func() error {
			var resourceId int

			// Check if resource already exists
			if existingResourceId, exists := templateNameToResourceId[tmpl.Name]; exists {
				resourceId = existingResourceId
				logger.L().Info("Using existing resource for command template",
					zap.String("template", tmpl.Name),
					zap.Int("resource_id", resourceId))
			} else {
				// Create new resource
				r, err := AddResource(ctx, tmpl.CreatorId, "command_template", tmpl.Name)
				if err != nil {
					return err
				}
				resourceId = r.ResourceId
				logger.L().Info("Created new resource for command template",
					zap.String("template", tmpl.Name),
					zap.Int("resource_id", resourceId))
			}

			// Update template with resource_id
			if err := dbpkg.DB.Model(&tmpl).Where("id=?", tmpl.Id).Update("resource_id", resourceId).Error; err != nil {
				return err
			}
			return nil
		})
	}

	if err = eg.Wait(); err != nil {
		logger.L().Fatal("migrate command and template resources failed", zap.Error(err))
	}
}

// GetCommandResources retrieves all command resources from ACL system
func GetCommandResources(ctx context.Context) ([]*Resource, error) {
	token, err := remote.GetAclToken(ctx)
	if err != nil {
		return nil, err
	}

	data := &ResourceResult{}
	url := fmt.Sprintf("%s/acl/resources", config.Cfg.Auth.Acl.Url)
	resp, err := remote.RC.R().
		SetHeader("App-Access-Token", token).
		SetQueryParams(map[string]string{
			"app_id":           config.Cfg.Auth.Acl.AppId,
			"resource_type_id": "command",
			"page_size":        "1000", // Get all resources
		}).
		SetResult(data).
		Get(url)

	if err = remote.HandleErr(err, resp, func(dt map[string]any) bool { return true }); err != nil {
		return nil, err
	}

	return data.Resources, nil
}

// GetCommandTemplateResources retrieves all command template resources from ACL system
func GetCommandTemplateResources(ctx context.Context) ([]*Resource, error) {
	token, err := remote.GetAclToken(ctx)
	if err != nil {
		return nil, err
	}

	data := &ResourceResult{}
	url := fmt.Sprintf("%s/acl/resources", config.Cfg.Auth.Acl.Url)
	resp, err := remote.RC.R().
		SetHeader("App-Access-Token", token).
		SetQueryParams(map[string]string{
			"app_id":           config.Cfg.Auth.Acl.AppId,
			"resource_type_id": "command_template",
			"page_size":        "1000", // Get all resources
		}).
		SetResult(data).
		Get(url)

	if err = remote.HandleErr(err, resp, func(dt map[string]any) bool { return true }); err != nil {
		return nil, err
	}

	return data.Resources, nil
}

func GetResourceTypes(ctx context.Context) (rt []*ResourceType, err error) {
	token, err := remote.GetAclToken(ctx)
	if err != nil {
		return
	}

	data := &ResourceTypeResp{}
	url := fmt.Sprintf("%s/acl/resource_types", config.Cfg.Auth.Acl.Url)
	resp, err := remote.RC.R().
		SetHeaders(map[string]string{
			"App-Access-Token": token,
		}).
		SetQueryParams(map[string]string{
			"app_id":    "oneterm",
			"page_size": "100",
		}).
		SetResult(data).
		Get(url)
	err = remote.HandleErr(err, resp, func(dt map[string]any) bool { return true })

	rt = data.Groups

	return
}

func AddResourceTypes(ctx context.Context, rt *ResourceType) (err error) {
	token, err := remote.GetAclToken(ctx)
	if err != nil {
		return
	}

	url := fmt.Sprintf("%s/acl/resource_types", config.Cfg.Auth.Acl.Url)
	resp, err := remote.RC.R().
		SetHeaders(map[string]string{
			"App-Access-Token": token,
		}).
		SetBody(rt).
		Post(url)
	err = remote.HandleErr(err, resp, func(dt map[string]any) bool { return true })

	return
}

func AddResource(ctx context.Context, uid int, resourceTypeId string, name string) (res *Resource, err error) {
	token, err := remote.GetAclToken(ctx)
	if err != nil {
		return
	}

	res = &Resource{}
	url := fmt.Sprintf("%s/acl/resources", config.Cfg.Auth.Acl.Url)
	resp, err := remote.RC.R().
		SetHeaders(map[string]string{
			"App-Access-Token": token,
			"X-User-Id":        cast.ToString(uid),
			"Accept-Language":  getAcceptLanguage(ctx)}).
		SetBody(map[string]any{
			"type_id": resourceTypeId,
			"name":    name,
			"uid":     uid,
		}).
		SetResult(&res).
		Post(url)
	err = remote.HandleErr(err, resp, func(dt map[string]any) bool { return true })
	return
}

func DeleteResource(ctx context.Context, uid int, resourceId int) (err error) {
	token, err := remote.GetAclToken(ctx)
	if err != nil {
		return
	}

	url := fmt.Sprintf("%v/acl/resources/%v", config.Cfg.Auth.Acl.Url, resourceId)
	resp, err := remote.RC.R().
		SetHeaders(map[string]string{
			"App-Access-Token": token,
			"X-User-Id":        cast.ToString(uid),
			"Accept-Language":  getAcceptLanguage(ctx)}).
		Delete(url)
	err = remote.HandleErr(err, resp, func(dt map[string]any) bool { return true })
	return
}

func UpdateResource(ctx context.Context, uid int, resourceId int, updates map[string]string) (err error) {
	token, err := remote.GetAclToken(ctx)
	if err != nil {
		return
	}

	url := fmt.Sprintf("%s/acl/resources/%d", config.Cfg.Auth.Acl.Url, resourceId)
	resp, err := remote.RC.R().
		SetHeaders(map[string]string{
			"App-Access-Token": token,
			"X-User-Id":        cast.ToString(uid),
			"Accept-Language":  getAcceptLanguage(ctx)}).
		SetFormData(updates).
		Put(url)
	err = remote.HandleErr(err, resp, func(dt map[string]any) bool { return true })
	return
}

func GetResourcePermissions(ctx context.Context, resourceId int) (res map[string]*ResourcePermissionsRespItem, err error) {
	token, err := remote.GetAclToken(ctx)
	if err != nil {
		return
	}
	res = make(map[string]*ResourcePermissionsRespItem)
	url := fmt.Sprintf("%v/acl/resources/%v/permissions", config.Cfg.Auth.Acl.Url, resourceId) //TODO config
	resp, err := remote.RC.R().
		SetHeader("App-Access-Token", token).
		SetResult(&res).
		Get(url)
	err = remote.HandleErr(err, resp, func(dt map[string]any) bool { return true })
	return
}

func getAcceptLanguage(ctx context.Context) string {
	ginCtx, ok := ctx.(*gin.Context)
	if !ok {
		return ""
	}
	return ginCtx.GetHeader("Accept-Language")
}
