// Login audit management service: query side (the write side lives in the
// deferred persistence of service.Auth.login). Delete/clean are ops entry
// points for retention control, gated by dedicated permissions
// (system:loginlog:delete / :clean).
package service

import (
	"context"
	"strconv"
	"strings"

	"github.com/stanley760/devops/devops-common/domain"
	"github.com/stanley760/devops/devops-infra/errs"
	"github.com/stanley760/devops/devops-system/internal/dao"
	"github.com/stanley760/devops/devops-system/internal/model"
	"github.com/stanley760/devops/devops-system/internal/types"
)

// LoginLog is the login audit service.
type LoginLog struct{}

// NewLoginLog creates the service (stateless, read-only dao access).
func NewLoginLog() *LoginLog { return &LoginLog{} }

// List returns a paginated query of login audit records.
func (s *LoginLog) List(ctx context.Context, q types.LoginLogPageQuery) (domain.PageResult[types.LoginLogDTO], error) {
	page, err := dao.ListLoginLogs(ctx, q)
	if err != nil {
		return domain.PageResult[types.LoginLogDTO]{}, err
	}
	return domain.MapPage(page, func(log model.LoginLog) types.LoginLogDTO {
		return types.LoginLogDTO{
			ID:          types.ID(log.ID),
			Username:    log.Username,
			UserID:      types.ID(log.UserID),
			IP:          log.IP,
			UserAgent:   log.UserAgent,
			GeoCountry:  log.GeoCountry,
			GeoProvince: log.GeoProvince,
			GeoCity:     log.GeoCity,
			GeoIsp:      log.GeoIsp,
			Result:      log.Result,
			Code:        log.Code,
			CreateTime:  log.CreateTime,
		}
	}), nil
}

// Delete batch-deletes audit rows (ids comma-separated, e.g. "12,34,56").
func (s *LoginLog) Delete(ctx context.Context, idsCSV string) error {
	ids, err := parseIDsCSV(idsCSV)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return errs.New(errs.CodeParam, "ids cannot be empty")
	}
	_, err = dao.DeleteLoginLogs(ctx, ids)
	return err
}

// Clean clears all audit rows of the current tenant.
func (s *LoginLog) Clean(ctx context.Context) error {
	_, err := dao.CleanLoginLogs(ctx)
	return err
}

// parseIDsCSV parses a comma-separated ID string (whitespace tolerated; any
// invalid entry rejects the whole batch, with no silent skipping —
// audit deletion must reconcile exactly).
func parseIDsCSV(s string) ([]int64, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	ids := make([]int64, 0, len(parts))
	for _, p := range parts {
		v, err := strconv.ParseInt(strings.TrimSpace(p), 10, 64)
		if err != nil || v <= 0 {
			return nil, errs.New(errs.CodeParam, "invalid ids: must be comma-separated positive integers (e.g. 12,34,56)")
		}
		ids = append(ids, v)
	}
	return ids, nil
}
