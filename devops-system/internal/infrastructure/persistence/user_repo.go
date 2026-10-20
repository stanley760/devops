package persistence

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/stanley760/devops/devops-common/domain"
	"github.com/stanley760/devops/devops-common/domain/gormx"
	"github.com/stanley760/devops/devops-infra/datascope"
	"github.com/stanley760/devops/devops-infra/errs"

	"github.com/stanley760/devops/devops-system/internal/domain/iam"
	"github.com/stanley760/devops/devops-system/internal/model"
)

// userRepo is the GORM implementation of iam.UserRepository.
type userRepo struct{}

// NewUserRepository constructs the user repository.
func NewUserRepository() iam.UserRepository { return userRepo{} }

// Save persists the user aggregate, and persists the aggregate's recorded domain events in the same transaction.
//
// Domain events and business rows are written in the same transaction, guaranteeing that "state
// has changed" and "pending projection facts" are persisted atomically.
func (r userRepo) Save(ctx context.Context, u *iam.User) error {
	conn := db(ctx)
	now := time.Now()

	if u.ID == 0 {
		u.UpdateTime = now
		if err := conn.Create(toUserPO(u)).Error; err != nil {
			return errs.Wrap(err, errs.CodeInternal, "failed to create user")
		}
	} else {
		upd := toUserUpdates(u)
		res := conn.Model(&model.User{}).Where("id = ?", u.ID).Updates(upd)
		if res.Error != nil {
			return errs.Wrap(res.Error, errs.CodeInternal, "failed to update user")
		}
		if res.RowsAffected == 0 {
			return errs.New(errs.CodeNotFound, "user not found")
		}
	}

	// role associations: clear, then rebuild (same transaction).
	if u.RoleIDs != nil {
		if err := r.replaceRolesTx(ctx, u.ID, u.RoleIDs); err != nil {
			return err
		}
	}

	// domain events and business writes are persisted in the same transaction.
	return appendEvents(ctx, u.PullEvents()...)
}

// Get loads the user aggregate by ID (including role references).
func (r userRepo) Get(ctx context.Context, id int64) (*iam.User, error) {
	var po model.User
	if err := db(ctx).Where("id = ?", id).First(&po).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.New(errs.CodeNotFound, "user not found")
		}
		return nil, errs.Wrap(err, errs.CodeInternal, "failed to query user")
	}
	u := toUserDomain(&po)
	roleIDs, err := r.roleIDs(ctx, po.ID)
	if err != nil {
		return nil, err
	}
	u.RoleIDs = roleIDs
	return u, nil
}

// GetByUsername loads by username (login scenario).
func (r userRepo) GetByUsername(ctx context.Context, username string) (*iam.User, error) {
	var po model.User
	if err := db(ctx).Where("username = ?", username).First(&po).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.New(errs.CodeNotFound, "user not found")
		}
		return nil, errs.Wrap(err, errs.CodeInternal, "failed to query user")
	}
	return toUserDomain(&po), nil
}

// List paginates users (row-level data-permission narrowing).
//
// datascope.Apply injects the visible scope from ctx; when not injected it fails closed with an
// error, consistent with the existing dao.ListUsers safety semantics.
func (r userRepo) List(ctx context.Context, f iam.UserFilter) (domain.PageResult[*iam.User], error) {
	conn := datascope.Apply(ctx, db(ctx).Model(&model.User{}),
		datascope.Columns{Dept: "dept_id", Owner: "id"})
	if f.Username != "" {
		conn = conn.Where("username LIKE ?", "%"+f.Username+"%")
	}
	if f.Mobile != "" {
		conn = conn.Where("mobile = ?", f.Mobile)
	}
	if f.Status != nil {
		conn = conn.Where("status = ?", *f.Status)
	}
	if f.DeptID != nil {
		conn = conn.Where("dept_id = ?", *f.DeptID)
	}

	page, err := gormx.FindPage[model.User](conn.Order("id DESC"), f.PageParam)
	if err != nil {
		return domain.PageResult[*iam.User]{}, errs.Wrap(err, errs.CodeInternal, "failed to query user list")
	}
	out := make([]*iam.User, 0, len(page.List))
	for i := range page.List {
		out = append(out, toUserDomain(&page.List[i]))
	}
	return domain.PageResult[*iam.User]{
		List: out, Total: page.Total, PageNum: page.PageNum,
		PageSize: page.PageSize, Pages: page.Pages,
	}, nil
}

// Delete soft-deletes the user and cleans up role associations, while persisting domain events.
//
// The unique-key rewrite is a storage contract: uk_tenant_username(tenant_id, username) and
// uk_tenant_mobile(tenant_id, mobile) require the username/mobile to be vacated after soft
// deletion, so that a user with the same name can be recreated. mobile is set to NULL (rather
// than "") to avoid the multiple-null conflict of the unique index.
func (r userRepo) Delete(ctx context.Context, u *iam.User, operator string) error {
	conn := db(ctx)
	now := time.Now()
	renamed := iam.RenameSoftDeleted(u.Username, u.ID, now)

	res := conn.Model(&model.User{}).Where("id = ?", u.ID).Updates(map[string]any{
		"username":   renamed,
		"mobile":     nil,
		"deleted_at": now,
		"updater":    operator,
	})
	if res.Error != nil {
		return errs.Wrap(res.Error, errs.CodeInternal, "failed to delete user")
	}
	if err := conn.Where("user_id = ?", u.ID).Delete(&model.UserRole{}).Error; err != nil {
		return errs.Wrap(err, errs.CodeInternal, "failed to delete user")
	}
	return appendEvents(ctx, u.PullEvents()...)
}

// UsernameExists reports whether the username is already taken.
func (r userRepo) UsernameExists(ctx context.Context, username string, excludeID int64) (bool, error) {
	return existsBy(ctx, &model.User{}, "username", username, excludeID, "failed to check username uniqueness")
}

// MobileExists reports whether the mobile number is already taken (an empty string counts as not taken).
func (r userRepo) MobileExists(ctx context.Context, mobile string, excludeID int64) (bool, error) {
	if mobile == "" {
		return false, nil
	}
	return existsBy(ctx, &model.User{}, "mobile", mobile, excludeID, "failed to check mobile uniqueness")
}

// ReplaceRoles rebuilds user-role associations.
func (r userRepo) ReplaceRoles(ctx context.Context, userID int64, roleIDs []int64) error {
	return r.replaceRolesTx(ctx, userID, roleIDs)
}

func (r userRepo) replaceRolesTx(ctx context.Context, userID int64, roleIDs []int64) error {
	conn := db(ctx)
	if err := conn.Where("user_id = ?", userID).Delete(&model.UserRole{}).Error; err != nil {
		return errs.Wrap(err, errs.CodeInternal, "failed to clean user roles")
	}
	if len(roleIDs) == 0 {
		return nil
	}
	rows := make([]model.UserRole, 0, len(roleIDs))
	for _, rid := range roleIDs {
		rows = append(rows, model.UserRole{UserID: userID, RoleID: rid})
	}
	if err := conn.Create(&rows).Error; err != nil {
		return errs.Wrap(err, errs.CodeInternal, "failed to save user roles")
	}
	return nil
}

func (r userRepo) roleIDs(ctx context.Context, userID int64) ([]int64, error) {
	var ids []int64
	if err := db(ctx).Model(&model.UserRole{}).
		Where("user_id = ?", userID).Pluck("role_id", &ids).Error; err != nil {
		return nil, errs.Wrap(err, errs.CodeInternal, "failed to query user roles")
	}
	return ids, nil
}

// RoleCodesByIDs looks up codes by role ID (also validates ownership).
//
// Safety direction: an error if any ID does not exist or belongs to another tenant — the tenant
// plugin guarantees rows from other tenants are not found, so the count comparison is
// equivalent to an ownership check.
func (r userRepo) RoleCodesByIDs(ctx context.Context, roleIDs []int64) ([]string, error) {
	if len(roleIDs) == 0 {
		return nil, nil
	}
	var roles []model.Role
	if err := db(ctx).Where("id IN ?", roleIDs).Find(&roles).Error; err != nil {
		return nil, errs.Wrap(err, errs.CodeInternal, "failed to query roles")
	}
	if len(roles) != len(roleIDs) {
		return nil, errs.New(errs.CodeParam, "invalid role included")
	}
	codes := make([]string, 0, len(roles))
	for _, x := range roles {
		codes = append(codes, x.Code)
	}
	return codes, nil
}

// TouchLogin records the login IP and time.
func (r userRepo) TouchLogin(ctx context.Context, userID int64, ip string) error {
	now := time.Now()
	if err := db(ctx).Model(&model.User{}).Where("id = ?", userID).Updates(map[string]any{
		"login_ip":   ip,
		"login_date": &now,
	}).Error; err != nil {
		return errs.Wrap(err, errs.CodeInternal, "failed to update login info")
	}
	return nil
}

// existsBy is the generic uniqueness check.
//
// Before the refactor this logic was copied 5 times in dao (username/mobile/role code/post code/tenant code);
// it is now converged into a single implementation.
func existsBy(ctx context.Context, m any, column, value string, excludeID int64, failMsg string) (bool, error) {
	conn := db(ctx).Model(m).Where(column+" = ?", value)
	if excludeID > 0 {
		conn = conn.Where("id != ?", excludeID)
	}
	var cnt int64
	if err := conn.Count(&cnt).Error; err != nil {
		return false, errs.Wrap(err, errs.CodeInternal, failMsg)
	}
	return cnt > 0, nil
}

// toUserPO domain → persistence object.
func toUserPO(u *iam.User) *model.User {
	return &model.User{
		BaseTenantModel: model.BaseTenantModel{
			BaseModel: model.BaseModel{Creator: u.Nickname, Updater: u.Nickname},
			TenantID:  u.TenantID,
		},
		Username:  u.Username,
		Password:  u.Password,
		Nickname:  u.Nickname,
		Remark:    u.Remark,
		DeptID:    u.DeptID,
		PostIDs:   iam.EncodeIDArray(u.PostIDs),
		Email:     u.Email,
		Mobile:    u.Mobile,
		Gender:    u.Gender,
		Avatar:    u.Avatar,
		Status:    int8(u.Status),
		LoginIP:   u.LoginIP,
		LoginDate: u.LoginDate,
	}
}

// toUserDomain persistence object → domain.
func toUserDomain(po *model.User) *iam.User {
	return &iam.User{
		ID:         po.ID,
		TenantID:   po.TenantID,
		Username:   po.Username,
		Password:   po.Password,
		Nickname:   po.Nickname,
		Remark:     po.Remark,
		DeptID:     po.DeptID,
		PostIDs:    iam.DecodeIDArray(po.PostIDs),
		Email:      po.Email,
		Mobile:     po.Mobile,
		Gender:     po.Gender,
		Avatar:     po.Avatar,
		Status:     iam.UserStatus(po.Status),
		LoginIP:    po.LoginIP,
		LoginDate:  po.LoginDate,
		CreateTime: po.CreateTime,
		UpdateTime: po.UpdateTime,
	}
}

// toUserUpdates domain → update column mapping.
//
// Before the refactor this was the hand-written map[string]any at service/user.go:169-189 with magic string keys;
// NULL/empty-string special cases (empty post_ids stored as NULL, empty mobile stored as NULL to
// avoid the unique index multiple-null conflict) are funneled here — storage constraints are the
// repository's responsibility and should not leak into the application layer.
func toUserUpdates(u *iam.User) map[string]any {
	upd := map[string]any{
		"username": u.Username,
		"nickname": u.Nickname,
		"remark":   u.Remark,
		"dept_id":  u.DeptID,
		"email":    u.Email,
		"gender":   u.Gender,
		"avatar":   u.Avatar,
		"status":   int8(u.Status),
	}
	// an empty set is stored as NULL: the json column rejects '', and multiple NULLs do not trigger unique index conflicts.
	if encoded := iam.EncodeIDArray(u.PostIDs); encoded != "" {
		upd["post_ids"] = encoded
	} else {
		upd["post_ids"] = nil
	}
	if u.Mobile != nil && *u.Mobile != "" {
		upd["mobile"] = *u.Mobile
	} else {
		upd["mobile"] = nil
	}
	return upd
}
