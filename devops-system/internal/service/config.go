// System configuration management service: the runtime-tunable configuration
// items of the admin platform (managed from the console, not from YAML).
//
// A configuration key is unique within a tenant and immutable after creation.
// Built-in items (type=1) can be edited but not deleted, so the platform always
// has a complete set of defaults.
package service

import (
	"context"

	"github.com/stanley760/devops/devops-common/domain"
	"github.com/stanley760/devops/devops-infra/errs"

	"github.com/stanley760/devops/devops-system/internal/auth"
	"github.com/stanley760/devops/devops-system/internal/dao"
	"github.com/stanley760/devops/devops-system/internal/model"
	"github.com/stanley760/devops/devops-system/internal/types"
)

// Config is the system configuration service.
type Config struct{}

// NewConfig creates the service.
func NewConfig() *Config { return &Config{} }

// List returns a paginated configuration query.
func (s *Config) List(ctx context.Context, q types.ConfigPageQuery) (domain.PageResult[types.ConfigDTO], error) {
	page, err := dao.ListConfigs(ctx, q)
	if err != nil {
		return domain.PageResult[types.ConfigDTO]{}, err
	}
	return domain.MapPage(page, func(c model.Config) types.ConfigDTO {
		return toConfigDTO(&c)
	}), nil
}

// Get returns the configuration details.
func (s *Config) Get(ctx context.Context, id types.ID) (*types.ConfigDTO, error) {
	c, err := dao.GetConfigByID(ctx, id.Int64())
	if err != nil {
		return nil, err
	}
	dto := toConfigDTO(c)
	return &dto, nil
}

// GetByKey returns the configuration item by key (the console reads single
// items by key, e.g. the site name).
func (s *Config) GetByKey(ctx context.Context, key string) (*types.ConfigDTO, error) {
	c, err := dao.GetConfigByKey(ctx, key)
	if err != nil {
		return nil, err
	}
	dto := toConfigDTO(c)
	return &dto, nil
}

// Create creates a configuration item: the key is required and unique within the tenant.
func (s *Config) Create(ctx context.Context, claims *auth.AccessClaims, req types.ConfigSaveReq) error {
	if req.ConfigKey == "" {
		return errs.New(errs.CodeParam, "configuration key cannot be empty")
	}
	exists, err := dao.ConfigKeyExists(ctx, req.ConfigKey, 0)
	if err != nil {
		return err
	}
	if exists {
		return errs.New(errs.CodeParam, "configuration key already exists")
	}
	c := &model.Config{
		BaseTenantModel: model.BaseTenantModel{
			BaseModel: model.BaseModel{Creator: claims.Username, Updater: claims.Username},
		},
		ConfigKey:   req.ConfigKey,
		ConfigName:  req.ConfigName,
		ConfigValue: req.ConfigValue,
		Type:        req.Type,
		Visible:     derefBool(req.Visible, true),
		Remark:      req.Remark,
	}
	return dao.CreateConfig(ctx, c)
}

// Update updates a configuration item (the key cannot be changed after creation).
func (s *Config) Update(ctx context.Context, claims *auth.AccessClaims, id types.ID, req types.ConfigSaveReq) error {
	c, err := dao.GetConfigByID(ctx, id.Int64())
	if err != nil {
		return err
	}
	return dao.UpdateConfig(ctx, c.ID, map[string]any{
		"config_name":  req.ConfigName,
		"config_value": req.ConfigValue,
		"visible":      derefBool(req.Visible, true),
		"remark":       req.Remark,
		"updater":      claims.Username,
	})
}

// Delete deletes a configuration item: built-in items cannot be deleted.
func (s *Config) Delete(ctx context.Context, id types.ID) error {
	c, err := dao.GetConfigByID(ctx, id.Int64())
	if err != nil {
		return err
	}
	if c.Type == 1 {
		return errs.New(errs.CodeParam, "built-in configuration cannot be deleted")
	}
	return dao.SoftDeleteConfig(ctx, c.ID, c.ConfigKey)
}

// toConfigDTO maps model.Config → DTO.
func toConfigDTO(c *model.Config) types.ConfigDTO {
	return types.ConfigDTO{
		ID:          types.ID(c.ID),
		ConfigKey:   c.ConfigKey,
		ConfigName:  c.ConfigName,
		ConfigValue: c.ConfigValue,
		Type:        c.Type,
		Visible:     c.Visible,
		Remark:      c.Remark,
		CreateTime:  c.CreateTime,
	}
}
