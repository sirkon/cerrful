package tracing

import (
	"go/token"

	"github.com/sirkon/rbtree"

	"github.com/sirkon/cerrful/internal/cir"
)

func NewContext() *Context {
	return &Context{tree: rbtree.New[*contextNodeSpan]()}
}

// Context holds all CIR statements collected for a single analysis scope.
// It serves as a lightweight container for statement nodes before tracing.
type Context struct {
	tree *rbtree.Tree[*contextNodeSpan]
}

type ContextSpan struct {
	start token.Pos
	end   token.Pos
}

// GetByPos exits the most specific (innermost) node covering `pos`.
func (c *Context) GetByPos(pos token.Pos) cir.Node {
	probe := &contextNodeSpan{start: pos, end: pos}
	res := c.tree.Search(probe)
	if res == nil {
		return nil
	}
	return descendSearch(res, pos)
}

// Add registers a node with its [start,end] token span.
// The RB-tree orders only disjoint spans; any overlap is reported back via
// InsertReturn, and we resolve it into a strict containment hierarchy.
// All ordering/balancing is handled by the underlying rbtree.
func (c *Context) Add(node cir.Node, s ContextSpan) {
	span := &contextNodeSpan{start: s.start, end: s.end, node: node}
	attachInto(c.tree, span)
}
