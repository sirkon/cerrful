package tracing

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/sirkon/cerrful/internal/cerrules"
	"github.com/sirkon/cerrful/internal/cir"
)

// ScrapEngine holds configured known wrappers, loggers, and constructors.
type ScrapEngine struct {
	news          map[Reference]NewSpec
	wraps         map[Reference]WrapSpec
	loggers       map[Reference]LoggerSpec
	ignoredErrors map[Reference]IgnoredError
	collectors    map[Reference]CollectorSpec

	r    *ReporterPhase
	fset *token.FileSet
}

func NewScrapEngine(r *ReporterPhase, fset *token.FileSet) *ScrapEngine {
	return &ScrapEngine{
		news:          make(map[Reference]NewSpec),
		wraps:         make(map[Reference]WrapSpec),
		loggers:       make(map[Reference]LoggerSpec),
		ignoredErrors: make(map[Reference]IgnoredError),
		collectors:    make(map[Reference]CollectorSpec),
		r:             r,
		fset:          fset,
	}
}

// --- Config-related -------------------------------------------------------------------------------------------------

// RegisterWrap registers a wrap function.
func (e *ScrapEngine) RegisterWrap(ref Reference, kind WrapKind) {
	e.wraps[ref] = WrapSpec{Ref: ref, Kind: kind}
}

// RegisterLogger registers a logger function.
func (e *ScrapEngine) RegisterLogger(ref Reference, kind LoggingKind) {
	e.loggers[ref] = LoggerSpec{Ref: ref, Kind: kind}
}

// RegisterNew registers an error-constructor function.
func (e *ScrapEngine) RegisterNew(ref Reference) {
	e.news[ref] = NewSpec{Ref: ref}
}

// RegisterIgnoreError registers an error type to be ignored.
func (e *ScrapEngine) RegisterIgnoreError(ref Reference) {
	e.ignoredErrors[ref] = IgnoredError{Ref: ref}
}

func (e *ScrapEngine) RegisterCollector(ref Reference) {
	e.collectors[ref] = CollectorSpec{Ref: ref}
}

// --- Actual logic ---------------------------------------------------------------------------------------------------

// Scrap traverses the file AST and records structural information
// about errors and their usage into the given context.
func (e *ScrapEngine) Scrap(
	ctx *Context,
	pass *analysis.Pass,
	file *ast.File,
) {
	e.scrapExprs(ctx, pass, file)
	e.scrapAssignsAndReturns(ctx, pass, file, nil)
	e.inspectSwitches(ctx, pass, file)
}

func (e *ScrapEngine) scrapExprs(ctx *Context, pass *analysis.Pass, file *ast.File) {
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {

		// --------------------------
		// 1. Call expressions
		// --------------------------
		case *ast.CallExpr:
			e.scrapCall(ctx, pass, node)
			return true

		// --------------------------
		// 2. Ident — alias to error variable
		// --------------------------
		case *ast.Ident:
			t := pass.TypesInfo.TypeOf(node)
			if t == nil || !isErrorType(t) {
				return true
			}

			ctx.Add(
				&cir.ExprAlias{Target: node.Name},
				spanAST(node),
			)
			return true

		// --------------------------
		// 3. SelectorExpr — only sentinel
		// --------------------------
		case *ast.SelectorExpr:
			t := pass.TypesInfo.TypeOf(node)
			if t == nil || !isErrorType(t) {
				return true
			}

			// Check X is package
			xIdent, ok := node.X.(*ast.Ident)
			if !ok {
				return true
			}

			obj := pass.TypesInfo.ObjectOf(xIdent)
			pkgName, ok := obj.(*types.PkgName)
			if !ok {
				return true // not pkg.Err
			}

			// This is exactly pkg.ErrXXX
			ctx.Add(
				&cir.ExprSentinel{
					Ref: cir.Reference{
						Package: pkgName.Imported().Path(),
						Name:    node.Sel.Name,
					},
				},
				spanAST(node),
			)
			return true
		}

		return true
	})
}

func isErrorType(t types.Type) bool {
	return types.Identical(
		t, types.Universe.Lookup("error").Type(),
	)
}

func (e *ScrapEngine) scrapAssignsAndReturns(ctx *Context, pass *analysis.Pass, src ast.Node, returns []*ast.Field) {
	ast.Inspect(src, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncDecl:
			e.errorMustBeLastInspection(pass, node.Type.Results)

			namedReturns := getNamedReturnValuesWithError(pass, node.Type.Results)
			if namedReturns == nil {
				return true
			}

			e.scrapAssignsAndReturns(ctx, pass, node.Body, namedReturns)
			return false

		case *ast.FuncLit:
			e.errorMustBeLastInspection(pass, node.Type.Results)

			namedReturns := getNamedReturnValuesWithError(pass, node.Type.Results)
			if namedReturns == nil {
				return true
			}

			e.scrapAssignsAndReturns(ctx, pass, node.Body, namedReturns)
			return false

		case *ast.ReturnStmt:
			switch {
			case len(node.Results) == 0 && len(returns) == 0:
				return true
			case len(node.Results) == 0 && len(returns) != 0:
				names := returns[len(returns)-1].Names
				last := names[len(names)-1]
				ctx.Add(
					&cir.Return{
						Val: &cir.ExprVar{
							Name: last.Name,
						},
					},
					spanAST(node),
				)
			case len(node.Results) != 0:
				expr := node.Results[len(node.Results)-1]
				t := pass.TypesInfo.TypeOf(expr)
				if t == nil || !isErrorType(t) {
					return true
				}

				unpar := stripParens(expr)
				v, errRpt := contextShouldGet[cir.Expr](ctx, unpar.Pos())
				if errRpt != nil {
					panic(errRpt(e.fset))
				}

				ctx.Add(
					&cir.Return{
						Val: v,
					},
					spanAST(node),
				)
			}

		case *ast.AssignStmt:
		default:
			return true
		}
	})
}

func (e *ScrapEngine) inspectSwitches(ctx *Context, pass *analysis.Pass, file *ast.File) {

}

func (e *ScrapEngine) scrapCallX(
	ctx *Context,
	pass *analysis.Pass,
	call *ast.CallExpr,
) {
	fn := e.resolveFn(pass, call)
	if fn == nil {
		return
	}

	ref := resolveFuncRef(fn)
	if ref == nil {
		return
	}

	span := spanAST(call)

	// error not last in returns
	if sig := fn.Sig; sig != nil {
		res := sig.Results()
		if res != nil && res.Len() > 1 {
			last := res.Len() - 1
			for i := 0; i < last; i++ {
				if types.Identical(
					res.At(i).Type(),
					types.Universe.Lookup("error").Type(),
				) {
					e.r.Report(cerrules.ErrorMustBeLastReturnValue(), "", span.start)
					ctx.Add(
						&cir.ExprCall{
							HasArgs: len(call.Args) > 0,
							Ref:     ref.CIR(),
						},
						span,
					)
					break
				}
			}
		}
	}

	// wrap
	if ws, ok := e.wraps[*ref]; ok {
		var src string
		var msg string

		switch ws.Kind {
		case WrapKindFmt:
			var isFmtNew bool
			src, msg, isFmtNew = e.scrapFmtDetails(pass, call, span)
			if isFmtNew {
				ctx.Add(
					&cir.ExprNew{
						Ref: ref.CIR(),
					},
					span,
				)
				return
			}

		case WrapKindErrors:
			if v, ok := call.Args[0].(*ast.Ident); ok {
				src = v.Name
			} else {
				e.r.Report(cerrules.FixBeforeUse(), "", span.start)
			}

			msgLit := extractStringLit(call.Args[1])
			if msgLit != nil {
				msg, _ = strconv.Unquote(msgLit.Value)
			}

		default:
			panic(fmt.Errorf("missing handling for wrap kind %s", ws.Kind))
		}
		ctx.Add(
			&cir.ExprWrap{
				Var: &cir.ExprVar{
					Name: src,
				},
				Msg: msg,
				Ref: ref.CIR(),
			},
			span,
		)
		return
	}

	// logger
	if ls, ok := e.loggers[*ref]; ok {
		var loggedErrors []*ast.Ident
		ast.Inspect(call, func(n ast.Node) bool {
			expr, ok := n.(ast.Expr)
			if !ok {
				return true
			}
			if types.Identical(
				pass.TypesInfo.TypeOf(expr),
				types.Universe.Lookup("error").Type(),
			) {
				id, ok := expr.(*ast.Ident)
				if !ok {
					e.r.Report(cerrules.FixBeforeUse(), "", expr.Pos())
					return true
				}

				loggedErrors = append(loggedErrors, id)
			}

			return true
		})

		for _, logged := range loggedErrors {
			ctx.Add(
				&cir.Log{
					Var:   cir.ExprVar{Name: logged.Name},
					Level: ls.Level.CIR(),
					Msg:   "", // Do we really need this?
					Ref:   ls.Ref.CIR(),
				},
				spanAST(logged),
			)
		}

		return
	}

	// new (constructor) — with fmt-style “is actually wrap” discrimination
	if ns, ok := e.news[*ref]; ok {
		ctx.Add(
			&cir.ExprNew{
				Ref: ns.Ref.CIR(),
			},
			span,
		)
		return
	}

	// unknown call
	ctx.Add(
		&cir.ExprCall{
			HasArgs: len(call.Args) > 0,
			Ref:     ref.CIR(),
		},
		span,
	)
}

func (e *ScrapEngine) scrapAssignX(
	ctx *Context,
	pass *analysis.Pass,
	as *ast.AssignStmt,
) {
	errorType := types.Universe.Lookup("error").Type()

	// TODO добавить логику
}

func (e *ScrapEngine) scrapReturnX(
	ctx *Context,
	pass *analysis.Pass,
	ret *ast.ReturnStmt,
) {
	// Here we can:
	// - detect propagation of ignored errors
	// - tag return-states for the tracer
}

func (e *ScrapEngine) scrapIf(
	ctx *Context,
	pass *analysis.Pass,
	stmt *ast.IfStmt,
) {
	// Будем анализировать:
	// - err != nil
	// - err == nil
	// - вызовы логгеров / wrap внутри веток
	// - объявления err в init:  if err := f(); err != nil { ... }
	// - branching-on-errors (для трассера: CER0XX)

	// Пока просто оставляем точку входа
}

func (e *ScrapEngine) scrapSwitch(
	ctx *Context,
	pass *analysis.Pass,
	stmt *ast.SwitchStmt,
) {
	// Интересует:
	// - switch err { case ... }       → branching over error
	// - switch x.(type)               → неактуально для ошибок, но может быть логгер
	// - presence of logger/wrap/new inside cases
}

func (e *ScrapEngine) scrapTypeSwitch(
	ctx *Context,
	pass *analysis.Pass,
	stmt *ast.TypeSwitchStmt,
) {
	// Интерес:
	// - switch err.(type)             → прототип type-based dispatch
	//   (в cerrful будет относиться к CER<typename>-ветвлениям)

	// Пока пусто
}

var dummyWrapFormatLit = &ast.BasicLit{
	Value: strconv.Quote(": %w"),
}

func (e *ScrapEngine) scrapFmtDetails(
	pass *analysis.Pass,
	call *ast.CallExpr,
	span ContextSpan,
) (
	src string,
	msg string,
	isFmtNew bool,
) {
	// Single-arg fmt.Errorf("msg") — это не wrap
	if len(call.Args) == 1 {
		return "", "", true
	}

	// --- FIND ERROR ARGUMENT ---
	var errDetected bool
	var errIndex int
	for i, expr := range call.Args[1:] {
		if types.Identical(
			pass.TypesInfo.TypeOf(expr),
			types.Universe.Lookup("error").Type(),
		) {
			errIndex = i
			errDetected = true
			break
		}
	}
	if !errDetected {
		// fmt.Errorf("msg", x, y) без error → это fmt-new
		return "", "", true
	}

	// --- FORMAT CHECK ---
	v, ok := call.Args[0].(*ast.BasicLit)
	if !ok {
		// ставим dummy, чтобы unquote не умер
		v = dummyWrapFormatLit
		e.r.Report(cerrules.AnnotationFormatMustBeLiteral(), "", span.start)
	}

	unquote, _ := strconv.Unquote(v.Value)
	const wrapSuffix = ": %w"
	if !strings.HasSuffix(unquote, wrapSuffix) {
		e.r.Report(cerrules.AnnotationFormatMustEndWithW(), "", span.start)
		unquote = wrapSuffix // dummy текст, чтобы parsing не умер
	}
	msg = unquote[:len(unquote)-len(wrapSuffix)]

	// --- VARIABLE CHECK ---
	variable := call.Args[errIndex+1]
	if id, ok := variable.(*ast.Ident); ok {
		src = id.Name
	} else {
		e.r.Report(cerrules.FixBeforeUse(), "", span.start)
	}

	return src, msg, false
}

func (e *ScrapEngine) extractErrorVarsFromLogging(
	pass *analysis.Pass,
	call *ast.CallExpr,
) []*cir.ExprVar {

}

func (e *ScrapEngine) resolveFn(
	pass *analysis.Pass,
	call *ast.CallExpr,
) *Fn {
	fun := call.Fun

	// Unwrap parentheses
	for {
		if p, ok := fun.(*ast.ParenExpr); ok {
			fun = p.X
		} else {
			break
		}
	}

	switch fn := fun.(type) {

	// case 1: simple identifier, like f()
	case *ast.Ident:
		obj := pass.TypesInfo.Uses[fn]
		if obj == nil {
			return nil
		}
		fnObj, ok := obj.(*types.Func)
		if !ok {
			return nil
		}
		sig, ok := fnObj.Type().(*types.Signature)
		if !ok {
			return nil
		}
		return &Fn{Name: fnObj.Name(), Sig: sig, Obj: fnObj}

	// case 2: selector, like pkg.Foo(), obj.Method()
	case *ast.SelectorExpr:
		sel := pass.TypesInfo.Selections[fn]
		if sel != nil {
			// method of a type: x.Method()
			fnObj, ok := sel.Obj().(*types.Func)
			if !ok {
				return nil
			}
			sig, ok := fnObj.Type().(*types.Signature)
			if !ok {
				return nil
			}
			return &Fn{Name: fnObj.Name(), Sig: sig, Obj: fnObj}
		}

		// top-level: pkg.Func
		obj := pass.TypesInfo.Uses[fn.Sel]
		if obj == nil {
			return nil
		}
		fnObj, ok := obj.(*types.Func)
		if !ok {
			return nil
		}
		sig, ok := fnObj.Type().(*types.Signature)
		if !ok {
			return nil
		}
		return &Fn{Name: fnObj.Name(), Sig: sig, Obj: fnObj}

	default:
		// Could be IndexExpr, FuncLit, TypeAssert, etc.
		// Try last resort: TypeOf(fun)
		t := pass.TypesInfo.TypeOf(fun)
		if t == nil {
			return nil
		}
		if sig, ok := t.(*types.Signature); ok {
			// Synthetic function value (func literal, etc.)
			return &Fn{
				Name: "<func>",
				Sig:  sig,
				Obj:  nil,
			}
		}
		return nil
	}
}

func (e *ScrapEngine) errorMustBeLastInspection(pass *analysis.Pass, returns *ast.FieldList) {
	if returns == nil {
		return
	}

	lastIndex := len(returns.List) - 1
	for i, ret := range returns.List {
		t := pass.TypesInfo.TypeOf(ret.Type)
		if t == nil {
			continue
		}

		if !types.Identical(t, types.Universe.Lookup("error").Type()) {
			continue
		}

		if i != lastIndex {
			e.r.Report(cerrules.ErrorMustBeLastReturnValue(), "", ret.Pos())
			continue
		}

		if len(ret.Names) > 1 {
			names := ret.Names[:len(ret.Names)-1]
			for _, name := range names {
				e.r.Report(cerrules.ErrorMustBeLastReturnValue(), "", name.Pos())
			}
		}
	}
}

func (e *ScrapEngine) panicMissingNodeBlob(pos token.Pos, what string) error {
	return fmt.Errorf("%s missing %s definition for this place", e.fset.Position(pos), what)
}

func (e *ScrapEngine) panicNotAnExprBlob(pos token.Pos, cirNode cir.Node) error {
	return fmt.Errorf("%s CIR expression defintion expected, got %T", e.fset.Position(pos), cirNode)
}

func getNamedReturnValuesWithError(pass *analysis.Pass, returns *ast.FieldList) []*ast.Field {
	if returns == nil {
		return nil
	}

	if len(returns.List) == 0 {
		return nil
	}

	last := returns.List[len(returns.List)-1]
	if len(last.Names) == 0 {
		return nil
	}

	t := pass.TypesInfo.TypeOf(last.Type)
	if t == nil || !isErrorType(t) {
		return nil
	}

	return returns.List
}

func stripParens(expr ast.Expr) ast.Expr {
	v, ok := expr.(*ast.ParenExpr)
	if !ok {
		return expr
	}

	return stripParens(v.X)
}

type Fn struct {
	Name string
	Sig  *types.Signature
	Obj  *types.Func // может быть nil для интерфейсных методов
}

func resolveFuncRef(fn *Fn) *Reference {
	if fn == nil || fn.Obj == nil {
		return nil // интерфейсные методы не имеют референции
	}

	obj := fn.Obj
	pkg := obj.Pkg()
	if pkg == nil {
		return nil
	}

	ref := &Reference{
		Package: pkg.Path(),
		Name:    obj.Name(),
	}

	// Если это метод → достаём тип-ресивер
	if sig := obj.Type().(*types.Signature); sig.Recv() != nil {
		if nt, ok := sig.Recv().Type().(*types.Named); ok {
			ref.Type = nt.Obj().Name()
		}
	}

	return ref
}

func extractStringLit(v ast.Expr) *ast.BasicLit {
	switch vv := v.(type) {
	case *ast.BasicLit:
		return vv
	case *ast.BinaryExpr:
		return extractStringLit(vv.X)
	default:
		return nil
	}
}

func spanAST(n ast.Node) ContextSpan {
	return ContextSpan{
		start: n.Pos(),
		end:   n.End(),
	}
}
