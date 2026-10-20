// User management dao (P5): pure model queries get tenant filtering injected by TenantPlugin.
package dao

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/stanley760/devops/devops-common/domain"
	"github.com/stanley760/devops/devops-common/domain/gormx"
	"github.com/stanley760/devops/devops-infra/datascope"
	"github.com/stanley760/devops/devops-infra/errs"

	"github.com/stanley760/devops/devops-system/internal/model"
	"github.com/stanley760/devops/devops-system/internal/tenantdb"
	"github.com/stanley760/devops/devops-system/internal/types"
)

// ListUsers paginates users (filter by fuzzy username/mobile/status/dept; row-level data-permission narrowing).
// datascope.Apply injects the visible scope from ctx (the users route mounts the HumaDataScope
// resolution middleware); if not injected, the query fails closed to prevent a misconfiguration
// from degrading into full visibility.
func ListUsers(ctx context.Context, q types.UserPageQuery) (domain.PageResult[model.User], error) {
	db := datascope.Apply(ctx, tenantdb.DB(ctx).Model(&model.User{}),
		datascope.Columns{Dept: "dept_id", Owner: "id"})
	if q.Username != "" {
		db = db.Where("username LIKE ?", "%"+q.Username+"%")
	}
	if q.Mobile != "" {
		db = db.Where("mobile = ?", q.Mobile)
	}
	if q.Status != nil {
		db = db.Where("status = ?", *q.Status)
	}
	if q.DeptID != nil {
		db = db.Where("dept_id = ?", q.DeptID.Int64())
	}

	page, err := gormx.FindPage[model.User](
		db.Order("id DESC"),
		domain.PageParam{PageNum: q.PageNum, PageSize: q.PageSize},
	)
	if err != nil {
		return domain.PageResult[model.User]{}, errs.Wrap(err, errs.CodeInternal, "failed to query user list")
	}
	return page, nil
}

// UsernameExists reports whether the username is already taken within the tenant (excludeID excludes itself, for the update case).
func UsernameExists(ctx context.Context, username string, excludeID int64) (bool, error) {
	db := tenantdb.DB(ctx).Model(&model.User{}).Where("username = ?", username)
	if excludeID > 0 {
		db = db.Where("id != ?", excludeID)
	}
	var cnt int64
	if err := db.Count(&cnt).Error; err != nil {
		return false, errs.Wrap(err, errs.CodeInternal, "failed to check username uniqueness")
	}
	return cnt > 0, nil
}

// MobileExists reports whether the mobile number is already taken within the tenant.
func MobileExists(ctx context.Context, mobile string, excludeID int64) (bool, error) {
	if mobile == "" {
		return false, nil
	}
	db := tenantdb.DB(ctx).Model(&model.User{}).Where("mobile = ?", mobile)
	if excludeID > 0 {
		db = db.Where("id != ?", excludeID)
	}
	var cnt int64
	if err := db.Count(&cnt).Error; err != nil {
		return false, errs.Wrap(err, errs.CodeInternal, "failed to check mobile uniqueness")
	}
	return cnt > 0, nil
}

// CreateUser creates a user (primary key is assigned via BeforeCreate).
func CreateUser(ctx context.Context, u *model.User) error {
	if err := tenantdb.DB(ctx).Create(u).Error; err != nil {
		return errs.Wrap(err, errs.CodeInternal, "failed to create user")
	}
	return nil
}

// UpdateUser updates mutable user fields (map form, non-zero fields only).
func UpdateUser(ctx context.Context, userID int64, updates map[string]any) error {
	res := tenantdb.DB(ctx).Model(&model.User{}).
		Where("id = ?", userID).Updates(updates)
	if res.Error != nil {
		return errs.Wrap(res.Error, errs.CodeInternal, "failed to update user")
	}
	if res.RowsAffected == 0 {
		return errs.New(errs.CodeNotFound, "user not found")
	}
	return nil
}

// SoftDeleteUserTx soft-deletes a user (within a transaction): rewrites username/mobile to
// bypass the tenant-scoped unique index, ensuring the same username can be created again
// (the "soft delete, same-name recreate" approach).
func SoftDeleteUserTx(tx *gorm.DB, userID int64, u *model.User, operator string) error {
	now := time.Now()
	res := tx.Model(&model.User{}).Where("id = ?", userID).Updates(map[string]any{
		"username":   fmt.Sprintf("%s_del_%d_%d", u.Username, userID, now.Unix()),
		"mobile":     nil,
		"deleted_at": now,
		"updater":    operator,
	})
	if res.Error != nil {
		return errs.Wrap(res.Error, errs.CodeInternal, "failed to delete user")
	}
	return nil
}

// SyncUserRolesTx rebuilds user-role associations within a transaction (clear, then batch insert).
func SyncUserRolesTx(tx *gorm.DB, userID int64, roleIDs []int64) error {
	if err := tx.Where("user_id = ?", userID).Delete(&model.UserRole{}).Error; err != nil {
		return errs.Wrap(err, errs.CodeInternal, "failed to clean user roles")
	}
	if len(roleIDs) == 0 {
		return nil
	}
	rows := make([]model.UserRole, 0, len(roleIDs))
	for _, rid := range roleIDs {
		rows = append(rows, model.UserRole{UserID: userID, RoleID: rid})
	}
	return tx.Create(&rows).Error
}

// CreateUserWithRoles creates a user + initial role associations (single transaction).
func CreateUserWithRoles(ctx context.Context, u *model.User, roleIDs []int64) error {
	return WithTx(ctx, func(tx *gorm.DB) error {
		if err := tx.Create(u).Error; err != nil {
			return errs.Wrap(err, errs.CodeInternal, "failed to create user")
		}
		if len(roleIDs) > 0 {
			rows := make([]model.UserRole, 0, len(roleIDs))
			for _, rid := range roleIDs {
				rows = append(rows, model.UserRole{UserID: u.ID, RoleID: rid})
			}
			if err := tx.Create(&rows).Error; err != nil {
				return errs.Wrap(err, errs.CodeInternal, "failed to save user roles")
			}
		}
		return nil
	})
}

// UpdateUserWithRoles updates user fields + rebuilds role associations (single transaction).
func UpdateUserWithRoles(ctx context.Context, userID int64, updates map[string]any, roleIDs []int64) error {
	return WithTx(ctx, func(tx *gorm.DB) error {
		if err := tx.Model(&model.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
			return errs.Wrap(err, errs.CodeInternal, "failed to update user")
		}
		return SyncUserRolesTx(tx, userID, roleIDs)
	})
}

// DeleteUser soft-deletes a user + clears role associations (single transaction).
func DeleteUser(ctx context.Context, u *model.User, operator string) error {
	return WithTx(ctx, func(tx *gorm.DB) error {
		if err := SoftDeleteUserTx(tx, u.ID, u, operator); err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", u.ID).Delete(&model.UserRole{}).Error; err != nil {
			return errs.Wrap(err, errs.CodeInternal, "failed to delete user")
		}
		return nil
	})
}

// GetUserRoleIDs returns the user's role ID list within the tenant.
func GetUserRoleIDs(ctx context.Context, userID int64) ([]int64, error) {
	var ids []int64
	err := tenantdb.DB(ctx).Model(&model.UserRole{}).
		Where("user_id = ?", userID).Pluck("role_id", &ids).Error
	if err != nil {
		return nil, errs.Wrap(err, errs.CodeInternal, "failed to query user roles")
	}
	return ids, nil
}

// RoleCodesByIDs looks up role codes by a set of role IDs (within the tenant; also validates ownership).
// Returns the codes in input order; an error is returned if any ID does not exist or belongs to another tenant.
func RoleCodesByIDs(ctx context.Context, roleIDs []int64) ([]string, error) {
	if len(roleIDs) == 0 {
		return nil, nil
	}
	var roles []model.Role
	err := tenantdb.DB(ctx).
		Where("id IN ?", roleIDs).Find(&roles).Error
	if err != nil {
		return nil, errs.Wrap(err, errs.CodeInternal, "failed to query roles")
	}
	if len(roles) != len(roleIDs) {
		return nil, errs.New(errs.CodeParam, "invalid role included")
	}
	codes := make([]string, 0, len(roles))
	for _, r := range roles {
		codes = append(codes, r.Code)
	}
	return codes, nil
}

// PostIDsOf converts a user's post_ids JSON string to []ID. The only stored form is a numeric
// array (funneled on the write side by PostIDsJSON); string elements (["4"]) are treated as dirty
// data — a whole-parse failure returns an empty set, with no compatibility read.
func PostIDsOf(postIDs string) []types.ID {
	if postIDs == "" {
		return []types.ID{}
	}
	var ids []int64
	if err := json.Unmarshal([]byte(postIDs), &ids); err != nil {
		return []types.ID{}
	}
	out := make([]types.ID, 0, len(ids))
	for _, id := range ids {
		out = append(out, types.ID(id))
	}
	return out
}

// PostIDsJSON converts []ID to a JSON string (elements are cast to int64 before marshaling;
// the persisted form is a numeric array [4,5]; an empty set returns "", and the caller decides
// whether to write NULL).
// []types.ID is not marshaled directly — its MarshalJSON emits string elements (["4","5"],
// the API wire contract), which would leak into the storage layer and be inconsistent with the
// seed data and the numeric matching semantics of JSON_CONTAINS(post_ids, 4).
func PostIDsJSON(ids []types.ID) string {
	if len(ids) == 0 {
		return ""
	}
	i64s := make([]int64, 0, len(ids))
	for _, id := range ids {
		i64s = append(i64s, id.Int64())
	}
	b, err := json.Marshal(i64s)
	if err != nil {
		return ""
	}
	return string(b)
}
