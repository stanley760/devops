// Operation audit management endpoints (huma). Query side only; the write
// side is persisted automatically by the audit collection middleware.
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

// OperationLogHandler serves the operation audit endpoints.
type OperationLogHandler struct {
	svc *service.OperationLog
}

// NewOperationLogHandler creates the handler.
func NewOperationLogHandler(svc *service.OperationLog) *OperationLogHandler {
	return &OperationLogHandler{svc: svc}
}

// ---- Input / Output ----

// OperationLogListInput holds the GET /system/api/v1/operation-logs query parameters.
type OperationLogListInput struct {
	domain.PageParam
	Module    string                        `query:"module" maxLength:"50" doc:"Top-level module, exact match"`
	SubModule string                        `query:"sub_module" maxLength:"50" doc:"Sub-module, exact match"`
	UserName  string                        `query:"user_name" maxLength:"30" doc:"Operator name, fuzzy match"`
	UserIP    string                        `query:"user_ip" maxLength:"50" doc:"Client IP, fuzzy match"`
	Result    humaparam.Optional[int8]      `query:"result" minimum:"0" maximum:"1" doc:"Execution result 0: success; 1: failure; omitted means no filtering"`
	BeginTime humaparam.Optional[time.Time] `query:"begin_time" doc:"Start time (inclusive), RFC3339 format"`
	EndTime   humaparam.Optional[time.Time] `query:"end_time" doc:"End time (inclusive), RFC3339 format"`
}

// OperationLogListOutput is the paginated operation audit list envelope.
type OperationLogListOutput struct {
	Body commonResp.BaseResponse[domain.PageResult[types.OperationLogDTO]]
}

// OperationLogDeleteInput is DELETE /system/api/v1/operation-logs (ids are comma-separated).
type OperationLogDeleteInput struct {
	IDs string `query:"ids" maxLength:"2000" doc:"Comma-separated list of record IDs to delete, e.g. 12,34,56"`
}

// OperationLogEmptyOutput is the no-data envelope (delete/clean).
type OperationLogEmptyOutput struct {
	Body commonResp.BaseResponse[any]
}

// ---- Handlers ----

// List handles GET /system/api/v1/operation-logs.
func (h *OperationLogHandler) List(ctx context.Context, in *OperationLogListInput) (*OperationLogListOutput, error) {
	page, err := h.svc.List(ctx, types.OperationLogPageQuery{
		PageParam: in.PageParam,
		Module:    in.Module,
		SubModule: in.SubModule,
		UserName:  in.UserName,
		UserIP:    in.UserIP,
		Result:    in.Result.Ptr(),
		BeginTime: in.BeginTime.Ptr(),
		EndTime:   in.EndTime.Ptr(),
	})
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &OperationLogListOutput{Body: response.OKBody(page)}, nil
}

// Delete handles DELETE /system/api/v1/operation-logs (batch delete).
func (h *OperationLogHandler) Delete(ctx context.Context, in *OperationLogDeleteInput) (*OperationLogEmptyOutput, error) {
	if err := h.svc.Delete(ctx, in.IDs); err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &OperationLogEmptyOutput{Body: response.EmptyBody()}, nil
}

// Clean handles DELETE /system/api/v1/operation-logs/clean (clears the current tenant).
func (h *OperationLogHandler) Clean(ctx context.Context, _ *struct{}) (*OperationLogEmptyOutput, error) {
	if err := h.svc.Clean(ctx); err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &OperationLogEmptyOutput{Body: response.EmptyBody()}, nil
}
