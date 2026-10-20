// System configuration dao: t_system_config (tenant table).
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

// ListConfigs paginates configuration items (filter by key/name/type/visible).
func ListConfigs(ctx context.Context, q types.ConfigPageQuery) (domain.PageResult[model.Config], error) {
	db := tenantdb.DB(ctx).Model(&model.Config{})
	if q.ConfigKey != "" {
		db = db.Where("config_key LIKE ?", "%"+q.ConfigKey+"%")
	}
	if q.ConfigName != "" {
		db = db.Where("config_name LIKE ?", "%"+q.ConfigName+"%")
	}
	if q.Type != nil {
		db = db.Where("type = ?", *q.Type)
	}
	if q.Visible != nil {
		db = db.Where("visible = ?", *q.Visible)
	}

	page, err := gormx.FindPage[model.Config](
		db.Order("id ASC"),
		domain.PageParam{PageNum: q.PageNum, PageSize: q.PageSize},
	)
	if err != nil {
		return domain.PageResult[model.Config]{}, errs.Wrap(err, errs.CodeInternal, "failed to query configuration list")
	}
	return page, nil
}

// GetConfigByID fetches a configuration item by ID (within the tenant).
func GetConfigByID(ctx context.Context, id int64) (*model.Config, error) {
	var c model.Config
	err := tenantdb.DB(ctx).Where("id = ?", id).First(&c).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.New(errs.CodeNotFound, "configuration not found")
		}
		return nil, errs.Wrap(err, errs.CodeInternal, "failed to query configuration")
	}
	return &c, nil
}

// GetConfigByKey fetches a configuration item by key (within the tenant).
func GetConfigByKey(ctx context.Context, key string) (*model.Config, error) {
	var c model.Config
	err := tenantdb.DB(ctx).Where("config_key = ?", key).First(&c).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.New(errs.CodeNotFound, "configuration not found")
		}
		return nil, errs.Wrap(err, errs.CodeInternal, "failed to query configuration")
	}
	return &c, nil
}

// ConfigKeyExists reports whether the configuration key is already taken within the tenant.
func ConfigKeyExists(ctx context.Context, key string, excludeID int64) (bool, error) {
	db := tenantdb.DB(ctx).Model(&model.Config{}).Where("config_key = ?", key)
	if excludeID > 0 {
		db = db.Where("id != ?", excludeID)
	}
	var cnt int64
	if err := db.Count(&cnt).Error; err != nil {
		return false, errs.Wrap(err, errs.CodeInternal, "failed to check configuration key uniqueness")
	}
	return cnt > 0, nil
}

// CreateConfig creates a configuration item (primary key is assigned).
func CreateConfig(ctx context.Context, c *model.Config) error {
	if err := tenantdb.DB(ctx).Create(c).Error; err != nil {
		return errs.Wrap(err, errs.CodeInternal, "failed to create configuration")
	}
	return nil
}

// UpdateConfig updates configuration fields.
func UpdateConfig(ctx context.Context, id int64, updates map[string]any) error {
	res := tenantdb.DB(ctx).Model(&model.Config{}).
		Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return errs.Wrap(res.Error, errs.CodeInternal, "failed to update configuration")
	}
	if res.RowsAffected == 0 {
		return errs.New(errs.CodeNotFound, "configuration not found")
	}
	return nil
}

// SoftDeleteConfigTx soft-deletes a configuration item within a transaction: rewrites the key
// to bypass the tenant-scoped unique index (the same key can be recreated).
func SoftDeleteConfigTx(tx *gorm.DB, id int64, key string) error {
	res := tx.Model(&model.Config{}).Where("id = ?", id).Updates(map[string]any{
		"config_key": key + "_del_" + types.ID(id).String(),
		"deleted_at": time.Now(),
	})
	if res.Error != nil {
		return errs.Wrap(res.Error, errs.CodeInternal, "failed to delete configuration")
	}
	return nil
}

// SoftDeleteConfig soft-deletes a configuration item (single transaction, see SoftDeleteConfigTx).
func SoftDeleteConfig(ctx context.Context, id int64, key string) error {
	return WithTx(ctx, func(tx *gorm.DB) error {
		return SoftDeleteConfigTx(tx, id, key)
	})
}
