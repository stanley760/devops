// datascope pure-computation unit tests: scope merge matrix, department tree expansion, Apply SQL injection (sqlite in-memory database),
// single-point visibility, and fail-closed.
package datascope_test

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/stanley760/devops/devops-infra/datascope"
)

func TestParseDeptIDs(t *testing.T) {
	cases := []struct {
		name, in string
		want     []int64
	}{
		{"JSON array", "[1,2,3]", []int64{1, 2, 3}},
		{"empty string", "", nil},
		{"invalid JSON", "1,2,3", nil},
		{"non-positive filtered", "[0,-1,5]", []int64{5}},
		{"empty array", "[]", []int64{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := datascope.ParseDeptIDs(c.in)
			if len(got) != len(c.want) {
				t.Fatalf("ParseDeptIDs(%q) = %v, want %v", c.in, got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("ParseDeptIDs(%q) = %v, want %v", c.in, got, c.want)
				}
			}
		})
	}
}

func TestNormalizeDeptIDs(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"numeric array as-is", "[1,2,3]", "[1,2,3]"},
		{"whitespace tolerated", "  [1,2]  ", "[1,2]"},
		{"re-marshalled after non-positive filtering", "[0,-1,5]", "[5]"},
		{"empty string", "", ""},
		{"empty array", "[]", ""},
		{"invalid input", "abc", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := datascope.NormalizeDeptIDs(c.in); got != c.want {
				t.Fatalf("NormalizeDeptIDs(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestExpandSubtree(t *testing.T) {
	// tree: 1 → 2 → 4, 2 → 5, 3 (independent subtree)
	children := datascope.ChildrenMap([]datascope.DeptNode{
		{ID: 2, ParentID: 1}, {ID: 3, ParentID: 0}, {ID: 4, ParentID: 2}, {ID: 5, ParentID: 2},
	})
	got := datascope.ExpandSubtree(children, []int64{1})
	want := []int64{1, 2, 4, 5}
	if len(got) != len(want) {
		t.Fatalf("ExpandSubtree = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ExpandSubtree = %v, want %v", got, want)
		}
	}
	// expansion does not leak into sibling subtrees
	got = datascope.ExpandSubtree(children, []int64{2})
	if len(got) != 3 || got[0] != 2 || got[1] != 4 || got[2] != 5 {
		t.Fatalf("ExpandSubtree(2) = %v, want [2 4 5]", got)
	}
}

func TestMerge(t *testing.T) {
	children := datascope.ChildrenMap([]datascope.DeptNode{
		{ID: 10, ParentID: 0}, {ID: 11, ParentID: 10}, {ID: 20, ParentID: 0},
	})
	cases := []struct {
		name      string
		roles     []datascope.RoleScope
		ownDept   int64
		wantAll   bool
		wantSelf  bool
		wantDepts []int64
	}{
		{"no roles → self only", nil, 10, false, true, nil},
		{"any role all → all", []datascope.RoleScope{
			{DataScope: datascope.ScopeSelf}, {DataScope: datascope.ScopeAll},
		}, 10, true, false, nil},
		{"below overrides self only (subtree expansion)", []datascope.RoleScope{
			{DataScope: datascope.ScopeSelf}, {DataScope: datascope.ScopeDeptBelow},
		}, 10, false, false, []int64{10, 11}},
		{"own department overrides custom", []datascope.RoleScope{
			{DataScope: datascope.ScopeCustom, DataScopeDeptIDs: "[20]"},
			{DataScope: datascope.ScopeDept},
		}, 10, false, false, []int64{10}},
		{"custom overrides self only", []datascope.RoleScope{
			{DataScope: datascope.ScopeSelf},
			{DataScope: datascope.ScopeCustom, DataScopeDeptIDs: "[20]"},
		}, 10, false, false, []int64{20}},
		{"multiple custom roles take union (ordered)", []datascope.RoleScope{
			{DataScope: datascope.ScopeCustom, DataScopeDeptIDs: "[20]"},
			{DataScope: datascope.ScopeCustom, DataScopeDeptIDs: "[11,10]"},
		}, 10, false, false, []int64{10, 11, 20}},
		{"own department without department → no rows visible", []datascope.RoleScope{{DataScope: datascope.ScopeDept}}, 0, false, false, nil},
		{"below without department → no rows visible", []datascope.RoleScope{{DataScope: datascope.ScopeDeptBelow}}, 0, false, false, nil},
		{"custom but department set invalid → no rows visible", []datascope.RoleScope{{DataScope: datascope.ScopeCustom, DataScopeDeptIDs: "abc"}}, 10, false, false, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := datascope.Merge(c.roles, c.ownDept, 77, children)
			if got.All != c.wantAll {
				t.Fatalf("All = %v, want %v", got.All, c.wantAll)
			}
			if got.SelfOnly != c.wantSelf {
				t.Fatalf("SelfOnly = %v, want %v", got.SelfOnly, c.wantSelf)
			}
			if c.wantSelf && got.SelfID != 77 {
				t.Fatalf("SelfID = %d, want 77", got.SelfID)
			}
			if len(got.DeptIDs) != len(c.wantDepts) {
				t.Fatalf("DeptIDs = %v, want %v", got.DeptIDs, c.wantDepts)
			}
			for i := range c.wantDepts {
				if got.DeptIDs[i] != c.wantDepts[i] {
					t.Fatalf("DeptIDs = %v, want %v", got.DeptIDs, c.wantDepts)
				}
			}
		})
	}
}

func TestVisible(t *testing.T) {
	cases := []struct {
		name          string
		scope         datascope.Scope
		deptID, rowID int64
		want          bool
	}{
		{"all always visible", datascope.Scope{All: true}, 999, 888, true},
		{"self only matches self", datascope.Scope{SelfOnly: true, SelfID: 7}, 10, 7, true},
		{"self only cannot see others", datascope.Scope{SelfOnly: true, SelfID: 7}, 10, 8, false},
		{"visible within department set", datascope.Scope{DeptIDs: []int64{10, 11}}, 11, 99, true},
		{"invisible outside department set", datascope.Scope{DeptIDs: []int64{10, 11}}, 20, 99, false},
		{"empty set always invisible", datascope.Scope{}, 10, 7, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.scope.Visible(c.deptID, c.rowID); got != c.want {
				t.Fatalf("Visible(%d, %d) = %v, want %v", c.deptID, c.rowID, got, c.want)
			}
		})
	}
}

// applyDB builds a sqlite in-memory database (bare table users: id/dept_id, bypassing model dependencies to focus on SQL behavior).
func applyDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:datascope_test?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrator().DropTable("users"); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, dept_id INTEGER)").Error; err != nil {
		t.Fatal(err)
	}
	for i, dept := range map[int64]int64{1: 10, 2: 11, 3: 20, 4: 20, 5: 30} {
		if err := db.Exec("INSERT INTO users (id, dept_id) VALUES (?, ?)", i, dept).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func countIDs(t *testing.T, db *gorm.DB) []int64 {
	t.Helper()
	var ids []int64
	if err := db.Table("users").Order("id").Pluck("id", &ids).Error; err != nil {
		t.Fatal(err)
	}
	return ids
}

func TestApply(t *testing.T) {
	cols := datascope.Columns{Dept: "dept_id", Owner: "id"}
	cases := []struct {
		name  string
		scope datascope.Scope
		want  []int64
	}{
		{"all → no filtering", datascope.Scope{All: true}, []int64{1, 2, 3, 4, 5}},
		{"self only → owner match", datascope.Scope{SelfOnly: true, SelfID: 3}, []int64{3}},
		{"department set → IN filter", datascope.Scope{DeptIDs: []int64{10, 11}}, []int64{1, 2}},
		{"empty set → no rows visible", datascope.Scope{}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			db := applyDB(t)
			ctx := datascope.WithScope(context.Background(), c.scope)
			got := countIDs(t, datascope.Apply(ctx, db, cols))
			if len(got) != len(c.want) {
				t.Fatalf("rows = %v, want %v", got, c.want)
			}
			for i := range c.want {
				if got[i] != c.want[i] {
					t.Fatalf("rows = %v, want %v", got, c.want)
				}
			}
		})
	}
}

func TestApplyFailClosedWithoutScope(t *testing.T) {
	db := applyDB(t)
	// ctx carries no data permission: the query must fail (rather than degrade to fully visible)
	var ids []int64
	err := datascope.Apply(context.Background(), db, datascope.Columns{Dept: "dept_id", Owner: "id"}).
		Table("users").Pluck("id", &ids).Error
	if err == nil {
		t.Fatalf("query should error when data scope context is missing, got rows %v", ids)
	}
}
