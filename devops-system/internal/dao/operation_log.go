// Operation audit read side (the write side is handled by the humakit audit-collection middleware via audit.Sink).
package dao

import (
	"context"

	"github.com/stanley760/devops/devops-common/domain"
	"github.com/stanley760/devops/devops-common/domain/gormx"
	"github.com/stanley760/devops/devops-infra/errs"

	"github.com/stanley760/devops/devops-system/internal/model"
	"github.com/stanley760/devops/devops-system/internal/tenantdb"
	"github.com/stanley760/devops/devops-system/internal/types"
)

// ListOperationLogs paginates operation audit rows (newest first).
func ListOperationLogs(ctx context.Context, q types.OperationLogPageQuery) (domain.PageResult[model.OperationLog], error) {
	db := tenantdb.DB(ctx).Model(&model.OperationLog{})
	if q.Module != "" {
		db = db.Where("module = ?", q.Module)
	}
	if q.SubModule != "" {
		db = db.Where("sub_module = ?", q.SubModule)
	}
	if q.UserName != "" {
		db = db.Where("user_name LIKE ?", "%"+q.UserName+"%")
	}
	if q.UserIP != "" {
		db = db.Where("user_ip LIKE ?", "%"+q.UserIP+"%")
	}
	if q.Result != nil {
		db = db.Where("result = ?", *q.Result)
	}
	if q.BeginTime != nil {
		db = db.Where("create_time >= ?", *q.BeginTime)
	}
	if q.EndTime != nil {
		db = db.Where("create_time <= ?", *q.EndTime)
	}

	page, err := gormx.FindPage[model.OperationLog](
		db.Order("id DESC"),
		domain.PageParam{PageNum: q.PageNum, PageSize: q.PageSize},
	)
	if err != nil {
		return domain.PageResult[model.OperationLog]{}, errs.Wrap(err, errs.CodeInternal, "failed to query operation audit list")
	}
	return page, nil
}

// DeleteOperationLogs batch-deletes audit rows.
//
// Physical delete: audit rows have no soft-delete semantics; keeping deleted_at rows would only keep the table growing.
// Returns the number of rows actually deleted.
func DeleteOperationLogs(ctx context.Context, ids []int64) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	res := tenantdb.DB(ctx).Unscoped().
		Where("id IN ?", ids).Delete(&model.OperationLog{})
	if res.Error != nil {
		return 0, errs.Wrap(res.Error, errs.CodeInternal, "failed to delete operation audit rows")
	}
	return res.RowsAffected, nil
}

// CleanOperationLogs deletes all audit rows of the current tenant, returning the deleted row count.
//
// After Unscoped, TenantPlugin no longer injects the tenant condition, so tenant_id must be
// specified explicitly — otherwise rows of other tenants would be cleaned as well.
func CleanOperationLogs(ctx context.Context) (int64, error) {
	tid, err := tenantOf(ctx)
	if err != nil {
		return 0, err
	}
	res := tenantdb.DB(ctx).Unscoped().
		Where("tenant_id = ?", tid).Delete(&model.OperationLog{})
	if res.Error != nil {
		return 0, errs.Wrap(res.Error, errs.CodeInternal, "failed to clean operation audit rows")
	}
	return res.RowsAffected, nil
}
