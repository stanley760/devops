// Menu management endpoints (huma).
package api

import (
	"context"

	commonResp "github.com/stanley760/devops/devops-common/response"
	"github.com/stanley760/devops/devops-infra/errs"
	"github.com/stanley760/devops/devops-infra/http/humaparam"
	"github.com/stanley760/devops/devops-infra/http/response"

	"github.com/stanley760/devops/devops-system/internal/auth"
	"github.com/stanley760/devops/devops-system/internal/service"
	"github.com/stanley760/devops/devops-system/internal/types"
)

// MenuHandler serves the menu management endpoints.
type MenuHandler struct {
	svc *service.Menu
}

// NewMenuHandler creates the handler.
func NewMenuHandler(svc *service.Menu) *MenuHandler { return &MenuHandler{svc: svc} }

// ---- Input / Output ----

// MenuListInput holds the GET /system/api/v1/menus query parameters (returns a tree; status is optional).
type MenuListInput struct {
	Status humaparam.Optional[int8] `query:"status" minimum:"0" maximum:"1" doc:"Menu status 0: disabled; 1: enabled"`
}

// MenuListOutput is the menu tree envelope.
type MenuListOutput struct {
	Body commonResp.BaseResponse[[]types.MenuTreeResp]
}

// MenuIDInput holds the path parameter.
type MenuIDInput struct {
	ID types.ParamID `path:"id" doc:"Menu ID"`
}

// MenuGetOutput is the menu detail envelope.
type MenuGetOutput struct {
	Body commonResp.BaseResponse[types.MenuDTO]
}

// MenuCreateInput is POST /system/api/v1/menus.
type MenuCreateInput struct {
	Body types.MenuSaveReq
}

// MenuEmptyOutput is the no-data envelope.
type MenuEmptyOutput struct {
	Body commonResp.BaseResponse[any]
}

// MenuUpdateInput is PUT /system/api/v1/menus/{id} (a permission change cascades to rebuild the p rows of referencing roles).
type MenuUpdateInput struct {
	ID   types.ParamID `path:"id" doc:"Menu ID"`
	Body types.MenuSaveReq
}

// ---- Handlers ----

// List handles GET /system/api/v1/menus?status= (returns a tree).
func (h *MenuHandler) List(ctx context.Context, in *MenuListInput) (*MenuListOutput, error) {
	tree, err := h.svc.List(ctx, in.Status.Ptr())
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &MenuListOutput{Body: response.OKBody(tree)}, nil
}

// Get handles GET /system/api/v1/menus/{id}.
func (h *MenuHandler) Get(ctx context.Context, in *MenuIDInput) (*MenuGetOutput, error) {
	id, err := types.Parse(in.ID)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	resp, err := h.svc.Get(ctx, id)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &MenuGetOutput{Body: response.OKBody(*resp)}, nil
}

// Create handles POST /system/api/v1/menus.
func (h *MenuHandler) Create(ctx context.Context, in *MenuCreateInput) (*MenuEmptyOutput, error) {
	if err := h.svc.Create(ctx, auth.ClaimsFrom(ctx), in.Body); err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &MenuEmptyOutput{Body: response.EmptyBody()}, nil
}

// Update handles PUT /system/api/v1/menus/{id} (a permission change cascades to rebuild the p rows of referencing roles).
func (h *MenuHandler) Update(ctx context.Context, in *MenuUpdateInput) (*MenuEmptyOutput, error) {
	id, err := types.Parse(in.ID)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	if err := h.svc.Update(ctx, auth.ClaimsFrom(ctx), id, in.Body); err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &MenuEmptyOutput{Body: response.EmptyBody()}, nil
}

// Delete handles DELETE /system/api/v1/menus/{id}.
func (h *MenuHandler) Delete(ctx context.Context, in *MenuIDInput) (*MenuEmptyOutput, error) {
	id, err := types.Parse(in.ID)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	if err := h.svc.Delete(ctx, id); err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &MenuEmptyOutput{Body: response.EmptyBody()}, nil
}
