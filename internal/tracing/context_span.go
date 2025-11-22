package tracing

import (
	"fmt"
	"go/token"

	"github.com/sirkon/rbtree"

	"github.com/sirkon/cerrful/internal/cir"
)

// contextNodeSpan stores a [start,end] span for one or more CIR nodes and,
// if needed, a nested RB-tree for child spans fully contained in this span.
type contextNodeSpan struct {
	start token.Pos
	end   token.Pos

	nodes    []cir.Node
	children *rbtree.Tree[*contextNodeSpan]
}

// Cmp defines ordering for the RB-tree as "disjoint by position".
// - return -1 if this span is strictly before other (ends before other.start)
// - return +1 if this span is strictly after other (starts after other.end)
// - return 0 otherwise (some form of overlap/containment).
//
// This comparator, combined with the attachInto logic, guarantees we never
// store partially-overlapping spans in the tree: only disjoint or containment.
func (a *contextNodeSpan) Cmp(b *contextNodeSpan) int {
	switch {
	case a.end < b.start:
		return -1
	case a.start > b.end:
		return 1
	}
	return 0 // overlapping (containment or equal boundaries)
}

func (a *contextNodeSpan) Span() ContextSpan {
	return ContextSpan{
		start: a.start,
		end:   a.end,
	}
}

func contains(a, b *contextNodeSpan) bool {
	return a.start <= b.start && a.end >= b.end
}

// attachInto inserts span s into RB-tree t, using the following containment rules:
//   - If t has no overlapping node, s is inserted as a sibling in t.
//   - If an overlapping node r exists and spans are equal, we simply append
//     s.nodes to r.nodes.
//   - If an overlapping node r exists and s contains r, mutate r in-place to
//     become s (so the pointer already present in the tree now represents s),
//     and then re-attach the old r as a child of the new s.
//   - If r contains s, recursively attach s into r.children.
//
// Under the no-partial-overlap invariant, these are the only cases we must handle.
func attachInto(t *rbtree.Tree[*contextNodeSpan], s *contextNodeSpan) {
	r := t.InsertReturn(s)
	if r == s {
		// Disjoint: brand new top-level entry.
		return
	}

	// Overlap or equal span found.
	// If spans are exactly equal, just append nodes to the existing span.
	if r.start == s.start && r.end == s.end {
		r.nodes = append(r.nodes, s.nodes...)
		return
	}

	// Proper containment: decide by superspan.
	if contains(s, r) {
		// s — superspan, overwrite r in-place but keep the old node as a child.
		old := *r
		*r = *s

		if r.children == nil {
			r.children = rbtree.New[*contextNodeSpan]()
		}
		attachInto(r.children, &old)
		return
	}

	if contains(r, s) {
		// New span is a subspan of the existing node `r` — descend.
		if r.children == nil {
			r.children = rbtree.New[*contextNodeSpan]()
		}

		n := *s
		attachInto(r.children, &n)
		return
	}

	// If we arrive here, it's a partial-overlap situation which violates our model assumptions.
	// For robustness in debug builds we panic explicitly to surface bad span construction.
	panic(fmt.Errorf("attachInto: detected partial overlap spans new(%s) vs old(%s)", s.Span(), r.Span()))
}

func descendSearch(n *contextNodeSpan, pos token.Pos) []cir.Node {
	if n == nil {
		return nil
	}
	if n.children == nil {
		return n.nodes
	}

	probe := &contextNodeSpan{start: pos, end: pos}
	child := n.children.Search(probe)
	if child != nil {
		// спускаемся рекурсивно — если нашли хотя бы один узел → он самый глубокий
		deeper := descendSearch(child, pos)
		if len(deeper) != 0 {
			return deeper
		}
	}

	// иначе остаёмся на текущем span
	return n.nodes
}
