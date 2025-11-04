package tracing

import (
	"go/token"
	"testing"

	"github.com/sirkon/cerrful/internal/cir"
)

func TestContextSpanDepthPattern_ASCII(t *testing.T) {
	ctx := NewContext()

	varn := func(name string) *cir.ExprVar {
		return &cir.ExprVar{
			Name: name,
		}
	}

	if ctx.GetByPos(0) != nil {
		t.Fatal("nothing was expected at pos 0 right now")
	}

	ctx.Add(varn("ground"), 0, 200)

	res := ctx.GetByPos(10)
	exprVar := res.(*cir.ExprVar)
	if exprVar.Name != "ground" {
		t.Fatal("ground was expected at pos 10")
	}

	ctx.Add(varn("mid1"), 10, 90)
	ctx.Add(varn("mid11"), 20, 30)
	ctx.Add(varn("mid12"), 40, 80)
	ctx.Add(varn("mid13"), 85, 88)
	ctx.Add(varn("mid2"), 110, 190)
	ctx.Add(varn("mid21"), 120, 130)

	type test struct {
		name  string
		pos   token.Pos
		isnil bool
	}
	testingFunc := func(tt test) func(t *testing.T) {
		return func(t *testing.T) {
			node := ctx.GetByPos(tt.pos)
			if node == nil && !tt.isnil {
				t.Fatalf("node %q was not found at position %d", tt.name, tt.pos)
			}
			if node != nil && tt.isnil {
				t.Fatalf("no node was expected at position %d, got %q", tt.pos, node.(*cir.ExprVar).Name)
			}
			if node == nil && tt.isnil {
				t.Logf("no node was found at %d as was expected", tt.pos)
			}
			if node != nil {
				x := node.(*cir.ExprVar)
				if x.Name != tt.name {
					t.Fatalf("node %q was expected, got %q at position %d", tt.name, x.Name, tt.pos)
				}
				t.Logf("expected node %q found at %d", tt.name, tt.pos)
			}
		}
	}

	tests := []test{
		{
			name:  "ground",
			pos:   0,
			isnil: false,
		},
		{
			name:  "ground",
			pos:   5,
			isnil: false,
		},
		{
			name:  "ground",
			pos:   200,
			isnil: false,
		},
		{
			name:  "mid1",
			pos:   90,
			isnil: false,
		},
		{
			name:  "mid11",
			pos:   25,
			isnil: false,
		},
		{
			name:  "mid12",
			pos:   41,
			isnil: false,
		},
		{
			name:  "mid12",
			pos:   79,
			isnil: false,
		},
		{
			name:  "mid13",
			pos:   86,
			isnil: false,
		},
		{
			name:  "ground",
			pos:   100,
			isnil: false,
		},
		{
			name:  "mid2",
			pos:   115,
			isnil: false,
		},
		{
			name:  "mid21",
			pos:   125,
			isnil: false,
		},
		{
			name:  "on-the-left",
			pos:   -1,
			isnil: true,
		},
		{
			name:  "on-the-right",
			pos:   201,
			isnil: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, testingFunc(tt))
	}

	ctx.Add(varn("underground"), -10, 300)
	tests = []test{
		{
			name:  "underground",
			pos:   -5,
			isnil: false,
		},
		{
			name:  "underground",
			pos:   250,
			isnil: false,
		},
		{
			name:  "ground",
			pos:   2,
			isnil: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, testingFunc(tt))
	}
}
