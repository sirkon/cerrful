package tracing

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"

	"github.com/sirkon/cerrful/internal/cir"
)

// === SCRAP STMT ============================================================

func (e *ScrapEngine) scrapStmt(ctx *Context, pass *analysis.Pass, stmt ast.Stmt) {
	switch s := stmt.(type) {

	case *ast.AssignStmt:
		e.scrapAssign(ctx, pass, s)

	case *ast.ReturnStmt:
		e.scrapReturn(ctx, pass, s)

	case *ast.ExprStmt:
		e.scrapExpr(ctx, pass, s.X)

	case *ast.IfStmt:
		e.scrapStmt(ctx, pass, s.Init)
		e.scrapExpr(ctx, pass, s.Cond)
		for _, st := range s.Body.List {
			e.scrapStmt(ctx, pass, st)
		}
		if s.Else != nil {
			// TODO: support else-block
		}

	case *ast.ForStmt:
		if s.Cond != nil {
			_ = e.scrapExpr(ctx, pass, s.Cond)
		}
		for _, st := range s.Body.List {
			e.scrapStmt(ctx, pass, st)
		}

	case *ast.BlockStmt:
		for _, st := range s.List {
			e.scrapStmt(ctx, pass, st)
		}

	case *ast.GoStmt:
		e.scrapExpr(ctx, pass, s.Call)

	case *ast.DeferStmt:
		e.scrapExpr(ctx, pass, s.Call)
	}
}

func (e *ScrapEngine) scrapAssign(ctx *Context, pass *analysis.Pass, a *ast.AssignStmt) {
	lhs := make([]cir.Expr, 0, len(a.Lhs))
	for _, l := range a.Lhs {
		lhs = append(lhs, e.scrapExpr(ctx, pass, l))
	}

	rhs := make([]cir.Expr, 0, len(a.Rhs))
	for _, r := range a.Rhs {
		rhs = append(rhs, e.scrapExpr(ctx, pass, r))
	}

	node := &cir.Assign{LHS: lhs, RHS: rhs}
	ctx.Add(node, spanAST(a))

	// CER061: NoIgnoredErrors (basic check)
	e.checkCER061(ctx, pass, a, lhs, rhs)
}

func (e *ScrapEngine) scrapReturn(ctx *Context, pass *analysis.Pass, r *ast.ReturnStmt) {
	if len(r.Results) == 0 {
		node := &cir.Return{Val: nil}
		ctx.Add(node, spanAST(r))
		return
	}
	last := r.Results[len(r.Results)-1]
	val := e.scrapExpr(ctx, pass, last)
	node := &cir.Return{Val: val}
	ctx.Add(node, spanAST(r))
}

// === SCRAP EXPR ============================================================

func (e *ScrapEngine) scrapExpr(ctx *Context, pass *analysis.Pass, expr ast.Expr) cir.Expr {
	if expr == nil {
		return nil
	}

	switch x := expr.(type) {
	case *ast.Ident:
		return e.scrapIdent(ctx, pass, x)

	case *ast.BasicLit:
		return e.scrapBasicLit(ctx, pass, x)

	case *ast.ParenExpr:
		return e.scrapExpr(ctx, pass, x.X)

	case *ast.CallExpr:
		return e.scrapCall(ctx, pass, x)

	case *ast.SelectorExpr:
		return e.scrapSelector(ctx, pass, x)

	case *ast.TypeAssertExpr:
		return e.scrapTypeAssert(ctx, pass, x)

	case *ast.UnaryExpr:
		return e.scrapUnary(ctx, pass, x)

	case *ast.BinaryExpr:
		return e.scrapBinary(ctx, pass, x)

	case *ast.IndexExpr:
		return e.scrapIndex(ctx, pass, x)

	case *ast.SliceExpr:
		return e.scrapSlice(ctx, pass, x)

	case *ast.CompositeLit:
		return e.scrapCompositeLit(ctx, pass, x)

	default:
		return nil
	}
}

func (e *ScrapEngine) scrapIdent(ctx *Context, pass *analysis.Pass, id *ast.Ident) cir.Expr {
	node := &cir.ExprVar{Name: id.Name}
	ctx.SetByPos(id.Pos(), node)
	return node
}

func (e *ScrapEngine) scrapBasicLit(ctx *Context, pass *analysis.Pass, lit *ast.BasicLit) cir.Expr {
	node := &cir.ExprLiteral{Value: lit.Value}
	ctx.SetByPos(lit.Pos(), node)
	return node
}

func (e *ScrapEngine) scrapSelector(ctx *Context, pass *analysis.Pass, sel *ast.SelectorExpr) cir.Expr {
	x := e.scrapExpr(ctx, pass, sel.X)
	node := &cir.ExprSelector{X: x, Sel: sel.Sel.Name}
	ctx.SetByPos(sel.Pos(), node)
	return node
}

func (e *ScrapEngine) scrapUnary(ctx *Context, pass *analysis.Pass, u *ast.UnaryExpr) cir.Expr {
	inner := e.scrapExpr(ctx, pass, u.X)
	node := &cir.ExprUnary{Op: u.Op, Expr: inner}
	ctx.SetByPos(u.Pos(), node)
	return node
}

func (e *ScrapEngine) scrapBinary(ctx *Context, pass *analysis.Pass, b *ast.BinaryExpr) cir.Expr {
	left := e.scrapExpr(ctx, pass, b.X)
	right := e.scrapExpr(ctx, pass, b.Y)
	node := &cir.ExprBinary{Op: b.Op, Left: left, Right: right}
	ctx.SetByPos(b.Pos(), node)
	return node
}

func (e *ScrapEngine) scrapTypeAssert(ctx *Context, pass *analysis.Pass, ta *ast.TypeAssertExpr) cir.Expr {
	recv := e.scrapExpr(ctx, pass, ta.X)
	t := pass.TypesInfo.TypeOf(ta.Type)
	node := &cir.ExprTypeAssert{Value: recv, Type: t}
	ctx.SetByPos(ta.Pos(), node)
	return node
}

func (e *ScrapEngine) scrapCall(ctx *Context, pass *analysis.Pass, call *ast.CallExpr) cir.Expr {
	fun := e.scrapExpr(ctx, pass, call.Fun)
	args := make([]cir.Expr, 0, len(call.Args))
	for _, a := range call.Args {
		args = append(args, e.scrapExpr(ctx, pass, a))
	}

	t := pass.TypesInfo.TypeOf(call.Fun)

	if e.isWrapCall(pass, call.Fun, t) {
		inner := e.findInnerErrorArg(args)
		node := &cir.ExprWrap{Inner: inner, Args: args}
		ctx.SetByPos(call.Pos(), node)
		return node
	}

	if e.isLogCall(pass, call.Fun, t) {
		node := &cir.ExprLog{Args: args}
		ctx.SetByPos(call.Pos(), node)
		return node
	}

	node := &cir.ExprCall{Fun: fun, Args: args}
	ctx.SetByPos(call.Pos(), node)
	return node
}

func (e *ScrapEngine) scrapIndex(ctx *Context, pass *analysis.Pass, idx *ast.IndexExpr) cir.Expr {
	x := e.scrapExpr(ctx, pass, idx.X)
	i := e.scrapExpr(ctx, pass, idx.Index)
	node := &cir.ExprIndex{X: x, Index: i}
	ctx.SetByPos(idx.Pos(), node)
	return node
}

func (e *ScrapEngine) scrapSlice(ctx *Context, pass *analysis.Pass, s *ast.SliceExpr) cir.Expr {
	x := e.scrapExpr(ctx, pass, s.X)
	lo := e.scrapExpr(ctx, pass, s.Low)
	hi := e.scrapExpr(ctx, pass, s.High)
	max := e.scrapExpr(ctx, pass, s.Max)
	node := &cir.ExprSlice{X: x, Low: lo, High: hi, Max: max}
	ctx.SetByPos(s.Pos(), node)
	return node
}

func (e *ScrapEngine) scrapCompositeLit(ctx *Context, pass *analysis.Pass, cl *ast.CompositeLit) cir.Expr {
	elems := make([]cir.Expr, 0, len(cl.Elts))
	for _, e2 := range cl.Elts {
		elems = append(elems, e.scrapExpr(ctx, pass, e2))
	}
	node := &cir.ExprComposite{Elems: elems}
	ctx.SetByPos(cl.Pos(), node)
	return node
}
