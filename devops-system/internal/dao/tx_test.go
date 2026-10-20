// Behavioral contract of the transaction-body dao functions (sqlite in-memory DB):
//   - replace semantics of SyncRoleMenusTx / SyncUserRolesTx (clear old + insert new; empty set clears)
//   - unique-key rewrite of SoftDeleteRoleTx / SoftDeletePostTx (same code can be recreated after soft delete)
package dao_test

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/stanley760/devops/devops-system/internal/dao"
	"github.com/stanley760/devops/devops-system/internal/model"
)

// txDB creates a sqlite in-memory DB and migrates the tables needed by this test group.
func txDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:dao_tx_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Role{}, &model.Post{}, &model.UserRole{}, &model.RoleMenu{}); err != nil {
		t.Fatalf("failed to create tables: %v", err)
	}
	return db
}

func menuIDsOf(t *testing.T, db *gorm.DB, roleID int64) []int64 {
	t.Helper()
	var ids []int64
	if err := db.Model(&model.RoleMenu{}).Where("role_id = ?", roleID).Pluck("menu_id", &ids).Error; err != nil {
		t.Fatalf("failed to query role_menu: %v", err)
	}
	return ids
}

// TestSyncRoleMenusReplace: first insert -> swap set (old ones all deleted) -> empty set (cleared).
func TestSyncRoleMenusReplace(t *testing.T) {
	db := txDB(t)

	if err := dao.SyncRoleMenusTx(db, 1, []int64{10, 11}); err != nil {
		t.Fatalf("first write failed: %v", err)
	}
	if got := menuIDsOf(t, db, 1); len(got) != 2 {
		t.Fatalf("menu count after first write = %d, want 2", len(got))
	}

	if err := dao.SyncRoleMenusTx(db, 1, []int64{20}); err != nil {
		t.Fatalf("replace failed: %v", err)
	}
	got := menuIDsOf(t, db, 1)
	if len(got) != 1 || got[0] != 20 {
		t.Fatalf("menus after replace = %v, want [20]", got)
	}

	if err := dao.SyncRoleMenusTx(db, 1, nil); err != nil {
		t.Fatalf("clear failed: %v", err)
	}
	if got := menuIDsOf(t, db, 1); len(got) != 0 {
		t.Fatalf("menus after empty-set replace = %v, want empty", got)
	}

	// associations of other roles are unaffected
	if err := dao.SyncRoleMenusTx(db, 2, []int64{30}); err != nil {
		t.Fatalf("write for role 2 failed: %v", err)
	}
	if got := menuIDsOf(t, db, 2); len(got) != 1 || got[0] != 30 {
		t.Fatalf("role 2 menus = %v, want [30]", got)
	}
}

// TestSyncUserRolesReplace: same replace semantics for user-role associations.
func TestSyncUserRolesReplace(t *testing.T) {
	db := txDB(t)

	if err := dao.SyncUserRolesTx(db, 7, []int64{1, 2}); err != nil {
		t.Fatalf("first write failed: %v", err)
	}
	if err := dao.SyncUserRolesTx(db, 7, []int64{3}); err != nil {
		t.Fatalf("replace failed: %v", err)
	}
	var ids []int64
	if err := db.Model(&model.UserRole{}).Where("user_id = ?", 7).Pluck("role_id", &ids).Error; err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(ids) != 1 || ids[0] != 3 {
		t.Fatalf("roles after replace = %v, want [3]", ids)
	}

	if err := dao.SyncUserRolesTx(db, 7, nil); err != nil {
		t.Fatalf("clear failed: %v", err)
	}
	var cnt int64
	if err := db.Model(&model.UserRole{}).Where("user_id = ?", 7).Count(&cnt).Error; err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if cnt != 0 {
		t.Fatalf("association count after empty-set replace = %d, want 0", cnt)
	}
}

// TestSoftDeleteUniqueRewrite: soft delete rewrites the unique key; the same code can be created again
// after soft delete (no conflict with the tenant-scoped unique index).
func TestSoftDeleteUniqueRewrite(t *testing.T) {
	db := txDB(t)

	role := &model.Role{Name: "运维", Code: "ops"}
	role.ID = 42
	if err := db.Create(role).Error; err != nil {
		t.Fatalf("failed to create role: %v", err)
	}
	if err := dao.SoftDeleteRoleTx(db, role.ID, role.Code); err != nil {
		t.Fatalf("failed to soft-delete role: %v", err)
	}
	var code string
	if err := db.Unscoped().Model(&model.Role{}).Where("id = ?", role.ID).Pluck("code", &code).Error; err != nil {
		t.Fatalf("failed to query soft-deleted role: %v", err)
	}
	if want := "ops_del_42"; code != want {
		t.Fatalf("code after soft delete = %q, want %q", code, want)
	}
	// not visible under the default scope (deleted_at is set)
	var alive int64
	if err := db.Model(&model.Role{}).Where("id = ?", role.ID).Count(&alive).Error; err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if alive != 0 {
		t.Fatalf("still %d row(s) visible under default scope after soft delete", alive)
	}

	post := &model.Post{Code: "ceo", Name: "CEO"}
	post.ID = 99
	if err := db.Create(post).Error; err != nil {
		t.Fatalf("failed to create post: %v", err)
	}
	if err := dao.SoftDeletePostTx(db, post.ID, post.Code); err != nil {
		t.Fatalf("failed to soft-delete post: %v", err)
	}
	var pcode string
	if err := db.Unscoped().Model(&model.Post{}).Where("id = ?", post.ID).Pluck("code", &pcode).Error; err != nil {
		t.Fatalf("failed to query soft-deleted post: %v", err)
	}
	if want := "ceo_del_99"; pcode != want {
		t.Fatalf("code after soft delete = %q, want %q", pcode, want)
	}
}

// WithTx goes through tenantdb.DB(ctx), which the main path depends on a real data source;
// here we only verify that the transaction rollback semantics work.
func TestWithTxRollback(t *testing.T) {
	db := txDB(t)
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := dao.SyncRoleMenusTx(tx, 5, []int64{1}); err != nil {
			return err
		}
		return context.Canceled // simulate transaction interruption
	})
	if err == nil {
		t.Fatal("expected the transaction to return an error")
	}
	if got := menuIDsOf(t, db, 5); len(got) != 0 {
		t.Fatalf("%d leftover row(s) after rollback, want 0", len(got))
	}
}
