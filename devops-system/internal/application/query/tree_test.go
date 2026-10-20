package query

import (
	"testing"

	"github.com/stanley760/devops/devops-common/domain"
)

// node is the payload used for tree tests.
type node struct {
	ID       int64
	ParentID int64
	Order    int
	Name     string
}

func TestBuildTree(t *testing.T) {
	items := []node{
		{ID: 1, ParentID: 0, Order: 2, Name: "root-b"},
		{ID: 2, ParentID: 0, Order: 1, Name: "root-a"},
		{ID: 3, ParentID: 1, Name: "child-of-1"},
		{ID: 4, ParentID: 3, Name: "grandchild"},
		// parent does not exist → should land at the root level
		{ID: 5, ParentID: 99, Name: "orphan"},
	}

	tree := BuildTree(items,
		func(n node) int64 { return n.ID },
		func(n node) int64 { return n.ParentID },
		func(n node) int { return n.Order },
	)

	if len(tree) != 3 {
		t.Fatalf("root count = %d, want 3 (1,2,5)", len(tree))
	}
	// roots sorted by Order ascending: 2 (order=1) should come before 1 (order=2)
	if tree[0].ID != 2 || tree[1].ID != 1 {
		t.Fatalf("root order = [%d %d], want [2 1]", tree[0].ID, tree[1].ID)
	}

	// subtree mounting correctness
	var rootB *TreeNode[node]
	for i := range tree {
		if tree[i].ID == 1 {
			rootB = &tree[i]
		}
	}
	if rootB == nil {
		t.Fatal("root node with ID=1 not found")
	}
	if len(rootB.Children) != 1 || rootB.Children[0].ID != 3 {
		t.Fatalf("children of ID=1 = %v, want [3]", idsOf(rootB.Children))
	}
	if len(rootB.Children[0].Children) != 1 || rootB.Children[0].Children[0].ID != 4 {
		t.Fatalf("children of ID=3 = %v, want [4]", idsOf(rootB.Children[0].Children))
	}
}

// TestBuildTreeSelfParentGuard checks that a self-referencing node lands at the root instead of looping forever.
func TestBuildTreeSelfParentGuard(t *testing.T) {
	items := []node{{ID: 7, ParentID: 7, Name: "self"}}
	tree := BuildTree(items,
		func(n node) int64 { return n.ID },
		func(n node) int64 { return n.ParentID },
		func(n node) int { return n.Order },
	)
	if len(tree) != 1 || tree[0].ID != 7 {
		t.Fatalf("self-referencing node should land at the root, got %d roots", len(tree))
	}
}

func TestFlattenTree(t *testing.T) {
	items := []node{
		{ID: 1, ParentID: 0, Name: "a"},
		{ID: 2, ParentID: 1, Name: "b"},
		{ID: 3, ParentID: 2, Name: "c"},
	}
	tree := BuildTree(items,
		func(n node) int64 { return n.ID },
		func(n node) int64 { return n.ParentID },
		func(n node) int { return n.Order },
	)
	flat := FlattenTree(tree)
	if len(flat) != 3 {
		t.Fatalf("flattened count = %d, want 3", len(flat))
	}
	if flat[0].ID != 1 || flat[1].ID != 2 || flat[2].ID != 3 {
		t.Fatalf("flattened order = [%d %d %d], want [1 2 3]", flat[0].ID, flat[1].ID, flat[2].ID)
	}
}

// TestBuildTreePreservesOrderZero checks that roots with Order=0 keep their load order.
func TestBuildTreePreservesOrderZero(t *testing.T) {
	items := []node{
		{ID: 10, ParentID: 0, Name: "first"},
		{ID: 20, ParentID: 0, Order: 5, Name: "second"},
	}
	tree := BuildTree(items,
		func(n node) int64 { return n.ID },
		func(n node) int64 { return n.ParentID },
		func(n node) int { return n.Order },
	)
	if tree[0].ID != 10 {
		t.Fatalf("first root = %d, want 10 (Order=0 should keep the original order)", tree[0].ID)
	}
}

func idsOf[T any](nodes []TreeNode[T]) []int64 {
	out := make([]int64, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, n.ID)
	}
	return out
}

// TestPageResultMapping checks that pagination metadata is preserved through conversion.
func TestPageResultMapping(t *testing.T) {
	in := domain.PageResult[int]{
		List: []int{1, 2, 3}, Total: 42, PageNum: 2, PageSize: 3, Pages: 14,
	}
	out := domain.MapPage(in, func(v int) string { return string(rune('a' + v - 1)) })
	if out.Total != in.Total || out.PageNum != in.PageNum || out.PageSize != in.PageSize || out.Pages != in.Pages {
		t.Fatalf("pagination metadata not preserved: %+v", out)
	}
	if len(out.List) != 3 || out.List[0] != "a" {
		t.Fatalf("list conversion error: %v", out.List)
	}
}
