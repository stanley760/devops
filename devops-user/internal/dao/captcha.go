// Captcha settings dao (C-side).
//
// The C side is single-tenant with no tenant context of its own, but it shares the `devops`
// database with devops-system. The captcha settings are therefore read from the same
// t_system_config / t_system_dict_data tables, with tenant_id fixed to auth.DefaultTenantID (the
// sentinel tenant the shared session layer also uses).
//
// ⚠️ These tables are created and owned by devops-system's dbinit. This package only reads them, and
// every read tolerates the tables being absent (a fresh database where devops-system has not run
// migrate yet): the callers fall back to secure defaults rather than failing the login.
package dao

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/stanley760/devops/devops-infra/errs"
)

// CaptchaConfig is one row of t_system_config (the columns the C side needs).
type CaptchaConfig struct {
	ConfigKey   string
	ConfigValue string
	Type        int8
}

// TableName is the table name (the C side has no GORM model for B-side tables, so the table name is
// declared explicitly here).
func (CaptchaConfig) TableName() string { return "t_system_config" }

// DictData is one row of t_system_dict_data (the columns the C side needs).
type DictData struct {
	DictType string
	Label    string
	Value    string
	Sort     int
	Status   int8
}

// TableName is the table name.
func (DictData) TableName() string { return "t_system_dict_data" }

// GetSystemConfigValue reads a t_system_config value scoped to tenantID.
func GetSystemConfigValue(ctx context.Context, tenantID int64, key string) (string, error) {
	var c CaptchaConfig
	err := db(ctx).Where("tenant_id = ? AND config_key = ?", tenantID, key).First(&c).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", errs.New(errs.CodeNotFound, "configuration not found")
		}
		// A missing table surfaces here too (fresh DB before devops-system migrate); the caller
		// treats any error as "unavailable" and falls back to its default.
		return "", errs.Wrap(err, errs.CodeInternal, "failed to query configuration")
	}
	return c.ConfigValue, nil
}

// ListSystemDictValues returns the `value` column of the enabled items under one dictionary type,
// scoped to tenantID, in sort order.
func ListSystemDictValues(ctx context.Context, tenantID int64, dictType string) ([]string, error) {
	var rows []DictData
	err := db(ctx).
		Where("tenant_id = ? AND dict_type = ? AND status = ?", tenantID, dictType, 1).
		Order("sort ASC, id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, errs.Wrap(err, errs.CodeInternal, "failed to query dictionary values")
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Value)
	}
	return out, nil
}
