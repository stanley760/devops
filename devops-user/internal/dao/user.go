// Package dao is the data access layer (C side).
//
// Difference from devops-system/internal/dao: no tenant filtering — the C side
// is single-tenant, so no tenantdb.DB(ctx) wrapper is needed, and there is no
// "fail fast if ctx lacks a tenant" check.
//
// All queries treat gorm.ErrRecordNotFound as "not found" and convert it to an
// errs.BizError, preventing upper layers from seeing raw GORM errors (which
// would leak SQL details to the frontend).
package dao

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/stanley760/devops/devops-infra/errs"

	client "github.com/stanley760/devops/devops-infra/database/mysql"

	"github.com/stanley760/devops/devops-user/internal/model"
)

// db returns the global GORM client (nil before Init; the caller's wiring
// guarantees it is non-nil).
func db(_ context.Context) *gorm.DB { return client.Get() }

// GetUserByUsername gets a user by username.
//
// ⚠️ Message "incorrect account or password": login failure and
// user-not-found **must** return the same error, otherwise attackers could
// enumerate which usernames exist.
func GetUserByUsername(ctx context.Context, username string) (*model.User, error) {
	var u model.User
	err := db(ctx).Where("username = ?", username).First(&u).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// It **must be exactly identical** to the wrong-password branch in
			// service.Login (code + message), otherwise attackers could
			// enumerate which usernames exist from response differences.
			return nil, errs.New(errs.CodeUnauthorized, "incorrect account or password")
		}
		return nil, errs.Wrap(err, errs.CodeInternal, "failed to query user")
	}
	return &u, nil
}

// GetUserByID gets a user by ID.
func GetUserByID(ctx context.Context, userID int64) (*model.User, error) {
	var u model.User
	err := db(ctx).Where("id = ?", userID).First(&u).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.New(errs.CodeNotFound, "user does not exist")
		}
		return nil, errs.Wrap(err, errs.CodeInternal, "failed to query user")
	}
	return &u, nil
}

// IsUsernameTaken reports whether the username is already taken (a pre-check
// before registration, to give a friendly message).
//
// True uniqueness is guaranteed by the database unique index uk_username:
// this function only intercepts early; under a race two requests can still
// pass simultaneously, and the database constraint is the final arbiter (see
// CreateUser).
func IsUsernameTaken(ctx context.Context, username string) (bool, error) {
	var cnt int64
	if err := db(ctx).Model(&model.User{}).Where("username = ?", username).Count(&cnt).Error; err != nil {
		return false, errs.Wrap(err, errs.CodeInternal, "failed to query username")
	}
	return cnt > 0, nil
}

// CreateUser creates a user.
//
// ⚠️ Unique conflicts (uk_username / uk_email / uk_mobile) must produce
// different messages **per specific constraint**: lumping them all into
// "username already registered" would show that message to a user who filled
// in the email field, which is clearly misleading (seen in practice: an email
// duplicate showed "username already registered").
func CreateUser(ctx context.Context, u *model.User) error {
	u.Normalize()
	if err := db(ctx).Create(u).Error; err != nil {
		if dup, index := duplicateField(err); dup {
			// index is the **index name** (uk_username / uk_email /
			// uk_mobile), not the field name.
			switch index {
			case "uk_username":
				return errs.New(errs.CodeParam, "username already registered")
			case "uk_email":
				return errs.New(errs.CodeParam, "email already bound to another account")
			case "uk_mobile":
				return errs.New(errs.CodeParam, "mobile number already bound to another account")
			default:
				return errs.New(errs.CodeParam, "this information is already bound to another account")
			}
		}
		return errs.Wrap(err, errs.CodeInternal, "failed to create user")
	}
	return nil
}

// UpdateProfile updates the profile field by field.
//
// Uses a map instead of a struct update to avoid "omitted fields being
// overwritten with zero values" — the most common bug source in partial-update
// endpoints. An empty map returns directly, avoiding a pointless UPDATE.
func UpdateProfile(ctx context.Context, userID int64, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	res := db(ctx).Model(&model.User{}).Where("id = ?", userID).Updates(fields)
	if res.Error != nil {
		if dup, index := duplicateField(res.Error); dup {
			// index is the index name (uk_email / uk_mobile), not the field
			// name.
			switch index {
			case "uk_email":
				return errs.New(errs.CodeParam, "email already bound to another account")
			case "uk_mobile":
				return errs.New(errs.CodeParam, "mobile number already bound to another account")
			default:
				return errs.New(errs.CodeParam, "this information is already bound to another account")
			}
		}
		return errs.Wrap(res.Error, errs.CodeInternal, "failed to update profile")
	}
	if res.RowsAffected == 0 {
		return errs.New(errs.CodeNotFound, "user does not exist")
	}
	return nil
}

// SetDisabled bans/unbans (used by the admin side; the current skeleton
// exposes no HTTP endpoint).
func SetDisabled(ctx context.Context, userID int64, disabled bool) error {
	res := db(ctx).Model(&model.User{}).Where("id = ?", userID).Update("disabled", disabled)
	if res.Error != nil {
		return errs.Wrap(res.Error, errs.CodeInternal, "failed to update disabled status")
	}
	if res.RowsAffected == 0 {
		return errs.New(errs.CodeNotFound, "user does not exist")
	}
	return nil
}

// UpdatePassword updates the password hash.
//
// It does not verify the old password — that is the service layer's job
// (verify first, then call this method). Uses Model+Where instead of Save,
// because Save would write zero values over other columns.
func UpdatePassword(ctx context.Context, userID int64, hashed string) error {
	res := db(ctx).Model(&model.User{}).Where("id = ?", userID).Update("password", hashed)
	if res.Error != nil {
		return errs.Wrap(res.Error, errs.CodeInternal, "failed to update password")
	}
	if res.RowsAffected == 0 {
		return errs.New(errs.CodeNotFound, "user does not exist")
	}
	return nil
}
