// Operation audit management service: query side (the write side is persisted
// automatically by the humakit audit collection middleware). Delete/clean are
// ops entry points for retention control, gated by dedicated permissions
// (system:operationlog:delete / :clean).
package service

import (
	"context"

	"github.com/stanley760/devops/devops-common/domain"
	"github.com/stanley760/devops/devops-infra/errs"

	"github.com/stanley760/devops/devops-system/internal/dao"
	"github.com/stanley760/devops/devops-system/internal/model"
	"github.com/stanley760/devops/devops-system/internal/types"
)

// OperationLog is the operation audit service.
type OperationLog struct{}

// NewOperationLog creates the service (stateless, read-only dao access).
func NewOperationLog() *OperationLog { return &OperationLog{} }

// List returns a paginated query of operation audit records.
func (s *OperationLog) List(ctx context.Context, q types.OperationLogPageQuery) (domain.PageResult[types.OperationLogDTO], error) {
	page, err := dao.ListOperationLogs(ctx, q)
	if err != nil {
		return domain.PageResult[types.OperationLogDTO]{}, err
	}
	return domain.MapPage(page, func(log model.OperationLog) types.OperationLogDTO {
		return types.OperationLogDTO{
			ID:            types.ID(log.ID),
			TraceID:       log.TraceID,
			UserID:        types.ID(log.UserID),
			UserName:      log.UserName,
			UserType:      log.UserType,
			UserTypeName:  log.UserType.String(),
			Module:        log.Module,
			SubModule:     log.SubModule,
			BizID:         types.ID(log.BizID),
			Contents:      log.Contents,
			Extra:         log.Extra,
			RequestMethod: log.RequestMethod,
			RequestURL:    log.RequestURL,
			UserIP:        log.UserIP,
			UserAgent:     log.UserAgent,
			GeoCountry:    log.GeoCountry,
			GeoProvince:   log.GeoProvince,
			GeoCity:       log.GeoCity,
			GeoIsp:        log.GeoISP,
			Result:        log.Result,
			Code:          log.Code,
			CreateTime:    log.CreateTime,
		}
	}), nil
}

// Delete batch-deletes audit rows (ids comma-separated, e.g. "12,34,56").
func (s *OperationLog) Delete(ctx context.Context, idsCSV string) error {
	ids, err := parseIDsCSV(idsCSV)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return errs.New(errs.CodeParam, "ids cannot be empty")
	}
	_, err = dao.DeleteOperationLogs(ctx, ids)
	return err
}

// Clean clears all audit rows of the current tenant.
func (s *OperationLog) Clean(ctx context.Context) error {
	_, err := dao.CleanOperationLogs(ctx)
	return err
}
