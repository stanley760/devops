// System configuration management endpoints (huma).
package api

import (
	"context"

	"github.com/stanley760/devops/devops-common/domain"
	commonResp "github.com/stanley760/devops/devops-common/response"
	"github.com/stanley760/devops/devops-infra/errs"
	"github.com/stanley760/devops/devops-infra/http/humaparam"
	"github.com/stanley760/devops/devops-infra/http/response"

	"github.com/stanley760/devops/devops-system/internal/auth"
	"github.com/stanley760/devops/devops-system/internal/service"
	"github.com/stanley760/devops/devops-system/internal/types"
)

// ConfigHandler serves the system configuration endpoints.
type ConfigHandler struct {
	svc *service.Config
}

// NewConfigHandler creates the handler.
func NewConfigHandler(svc *service.Config) *ConfigHandler { return &ConfigHandler{svc: svc} }

// ---- Input / Output ----

// ConfigListInput holds the GET /system/api/v1/configs query parameters.
type ConfigListInput struct {
	domain.PageParam
	ConfigKey  string                   `query:"config_key" maxLength:"100" doc:"Configuration key"`
	ConfigName string                   `query:"config_name" maxLength:"100" doc:"Configuration name"`
	Type       humaparam.Optional[int8] `query:"type" minimum:"1" maximum:"2" doc:"Configuration type 1: built-in 2: custom"`
	Visible    humaparam.Optional[bool] `query:"visible" doc:"Whether the frontend shows it"`
}

// ConfigListOutput is the paginated configuration list envelope.
type ConfigListOutput struct {
	Body commonResp.BaseResponse[domain.PageResult[types.ConfigDTO]]
}

// ConfigIDInput holds the path parameter.
type ConfigIDInput struct {
	ID types.ParamID `path:"id" doc:"Configuration ID"`
}

// ConfigGetOutput is the configuration detail envelope.
type ConfigGetOutput struct {
	Body commonResp.BaseResponse[types.ConfigDTO]
}

// ConfigKeyInput holds the path parameter (lookup by key).
type ConfigKeyInput struct {
	Key string `path:"key" maxLength:"100" doc:"Configuration key"`
}

// ConfigCreateInput is POST /system/api/v1/configs.
type ConfigCreateInput struct {
	Body types.ConfigSaveReq
}

// ConfigUpdateInput is PUT /system/api/v1/configs/{id}.
type ConfigUpdateInput struct {
	ID   types.ParamID `path:"id" doc:"Configuration ID"`
	Body types.ConfigSaveReq
}

// ConfigEmptyOutput is the no-data envelope.
type ConfigEmptyOutput struct {
	Body commonResp.BaseResponse[any]
}

// ---- Handlers ----

// List handles GET /system/api/v1/configs.
func (h *ConfigHandler) List(ctx context.Context, in *ConfigListInput) (*ConfigListOutput, error) {
	page, err := h.svc.List(ctx, types.ConfigPageQuery{
		PageParam:  in.PageParam,
		ConfigKey:  in.ConfigKey,
		ConfigName: in.ConfigName,
		Type:       in.Type.Ptr(),
		Visible:    in.Visible.Ptr(),
	})
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &ConfigListOutput{Body: response.OKBody(page)}, nil
}

// Get handles GET /system/api/v1/configs/{id}.
func (h *ConfigHandler) Get(ctx context.Context, in *ConfigIDInput) (*ConfigGetOutput, error) {
	id, err := types.Parse(in.ID)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	resp, err := h.svc.Get(ctx, id)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &ConfigGetOutput{Body: response.OKBody(*resp)}, nil
}

// GetByKey handles GET /system/api/v1/configs/key/{key}.
func (h *ConfigHandler) GetByKey(ctx context.Context, in *ConfigKeyInput) (*ConfigGetOutput, error) {
	resp, err := h.svc.GetByKey(ctx, in.Key)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &ConfigGetOutput{Body: response.OKBody(*resp)}, nil
}

// Create handles POST /system/api/v1/configs.
func (h *ConfigHandler) Create(ctx context.Context, in *ConfigCreateInput) (*ConfigEmptyOutput, error) {
	if err := h.svc.Create(ctx, auth.ClaimsFrom(ctx), in.Body); err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &ConfigEmptyOutput{Body: response.EmptyBody()}, nil
}

// Update handles PUT /system/api/v1/configs/{id} (the key cannot be changed).
func (h *ConfigHandler) Update(ctx context.Context, in *ConfigUpdateInput) (*ConfigEmptyOutput, error) {
	id, err := types.Parse(in.ID)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	if err := h.svc.Update(ctx, auth.ClaimsFrom(ctx), id, in.Body); err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &ConfigEmptyOutput{Body: response.EmptyBody()}, nil
}

// Delete handles DELETE /system/api/v1/configs/{id} (built-in items cannot be deleted).
func (h *ConfigHandler) Delete(ctx context.Context, in *ConfigIDInput) (*ConfigEmptyOutput, error) {
	id, err := types.Parse(in.ID)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	if err := h.svc.Delete(ctx, id); err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &ConfigEmptyOutput{Body: response.EmptyBody()}, nil
}
