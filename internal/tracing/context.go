package tracing

import (
	"fmt"
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

func (s ContextSpan) String() string {
	return fmt.Sprintf("[%d, %d]", s.start, s.end)
}

// Add registers a node with its [start,end] token span.
// The RB-tree orders only disjoint spans; any overlap is reported back via
// InsertReturn, and we resolve it into a strict containment hierarchy.
// All ordering/balancing is handled by the underlying rbtree.
func (c *Context) Add(node cir.Node, s ContextSpan) {
	span := &contextNodeSpan{start: s.start, end: s.end, nodes: []cir.Node{node}}
	attachInto(c.tree, span)
}

// GetByPos exits the most specific (innermost) span covering `pos` and returns
// all CIR nodes attached to that span (in the order they were added).
func (c *Context) GetByPos(pos token.Pos) []cir.Node {
	probe := &contextNodeSpan{start: pos, end: pos}
	res := c.tree.Search(probe)
	if res == nil {
		return nil
	}
	return descendSearch(res, pos)
}

func contextShouldGet[T any](ctx *Context, pos token.Pos) (res T, rpt panicReporter) {
	nodes := ctx.GetByPos(pos)
	if len(nodes) == 0 {
		return res, func(fset *token.FileSet) error {
			return fmt.Errorf("%s CIR node defintion expected but not found", fset.Position(pos))
		}
	}

	for _, node := range nodes {
		if v, ok := node.(T); ok {
			return v, nil
		}
	}

	return res, func(fset *token.FileSet) error {
		var got any
		if len(nodes) > 0 {
			got = nodes[0]
		}
		return fmt.Errorf("%s expected %T CIR node found %T", fset.Position(pos), res, got)
	}
}

type panicReporter func(fset *token.FileSet) error
