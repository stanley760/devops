// Department/post management dao.
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

// ---- Departments ----

// ListDepts returns all departments (the tree is assembled in memory by the service).
func ListDepts(ctx context.Context, status *int8) ([]model.Dept, error) {
	db := tenantdb.DB(ctx).Model(&model.Dept{})
	if status != nil {
		db = db.Where("status = ?", *status)
	}
	var depts []model.Dept
	if err := db.Order("sort ASC, id ASC").Find(&depts).Error; err != nil {
		return nil, errs.Wrap(err, errs.CodeInternal, "failed to query departments")
	}
	return depts, nil
}

// GetDeptByID fetches a department by ID (within the tenant).
func GetDeptByID(ctx context.Context, deptID int64) (*model.Dept, error) {
	var d model.Dept
	err := tenantdb.DB(ctx).Where("id = ?", deptID).First(&d).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.New(errs.CodeNotFound, "department not found")
		}
		return nil, errs.Wrap(err, errs.CodeInternal, "failed to query department")
	}
	return &d, nil
}

// DeptNames batch-fetches department names by ID for display (e.g. the dept column
// in the user list). One query fetches all names, avoiding per-row N+1; unknown IDs
// are simply absent from the result.
func DeptNames(ctx context.Context, deptIDs []int64) (map[int64]string, error) {
	if len(deptIDs) == 0 {
		return nil, nil
	}
	type row struct {
		ID   int64
		Name string
	}
	var rows []row
	err := tenantdb.DB(ctx).Model(&model.Dept{}).
		Where("id IN ?", deptIDs).Select("id, name").Find(&rows).Error
	if err != nil {
		return nil, errs.Wrap(err, errs.CodeInternal, "failed to query department names")
	}
	out := make(map[int64]string, len(rows))
	for _, r := range rows {
		out[r.ID] = r.Name
	}
	return out, nil
}

// CreateDept creates a department (primary key is assigned).
func CreateDept(ctx context.Context, d *model.Dept) error {
	if err := tenantdb.DB(ctx).Create(d).Error; err != nil {
		return errs.Wrap(err, errs.CodeInternal, "failed to create department")
	}
	return nil
}

// UpdateDept updates department fields.
func UpdateDept(ctx context.Context, deptID int64, updates map[string]any) error {
	res := tenantdb.DB(ctx).Model(&model.Dept{}).
		Where("id = ?", deptID).Updates(updates)
	if res.Error != nil {
		return errs.Wrap(res.Error, errs.CodeInternal, "failed to update department")
	}
	if res.RowsAffected == 0 {
		return errs.New(errs.CodeNotFound, "department not found")
	}
	return nil
}

// DeptHasChildren reports whether non-deleted child departments exist.
func DeptHasChildren(ctx context.Context, deptID int64) (bool, error) {
	var cnt int64
	err := tenantdb.DB(ctx).Model(&model.Dept{}).
		Where("parent_id = ?", deptID).Count(&cnt).Error
	if err != nil {
		return false, errs.Wrap(err, errs.CodeInternal, "failed to query child departments")
	}
	return cnt > 0, nil
}

// DeptHasUsers reports whether the department has users.
func DeptHasUsers(ctx context.Context, deptID int64) (bool, error) {
	var cnt int64
	err := tenantdb.DB(ctx).Model(&model.User{}).
		Where("dept_id = ?", deptID).Count(&cnt).Error
	if err != nil {
		return false, errs.Wrap(err, errs.CodeInternal, "failed to query department users")
	}
	return cnt > 0, nil
}

// DeleteDept soft-deletes a department (the caller guarantees no child departments and no users).
func DeleteDept(ctx context.Context, deptID int64) error {
	res := tenantdb.DB(ctx).Delete(&model.Dept{}, deptID)
	if res.Error != nil {
		return errs.Wrap(res.Error, errs.CodeInternal, "failed to delete department")
	}
	if res.RowsAffected == 0 {
		return errs.New(errs.CodeNotFound, "department not found")
	}
	return nil
}

// ---- Posts ----

// ListPosts paginates posts (filter by code/name/status).
func ListPosts(ctx context.Context, q types.PostPageQuery) (domain.PageResult[model.Post], error) {
	db := tenantdb.DB(ctx).Model(&model.Post{})
	if q.Code != "" {
		db = db.Where("code = ?", q.Code)
	}
	if q.Name != "" {
		db = db.Where("name LIKE ?", "%"+q.Name+"%")
	}
	if q.Status != nil {
		db = db.Where("status = ?", *q.Status)
	}

	page, err := gormx.FindPage[model.Post](
		db.Order("sort ASC, id ASC"),
		domain.PageParam{PageNum: q.PageNum, PageSize: q.PageSize},
	)
	if err != nil {
		return domain.PageResult[model.Post]{}, errs.Wrap(err, errs.CodeInternal, "failed to query post list")
	}
	return page, nil
}

// GetPostByID fetches a post by ID (within the tenant).
func GetPostByID(ctx context.Context, postID int64) (*model.Post, error) {
	var p model.Post
	err := tenantdb.DB(ctx).Where("id = ?", postID).First(&p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.New(errs.CodeNotFound, "post not found")
		}
		return nil, errs.Wrap(err, errs.CodeInternal, "failed to query post")
	}
	return &p, nil
}

// PostCodeExists reports whether the post code is already taken within the tenant.
func PostCodeExists(ctx context.Context, code string, excludeID int64) (bool, error) {
	db := tenantdb.DB(ctx).Model(&model.Post{}).Where("code = ?", code)
	if excludeID > 0 {
		db = db.Where("id != ?", excludeID)
	}
	var cnt int64
	if err := db.Count(&cnt).Error; err != nil {
		return false, errs.Wrap(err, errs.CodeInternal, "failed to check post code uniqueness")
	}
	return cnt > 0, nil
}

// CreatePost creates a post (primary key is assigned).
func CreatePost(ctx context.Context, p *model.Post) error {
	if err := tenantdb.DB(ctx).Create(p).Error; err != nil {
		return errs.Wrap(err, errs.CodeInternal, "failed to create post")
	}
	return nil
}

// UpdatePost updates post fields.
func UpdatePost(ctx context.Context, postID int64, updates map[string]any) error {
	res := tenantdb.DB(ctx).Model(&model.Post{}).
		Where("id = ?", postID).Updates(updates)
	if res.Error != nil {
		return errs.Wrap(res.Error, errs.CodeInternal, "failed to update post")
	}
	if res.RowsAffected == 0 {
		return errs.New(errs.CodeNotFound, "post not found")
	}
	return nil
}

// PostReferencedByUsers reports whether the post is referenced by users' post_ids (JSON_CONTAINS).
func PostReferencedByUsers(ctx context.Context, postID int64) (bool, error) {
	var cnt int64
	err := tenantdb.DB(ctx).Model(&model.User{}).
		Where("JSON_CONTAINS(post_ids, ?)", postID).Count(&cnt).Error
	if err != nil {
		return false, errs.Wrap(err, errs.CodeInternal, "failed to query post references")
	}
	return cnt > 0, nil
}

// SoftDeletePostTx soft-deletes a post within a transaction: rewrites the code to bypass
// the tenant-scoped unique index (a post with the same name can be recreated).
func SoftDeletePostTx(tx *gorm.DB, postID int64, code string) error {
	res := tx.Model(&model.Post{}).Where("id = ?", postID).Updates(map[string]any{
		"code":       code + "_del_" + types.ID(postID).String(),
		"deleted_at": time.Now(),
	})
	if res.Error != nil {
		return errs.Wrap(res.Error, errs.CodeInternal, "failed to delete post")
	}
	return nil
}

// SoftDeletePost soft-deletes a post (single transaction, see SoftDeletePostTx).
func SoftDeletePost(ctx context.Context, postID int64, code string) error {
	return WithTx(ctx, func(tx *gorm.DB) error {
		return SoftDeletePostTx(tx, postID, code)
	})
}
