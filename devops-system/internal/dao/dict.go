// Dictionary management dao: t_system_dict_type / t_system_dict_data (tenant tables).
package dao

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/stanley760/devops/devops-common/domain"
	"github.com/stanley760/devops/devops-common/domain/gormx"
	"github.com/stanley760/devops/devops-infra/errs"

	"github.com/stanley760/devops/devops-system/internal/model"
	"github.com/stanley760/devops/devops-system/internal/tenantdb"
	"github.com/stanley760/devops/devops-system/internal/types"
)

// ---- Dictionary types ----

// ListDictTypes paginates dictionary types (filter by name/type/status).
func ListDictTypes(ctx context.Context, q types.DictTypePageQuery) (domain.PageResult[model.DictType], error) {
	db := tenantdb.DB(ctx).Model(&model.DictType{})
	if q.Name != "" {
		db = db.Where("name LIKE ?", "%"+q.Name+"%")
	}
	if q.Type != "" {
		db = db.Where("type LIKE ?", "%"+q.Type+"%")
	}
	if q.Status != nil {
		db = db.Where("status = ?", *q.Status)
	}

	page, err := gormx.FindPage[model.DictType](
		db.Order("id ASC"),
		domain.PageParam{PageNum: q.PageNum, PageSize: q.PageSize},
	)
	if err != nil {
		return domain.PageResult[model.DictType]{}, errs.Wrap(err, errs.CodeInternal, "failed to query dictionary type list")
	}
	return page, nil
}

// GetDictTypeByID fetches a dictionary type by ID (within the tenant).
func GetDictTypeByID(ctx context.Context, id int64) (*model.DictType, error) {
	var t model.DictType
	err := tenantdb.DB(ctx).Where("id = ?", id).First(&t).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.New(errs.CodeNotFound, "dictionary type not found")
		}
		return nil, errs.Wrap(err, errs.CodeInternal, "failed to query dictionary type")
	}
	return &t, nil
}

// GetDictTypeByCode fetches a dictionary type by its type code (within the tenant).
func GetDictTypeByCode(ctx context.Context, code string) (*model.DictType, error) {
	var t model.DictType
	err := tenantdb.DB(ctx).Where("type = ?", code).First(&t).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.New(errs.CodeNotFound, "dictionary type not found")
		}
		return nil, errs.Wrap(err, errs.CodeInternal, "failed to query dictionary type")
	}
	return &t, nil
}

// DictTypeCodeExists reports whether the dictionary type code is already taken within the tenant.
func DictTypeCodeExists(ctx context.Context, code string, excludeID int64) (bool, error) {
	db := tenantdb.DB(ctx).Model(&model.DictType{}).Where("type = ?", code)
	if excludeID > 0 {
		db = db.Where("id != ?", excludeID)
	}
	var cnt int64
	if err := db.Count(&cnt).Error; err != nil {
		return false, errs.Wrap(err, errs.CodeInternal, "failed to check dictionary type code uniqueness")
	}
	return cnt > 0, nil
}

// CreateDictType creates a dictionary type (primary key is assigned).
func CreateDictType(ctx context.Context, t *model.DictType) error {
	if err := tenantdb.DB(ctx).Create(t).Error; err != nil {
		return errs.Wrap(err, errs.CodeInternal, "failed to create dictionary type")
	}
	return nil
}

// UpdateDictType updates dictionary type fields.
func UpdateDictType(ctx context.Context, id int64, updates map[string]any) error {
	res := tenantdb.DB(ctx).Model(&model.DictType{}).
		Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return errs.Wrap(res.Error, errs.CodeInternal, "failed to update dictionary type")
	}
	if res.RowsAffected == 0 {
		return errs.New(errs.CodeNotFound, "dictionary type not found")
	}
	return nil
}

// DictTypeHasData reports whether non-deleted dictionary items exist under the type.
func DictTypeHasData(ctx context.Context, code string) (bool, error) {
	var cnt int64
	err := tenantdb.DB(ctx).Model(&model.DictData{}).
		Where("dict_type = ?", code).Count(&cnt).Error
	if err != nil {
		return false, errs.Wrap(err, errs.CodeInternal, "failed to query dictionary items")
	}
	return cnt > 0, nil
}

// SoftDeleteDictTypeTx soft-deletes a dictionary type within a transaction: rewrites the type
// code to bypass the tenant-scoped unique index (the same code can be recreated).
func SoftDeleteDictTypeTx(tx *gorm.DB, id int64, code string) error {
	res := tx.Model(&model.DictType{}).Where("id = ?", id).Updates(map[string]any{
		"type":       code + "_del_" + types.ID(id).String(),
		"deleted_at": time.Now(),
	})
	if res.Error != nil {
		return errs.Wrap(res.Error, errs.CodeInternal, "failed to delete dictionary type")
	}
	return nil
}

// SoftDeleteDictType soft-deletes a dictionary type (single transaction, see SoftDeleteDictTypeTx).
func SoftDeleteDictType(ctx context.Context, id int64, code string) error {
	return WithTx(ctx, func(tx *gorm.DB) error {
		return SoftDeleteDictTypeTx(tx, id, code)
	})
}

// ---- Dictionary data ----

// ListDictData paginates dictionary items (filter by type/label/value/status).
func ListDictData(ctx context.Context, q types.DictDataPageQuery) (domain.PageResult[model.DictData], error) {
	db := tenantdb.DB(ctx).Model(&model.DictData{})
	if q.DictType != "" {
		db = db.Where("dict_type = ?", q.DictType)
	}
	if q.Label != "" {
		db = db.Where("label LIKE ?", "%"+q.Label+"%")
	}
	if q.Value != "" {
		db = db.Where("value LIKE ?", "%"+q.Value+"%")
	}
	if q.Status != nil {
		db = db.Where("status = ?", *q.Status)
	}

	page, err := gormx.FindPage[model.DictData](
		db.Order("dict_type ASC, sort ASC, id ASC"),
		domain.PageParam{PageNum: q.PageNum, PageSize: q.PageSize},
	)
	if err != nil {
		return domain.PageResult[model.DictData]{}, errs.Wrap(err, errs.CodeInternal, "failed to query dictionary item list")
	}
	return page, nil
}

// GetDictDataByID fetches a dictionary item by ID (within the tenant).
func GetDictDataByID(ctx context.Context, id int64) (*model.DictData, error) {
	var d model.DictData
	err := tenantdb.DB(ctx).Where("id = ?", id).First(&d).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.New(errs.CodeNotFound, "dictionary item not found")
		}
		return nil, errs.Wrap(err, errs.CodeInternal, "failed to query dictionary item")
	}
	return &d, nil
}

// DictDataValueExists reports whether the same label/value pair already exists under the type.
func DictDataValueExists(ctx context.Context, dictType, value string, excludeID int64) (bool, error) {
	db := tenantdb.DB(ctx).Model(&model.DictData{}).
		Where("dict_type = ?", dictType).Where("value = ?", value)
	if excludeID > 0 {
		db = db.Where("id != ?", excludeID)
	}
	var cnt int64
	if err := db.Count(&cnt).Error; err != nil {
		return false, errs.Wrap(err, errs.CodeInternal, "failed to check dictionary item uniqueness")
	}
	return cnt > 0, nil
}

// CreateDictData creates a dictionary item (primary key is assigned).
func CreateDictData(ctx context.Context, d *model.DictData) error {
	if err := tenantdb.DB(ctx).Create(d).Error; err != nil {
		return errs.Wrap(err, errs.CodeInternal, "failed to create dictionary item")
	}
	return nil
}

// UpdateDictData updates dictionary item fields.
func UpdateDictData(ctx context.Context, id int64, updates map[string]any) error {
	res := tenantdb.DB(ctx).Model(&model.DictData{}).
		Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return errs.Wrap(res.Error, errs.CodeInternal, "failed to update dictionary item")
	}
	if res.RowsAffected == 0 {
		return errs.New(errs.CodeNotFound, "dictionary item not found")
	}
	return nil
}

// SoftDeleteDictData soft-deletes a dictionary item (the value uniqueness is scoped to
// non-deleted rows, so no key rewrite is needed).
func SoftDeleteDictData(ctx context.Context, id int64) error {
	res := tenantdb.DB(ctx).Delete(&model.DictData{}, id)
	if res.Error != nil {
		return errs.Wrap(res.Error, errs.CodeInternal, "failed to delete dictionary item")
	}
	if res.RowsAffected == 0 {
		return errs.New(errs.CodeNotFound, "dictionary item not found")
	}
	return nil
}
