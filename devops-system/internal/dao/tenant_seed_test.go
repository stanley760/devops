// Behavioral contract of SeedTenantAdmin's menu bootstrap (sqlite in-memory DB):
//   - the new tenant's super admin is granted the full global menu catalog minus the
//     platform-only subtree (component "platform/" and its button descendants)
//   - the grants land under the new tenant's tenant_id (TenantPlugin fill on create)
package dao_test

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/stanley760/devops/devops-infra/database"
	"github.com/stanley760/devops/devops-infra/tenant"
	"github.com/stanley760/devops/devops-system/internal/dao"
	"github.com/stanley760/devops/devops-system/internal/model"
)

// seedDB creates a sqlite in-memory DB with the tenant plugin registered and migrates
// the tables used by the seed flow.
func seedDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Role{}, &model.User{}, &model.UserRole{}, &model.RoleMenu{}, &model.Menu{}); err != nil {
		t.Fatalf("failed to create tables: %v", err)
	}
	if err := tenant.RegisterPlugin(db); err != nil {
		t.Fatalf("failed to register tenant plugin: %v", err)
	}
	return db
}

func TestSeedTenantAdminGrantsMenus(t *testing.T) {
	db := seedDB(t)

	// global menu catalog: a normal directory/menu/button plus a platform subtree
	// (mirrors the seed layout: menu 7 租户管理 with component "platform/" and its buttons)
	fixture := []model.Menu{
		{BaseModel: model.BaseModel{ID: 1}, Name: "系统管理", Type: 1, Status: 1},
		{BaseModel: model.BaseModel{ID: 2}, Name: "用户管理", Type: 2, ParentID: 1, Component: "system/user/index", Status: 1},
		{BaseModel: model.BaseModel{ID: 22}, Name: "用户新增", Type: 3, ParentID: 2, Permission: "system:user:create", Status: 1},
		{BaseModel: model.BaseModel{ID: 7}, Name: "租户管理", Type: 2, ParentID: 1, Component: "platform/tenant/index", Status: 1},
		{BaseModel: model.BaseModel{ID: 72}, Name: "租户新增", Type: 3, ParentID: 7, Permission: "platform:tenant:create", Status: 1},
	}
	if err := db.Create(&fixture).Error; err != nil {
		t.Fatalf("failed to seed menus: %v", err)
	}

	role := &model.Role{
		BaseTenantModel: model.BaseTenantModel{BaseModel: model.BaseModel{ID: 1}},
		Name:            "超级管理员", Code: "super_admin", Type: 1, DataScope: 1, Status: 1,
	}
	admin := &model.User{
		BaseTenantModel: model.BaseTenantModel{BaseModel: model.BaseModel{ID: 1}},
		Username:        "admin", Password: "x", Nickname: "admin", Status: 1,
	}

	ctx := tenant.With(context.Background(), 42, "t42")
	if err := database.Transaction(ctx, db, func(ctx context.Context) error {
		return dao.SeedTenantAdmin(ctx, role, admin)
	}); err != nil {
		t.Fatalf("SeedTenantAdmin failed: %v", err)
	}

	// grants: the full catalog minus the platform subtree, all under the new tenant's tenant_id
	tctx := tenant.With(context.Background(), 42, "t42")
	var rows []model.RoleMenu
	if err := db.WithContext(tctx).Where("role_id = ?", role.ID).Find(&rows).Error; err != nil {
		t.Fatalf("failed to query role_menu: %v", err)
	}
	got := make(map[int64]bool, len(rows))
	for _, r := range rows {
		if r.TenantID != 42 {
			t.Fatalf("role_menu tenant_id = %d, want 42", r.TenantID)
		}
		got[r.MenuID] = true
	}
	if !got[1] || !got[2] || !got[22] {
		t.Fatalf("normal menus not granted: %v", got)
	}
	if got[7] || got[72] {
		t.Fatalf("platform menus must not be granted: %v", got)
	}

	// the user-role binding is tenant-scoped too
	var n int64
	if err := db.WithContext(tctx).Model(&model.UserRole{}).
		Where("user_id = ? AND role_id = ?", admin.ID, role.ID).Count(&n).Error; err != nil {
		t.Fatalf("failed to query user_role: %v", err)
	}
	if n != 1 {
		t.Fatalf("user_role count = %d, want 1", n)
	}
}
