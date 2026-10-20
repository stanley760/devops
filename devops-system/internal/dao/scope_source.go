package dao

import (
	"context"

	"github.com/stanley760/devops/devops-infra/datascope"
)

// Data source adapter for data-permission resolution: converts model.X into infra value
// objects (datascope.RoleScope / UserRef / DeptNode) for the Kit middleware's Deps binding.
//
// model only appears in the dao layer; middleware/humakit sees only value objects — this is
// the prerequisite for Kit to live in devops-infra (see devops-infra/security/humakit).

// UserScopes returns a user's role data-permission declarations within the tenant (datascope.RoleScope view).
func UserScopes(ctx context.Context, userID int64) ([]datascope.RoleScope, error) {
	roles, err := GetUserRoles(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]datascope.RoleScope, 0, len(roles))
	for _, r := range roles {
		out = append(out, datascope.RoleScope{
			Code:             r.Code,
			DataScope:        r.DataScope,
			DataScopeDeptIDs: r.DataScopeDeptIDs,
		})
	}
	return out, nil
}

// UserRef returns the minimal user view (ID + dept) needed for data-permission resolution.
func UserRef(ctx context.Context, userID int64) (datascope.UserRef, error) {
	u, err := GetUserByID(ctx, userID)
	if err != nil {
		return datascope.UserRef{}, err
	}
	return datascope.UserRef{ID: u.ID, DeptID: u.DeptID}, nil
}

// DeptNodes returns the minimal dept tree view (for subtree expansion).
func DeptNodes(ctx context.Context) ([]datascope.DeptNode, error) {
	depts, err := ListDepts(ctx, nil)
	if err != nil {
		return nil, err
	}
	out := make([]datascope.DeptNode, 0, len(depts))
	for _, d := range depts {
		out = append(out, datascope.DeptNode{ID: d.ID, ParentID: d.ParentID})
	}
	return out, nil
}
