// Login audit management endpoints (huma). Query side only; the write side
// is persisted automatically on login and has no endpoints.
package api

import (
	"context"
	"time"

	"github.com/stanley760/devops/devops-common/domain"
	commonResp "github.com/stanley760/devops/devops-common/response"
	"github.com/stanley760/devops/devops-infra/errs"
	"github.com/stanley760/devops/devops-infra/http/humaparam"
	"github.com/stanley760/devops/devops-infra/http/response"

	"github.com/stanley760/devops/devops-system/internal/service"
	"github.com/stanley760/devops/devops-system/internal/types"
)

// LoginLogHandler serves the login audit endpoints.
type LoginLogHandler struct {
	svc *service.LoginLog
}

// NewLoginLogHandler creates the handler.
func NewLoginLogHandler(svc *service.LoginLog) *LoginLogHandler { return &LoginLogHandler{svc: svc} }

// ---- Input / Output ----

// LoginLogListInput holds the GET /system/api/v1/login-logs query parameters
// (zero pagination values fall back in the dao; optional parameters use
// humaparam.Optional because the huma pipeline does not support bare pointer
// query parameters).
type LoginLogListInput struct {
	domain.PageParam
	Username  string                        `query:"username" maxLength:"30" doc:"Username, fuzzy match"`
	IP        string                        `query:"ip" maxLength:"50" doc:"Client IP, fuzzy match (supports subnet prefix, e.g. \"192.168.\")"`
	Result    humaparam.Optional[int8]      `query:"result" minimum:"0" maximum:"1" doc:"Login result 0: success; 1: failure; omitted means no filtering"`
	BeginTime humaparam.Optional[time.Time] `query:"begin_time" doc:"Start time (inclusive), RFC3339 format"` // RFC3339 (the OpenAPI standard format)
	EndTime   humaparam.Optional[time.Time] `query:"end_time" doc:"End time (inclusive), RFC3339 format"`
}

// LoginLogListOutput is the paginated login audit list envelope.
type LoginLogListOutput struct {
	Body commonResp.BaseResponse[domain.PageResult[types.LoginLogDTO]]
}

// LoginLogDeleteInput is DELETE /system/api/v1/login-logs (ids are comma-separated, e.g. ?ids=12,34,56).
type LoginLogDeleteInput struct {
	IDs string `query:"ids" maxLength:"2000" doc:"Comma-separated list of record IDs to delete, e.g. 12,34,56"`
}

// LoginLogEmptyOutput is the no-data envelope (delete/clean).
type LoginLogEmptyOutput struct {
	Body commonResp.BaseResponse[any]
}

// ---- Handlers ----

// List handles GET /system/api/v1/login-logs.
func (h *LoginLogHandler) List(ctx context.Context, in *LoginLogListInput) (*LoginLogListOutput, error) {
	page, err := h.svc.List(ctx, types.LoginLogPageQuery{
		PageParam: in.PageParam,
		Username:  in.Username,
		IP:        in.IP,
		Result:    in.Result.Ptr(),
		BeginTime: in.BeginTime.Ptr(),
		EndTime:   in.EndTime.Ptr(),
	})
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &LoginLogListOutput{Body: response.OKBody(page)}, nil
}

// Delete handles DELETE /system/api/v1/login-logs (batch delete).
func (h *LoginLogHandler) Delete(ctx context.Context, in *LoginLogDeleteInput) (*LoginLogEmptyOutput, error) {
	if err := h.svc.Delete(ctx, in.IDs); err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &LoginLogEmptyOutput{Body: response.EmptyBody()}, nil
}

// Clean handles DELETE /system/api/v1/login-logs/clean (clears the current tenant).
func (h *LoginLogHandler) Clean(ctx context.Context, _ *struct{}) (*LoginLogEmptyOutput, error) {
	if err := h.svc.Clean(ctx); err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &LoginLogEmptyOutput{Body: response.EmptyBody()}, nil
}
