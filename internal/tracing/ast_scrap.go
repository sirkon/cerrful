package tracing

import (
	"fmt"
	"go/ast"
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

	r *ReporterPhase
}

func NewScrapEngine(r *ReporterPhase) *ScrapEngine {
	return &ScrapEngine{
		news:          make(map[Reference]NewSpec),
		wraps:         make(map[Reference]WrapSpec),
		loggers:       make(map[Reference]LoggerSpec),
		ignoredErrors: make(map[Reference]IgnoredError),
		r:             r,
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

// --- Actual logic ---------------------------------------------------------------------------------------------------

// Scrap traverses the file AST and records structural information
// about errors and their usage into the given context.
func (e *ScrapEngine) Scrap(
	ctx *Context,
	pass *analysis.Pass,
	file *ast.File,
) {
	// gotypes shortcuts
	info := pass.TypesInfo
	fset := pass.Fset

	// Walk the AST
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {

		// ---------------------------------------
		// 1. Function calls → wrap/new/log
		// ---------------------------------------
		case *ast.CallExpr:
			e.scrapCall(ctx, pass, node)
			return true

		// ---------------------------------------
		// 2. Assignments and general expressions
		//    (may contain loggers or ignored errors)
		// ---------------------------------------
		case *ast.AssignStmt:
			e.scrapAssign(ctx, pass, node)
			return true

		// ---------------------------------------
		// 3. Return statements
		//    (may propagate ignored errors etc.)
		// ---------------------------------------
		case *ast.ReturnStmt:
			e.scrapReturn(ctx, pass, node)
			return true

		// ---------------------------------------
		// IF statements
		// ---------------------------------------
		case *ast.IfStmt:
			e.scrapIf(ctx, pass, node)
			return true

		// ---------------------------------------
		// SWITCH statements
		// ---------------------------------------
		case *ast.SwitchStmt:
			e.scrapSwitch(ctx, pass, node)
			return true

		case *ast.TypeSwitchStmt:
			e.scrapTypeSwitch(ctx, pass, node)
			return true

		default:
			return true
		}
	})
}

func (e *ScrapEngine) scrapCall(
	ctx *Context,
	pass *analysis.Pass,
	fn *Fn,
	call *ast.CallExpr,
) {
	ref := resolveFuncRef(fn)
	if ref == nil {
		return
	}

	span := ContextSpan{
		start: call.Pos(),
		end:   call.End(),
	}

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
		// TODO EXTRACT_LOGGING_COMPONENTS
		ctx.Add(
			&cir.Log{
				Var:   nil,
				Level: 0,
				Msg:   "",
				Ref:   ls.Ref.CIR(),
			},
			span,
		)
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

func (e *ScrapEngine) scrapAssign(
	ctx *Context,
	pass *analysis.Pass,
	as *ast.AssignStmt,
) {
	// Here we can:
	// - detect logging patterns
	// - detect ignored errors in multi-value returns
}

func (e *ScrapEngine) scrapReturn(
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
