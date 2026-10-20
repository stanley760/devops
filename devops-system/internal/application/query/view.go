package query

import (
	commonaudit "github.com/stanley760/devops/devops-common/audit"
	"github.com/stanley760/devops/devops-common/domain"

	"github.com/stanley760/devops/devops-system/internal/domain/auditlog"
	"github.com/stanley760/devops/devops-system/internal/domain/iam"
	"github.com/stanley760/devops/devops-system/internal/types"
)

// View mappings from domain objects to API DTOs.
//
// Before the refactor these mappings were scattered across the service
// layer: 8 hand-written toXxxDTO functions (plus two more inline duplicate
// implementations, where the toUserDTO copy in service/auth.go:235-250
// matched the version in service/user.go:258 field for field — one using
// derefString, the other an if-nil check: a classic mapping-drift hazard).
//
// They are now all consolidated in this file, with a single user mapping.

// idsToIDs converts int64 → []types.ID.
func idsToIDs(ids []int64) []types.ID {
	out := make([]types.ID, 0, len(ids))
	for _, id := range ids {
		out = append(out, types.ID(id))
	}
	return out
}

// derefString dereferences a nullable string (nil pointer → empty string).
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ---- Users ----

// UserDTO maps a domain user → API DTO (without the password).
func UserDTO(u *iam.User) types.UserDTO {
	dto := types.UserDTO{
		ID:       types.ID(u.ID),
		Username: u.Username,
		Nickname: u.Nickname,
		Email:    u.Email,
		Mobile:   derefString(u.Mobile),
		Gender:   u.Gender,
		Avatar:   u.Avatar,
		DeptID:   types.ID(u.DeptID),
		PostIDs:  idsToIDs(u.PostIDs),
		Remark:   u.Remark,
		LoginIP:  u.LoginIP,
	}
	if u.LoginDate != nil {
		dto.LoginDate = *u.LoginDate
	}
	return dto
}

// UserDetail maps user details (basic info + role/post IDs).
func UserDetail(u *iam.User) *types.UserDetailResp {
	dto := UserDTO(u)
	return &types.UserDetailResp{
		UserDTO: dto,
		RoleIDs: idsToIDs(u.RoleIDs),
		PostIDs: dto.PostIDs,
	}
}

// ---- Roles ----

// RoleDTO maps a domain role → API DTO.
func RoleDTO(r *iam.Role) types.RoleDTO {
	return types.RoleDTO{
		ID:               types.ID(r.ID),
		Name:             r.Name,
		Code:             r.Code,
		Sort:             r.Sort,
		DataScope:        r.DataScope,
		DataScopeDeptIDs: r.DataScopeDeptIDs,
		Status:           r.Status,
		Type:             r.Type,
		Remark:           r.Remark,
		CreateTime:       r.CreateTime,
	}
}

// RoleDetail maps role details (including assigned menu IDs).
func RoleDetail(r *iam.Role) *types.RoleDetailResp {
	return &types.RoleDetailResp{
		RoleDTO: RoleDTO(r),
		MenuIDs: idsToIDs(r.MenuIDs),
	}
}

// ---- Menus ----

// MenuDTO maps a domain menu → API DTO.
func MenuDTO(m *iam.Menu) types.MenuDTO {
	return types.MenuDTO{
		ID:            types.ID(m.ID),
		Name:          m.Name,
		Permission:    m.Permission,
		Type:          m.Type,
		Sort:          m.Sort,
		ParentID:      types.ID(m.ParentID),
		Path:          m.Path,
		Icon:          m.Icon,
		Component:     m.Component,
		ComponentName: m.ComponentName,
		Status:        m.Status,
		Visible:       m.Visible,
		KeepAlive:     m.KeepAlive,
		AlwaysShow:    m.AlwaysShow,
		CreateTime:    m.CreateTime,
	}
}

// MenuTree assembles the menu tree (reusing the generic BuildTree, eliminating the duplicate implementation in the pre-refactor service/menu.go).
func MenuTree(menus []*iam.Menu) []types.MenuTreeResp {
	nodes := BuildTree(menus,
		func(m *iam.Menu) int64 { return m.ID },
		func(m *iam.Menu) int64 { return m.ParentID },
		func(m *iam.Menu) int { return m.Sort },
	)
	return mapNodes(nodes, func(m *iam.Menu) types.MenuTreeResp {
		return types.MenuTreeResp{MenuDTO: MenuDTO(m)}
	}, func(n *types.MenuTreeResp, cs []types.MenuTreeResp) { n.Children = cs })
}

// ---- Departments ----

// DeptDTO maps a domain department → API DTO.
func DeptDTO(d *iam.Dept) types.DeptDTO {
	return types.DeptDTO{
		ID:           types.ID(d.ID),
		Name:         d.Name,
		ParentID:     types.ID(d.ParentID),
		Sort:         d.Sort,
		LeaderUserID: types.ID(d.LeaderUserID),
		Phone:        d.Phone,
		Email:        d.Email,
		Status:       d.Status,
		CreateTime:   d.CreateTime,
	}
}

// DeptTree assembles the department tree.
func DeptTree(depts []*iam.Dept) []types.DeptTreeResp {
	nodes := BuildTree(depts,
		func(d *iam.Dept) int64 { return d.ID },
		func(d *iam.Dept) int64 { return d.ParentID },
		func(d *iam.Dept) int { return d.Sort },
	)
	return mapNodes(nodes, func(d *iam.Dept) types.DeptTreeResp {
		return types.DeptTreeResp{DeptDTO: DeptDTO(d)}
	}, func(n *types.DeptTreeResp, cs []types.DeptTreeResp) { n.Children = cs })
}

// ---- Posts ----

// PostDTO maps a domain post → API DTO.
func PostDTO(p *iam.Post) types.PostDTO {
	return types.PostDTO{
		ID:         types.ID(p.ID),
		Code:       p.Code,
		Name:       p.Name,
		Sort:       p.Sort,
		Status:     p.Status,
		Remark:     p.Remark,
		CreateTime: p.CreateTime,
	}
}

// ---- Tenants ----

// TenantDTO maps a domain tenant → API DTO.
func TenantDTO(t *iam.Tenant) types.TenantDTO {
	return types.TenantDTO{
		ID:            types.ID(t.ID),
		Name:          t.Name,
		Code:          t.Code,
		ContactName:   t.ContactName,
		ContactMobile: t.ContactMobile,
		Status:        t.Status,
		ExpireTime:    t.ExpireTime,
		Remark:        t.Remark,
		CreateTime:    t.CreateTime,
	}
}

// TenantBrief is the tenant brief (embedded in the login response).
func TenantBrief(t *iam.Tenant) types.TenantBrief {
	return types.TenantBrief{ID: types.ID(t.ID), Name: t.Name, Code: t.Code}
}

// ---- Audit logs ----

// LoginLogDTO maps a login log → API DTO.
func LoginLogDTO(l *auditlog.LoginLog) types.LoginLogDTO {
	return types.LoginLogDTO{
		ID:          types.ID(l.ID),
		Username:    l.Username,
		UserID:      types.ID(l.UserID),
		IP:          l.IP,
		UserAgent:   l.UserAgent,
		GeoCountry:  l.GeoCountry,
		GeoProvince: l.GeoProvince,
		GeoCity:     l.GeoCity,
		GeoIsp:      l.GeoISP,
		Result:      l.Result,
		Code:        l.Code,
		CreateTime:  l.CreateTime,
	}
}

// OperationLogDTO maps an operation log → API DTO.
func OperationLogDTO(l *auditlog.OperationLog) types.OperationLogDTO {
	return types.OperationLogDTO{
		ID:            types.ID(l.ID),
		TraceID:       l.TraceID,
		UserID:        types.ID(l.UserID),
		UserName:      l.UserName,
		UserType:      commonaudit.UserType(l.UserType),
		UserTypeName:  commonaudit.UserType(l.UserType).String(),
		Module:        l.Module,
		SubModule:     l.SubModule,
		BizID:         types.ID(l.BizID),
		Contents:      l.Contents,
		Extra:         l.Extra,
		RequestMethod: l.RequestMethod,
		RequestURL:    l.RequestURL,
		UserIP:        l.UserIP,
		UserAgent:     l.UserAgent,
		GeoCountry:    l.GeoCountry,
		GeoProvince:   l.GeoProvince,
		GeoCity:       l.GeoCity,
		GeoIsp:        l.GeoISP,
		Result:        l.Result,
		Code:          l.Code,
		CreateTime:    l.CreateTime,
	}
}

// mapNodes maps a generic node tree onto an API DTO tree.
func mapNodes[S, D any](nodes []TreeNode[S], wrap func(S) D, attach func(*D, []D)) []D {
	out := make([]D, 0, len(nodes))
	var walk func([]TreeNode[S]) []D
	walk = func(ns []TreeNode[S]) []D {
		if len(ns) == 0 {
			return nil
		}
		res := make([]D, 0, len(ns))
		for _, n := range ns {
			d := wrap(n.Value)
			if len(n.Children) > 0 {
				attach(&d, walk(n.Children))
			}
			res = append(res, d)
		}
		return res
	}
	out = walk(nodes)
	return out
}

// MapPageEntities maps a domain page result → DTO page result (preserving pagination metadata).
func MapPageEntities[S, D any](page domain.PageResult[S], convert func(S) D) domain.PageResult[D] {
	return domain.MapPage(page, convert)
}
