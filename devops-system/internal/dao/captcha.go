// Captcha configuration dao: reads the captcha master switch / selected type from t_system_config,
// and the selectable type set from the t_system_dict_data dictionary.
//
// Why its own file rather than extending dao/config.go: the captcha settings are read on the
// **public login path** (before any tenant ctx exists for some callers), and keeping the lookup
// surface in one place makes the "which tables does login touch" question answerable at a glance.
package dao

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/stanley760/devops/devops-infra/errs"

	"github.com/stanley760/devops/devops-system/internal/model"
	"github.com/stanley760/devops/devops-system/internal/tenantdb"
)

// GetConfigValue reads a configuration item's value by key.
//
// Not found is returned as a business 404 (the caller decides whether that means "use the default"
// or "misconfigured"), and the tenant scope is injected by TenantPlugin.
func GetConfigValue(ctx context.Context, key string) (string, error) {
	var c model.Config
	err := tenantdb.DB(ctx).Where("config_key = ?", key).First(&c).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", errs.New(errs.CodeNotFound, "configuration not found")
		}
		return "", errs.Wrap(err, errs.CodeInternal, "failed to query configuration")
	}
	return c.ConfigValue, nil
}

// ListDictValues returns the `value` column of all enabled items under one dictionary type, sorted
// by sort order (dictionary items with sort=0 fall back to ID order for a stable result).
//
// Only enabled items are returned: a disabled dictionary item means the value is not selectable.
func ListDictValues(ctx context.Context, dictType string, status *int8) ([]string, error) {
	db := tenantdb.DB(ctx).Model(&model.DictData{}).Where("dict_type = ?", dictType)
	if status != nil {
		db = db.Where("status = ?", *status)
	}
	var rows []model.DictData
	if err := db.Order("sort ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, errs.Wrap(err, errs.CodeInternal, "failed to query dictionary values")
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Value)
	}
	return out, nil
}
