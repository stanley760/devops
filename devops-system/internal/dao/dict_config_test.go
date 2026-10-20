// Dictionary/configuration dao behavioral contract (sqlite in-memory DB):
//   - unique-key rewrite on soft delete (the same dictionary type code / config key
//     can be recreated after the soft delete, bypassing the tenant-scoped unique index);
//   - soft delete hides the row from the default scope but keeps it in the unscoped scope.
package dao_test

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/stanley760/devops/devops-system/internal/dao"
	"github.com/stanley760/devops/devops-system/internal/model"
)

// dictConfigDB creates a sqlite in-memory DB and migrates the tables needed by this test group.
func dictConfigDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:dao_dict_config_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.DictType{}, &model.DictData{}, &model.Config{}); err != nil {
		t.Fatalf("failed to create tables: %v", err)
	}
	return db
}

// TestSoftDeleteDictTypeRewritesCode: the dictionary type code is rewritten on soft delete,
// so the same code can be created again.
func TestSoftDeleteDictTypeRewritesCode(t *testing.T) {
	db := dictConfigDB(t)

	dt := &model.DictType{Name: "系统状态", Type: "sys_status", Status: 1}
	dt.ID = 1001
	dt.TenantID = 1
	if err := db.Create(dt).Error; err != nil {
		t.Fatalf("failed to create dictionary type: %v", err)
	}
	if err := dao.SoftDeleteDictTypeTx(db, dt.ID, dt.Type); err != nil {
		t.Fatalf("failed to soft-delete dictionary type: %v", err)
	}

	var code string
	if err := db.Unscoped().Model(&model.DictType{}).Where("id = ?", dt.ID).Pluck("type", &code).Error; err != nil {
		t.Fatalf("failed to query soft-deleted dictionary type: %v", err)
	}
	if want := "sys_status_del_1001"; code != want {
		t.Fatalf("type code after soft delete = %q, want %q", code, want)
	}

	// not visible under the default scope
	var alive int64
	if err := db.Model(&model.DictType{}).Where("id = ?", dt.ID).Count(&alive).Error; err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if alive != 0 {
		t.Fatalf("%d row(s) visible under default scope after soft delete, want 0", alive)
	}

	// the original code is free again
	again := &model.DictType{Name: "系统状态", Type: "sys_status", Status: 1}
	again.ID = 1002
	again.TenantID = 1
	if err := db.Create(again).Error; err != nil {
		t.Fatalf("recreating the same dictionary type code failed: %v", err)
	}
}

// TestSoftDeleteConfigRewritesKey: the configuration key is rewritten on soft delete,
// so the same key can be created again.
func TestSoftDeleteConfigRewritesKey(t *testing.T) {
	db := dictConfigDB(t)

	c := &model.Config{ConfigKey: "sys.ui.title", ConfigName: "控制台标题", ConfigValue: "DevOps", Type: 1}
	c.ID = 2001
	c.TenantID = 1
	if err := db.Create(c).Error; err != nil {
		t.Fatalf("failed to create configuration: %v", err)
	}
	if err := dao.SoftDeleteConfigTx(db, c.ID, c.ConfigKey); err != nil {
		t.Fatalf("failed to soft-delete configuration: %v", err)
	}

	var key string
	if err := db.Unscoped().Model(&model.Config{}).Where("id = ?", c.ID).Pluck("config_key", &key).Error; err != nil {
		t.Fatalf("failed to query soft-deleted configuration: %v", err)
	}
	if want := "sys.ui.title_del_2001"; key != want {
		t.Fatalf("config key after soft delete = %q, want %q", key, want)
	}

	var alive int64
	if err := db.Model(&model.Config{}).Where("id = ?", c.ID).Count(&alive).Error; err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if alive != 0 {
		t.Fatalf("%d row(s) visible under default scope after soft delete, want 0", alive)
	}

	again := &model.Config{ConfigKey: "sys.ui.title", ConfigName: "控制台标题", ConfigValue: "DevOps", Type: 2}
	again.ID = 2002
	again.TenantID = 1
	if err := db.Create(again).Error; err != nil {
		t.Fatalf("recreating the same configuration key failed: %v", err)
	}
}
