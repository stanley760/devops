// Department management endpoints (huma).
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

// DeptHandler serves the department management endpoints.
type DeptHandler struct {
	svc *service.Dept
}

// NewDeptHandler creates the handler.
func NewDeptHandler(svc *service.Dept) *DeptHandler { return &DeptHandler{svc: svc} }

// ---- Input / Output ----

// DeptListInput holds the GET /system/api/v1/depts query parameters (returns a tree; status is optional).
type DeptListInput struct {
	Status humaparam.Optional[int8] `query:"status" minimum:"0" maximum:"1" doc:"Dept status 0: disabled; 1: enabled"`
}

// DeptListOutput is the department tree envelope.
type DeptListOutput struct {
	Body commonResp.BaseResponse[[]types.DeptTreeResp]
}

// DeptIDInput holds the path parameter.
type DeptIDInput struct {
	ID types.ParamID `path:"id" doc:"Dept ID"`
}

// DeptGetOutput is the department detail envelope.
type DeptGetOutput struct {
	Body commonResp.BaseResponse[types.DeptDTO]
}

// DeptCreateInput is POST /system/api/v1/depts.
type DeptCreateInput struct {
	Body types.DeptSaveReq
}

// DeptEmptyOutput is the no-data envelope.
type DeptEmptyOutput struct {
	Body commonResp.BaseResponse[any]
}

// DeptUpdateInput is PUT /system/api/v1/depts/{id}.
type DeptUpdateInput struct {
	ID   types.ParamID `path:"id" doc:"Dept ID"`
	Body types.DeptSaveReq
}

// ---- Handlers ----

// List handles GET /system/api/v1/depts?status= (returns a tree).
func (h *DeptHandler) List(ctx context.Context, in *DeptListInput) (*DeptListOutput, error) {
	tree, err := h.svc.List(ctx, in.Status.Ptr())
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &DeptListOutput{Body: response.OKBody(tree)}, nil
}

// Get handles GET /system/api/v1/depts/{id}.
func (h *DeptHandler) Get(ctx context.Context, in *DeptIDInput) (*DeptGetOutput, error) {
	id, err := types.Parse(in.ID)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	resp, err := h.svc.Get(ctx, id)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &DeptGetOutput{Body: response.OKBody(*resp)}, nil
}

// Create handles POST /system/api/v1/depts.
func (h *DeptHandler) Create(ctx context.Context, in *DeptCreateInput) (*DeptEmptyOutput, error) {
	if err := h.svc.Create(ctx, auth.ClaimsFrom(ctx), in.Body); err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &DeptEmptyOutput{Body: response.EmptyBody()}, nil
}

// Update handles PUT /system/api/v1/depts/{id} (the parent must not be the node itself or one of its descendants, preventing cycles).
func (h *DeptHandler) Update(ctx context.Context, in *DeptUpdateInput) (*DeptEmptyOutput, error) {
	id, err := types.Parse(in.ID)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	if err := h.svc.Update(ctx, auth.ClaimsFrom(ctx), id, in.Body); err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &DeptEmptyOutput{Body: response.EmptyBody()}, nil
}

// Delete handles DELETE /system/api/v1/depts/{id}.
func (h *DeptHandler) Delete(ctx context.Context, in *DeptIDInput) (*DeptEmptyOutput, error) {
	id, err := types.Parse(in.ID)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	if err := h.svc.Delete(ctx, id); err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &DeptEmptyOutput{Body: response.EmptyBody()}, nil
}
