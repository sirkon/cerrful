package tracing

import (
	"go/token"

	"github.com/sirkon/rbtree"

	"github.com/sirkon/cerrful/internal/cir"
)

// contextNodeSpan stores a [start,end] span for a CIR node and, if needed,
// a nested RB-tree for child spans fully contained in this span.
type contextNodeSpan struct {
	start token.Pos
	end   token.Pos

	node     cir.Node
	children *rbtree.Tree[*contextNodeSpan]
}

// Cmp defines ordering for the RB-tree as "disjoint by position".
// - return -1 if this span is strictly before other (ends before other's start)
// - return  1 if this span is strictly after  other (starts after other's end)
// - return  0 if spans overlap in any way (including containment).
//
// NOTE: We rely on an *invariant of the input*: any two overlapping spans must
// be in a strict containment relationship (no partial overlaps). Under this
// invariant, "equal" (0) means either superspan/subspan. The RB-tree then gives
// us a handle (`InsertReturn`) to the overlapping node so we can perform the
// containment-structure fix-up ourselves.
func (n *contextNodeSpan) Cmp(other *contextNodeSpan) int {
	if n.end < other.start { // strictly before
		return -1
	}
	if n.start > other.end { // strictly after
		return 1
	}
	return 0 // overlapping (containment or equal boundaries)
}

func contains(a, b *contextNodeSpan) bool {
	return a.start <= b.start && a.end >= b.end
}

// attachInto inserts span s into RB-tree t, using the following containment rules:
//   - If t has no overlapping node, s is inserted as a sibling in t.
//   - If an overlapping node r exists and s contains r, mutate r in-place to become s
//     (so the pointer already present in the tree now represents s), and then re-attach
//     the old r as a child of the new s. This avoids needing a "Replace" operation.
//   - If r contains s, recursively attach s into r.children.
//
// Under the no-partial-overlap invariant, these are the only cases we must handle.
func attachInto(t *rbtree.Tree[*contextNodeSpan], s *contextNodeSpan) {
	r := t.InsertReturn(s)
	if r == s {
		// Disjoint: brand new top-level entry.
		return
	}

	// Overlap found. Decide by containment.
	if contains(s, r) {
		// s — superspan, overwrite r in-place.
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
		*s = *r

		attachInto(s.children, &n)
		return
	}

	// If we arrive here, it's a partial-overlap situation which violates our model assumptions.
	// For robustness in debug builds one might panic; in production we choose to treat as sibling.
	// However, keeping it explicit helps catch data issues during development.
	panic("attachInto: partial-overlap spans are not supported")
}

func descendSearch(n *contextNodeSpan, pos token.Pos) cir.Node {
	if n == nil {
		return nil
	}
	if n.children == nil {
		return n.node
	}
	probe := &contextNodeSpan{start: pos, end: pos}
	child := n.children.Search(probe)
	if child == nil {
		return n.node
	}
	if v := descendSearch(child, pos); v != nil {
		return v
	}
	return n.node
}
