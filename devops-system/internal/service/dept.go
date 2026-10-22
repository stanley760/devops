// Department/post management service.
//
// The department tree is assembled in memory (no ancestors column, aligned with
// yudao); updates validate that the parent is neither the node itself nor one
// of its descendants, preventing a subtree from being hung under itself and
// forming a cycle. A post code is unique within a tenant and cannot be changed
// after creation.
package service

import (
	"context"

	"github.com/stanley760/devops/devops-common/domain"
	"github.com/stanley760/devops/devops-infra/errs"
	"github.com/stanley760/devops/devops-system/internal/auth"
	"github.com/stanley760/devops/devops-system/internal/dao"
	"github.com/stanley760/devops/devops-system/internal/model"
	"github.com/stanley760/devops/devops-system/internal/types"
)

// Dept is the department management service.
type Dept struct{}

// NewDept creates the service.
func NewDept() *Dept { return &Dept{} }

// List returns the department tree (status filter optional).
func (s *Dept) List(ctx context.Context, status *int8) ([]types.DeptTreeResp, error) {
	depts, err := dao.ListDepts(ctx, status)
	if err != nil {
		return nil, err
	}
	tree, err := fillDeptLeaderNames(ctx, buildDeptTree(depts))
	if err != nil {
		return nil, err
	}
	return tree, nil
}

// fillDeptLeaderNames fills LeaderName for display using one batched query,
// walking the whole tree. An unresolvable leader leaves the name empty
// (e.g. leaderUserId = 0 means no leader), never a fabricated value.
func fillDeptLeaderNames(ctx context.Context, nodes []types.DeptTreeResp) ([]types.DeptTreeResp, error) {
	ids := make([]int64, 0, len(nodes))
	seen := make(map[int64]struct{}, len(nodes))
	var walk func(items []types.DeptTreeResp)
	walk = func(items []types.DeptTreeResp) {
		for i := range items {
			id := items[i].LeaderUserID.Int64()
			if id > 0 {
				if _, ok := seen[id]; !ok {
					seen[id] = struct{}{}
					ids = append(ids, id)
				}
			}
			if len(items[i].Children) > 0 {
				walk(items[i].Children)
			}
		}
	}
	walk(nodes)

	names, err := dao.UserNicknames(ctx, ids)
	if err != nil {
		return nil, err
	}
	var fill func(items []types.DeptTreeResp)
	fill = func(items []types.DeptTreeResp) {
		for i := range items {
			items[i].LeaderName = names[items[i].LeaderUserID.Int64()]
			if len(items[i].Children) > 0 {
				fill(items[i].Children)
			}
		}
	}
	fill(nodes)
	return nodes, nil
}

// Get returns the department details.
func (s *Dept) Get(ctx context.Context, id types.ID) (*types.DeptDTO, error) {
	d, err := dao.GetDeptByID(ctx, id.Int64())
	if err != nil {
		return nil, err
	}
	dto := toDeptDTO(d)
	if dto.LeaderUserID.Int64() > 0 {
		names, err := dao.UserNicknames(ctx, []int64{dto.LeaderUserID.Int64()})
		if err != nil {
			return nil, err
		}
		dto.LeaderName = names[dto.LeaderUserID.Int64()]
	}
	return &dto, nil
}

// Create creates a department: validates that the parent department exists.
func (s *Dept) Create(ctx context.Context, claims *auth.AccessClaims, req types.DeptSaveReq) error {
	if req.ParentID.Int64() > 0 {
		if _, err := dao.GetDeptByID(ctx, req.ParentID.Int64()); err != nil {
			return err
		}
	}
	d := &model.Dept{
		BaseTenantModel: model.BaseTenantModel{
			BaseModel: model.BaseModel{Creator: claims.Username, Updater: claims.Username},
		},
		Name:         req.Name,
		ParentID:     req.ParentID.Int64(),
		Sort:         req.Sort,
		LeaderUserID: req.LeaderUserID.Int64(),
		Phone:        req.Phone,
		Email:        derefString(req.Email),
		Status:       req.Status,
	}
	return dao.CreateDept(ctx, d)
}

// Update updates a department: the parent cannot be the node itself or one of
// its descendants (cycle prevention).
func (s *Dept) Update(ctx context.Context, claims *auth.AccessClaims, id types.ID, req types.DeptSaveReq) error {
	d, err := dao.GetDeptByID(ctx, id.Int64())
	if err != nil {
		return err
	}
	parentID := req.ParentID.Int64()
	if parentID != 0 {
		if parentID == d.ID {
			return errs.New(errs.CodeParam, "parent department cannot be itself")
		}
		if _, err := dao.GetDeptByID(ctx, parentID); err != nil {
			return err
		}
		// Descendant check: a cycle forms if parentID falls inside the subtree rooted at d
		depts, err := dao.ListDepts(ctx, nil)
		if err != nil {
			return err
		}
		childrenOf := make(map[int64][]int64, len(depts))
		for _, dep := range depts {
			childrenOf[dep.ParentID] = append(childrenOf[dep.ParentID], dep.ID)
		}
		if inSubtree(childrenOf, d.ID, parentID) {
			return errs.New(errs.CodeParam, "parent department cannot be a descendant of itself")
		}
	}

	return dao.UpdateDept(ctx, d.ID, map[string]any{
		"name":           req.Name,
		"parent_id":      parentID,
		"sort":           req.Sort,
		"leader_user_id": req.LeaderUserID.Int64(),
		"phone":          req.Phone,
		"email":          derefString(req.Email),
		"status":         req.Status,
		"updater":        claims.Username,
	})
}

// Delete deletes a department: departments with children or users cannot be deleted.
func (s *Dept) Delete(ctx context.Context, id types.ID) error {
	deptID := id.Int64()
	if _, err := dao.GetDeptByID(ctx, deptID); err != nil {
		return err
	}
	has, err := dao.DeptHasChildren(ctx, deptID)
	if err != nil {
		return err
	}
	if has {
		return errs.New(errs.CodeParam, "child departments exist, cannot delete")
	}
	has, err = dao.DeptHasUsers(ctx, deptID)
	if err != nil {
		return err
	}
	if has {
		return errs.New(errs.CodeParam, "users exist under this department, cannot delete")
	}
	return dao.DeleteDept(ctx, deptID)
}

// inSubtree reports whether target lies in the subtree rooted at root (BFS).
func inSubtree(childrenOf map[int64][]int64, root, target int64) bool {
	queue := []int64{root}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur == target {
			return true
		}
		queue = append(queue, childrenOf[cur]...)
	}
	return false
}

// buildDeptTree converts a department list into a tree (a parent outside the
// set, or 0, is a root).
func buildDeptTree(depts []model.Dept) []types.DeptTreeResp {
	byID := make(map[int64]*types.DeptTreeResp, len(depts))
	roots := make([]*types.DeptTreeResp, 0)
	for i := range depts {
		node := &types.DeptTreeResp{DeptDTO: toDeptDTO(&depts[i])}
		byID[depts[i].ID] = node
	}
	for i := range depts {
		d := &depts[i]
		node := byID[d.ID]
		if parent, ok := byID[d.ParentID]; ok && d.ParentID != d.ID {
			parent.Children = append(parent.Children, *node)
		} else {
			roots = append(roots, node)
		}
	}
	out := make([]types.DeptTreeResp, 0, len(roots))
	for _, n := range roots {
		out = append(out, *n)
	}
	return out
}

// toDeptDTO maps model.Dept → DTO.
func toDeptDTO(d *model.Dept) types.DeptDTO {
	return types.DeptDTO{
		ID:           types.ID(d.ID),
		Name:         d.Name,
		ParentID:     types.ID(d.ParentID),
		Sort:         d.Sort,
		LeaderUserID: types.ID(d.LeaderUserID),
		Phone:        d.Phone,
		Email:        d.Email,
		Status:       d.Status,
		CreateTime:   types.NewDateTime(d.CreateTime),
	}
}

// ---- Post ----

// Post is the post management service.
type Post struct{}

// NewPost creates the service.
func NewPost() *Post { return &Post{} }

// List returns a paginated query.
func (s *Post) List(ctx context.Context, q types.PostPageQuery) (domain.PageResult[types.PostDTO], error) {
	page, err := dao.ListPosts(ctx, q)
	if err != nil {
		return domain.PageResult[types.PostDTO]{}, err
	}
	return domain.MapPage(page, func(post model.Post) types.PostDTO {
		return toPostDTO(&post)
	}), nil
}

// Get returns the post details.
func (s *Post) Get(ctx context.Context, id types.ID) (*types.PostDTO, error) {
	p, err := dao.GetPostByID(ctx, id.Int64())
	if err != nil {
		return nil, err
	}
	dto := toPostDTO(p)
	return &dto, nil
}

// Create creates a post: the code is unique within a tenant.
func (s *Post) Create(ctx context.Context, claims *auth.AccessClaims, req types.PostSaveReq) error {
	exists, err := dao.PostCodeExists(ctx, req.Code, 0)
	if err != nil {
		return err
	}
	if exists {
		return errs.New(errs.CodeParam, "post code already exists")
	}
	p := &model.Post{
		BaseTenantModel: model.BaseTenantModel{
			BaseModel: model.BaseModel{Creator: claims.Username, Updater: claims.Username},
		},
		Code:   req.Code,
		Name:   req.Name,
		Sort:   req.Sort,
		Status: req.Status,
		Remark: req.Remark,
	}
	return dao.CreatePost(ctx, p)
}

// Update updates a post (the code cannot be changed after creation).
func (s *Post) Update(ctx context.Context, claims *auth.AccessClaims, id types.ID, req types.PostSaveReq) error {
	p, err := dao.GetPostByID(ctx, id.Int64())
	if err != nil {
		return err
	}
	return dao.UpdatePost(ctx, p.ID, map[string]any{
		"name":    req.Name,
		"sort":    req.Sort,
		"status":  req.Status,
		"remark":  req.Remark,
		"updater": claims.Username,
	})
}

// Delete deletes a post: posts referenced by users' post_ids cannot be deleted
// → soft-delete (the code is rewritten so a same-named post can be recreated).
func (s *Post) Delete(ctx context.Context, claims *auth.AccessClaims, id types.ID) error {
	p, err := dao.GetPostByID(ctx, id.Int64())
	if err != nil {
		return err
	}
	has, err := dao.PostReferencedByUsers(ctx, p.ID)
	if err != nil {
		return err
	}
	if has {
		return errs.New(errs.CodeParam, "post is referenced by users; unlink it first")
	}
	return dao.SoftDeletePost(ctx, p.ID, p.Code)
}

// toPostDTO maps model.Post → DTO.
func toPostDTO(p *model.Post) types.PostDTO {
	return types.PostDTO{
		ID:         types.ID(p.ID),
		Code:       p.Code,
		Name:       p.Name,
		Sort:       p.Sort,
		Status:     p.Status,
		Remark:     p.Remark,
		CreateTime: types.NewDateTime(p.CreateTime),
	}
}
