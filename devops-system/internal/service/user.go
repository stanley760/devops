// User management service: CRUD + casbin g row linkage.
//
// Consistency strategy: business rows (user + user_role) go through a DB
// transaction; the casbin g rows are rebuilt after the transaction commits.
// A g row write failure only causes "missing permissions" rather than
// over-permissioning (fail-safe), and an error is returned asking for a retry.
package service

import (
	"context"

	"github.com/stanley760/devops/devops-common/domain"
	"github.com/stanley760/devops/devops-infra/datascope"
	"github.com/stanley760/devops/devops-infra/errs"
	"github.com/stanley760/devops/devops-system/internal/auth"
	"github.com/stanley760/devops/devops-system/internal/authz"
	"github.com/stanley760/devops/devops-system/internal/dao"
	"github.com/stanley760/devops/devops-system/internal/model"
	"github.com/stanley760/devops/devops-system/internal/types"
	"golang.org/x/crypto/bcrypt"
)

// ensureUserVisible is the single choke point for data permissions
// (Get/Update/ResetPassword/Delete decide after a direct ID lookup):
// any access violation is presented as 404 (the same response as "user does
// not exist", guarding against existence probing). When the ctx carries no
// data permission, it fails closed with an error (controlled routes already
// mount HumaDataScope, so a missing mount is exposed immediately).
func ensureUserVisible(ctx context.Context, u *model.User) error {
	scope, ok := datascope.From(ctx)
	if !ok {
		return errs.New(errs.CodeInternal, "missing data permission context (the controlled endpoint lacks the resolution middleware)")
	}
	if !scope.Visible(u.DeptID, u.ID) {
		return errs.New(errs.CodeNotFound, "user not found")
	}
	return nil
}

// User is the user management service.
type User struct {
	authM           *auth.Manager
	invalidateScope func() // data permission cache invalidation (called when role assignments change)
}

// NewUser creates the service (authM is used to force logout; invalidateScope clears the data permission cache).
func NewUser(authM *auth.Manager, invalidateScope func()) *User {
	return &User{authM: authM, invalidateScope: invalidateScope}
}

// List returns a paginated query (tenant_id is injected by the plugin, naturally scoped to this tenant).
func (s *User) List(ctx context.Context, q types.UserPageQuery) (domain.PageResult[types.UserDTO], error) {
	page, err := dao.ListUsers(ctx, q)
	if err != nil {
		return domain.PageResult[types.UserDTO]{}, err
	}
	return domain.MapPage(page, func(user model.User) types.UserDTO {
		return toUserDTO(&user)
	}), nil
}

// Get returns the user details (basic info + role/post IDs; data permission single choke point).
func (s *User) Get(ctx context.Context, id types.ID) (*types.UserDetailResp, error) {
	u, err := dao.GetUserByID(ctx, id.Int64())
	if err != nil {
		return nil, err
	}
	if err := ensureUserVisible(ctx, u); err != nil {
		return nil, err
	}
	roleIDs, err := dao.GetUserRoleIDs(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	rids := make([]types.ID, 0, len(roleIDs))
	for _, rid := range roleIDs {
		rids = append(rids, types.ID(rid))
	}
	return &types.UserDetailResp{
		UserDTO: toUserDTO(u),
		RoleIDs: rids,
		PostIDs: dao.PostIDsOf(u.PostIDs),
	}, nil
}

// Create creates a user: tenant-unique validation → issue an ID and persist → role g row linkage.
func (s *User) Create(ctx context.Context, claims *auth.AccessClaims, req types.UserCreateReq) error {
	exists, err := dao.UsernameExists(ctx, req.Username, 0)
	if err != nil {
		return err
	}
	if exists {
		return errs.New(errs.CodeParam, "username already exists")
	}
	if req.Mobile != nil {
		exists, err = dao.MobileExists(ctx, *req.Mobile, 0)
		if err != nil {
			return err
		}
		if exists {
			return errs.New(errs.CodeParam, "mobile number already exists")
		}
	}
	roleCodes, err := dao.RoleCodesByIDs(ctx, idsToInt64(req.RoleIDs))
	if err != nil {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 10)
	if err != nil {
		return errs.Wrap(err, errs.CodeInternal, "failed to hash password")
	}

	u := &model.User{
		BaseTenantModel: model.BaseTenantModel{
			BaseModel: model.BaseModel{Creator: claims.Username, Updater: claims.Username},
		},
		Username: req.Username,
		Password: string(hash),
		Nickname: req.Nickname,
		DeptID:   req.DeptID.Int64(),
		PostIDs:  dao.PostIDsJSON(req.PostIDs),
		Email:    req.Email,
		Mobile:   req.Mobile,
		Gender:   req.Gender,
		Avatar:   req.Avatar,
		Remark:   req.Remark,
		Status:   req.Status,
	}
	if err := dao.CreateUserWithRoles(ctx, u, idsToInt64(req.RoleIDs)); err != nil {
		return err
	}

	if len(roleCodes) > 0 {
		if err := authz.RebuildUserGrouping(u.ID, claims.TenantID, roleCodes); err != nil {
			return errs.Wrap(err, errs.CodeInternal, "user created, but the role authorization write failed; please retry the update")
		}
	}
	return nil
}

// Update updates a user (username/roles/status etc. can be changed; disabling
// forces logout; data permission single choke point).
func (s *User) Update(ctx context.Context, claims *auth.AccessClaims, id types.ID, req types.UserUpdateReq) error {
	u, err := dao.GetUserByID(ctx, id.Int64())
	if err != nil {
		return err
	}
	if err := ensureUserVisible(ctx, u); err != nil {
		return err
	}
	if u.Username != req.Username {
		exists, err := dao.UsernameExists(ctx, req.Username, u.ID)
		if err != nil {
			return err
		}
		if exists {
			return errs.New(errs.CodeParam, "username already exists")
		}
	}
	if req.Mobile != nil && *req.Mobile != "" && (u.Mobile == nil || *u.Mobile != *req.Mobile) {
		exists, err := dao.MobileExists(ctx, *req.Mobile, u.ID)
		if err != nil {
			return err
		}
		if exists {
			return errs.New(errs.CodeParam, "mobile number already exists")
		}
	}
	roleCodes, err := dao.RoleCodesByIDs(ctx, idsToInt64(req.RoleIDs))
	if err != nil {
		return err
	}

	updates := map[string]any{
		"username": req.Username,
		"nickname": req.Nickname,
		"dept_id":  req.DeptID.Int64(),
		"email":    req.Email,
		"gender":   req.Gender,
		"avatar":   req.Avatar,
		"remark":   req.Remark,
		"status":   req.Status,
		"updater":  claims.Username,
	}
	if pj := dao.PostIDsJSON(req.PostIDs); pj != "" {
		updates["post_ids"] = pj
	} else {
		updates["post_ids"] = nil // empty posts are stored as NULL (the json column rejects '')
	}
	if req.Mobile != nil && *req.Mobile != "" {
		updates["mobile"] = *req.Mobile
	} else {
		updates["mobile"] = nil // clearing goes through NULL, so multiple NULLs never conflict on uk_tenant_mobile
	}

	if err := dao.UpdateUserWithRoles(ctx, u.ID, updates, idsToInt64(req.RoleIDs)); err != nil {
		return err
	}

	if err := authz.RebuildUserGrouping(u.ID, claims.TenantID, roleCodes); err != nil {
		return errs.Wrap(err, errs.CodeInternal, "user updated, but the role authorization refresh failed; please retry the update")
	}
	// Role assignments have changed: clear the data permission cache (a dept move also changes the visible scope)
	s.invalidateScope()
	// Status 1→0: disabling forces logout (existing sessions are invalidated immediately)
	if u.Status == 1 && req.Status == 0 {
		s.authM.RevokeAllSessions(ctx, claims.TenantID, u.ID)
	}
	return nil
}

// ResetPassword resets the password: update + revoke all sessions (force re-login; data permission single choke point).
func (s *User) ResetPassword(ctx context.Context, claims *auth.AccessClaims, id types.ID, req types.ResetPasswordReq) error {
	u, err := dao.GetUserByID(ctx, id.Int64())
	if err != nil {
		return err
	}
	if err := ensureUserVisible(ctx, u); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 10)
	if err != nil {
		return errs.Wrap(err, errs.CodeInternal, "failed to hash password")
	}
	if err := dao.UpdateUser(ctx, u.ID, map[string]any{
		"password": string(hash),
		"updater":  claims.Username,
	}); err != nil {
		return err
	}
	s.authM.RevokeAllSessions(ctx, claims.TenantID, u.ID)
	return nil
}

// Delete deletes a user: guards (self/builtin super admin) → soft-delete
// (rewrite the unique key) → clear associations and g rows → force logout.
func (s *User) Delete(ctx context.Context, claims *auth.AccessClaims, id types.ID) error {
	if id.Int64() == claims.UserID {
		return errs.New(errs.CodeParam, "cannot delete the currently logged-in user")
	}
	if id.Int64() == 1 {
		return errs.New(errs.CodeParam, "the builtin administrator cannot be deleted")
	}
	u, err := dao.GetUserByID(ctx, id.Int64())
	if err != nil {
		return err
	}
	if err := ensureUserVisible(ctx, u); err != nil {
		return err
	}

	if err := dao.DeleteUser(ctx, u, claims.Username); err != nil {
		return err
	}

	if err := authz.RemoveUserGrouping(u.ID, claims.TenantID); err != nil {
		return errs.Wrap(err, errs.CodeInternal, "user deleted, but the role authorization cleanup failed")
	}
	s.authM.RevokeAllSessions(ctx, claims.TenantID, u.ID)
	return nil
}

// toUserDTO maps model.User → DTO (hides the password).
func toUserDTO(u *model.User) types.UserDTO {
	dto := types.UserDTO{
		ID:       types.ID(u.ID),
		Username: u.Username,
		Nickname: u.Nickname,
		Email:    u.Email,
		Gender:   u.Gender,
		Avatar:   u.Avatar,
		DeptID:   types.ID(u.DeptID),
		PostIDs:  dao.PostIDsOf(u.PostIDs),
		Remark:   u.Remark,
		LoginIP:  u.LoginIP,
	}
	if u.Mobile != nil {
		dto.Mobile = *u.Mobile
	}
	if u.LoginDate != nil {
		dto.LoginDate = *u.LoginDate
	}
	return dto
}

// idsToInt64 maps []ID → []int64.
func idsToInt64(ids []types.ID) []int64 {
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		out = append(out, id.Int64())
	}
	return out
}
