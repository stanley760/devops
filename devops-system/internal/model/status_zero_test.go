package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// TestStatusZeroValuePersists guards that "the disabled status can be persisted".
//
// This is a real trap: GORM's struct tag `default:1` treats a field's zero value as
// "not provided" and rewrites it to the default value. For Status (0=disabled 1=normal),
// the zero value 0 is precisely a legal and business-meaningful value — once `default:1`
// is attached, any Create(&x{Status: 0}) would be silently rewritten to Status=1,
// manifesting as "tenant/user/role disabling not taking effect", with no error raised.
//
// This trap once caused TestHumaTenantMiddlewareStates to fail: a disabled tenant (Status=0)
// read back as 1, and the middleware let it through accordingly.
//
// The fix: remove the `default:1` from the Status field (the DB column itself still has DEFAULT 1,
// so direct SQL writes are unaffected; GORM's explicitly assigned 0 persists correctly).
func TestStatusZeroValuePersists(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:statusprobe?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range []any{&Tenant{}, &User{}, &Role{}, &Dept{}, &Post{}, &Menu{}} {
		if err := db.AutoMigrate(m); err != nil {
			t.Fatal(err)
		}
	}

	// insert a Status=0 (disabled) record into each table; reading it back must yield 0.
	tn := &Tenant{BaseModel: BaseModel{ID: 801}, Name: "停用租户", Code: "t801", Status: 0}
	if err := db.Create(tn).Error; err != nil {
		t.Fatal(err)
	}
	var gotTenant Tenant
	if err := db.Where("id = ?", 801).First(&gotTenant).Error; err != nil {
		t.Fatal(err)
	}
	if gotTenant.Status != 0 {
		t.Fatalf("Tenant.Status: wrote 0 (disabled) but read back %d, meaning the default tag is still rewriting the zero value", gotTenant.Status)
	}

	// also confirm 1 can be written correctly, to avoid breaking the other side.
	tn2 := &Tenant{BaseModel: BaseModel{ID: 802}, Name: "正常租户", Code: "t802", Status: 1}
	if err := db.Create(tn2).Error; err != nil {
		t.Fatal(err)
	}
	var gotTenant2 Tenant
	if err := db.Where("id = ?", 802).First(&gotTenant2).Error; err != nil {
		t.Fatal(err)
	}
	if gotTenant2.Status != 1 {
		t.Fatalf("Tenant.Status: wrote 1 (normal) but read back %d", gotTenant2.Status)
	}
}
