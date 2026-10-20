// Role management endpoints (huma).
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

// RoleHandler serves the role management endpoints.
type RoleHandler struct {
	svc *service.Role
}

// NewRoleHandler creates the handler.
func NewRoleHandler(svc *service.Role) *RoleHandler { return &RoleHandler{svc: svc} }

// RoleListInput holds the GET /system/api/v1/roles query parameters.
type RoleListInput struct {
	domain.PageParam
	Name   string                   `query:"name" maxLength:"30" doc:"Role name"`
	Code   string                   `query:"code" maxLength:"100" doc:"Role code"`
	Status humaparam.Optional[int8] `query:"status" minimum:"0" maximum:"1" doc:"Role status 0: disabled; 1: enabled"`
}

// RoleListOutput is the paginated role list envelope.
type RoleListOutput struct {
	Body commonResp.BaseResponse[domain.PageResult[types.RoleDTO]]
}

// RoleIDInput holds the path parameter.
type RoleIDInput struct {
	ID types.ParamID `path:"id" doc:"Role ID"`
}

// RoleGetOutput is the role detail envelope (role + assigned menu IDs).
type RoleGetOutput struct {
	Body commonResp.BaseResponse[types.RoleDetailResp]
}

// RoleCreateInput is POST /system/api/v1/roles.
type RoleCreateInput struct {
	Body types.RoleSaveReq
}

// RoleEmptyOutput is the no-data envelope.
type RoleEmptyOutput struct {
	Body commonResp.BaseResponse[any]
}

// RoleUpdateInput is PUT /system/api/v1/roles/{id}.
type RoleUpdateInput struct {
	ID   types.ParamID `path:"id" doc:"Role ID"`
	Body types.RoleSaveReq
}

// RoleAssignMenusInput is PUT /system/api/v1/roles/{id}/menus (rebuilds role_menu and the p rows).
type RoleAssignMenusInput struct {
	ID   types.ParamID `path:"id" doc:"Role ID"`
	Body types.RoleAssignMenusReq
}

// List handles GET /system/api/v1/roles.
func (h *RoleHandler) List(ctx context.Context, in *RoleListInput) (*RoleListOutput, error) {
	page, err := h.svc.List(ctx, types.RolePageQuery{
		PageParam: in.PageParam,
		Name:      in.Name,
		Code:      in.Code,
		Status:    in.Status.Ptr(),
	})
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &RoleListOutput{Body: response.OKBody(page)}, nil
}

// Get handles GET /system/api/v1/roles/{id}.
func (h *RoleHandler) Get(ctx context.Context, in *RoleIDInput) (*RoleGetOutput, error) {
	id, err := types.Parse(in.ID)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	resp, err := h.svc.Get(ctx, id)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &RoleGetOutput{Body: response.OKBody(*resp)}, nil
}

// Create handles POST /system/api/v1/roles.
func (h *RoleHandler) Create(ctx context.Context, in *RoleCreateInput) (*RoleEmptyOutput, error) {
	if err := h.svc.Create(ctx, auth.ClaimsFrom(ctx), in.Body); err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &RoleEmptyOutput{Body: response.EmptyBody()}, nil
}

// Update handles PUT /system/api/v1/roles/{id}.
func (h *RoleHandler) Update(ctx context.Context, in *RoleUpdateInput) (*RoleEmptyOutput, error) {
	id, err := types.Parse(in.ID)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	if err := h.svc.Update(ctx, auth.ClaimsFrom(ctx), id, in.Body); err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &RoleEmptyOutput{Body: response.EmptyBody()}, nil
}

// AssignMenus handles PUT /system/api/v1/roles/{id}/menus (rebuilds role_menu and the p-row snapshot).
func (h *RoleHandler) AssignMenus(ctx context.Context, in *RoleAssignMenusInput) (*RoleEmptyOutput, error) {
	id, err := types.Parse(in.ID)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	if err := h.svc.AssignMenus(ctx, auth.ClaimsFrom(ctx), id, in.Body); err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &RoleEmptyOutput{Body: response.EmptyBody()}, nil
}

// Delete handles DELETE /system/api/v1/roles/{id}.
func (h *RoleHandler) Delete(ctx context.Context, in *RoleIDInput) (*RoleEmptyOutput, error) {
	id, err := types.Parse(in.ID)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	if err := h.svc.Delete(ctx, auth.ClaimsFrom(ctx), id); err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &RoleEmptyOutput{Body: response.EmptyBody()}, nil
}
