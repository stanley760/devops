package dbinit

import (
	"fmt"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// testDB gives each test case an independent in-memory DB (sqlite dialect: tableExists takes the sqlite_master branch).
func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := strings.NewReplacer("/", "_", "\\", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	return db
}

// mapFS builds the script file system (path relative to sql/ -> content).
// Tests use sqlite-compatible SQL and do not execute the production MySQL scripts (those are only validated for naming/structure).
func mapFS(files map[string]string) fstest.MapFS {
	fs := fstest.MapFS{}
	for name, content := range files {
		fs["sql/"+name] = &fstest.MapFile{Data: []byte(content)}
	}
	return fs
}

func TestRunCreatesTablesAndSeeds(t *testing.T) {
	db := testDB(t)
	fs := mapFS(map[string]string{
		"schema/t_demo.sql": "CREATE TABLE IF NOT EXISTS t_demo (id INTEGER PRIMARY KEY, v TEXT);",
		"datas/t_demo.sql":  "INSERT INTO t_demo (id, v) VALUES (1, 'a');",
	})
	if err := runFS(db, fs); err != nil {
		t.Fatalf("first run: %v", err)
	}
	var cnt int64
	if err := db.Raw("SELECT COUNT(*) FROM t_demo").Scan(&cnt).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if cnt != 1 {
		t.Fatalf("rows = %d, want 1", cnt)
	}
}

func TestRunSecondRunSkipsEverything(t *testing.T) {
	db := testDB(t)
	fs := mapFS(map[string]string{
		"schema/t_demo.sql": "CREATE TABLE IF NOT EXISTS t_demo (id INTEGER PRIMARY KEY, v TEXT);",
		"datas/t_demo.sql":  "INSERT INTO t_demo (id, v) VALUES (1, 'a');",
	})
	if err := runFS(db, fs); err != nil {
		t.Fatalf("first run: %v", err)
	}
	// second run: the table already exists -> table creation and seeds are all skipped, no duplicate data
	if err := runFS(db, fs); err != nil {
		t.Fatalf("second run: %v", err)
	}
	var cnt int64
	if err := db.Raw("SELECT COUNT(*) FROM t_demo").Scan(&cnt).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if cnt != 1 {
		t.Fatalf("re-run rows = %d, want 1 (seed should be skipped when the table exists)", cnt)
	}
}

func TestRunDataSkippedWhenTablePreExists(t *testing.T) {
	db := testDB(t)
	// t_a exists before dbinit (pre-existing table); t_b is created by this run
	if err := db.Exec("CREATE TABLE t_a (id INTEGER PRIMARY KEY)").Error; err != nil {
		t.Fatalf("pre-create t_a: %v", err)
	}
	fs := mapFS(map[string]string{
		"schema/t_a.sql": "CREATE TABLE IF NOT EXISTS t_a (id INTEGER PRIMARY KEY);",
		"datas/t_a.sql":  "INSERT INTO t_a (id) VALUES (1);",
		"schema/t_b.sql": "CREATE TABLE IF NOT EXISTS t_b (id INTEGER PRIMARY KEY);",
		"datas/t_b.sql":  "INSERT INTO t_b (id) VALUES (1);",
	})
	if err := runFS(db, fs); err != nil {
		t.Fatalf("run: %v", err)
	}
	var cnt int64
	if err := db.Raw("SELECT COUNT(*) FROM t_a").Scan(&cnt).Error; err != nil {
		t.Fatalf("count t_a: %v", err)
	}
	if cnt != 0 {
		t.Fatalf("t_a rows = %d, want 0 (table not created by this run, seed should be skipped)", cnt)
	}
	if err := db.Raw("SELECT COUNT(*) FROM t_b").Scan(&cnt).Error; err != nil {
		t.Fatalf("count t_b: %v", err)
	}
	if cnt != 1 {
		t.Fatalf("t_b rows = %d, want 1 (table created by this run, seed should be executed)", cnt)
	}
}

func TestRunDDLFailureFailsClosed(t *testing.T) {
	// syntax error in a single CREATE TABLE statement (unclosed parenthesis): fail-closed at execution time
	fs := mapFS(map[string]string{
		"schema/t_demo.sql": "CREATE TABLE IF NOT EXISTS t_demo (id INTEGER PRIMARY KEY, v TEXT",
	})
	if err := runFS(testDB(t), fs); err == nil {
		t.Fatal("a malformed CREATE TABLE statement should error on execution")
	}
}

func TestRunRejectsMultiStmtDDL(t *testing.T) {
	fs := mapFS(map[string]string{
		"schema/t_demo.sql": "CREATE TABLE IF NOT EXISTS t_demo (id INTEGER PRIMARY KEY); INSERT INTO t_demo (id) VALUES (1);",
	})
	err := runFS(testDB(t), fs)
	if err == nil || !strings.Contains(err.Error(), "exactly 1 CREATE TABLE statement") {
		t.Fatalf("a multi-statement table creation script should error, got: %v", err)
	}
}

func TestRunRejectsStemContentMismatch(t *testing.T) {
	fs := mapFS(map[string]string{
		"schema/t_x.sql": "CREATE TABLE IF NOT EXISTS t_y (id INTEGER PRIMARY KEY);",
	})
	err := runFS(testDB(t), fs)
	if err == nil || !strings.Contains(err.Error(), "does not match the file name stem") {
		t.Fatalf("a table name mismatching the file name should error, got: %v", err)
	}
}

func TestRunRejectsDataWithoutDdl(t *testing.T) {
	fs := mapFS(map[string]string{
		"schema/t_a.sql": "CREATE TABLE IF NOT EXISTS t_a (id INTEGER PRIMARY KEY);",
		"datas/t_z.sql":  "INSERT INTO t_z (id) VALUES (1);",
	})
	err := runFS(testDB(t), fs)
	if err == nil || !strings.Contains(err.Error(), "no corresponding table creation script") {
		t.Fatalf("a seed without a corresponding table creation script should error, got: %v", err)
	}
}

func TestRunRejectsBadFilename(t *testing.T) {
	for _, name := range []string{"schema/demo.sql", "schema/0001_x.sql"} {
		fs := mapFS(map[string]string{
			name: "CREATE TABLE IF NOT EXISTS t_demo (id INTEGER PRIMARY KEY);",
		})
		err := runFS(testDB(t), fs)
		if err == nil || !strings.Contains(err.Error(), "is invalid") {
			t.Fatalf("%s should error on invalid naming, got: %v", name, err)
		}
	}
}

func TestRunEmptyFSOK(t *testing.T) {
	// no scripts: an empty FS is valid (succeeds with zero actions)
	if err := runFS(testDB(t), fstest.MapFS{}); err != nil {
		t.Fatalf("empty FS: %v", err)
	}
}

func TestEmbeddedSQLWellFormed(t *testing.T) {
	// structural contract of the real embedded scripts (no DB connection): naming/stem consistency,
	// every datas entry has a corresponding ddl, and schema covers all 10 business tables.
	ddls, err := loadScripts(scriptFS, "sql/schema", true)
	if err != nil {
		t.Fatalf("load ddl: %v", err)
	}
	datas, err := loadScripts(scriptFS, "sql/datas", false)
	if err != nil {
		t.Fatalf("load data: %v", err)
	}
	want := map[string]bool{
		"t_tenant":               true,
		"t_system_users":         true,
		"t_system_role":          true,
		"t_system_menu":          true,
		"t_system_dept":          true,
		"t_system_post":          true,
		"t_system_user_role":     true,
		"t_system_role_menu":     true,
		"t_system_login_log":     true,
		"t_system_operation_log": true,
		// Added in the DDD/CQRS refactor: domain event table (carries pending projection facts, no seed data).
		"t_system_domain_event": true,
		// Dictionary module: dictionary types + dictionary items.
		"t_system_dict_type": true,
		"t_system_dict_data": true,
		// System configuration module.
		"t_system_config": true,
	}
	got := make(map[string]bool, len(ddls))
	for _, s := range ddls {
		got[s.Table] = true
	}
	if len(got) != len(ddls) {
		t.Fatalf("duplicate table names in table creation scripts: %v", ddls)
	}
	for tbl := range want {
		if !got[tbl] {
			t.Errorf("missing table creation script sql/schema/%s.sql", tbl)
		}
	}
	for tbl := range got {
		if !want[tbl] {
			t.Errorf("extra table creation script %s", tbl)
		}
	}
	ddlSet := got
	for _, s := range datas {
		if !ddlSet[s.Table] {
			t.Errorf("seed %s has no corresponding table creation script", s.Name)
		}
	}
}
