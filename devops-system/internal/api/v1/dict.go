// Dictionary management endpoints (huma).
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

// DictHandler serves the dictionary management endpoints.
type DictHandler struct {
	svc *service.Dict
}

// NewDictHandler creates the handler.
func NewDictHandler(svc *service.Dict) *DictHandler { return &DictHandler{svc: svc} }

// ---- Input / Output ----

// DictTypeListInput holds the GET /system/api/v1/dict-types query parameters.
type DictTypeListInput struct {
	domain.PageParam
	Name   string                   `query:"name" maxLength:"100" doc:"Dictionary name"`
	Type   string                   `query:"type" maxLength:"100" doc:"Dictionary type code"`
	Status humaparam.Optional[int8] `query:"status" minimum:"0" maximum:"1" doc:"Dictionary type status 0: disabled 1: enabled"`
}

// DictTypeListOutput is the paginated dictionary type list envelope.
type DictTypeListOutput struct {
	Body commonResp.BaseResponse[domain.PageResult[types.DictTypeDTO]]
}

// DictTypeIDInput holds the path parameter.
type DictTypeIDInput struct {
	ID types.ParamID `path:"id" doc:"Dictionary type ID"`
}

// DictTypeGetOutput is the dictionary type detail envelope.
type DictTypeGetOutput struct {
	Body commonResp.BaseResponse[types.DictTypeDTO]
}

// DictTypeCreateInput is POST /system/api/v1/dict-types.
type DictTypeCreateInput struct {
	Body types.DictTypeSaveReq
}

// DictTypeUpdateInput is PUT /system/api/v1/dict-types/{id}.
type DictTypeUpdateInput struct {
	ID   types.ParamID `path:"id" doc:"Dictionary type ID"`
	Body types.DictTypeSaveReq
}

// DictDataListInput holds the GET /system/api/v1/dict-data query parameters.
type DictDataListInput struct {
	domain.PageParam
	DictType string                   `query:"dict_type" maxLength:"100" doc:"Dictionary type code"`
	Label    string                   `query:"label" maxLength:"100" doc:"Display label"`
	Value    string                   `query:"value" maxLength:"100" doc:"Stored value"`
	Status   humaparam.Optional[int8] `query:"status" minimum:"0" maximum:"1" doc:"Dictionary item status 0: disabled 1: enabled"`
}

// DictDataListOutput is the paginated dictionary item list envelope.
type DictDataListOutput struct {
	Body commonResp.BaseResponse[domain.PageResult[types.DictDataDTO]]
}

// DictDataIDInput holds the path parameter.
type DictDataIDInput struct {
	ID types.ParamID `path:"id" doc:"Dictionary item ID"`
}

// DictDataGetOutput is the dictionary item detail envelope.
type DictDataGetOutput struct {
	Body commonResp.BaseResponse[types.DictDataDTO]
}

// DictDataCreateInput is POST /system/api/v1/dict-data.
type DictDataCreateInput struct {
	Body types.DictDataSaveReq
}

// DictDataUpdateInput is PUT /system/api/v1/dict-data/{id}.
type DictDataUpdateInput struct {
	ID   types.ParamID `path:"id" doc:"Dictionary item ID"`
	Body types.DictDataSaveReq
}

// DictEmptyOutput is the no-data envelope.
type DictEmptyOutput struct {
	Body commonResp.BaseResponse[any]
}

// DictTypeQueryInput holds the GET /system/api/v1/dict-data/simple query parameters
// (the enabled items of one dictionary type, for the frontend dropdown).
type DictTypeQueryInput struct {
	DictType string `query:"dict_type" required:"true" maxLength:"100" doc:"Dictionary type code"`
}

// DictTypeSimpleOutput is the dictionary item list envelope (no pagination).
type DictTypeSimpleOutput struct {
	Body commonResp.BaseResponse[[]types.DictDataDTO]
}

// ---- Handlers ----

// ListTypes handles GET /system/api/v1/dict-types.
func (h *DictHandler) ListTypes(ctx context.Context, in *DictTypeListInput) (*DictTypeListOutput, error) {
	page, err := h.svc.ListTypes(ctx, types.DictTypePageQuery{
		PageParam: in.PageParam,
		Name:      in.Name,
		Type:      in.Type,
		Status:    in.Status.Ptr(),
	})
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &DictTypeListOutput{Body: response.OKBody(page)}, nil
}

// GetType handles GET /system/api/v1/dict-types/{id}.
func (h *DictHandler) GetType(ctx context.Context, in *DictTypeIDInput) (*DictTypeGetOutput, error) {
	id, err := types.Parse(in.ID)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	resp, err := h.svc.GetType(ctx, id)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &DictTypeGetOutput{Body: response.OKBody(*resp)}, nil
}

// CreateType handles POST /system/api/v1/dict-types.
func (h *DictHandler) CreateType(ctx context.Context, in *DictTypeCreateInput) (*DictEmptyOutput, error) {
	if err := h.svc.CreateType(ctx, auth.ClaimsFrom(ctx), in.Body); err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &DictEmptyOutput{Body: response.EmptyBody()}, nil
}

// UpdateType handles PUT /system/api/v1/dict-types/{id} (the type code cannot be changed).
func (h *DictHandler) UpdateType(ctx context.Context, in *DictTypeUpdateInput) (*DictEmptyOutput, error) {
	id, err := types.Parse(in.ID)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	if err := h.svc.UpdateType(ctx, auth.ClaimsFrom(ctx), id, in.Body); err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &DictEmptyOutput{Body: response.EmptyBody()}, nil
}

// DeleteType handles DELETE /system/api/v1/dict-types/{id}.
func (h *DictHandler) DeleteType(ctx context.Context, in *DictTypeIDInput) (*DictEmptyOutput, error) {
	id, err := types.Parse(in.ID)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	if err := h.svc.DeleteType(ctx, id); err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &DictEmptyOutput{Body: response.EmptyBody()}, nil
}

// ListData handles GET /system/api/v1/dict-data.
func (h *DictHandler) ListData(ctx context.Context, in *DictDataListInput) (*DictDataListOutput, error) {
	page, err := h.svc.ListData(ctx, types.DictDataPageQuery{
		PageParam: in.PageParam,
		DictType:  in.DictType,
		Label:     in.Label,
		Value:     in.Value,
		Status:    in.Status.Ptr(),
	})
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &DictDataListOutput{Body: response.OKBody(page)}, nil
}

// GetData handles GET /system/api/v1/dict-data/{id}.
func (h *DictHandler) GetData(ctx context.Context, in *DictDataIDInput) (*DictDataGetOutput, error) {
	id, err := types.Parse(in.ID)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	resp, err := h.svc.GetData(ctx, id)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &DictDataGetOutput{Body: response.OKBody(*resp)}, nil
}

// CreateData handles POST /system/api/v1/dict-data.
func (h *DictHandler) CreateData(ctx context.Context, in *DictDataCreateInput) (*DictEmptyOutput, error) {
	if err := h.svc.CreateData(ctx, auth.ClaimsFrom(ctx), in.Body); err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &DictEmptyOutput{Body: response.EmptyBody()}, nil
}

// UpdateData handles PUT /system/api/v1/dict-data/{id}.
func (h *DictHandler) UpdateData(ctx context.Context, in *DictDataUpdateInput) (*DictEmptyOutput, error) {
	id, err := types.Parse(in.ID)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	if err := h.svc.UpdateData(ctx, auth.ClaimsFrom(ctx), id, in.Body); err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &DictEmptyOutput{Body: response.EmptyBody()}, nil
}

// DeleteData handles DELETE /system/api/v1/dict-data/{id}.
func (h *DictHandler) DeleteData(ctx context.Context, in *DictDataIDInput) (*DictEmptyOutput, error) {
	id, err := types.Parse(in.ID)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	if err := h.svc.DeleteData(ctx, id); err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &DictEmptyOutput{Body: response.EmptyBody()}, nil
}

// ListByType handles GET /system/api/v1/dict-data/simple?dict_type= (the enabled items
// of one dictionary type, sorted; used by the frontend dropdowns).
func (h *DictHandler) ListByType(ctx context.Context, in *DictTypeQueryInput) (*DictTypeSimpleOutput, error) {
	status := int8(1)
	page, err := h.svc.ListData(ctx, types.DictDataPageQuery{
		DictType: in.DictType,
		Status:   &status,
	})
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &DictTypeSimpleOutput{Body: response.OKBody(page.List)}, nil
}
