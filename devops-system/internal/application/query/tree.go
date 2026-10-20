// Package query is the CQRS read side: query definitions and handlers.
//
// The read side deliberately bypasses aggregates: queries trigger no
// business rules and produce no side effects; they read the read model
// directly and assemble view DTOs. This is the value of CQRS — separating
// the two conflicting goals of "writes must enforce invariants" and "reads
// must be fast and free to JOIN", each in its own optimal form.
//
// Contract boundary: this package's output types are API contract DTOs
// (internal/types), whose JSON shape is reflected into OpenAPI by huma. The
// domain and repository layers know nothing about these DTOs (see the single
// ID contract note in the internal/types package) — the dependency direction
// is query → types, never the reverse.
package query

import (
	"sort"
)

// TreeNode is a generic tree node.
//
// Purpose: eliminate the three verbatim-duplicated tree assembly
// implementations from before the refactor — service/dept.go:143-164
// (department tree), service/menu.go:204-225 (menu tree), and
// service/auth.go:267-293 (frontend router tree), whose predicate
// `if parent, ok := byID[x.ParentID]; ok && x.ParentID != x.ID` was
// identical in all three.
//
// They are now unified into the single BuildTree implementation.
type TreeNode[T any] struct {
	// ID is the node ID (the unique key in the flat list).
	ID int64
	// ParentID is the parent node ID; 0 means root.
	ParentID int64
	// Order is the sort value; when 0, nodes keep their load order stably.
	Order int
	// Value is the node payload (a DTO pointer).
	Value T
	// Children holds the child nodes.
	Children []TreeNode[T]
}

// ptrNode is the intermediate node used during assembly (internal only).
//
// Pointers are required instead of TreeNode values: if nodes were
// value-copied at mount time, what the parent stored would be a snapshot of
// "the moment of mounting", and descendants attached later would not appear
// in that copy (classic symptom: missing grandchildren).
type ptrNode[T any] struct {
	id       int64
	parentID int64
	order    int
	value    T
	children []*ptrNode[T]
}

// sortPtrRoots sorts root nodes: Order ascending, with 0 treated as
// "does not participate in sorting" so the original order is kept.
//
// A stable sort is used: Order 0 means no order was specified, so the
// original load order is preserved, matching the `sort ASC` semantics of the
// three pre-existing tree implementations.
func sortPtrRoots[T any](roots []*ptrNode[T]) {
	if len(roots) < 2 {
		return
	}
	sort.SliceStable(roots, func(i, j int) bool {
		a, b := roots[i].order, roots[j].order
		if a == 0 || b == 0 {
			return false
		}
		return a < b
	})
}

// BuildTree assembles a tree from a flat list.
//
// Semantics (aligned verbatim with the three pre-existing implementations):
//   - If the parent is absent from the list, or the parent is the node
//     itself, the node is placed at the root level;
//   - Root nodes are sorted by Order ascending, and nodes with Order 0 keep
//     their original load order (stable).
func BuildTree[T any](items []T, idOf func(T) int64, parentOf func(T) int64, orderOf func(T) int) []TreeNode[T] {
	byID := make(map[int64]*ptrNode[T], len(items))
	order := make([]int64, 0, len(items))
	for _, it := range items {
		id := idOf(it)
		n := &ptrNode[T]{id: id, parentID: parentOf(it), order: orderOf(it), value: it}
		byID[id] = n
		order = append(order, id)
	}

	roots := make([]*ptrNode[T], 0, len(items))
	for _, id := range order {
		n := byID[id]
		parent, ok := byID[n.parentID]
		if ok && n.parentID != n.id {
			parent.children = append(parent.children, n)
			continue
		}
		roots = append(roots, n)
	}

	sortPtrRoots(roots)

	var walk func(n *ptrNode[T]) TreeNode[T]
	walk = func(n *ptrNode[T]) TreeNode[T] {
		out := TreeNode[T]{
			ID:       n.id,
			ParentID: n.parentID,
			Order:    n.order,
			Value:    n.value,
		}
		if len(n.children) > 0 {
			out.Children = make([]TreeNode[T], 0, len(n.children))
			for _, c := range n.children {
				out.Children = append(out.Children, walk(c))
			}
		}
		return out
	}

	out := make([]TreeNode[T], 0, len(roots))
	for _, r := range roots {
		out = append(out, walk(r))
	}
	return out
}

// sortRoots sorts root nodes: Order ascending, with 0 treated as
// "does not participate in sorting" so the original order is kept.
//
// A stable sort is used instead of insertion sort: the semantics are more
// obvious and the number of root nodes is small, so no hand-written sorting
// algorithm is needed.
func sortRoots[T any](roots []*TreeNode[T]) {
	if len(roots) < 2 {
		return
	}
	sort.SliceStable(roots, func(i, j int) bool {
		a, b := roots[i].Order, roots[j].Order
		if a == 0 || b == 0 {
			return false // keep the original order when either side has no order specified
		}
		return a < b
	})
}

// FlattenTree flattens a tree back into a list (some endpoints need the flat form).
func FlattenTree[T any](nodes []TreeNode[T]) []T {
	out := make([]T, 0, len(nodes))
	var walk func([]TreeNode[T])
	walk = func(ns []TreeNode[T]) {
		for _, n := range ns {
			out = append(out, n.Value)
			walk(n.Children)
		}
	}
	walk(nodes)
	return out
}
