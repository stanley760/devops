// User management endpoints (huma).
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

// UserHandler serves the user management endpoints.
type UserHandler struct {
	svc *service.User
}

// NewUserHandler creates the handler.
func NewUserHandler(svc *service.User) *UserHandler { return &UserHandler{svc: svc} }

// ---- Input / Output ----

// UserListInput holds the GET /system/api/v1/users query parameters.
type UserListInput struct {
	domain.PageParam
	Username string                            `query:"username" maxLength:"30" doc:"User name"`
	Mobile   string                            `query:"mobile" maxLength:"20" doc:"Mobile number"`
	Status   humaparam.Optional[int8]          `query:"status" minimum:"0" maximum:"1" doc:"Account status 0: disabled; 1: enabled"`
	DeptID   humaparam.Optional[types.ParamID] `query:"dept_id" doc:"Associated dept ID"`
}

// UserListOutput is the paginated user list envelope.
type UserListOutput struct {
	Body commonResp.BaseResponse[domain.PageResult[types.UserDTO]]
}

// UserIDInput holds the path parameter.
type UserIDInput struct {
	ID types.ParamID `path:"id" doc:"User ID"`
}

// UserGetOutput is the user detail envelope (basic info + role/post IDs).
type UserGetOutput struct {
	Body commonResp.BaseResponse[types.UserDetailResp]
}

// UserCreateInput is POST /system/api/v1/users (role assignment cascades to the g rows).
type UserCreateInput struct {
	Body types.UserCreateReq
}

// UserEmptyOutput is the no-data envelope.
type UserEmptyOutput struct {
	Body commonResp.BaseResponse[any]
}

// UserUpdateInput is PUT /system/api/v1/users/{id} (passwords go through the reset endpoint).
type UserUpdateInput struct {
	ID   types.ParamID `path:"id" doc:"User ID"`
	Body types.UserUpdateReq
}

// UserResetPasswordInput is PUT /system/api/v1/users/{id}/password (a reset kicks the user offline).
type UserResetPasswordInput struct {
	ID   types.ParamID `path:"id" doc:"User ID"`
	Body types.ResetPasswordReq
}

// ---- Handlers ----

// List handles GET /system/api/v1/users.
func (h *UserHandler) List(ctx context.Context, in *UserListInput) (*UserListOutput, error) {
	var deptID *types.ID
	if in.DeptID.IsSet {
		id, err := in.DeptID.Value.Parse()
		if err != nil {
			return nil, errs.ToHumaError(err)
		}
		deptID = &id
	}
	page, err := h.svc.List(ctx, types.UserPageQuery{
		PageParam: in.PageParam,
		Username:  in.Username,
		Mobile:    in.Mobile,
		Status:    in.Status.Ptr(),
		DeptID:    deptID,
	})
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &UserListOutput{Body: response.OKBody(page)}, nil
}

// Get handles GET /system/api/v1/users/{id}.
func (h *UserHandler) Get(ctx context.Context, in *UserIDInput) (*UserGetOutput, error) {
	id, err := types.Parse(in.ID)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	resp, err := h.svc.Get(ctx, id)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &UserGetOutput{Body: response.OKBody(*resp)}, nil
}

// Create handles POST /system/api/v1/users.
func (h *UserHandler) Create(ctx context.Context, in *UserCreateInput) (*UserEmptyOutput, error) {
	if err := h.svc.Create(ctx, auth.ClaimsFrom(ctx), in.Body); err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &UserEmptyOutput{Body: response.EmptyBody()}, nil
}

// Update handles PUT /system/api/v1/users/{id}.
func (h *UserHandler) Update(ctx context.Context, in *UserUpdateInput) (*UserEmptyOutput, error) {
	id, err := types.Parse(in.ID)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	if err := h.svc.Update(ctx, auth.ClaimsFrom(ctx), id, in.Body); err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &UserEmptyOutput{Body: response.EmptyBody()}, nil
}

// ResetPassword handles PUT /system/api/v1/users/{id}/password (a reset kicks the user offline).
func (h *UserHandler) ResetPassword(ctx context.Context, in *UserResetPasswordInput) (*UserEmptyOutput, error) {
	id, err := types.Parse(in.ID)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	if err := h.svc.ResetPassword(ctx, auth.ClaimsFrom(ctx), id, in.Body); err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &UserEmptyOutput{Body: response.EmptyBody()}, nil
}

// Delete handles DELETE /system/api/v1/users/{id}.
func (h *UserHandler) Delete(ctx context.Context, in *UserIDInput) (*UserEmptyOutput, error) {
	id, err := types.Parse(in.ID)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	if err := h.svc.Delete(ctx, auth.ClaimsFrom(ctx), id); err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &UserEmptyOutput{Body: response.EmptyBody()}, nil
}
