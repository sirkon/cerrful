package tracing

import (
	"go/token"
	"strings"
	"testing"

	"github.com/sirkon/cerrful/internal/cir"
)

func TestContextSpanDepthPattern(t *testing.T) {
	ctx := NewContext()

	varn := func(name string) *cir.ExprVar {
		return &cir.ExprVar{
			Name: name,
		}
	}

	span := func(a, b token.Pos) ContextSpan {
		return ContextSpan{
			start: a,
			end:   b,
		}
	}

	if len(ctx.GetByPos(0)) != 0 {
		t.Fatal("nothing was expected at pos 0 right now")
	}

	ctx.Add(varn("ground"), span(0, 200))

	nodes := ctx.GetByPos(10)
	if len(nodes) == 0 {
		t.Fatal("ground was expected at pos 10")
	}
	exprVar := nodes[0].(*cir.ExprVar)
	if exprVar.Name != "ground" {
		t.Fatal("ground was expected at pos 10")
	}

	ctx.Add(varn("underground"), span(-10, 300))
	ctx.Add(varn("mid1"), span(10, 90))
	ctx.Add(varn("mid11"), span(20, 30))
	ctx.Add(varn("mid12"), span(85, 88))
	ctx.Add(varn("mid12'"), span(85, 88))
	ctx.Add(varn("mid2"), span(110, 190))
	ctx.Add(varn("antarctic"), span(1000, 5000))

	type test struct {
		name  string
		pos   token.Pos
		nodes []string
	}
	tests := []test{
		{
			name: "out",
			pos:  -20,
		},
		{
			name:  "underground",
			pos:   -5,
			nodes: []string{"underground"},
		},
		{
			name:  "underground",
			pos:   250,
			nodes: []string{"underground"},
		},
		{
			name:  "ground",
			pos:   0,
			nodes: []string{"ground"},
		},
		{
			name:  "mid1",
			pos:   15,
			nodes: []string{"mid1"},
		},
		{
			name:  "mid11",
			pos:   25,
			nodes: []string{"mid11"},
		},
		{
			name:  "mid12s",
			pos:   86,
			nodes: []string{"mid12", "mid12'"},
		},
		{
			name:  "mid2",
			pos:   150,
			nodes: []string{"mid2"},
		},
		{
			name:  "antarctic",
			pos:   2000,
			nodes: []string{"antarctic"},
		},
		{
			name: "out",
			pos:  100000,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodes := ctx.GetByPos(tt.pos)
			var textNodes []string
			for _, node := range nodes {
				textNodes = append(textNodes, node.(*cir.ExprVar).Name)
			}
			if len(nodes) != len(tt.nodes) {
				t.Fatalf(
					"expected %s got %s",
					strings.Join(tt.nodes, "."),
					strings.Join(textNodes, "."),
				)
			}
		})
	}
}
