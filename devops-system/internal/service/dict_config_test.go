// Dictionary / system configuration service business rules (sqlite in-memory DB):
//   - the dictionary type code and the configuration key are unique within a tenant;
//   - a dictionary item must belong to an existing dictionary type, and its value is unique under that type;
//   - a dictionary type that still has items cannot be deleted;
//   - a built-in configuration (type=1) cannot be deleted, but a custom one can.
package service_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	commonmodel "github.com/stanley760/devops/devops-common/model"
	"github.com/stanley760/devops/devops-infra/database"
	"github.com/stanley760/devops/devops-system/internal/auth"
	"github.com/stanley760/devops/devops-system/internal/model"
	"github.com/stanley760/devops/devops-system/internal/service"
	"github.com/stanley760/devops/devops-system/internal/types"
)

// seqAssigner is a test ID assigner: hands out sequential IDs (the real one is the Snowflake client).
type seqAssigner struct{ next int64 }

func (a *seqAssigner) Assign(_ *gorm.DB, dest *int64) error {
	a.next++
	*dest = a.next
	return nil
}

// dictSvcDB creates a sqlite in-memory DB and migrates the tables needed by this test group.
// The DB name is derived from the test name: the shared in-memory cache would otherwise
// let one test's rows leak into the next.
func dictSvcDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := strings.NewReplacer("/", "_", "\\", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name)), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.DictType{}, &model.DictData{}, &model.Config{}); err != nil {
		t.Fatalf("failed to create tables: %v", err)
	}
	return db
}

// runInDB executes fn with the sqlite DB injected into the context as the current executor
// (the dao layer obtains its connection from the context, so no real data source is needed).
func runInDB(t *testing.T, db *gorm.DB, fn func(ctx context.Context) error) {
	t.Helper()
	if err := database.Transaction(context.Background(), db, func(ctx context.Context) error {
		return fn(ctx)
	}); err != nil {
		t.Fatalf("transaction failed: %v", err)
	}
}

func testClaims() *auth.AccessClaims {
	return &auth.AccessClaims{UserID: 1, Username: "admin", TenantID: 1}
}

// TestDictTypeCodeUnique: the dictionary type code is unique within a tenant.
func TestDictTypeCodeUnique(t *testing.T) {
	db := dictSvcDB(t)
	commonmodel.Configure(&seqAssigner{}, nil)
	svc := service.NewDict()

	runInDB(t, db, func(ctx context.Context) error {
		if err := svc.CreateType(ctx, testClaims(), types.DictTypeSaveReq{Name: "系统状态", Type: "sys_status", Status: 1}); err != nil {
			return err
		}
		// a second type with the same code is rejected
		if err := svc.CreateType(ctx, testClaims(), types.DictTypeSaveReq{Name: "重复", Type: "sys_status", Status: 1}); err == nil {
			t.Fatal("a duplicate dictionary type code should be rejected")
		}
		return nil
	})
}

// TestDictDataRequiresTypeAndUniqueValue: a dictionary item must belong to an existing
// dictionary type, and its value is unique under that type.
func TestDictDataRequiresTypeAndUniqueValue(t *testing.T) {
	db := dictSvcDB(t)
	commonmodel.Configure(&seqAssigner{}, nil)
	svc := service.NewDict()

	runInDB(t, db, func(ctx context.Context) error {
		// unknown dictionary type → rejected
		if err := svc.CreateData(ctx, testClaims(), types.DictDataSaveReq{DictType: "nope", Label: "正常", Value: "1", Status: 1}); err == nil {
			t.Fatal("a dictionary item under an unknown type should be rejected")
		}
		if err := svc.CreateType(ctx, testClaims(), types.DictTypeSaveReq{Name: "系统状态", Type: "sys_status", Status: 1}); err != nil {
			return err
		}
		if err := svc.CreateData(ctx, testClaims(), types.DictDataSaveReq{DictType: "sys_status", Label: "正常", Value: "1", Status: 1}); err != nil {
			return err
		}
		// the same value under the same type → rejected
		if err := svc.CreateData(ctx, testClaims(), types.DictDataSaveReq{DictType: "sys_status", Label: "正常", Value: "1", Status: 1}); err == nil {
			t.Fatal("a duplicate dictionary value under the same type should be rejected")
		}
		// the same value under another type → allowed
		if err := svc.CreateType(ctx, testClaims(), types.DictTypeSaveReq{Name: "性别", Type: "sys_sex", Status: 1}); err != nil {
			return err
		}
		if err := svc.CreateData(ctx, testClaims(), types.DictDataSaveReq{DictType: "sys_sex", Label: "正常", Value: "1", Status: 1}); err != nil {
			return err
		}
		return nil
	})
}

// TestDictTypeWithItemsCannotBeDeleted: a dictionary type that still has items cannot be deleted.
func TestDictTypeWithItemsCannotBeDeleted(t *testing.T) {
	db := dictSvcDB(t)
	commonmodel.Configure(&seqAssigner{}, nil)
	svc := service.NewDict()

	runInDB(t, db, func(ctx context.Context) error {
		if err := svc.CreateType(ctx, testClaims(), types.DictTypeSaveReq{Name: "系统状态", Type: "sys_status", Status: 1}); err != nil {
			return err
		}
		if err := svc.CreateData(ctx, testClaims(), types.DictDataSaveReq{DictType: "sys_status", Label: "正常", Value: "1", Status: 1}); err != nil {
			return err
		}
		page, err := svc.ListTypes(ctx, types.DictTypePageQuery{})
		if err != nil {
			return err
		}
		if len(page.List) != 1 {
			t.Fatalf("dictionary type count = %d, want 1", len(page.List))
		}
		id := page.List[0].ID
		if err := svc.DeleteType(ctx, id); err == nil {
			t.Fatal("a dictionary type that still has items should not be deletable")
		}
		// after removing the item, the type becomes deletable
		dataPage, err := svc.ListData(ctx, types.DictDataPageQuery{DictType: "sys_status"})
		if err != nil {
			return err
		}
		if len(dataPage.List) != 1 {
			t.Fatalf("dictionary item count = %d, want 1", len(dataPage.List))
		}
		if err := svc.DeleteData(ctx, dataPage.List[0].ID); err != nil {
			return err
		}
		if err := svc.DeleteType(ctx, id); err != nil {
			return err
		}
		return nil
	})
}

// TestConfigKeyUniqueAndBuiltinProtection: the configuration key is unique within a tenant;
// a built-in item (type=1) cannot be deleted, a custom one (type=2) can.
func TestConfigKeyUniqueAndBuiltinProtection(t *testing.T) {
	db := dictSvcDB(t)
	commonmodel.Configure(&seqAssigner{}, nil)
	svc := service.NewConfig()

	runInDB(t, db, func(ctx context.Context) error {
		if err := svc.Create(ctx, testClaims(), types.ConfigSaveReq{ConfigKey: "sys.ui.title", ConfigName: "控制台标题", ConfigValue: "DevOps", Type: 1}); err != nil {
			return err
		}
		// a duplicate key is rejected
		if err := svc.Create(ctx, testClaims(), types.ConfigSaveReq{ConfigKey: "sys.ui.title", ConfigName: "重复", Type: 2}); err == nil {
			t.Fatal("a duplicate configuration key should be rejected")
		}
		// a built-in item cannot be deleted
		page, err := svc.List(ctx, types.ConfigPageQuery{})
		if err != nil {
			return err
		}
		if len(page.List) != 1 {
			t.Fatalf("configuration count = %d, want 1", len(page.List))
		}
		builtin := page.List[0]
		if err := svc.Delete(ctx, builtin.ID); err == nil {
			t.Fatal("a built-in configuration should not be deletable")
		}
		// a custom item can be deleted
		if err := svc.Create(ctx, testClaims(), types.ConfigSaveReq{ConfigKey: "sys.custom", ConfigName: "自定义项", Type: 2}); err != nil {
			return err
		}
		page, err = svc.List(ctx, types.ConfigPageQuery{})
		if err != nil {
			return err
		}
		var custom types.ConfigDTO
		for _, c := range page.List {
			if c.ConfigKey == "sys.custom" {
				custom = c
			}
		}
		if custom.ID == 0 {
			t.Fatal("the custom configuration was not created")
		}
		if err := svc.Delete(ctx, custom.ID); err != nil {
			return err
		}
		return nil
	})
}
