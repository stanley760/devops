// Tenant management endpoints (huma; platform-level routes use kit.plat).
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

// TenantHandler serves the tenant management endpoints.
type TenantHandler struct {
	svc *service.Tenant
}

// NewTenantHandler creates the handler.
func NewTenantHandler(svc *service.Tenant) *TenantHandler { return &TenantHandler{svc: svc} }

// ---- Input / Output ----

// TenantListInput holds the GET /system/api/v1/tenants query parameters.
type TenantListInput struct {
	domain.PageParam
	Name   string                   `query:"name" maxLength:"30" doc:"Tenant name"`
	Code   string                   `query:"code" maxLength:"30" doc:"Tenant code"`
	Status humaparam.Optional[int8] `query:"status" minimum:"0" maximum:"1" doc:"Tenant status 0: disabled; 1: enabled"`
}

// TenantListOutput is the paginated tenant list envelope.
type TenantListOutput struct {
	Body commonResp.BaseResponse[domain.PageResult[types.TenantDTO]]
}

// TenantIDInput holds the path parameter.
type TenantIDInput struct {
	ID types.ParamID `path:"id" doc:"Tenant ID"`
}

// TenantGetOutput is the tenant detail envelope.
type TenantGetOutput struct {
	Body commonResp.BaseResponse[types.TenantDTO]
}

// TenantCreateInput is POST /system/api/v1/tenants (includes the tenant's initial super admin, who can log in right after creation).
type TenantCreateInput struct {
	Body types.TenantCreateReq
}

// TenantCreateOutput is the create response envelope (returns the new tenant).
type TenantCreateOutput struct {
	Body commonResp.BaseResponse[types.TenantDTO]
}

// TenantEmptyOutput is the no-data envelope.
type TenantEmptyOutput struct {
	Body commonResp.BaseResponse[any]
}

// TenantUpdateInput is PUT /system/api/v1/tenants/{id} (name / contact / expiry time).
type TenantUpdateInput struct {
	ID   types.ParamID `path:"id" doc:"Tenant ID"`
	Body types.TenantUpdateReq
}

// TenantStatusInput is PUT /system/api/v1/tenants/{id}/status (disabling takes everyone offline).
type TenantStatusInput struct {
	ID   types.ParamID `path:"id" doc:"Tenant ID"`
	Body types.TenantStatusReq
}

// ---- Handlers ----

// List handles GET /system/api/v1/tenants.
func (h *TenantHandler) List(ctx context.Context, in *TenantListInput) (*TenantListOutput, error) {
	page, err := h.svc.List(ctx, types.TenantPageQuery{
		PageParam: in.PageParam,
		Name:      in.Name,
		Code:      in.Code,
		Status:    in.Status.Ptr(),
	})
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &TenantListOutput{Body: response.OKBody(page)}, nil
}

// Get handles GET /system/api/v1/tenants/{id}.
func (h *TenantHandler) Get(ctx context.Context, in *TenantIDInput) (*TenantGetOutput, error) {
	id, err := types.Parse(in.ID)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	resp, err := h.svc.Get(ctx, id)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &TenantGetOutput{Body: response.OKBody(*resp)}, nil
}

// Create handles POST /system/api/v1/tenants (includes the tenant's initial super admin).
func (h *TenantHandler) Create(ctx context.Context, in *TenantCreateInput) (*TenantCreateOutput, error) {
	resp, err := h.svc.Create(ctx, auth.ClaimsFrom(ctx), in.Body)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &TenantCreateOutput{Body: response.OKBody(*resp)}, nil
}

// Update handles PUT /system/api/v1/tenants/{id} (name / contact / expiry time).
func (h *TenantHandler) Update(ctx context.Context, in *TenantUpdateInput) (*TenantEmptyOutput, error) {
	id, err := types.Parse(in.ID)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	if err := h.svc.Update(ctx, auth.ClaimsFrom(ctx), id, in.Body); err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &TenantEmptyOutput{Body: response.EmptyBody()}, nil
}

// UpdateStatus handles PUT /system/api/v1/tenants/{id}/status (disabling takes everyone offline).
func (h *TenantHandler) UpdateStatus(ctx context.Context, in *TenantStatusInput) (*TenantEmptyOutput, error) {
	id, err := types.Parse(in.ID)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	if err := h.svc.UpdateStatus(ctx, auth.ClaimsFrom(ctx), id, in.Body); err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &TenantEmptyOutput{Body: response.EmptyBody()}, nil
}

// Delete handles DELETE /system/api/v1/tenants/{id} (the default tenant cannot be deleted; disable it first).
func (h *TenantHandler) Delete(ctx context.Context, in *TenantIDInput) (*TenantEmptyOutput, error) {
	id, err := types.Parse(in.ID)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	if err := h.svc.Delete(ctx, auth.ClaimsFrom(ctx), id); err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &TenantEmptyOutput{Body: response.EmptyBody()}, nil
}
