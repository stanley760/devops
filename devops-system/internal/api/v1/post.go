// Post management endpoints (huma).
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

// PostHandler serves the post management endpoints.
type PostHandler struct {
	svc *service.Post
}

// NewPostHandler creates the handler.
func NewPostHandler(svc *service.Post) *PostHandler { return &PostHandler{svc: svc} }

// ---- Input / Output ----

// PostListInput holds the GET /system/api/v1/posts query parameters (zero pagination values fall back in the dao, so pagination fields carry no minimum).
type PostListInput struct {
	domain.PageParam
	Code   string                   `query:"code" maxLength:"64" doc:"Post code"`
	Name   string                   `query:"name" maxLength:"50" doc:"Post name"`
	Status humaparam.Optional[int8] `query:"status" minimum:"0" maximum:"1" doc:"Post status 0: disabled; 1: enabled"`
}

// PostListOutput is the paginated post list envelope.
type PostListOutput struct {
	Body commonResp.BaseResponse[domain.PageResult[types.PostDTO]]
}

// PostIDInput holds the path parameter.
type PostIDInput struct {
	ID types.ParamID `path:"id" doc:"Post ID"`
}

// PostGetOutput is the post detail envelope.
type PostGetOutput struct {
	Body commonResp.BaseResponse[types.PostDTO]
}

// PostCreateInput is POST /system/api/v1/posts.
type PostCreateInput struct {
	Body types.PostSaveReq
}

// PostEmptyOutput is the no-data envelope (create/update/delete).
type PostEmptyOutput struct {
	Body commonResp.BaseResponse[any]
}

// PostUpdateInput is PUT /system/api/v1/posts/{id}.
type PostUpdateInput struct {
	ID   types.ParamID `path:"id" doc:"Post ID"`
	Body types.PostSaveReq
}

// ---- Handlers ----

// List handles GET /system/api/v1/posts.
func (h *PostHandler) List(ctx context.Context, in *PostListInput) (*PostListOutput, error) {
	page, err := h.svc.List(ctx, types.PostPageQuery{
		PageParam: in.PageParam,
		Code:      in.Code,
		Name:      in.Name,
		Status:    in.Status.Ptr(),
	})
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &PostListOutput{Body: response.OKBody(page)}, nil
}

// Get handles GET /system/api/v1/posts/{id}.
func (h *PostHandler) Get(ctx context.Context, in *PostIDInput) (*PostGetOutput, error) {
	id, err := types.Parse(in.ID)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	resp, err := h.svc.Get(ctx, id)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &PostGetOutput{Body: response.OKBody(*resp)}, nil
}

// Create handles POST /system/api/v1/posts (the code cannot be changed after creation).
func (h *PostHandler) Create(ctx context.Context, in *PostCreateInput) (*PostEmptyOutput, error) {
	if err := h.svc.Create(ctx, auth.ClaimsFrom(ctx), in.Body); err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &PostEmptyOutput{Body: response.EmptyBody()}, nil
}

// Update handles PUT /system/api/v1/posts/{id}.
func (h *PostHandler) Update(ctx context.Context, in *PostUpdateInput) (*PostEmptyOutput, error) {
	id, err := types.Parse(in.ID)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	if err := h.svc.Update(ctx, auth.ClaimsFrom(ctx), id, in.Body); err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &PostEmptyOutput{Body: response.EmptyBody()}, nil
}

// Delete handles DELETE /system/api/v1/posts/{id}.
func (h *PostHandler) Delete(ctx context.Context, in *PostIDInput) (*PostEmptyOutput, error) {
	id, err := types.Parse(in.ID)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	if err := h.svc.Delete(ctx, auth.ClaimsFrom(ctx), id); err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &PostEmptyOutput{Body: response.EmptyBody()}, nil
}
