// Package cerrful implements a Go AST -> CIR translator per the Cerrful Rulebook.
// v18.3 (2025-10-21)
// Changes from v18.2:
// - Success-path returns are omitted entirely from CIR (e.g., `return nil`, `return x, nil`, etc.).
// - Keep fixes: alias handling before wrap, last-result-only error handling, no phantasy assigns.
package cerrful

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
)

// ---------- CIR model ----------

type Node interface{ isNode() }

type Pos struct {
	File string
	Line int
	Col  int
}

// AssignSource is an ADT describing the semantic origin of an assigned error value.
type AssignSource interface{ isAssignSource() }

// AssignSourceCtor represents errors.New / fmt.Errorf w/o %w (i.e., constructor).
type AssignSourceCtor struct {
	Msg string // message (unquoted in model, Pretty quotes it)
	Via string // e.g., "errors.New" or "fmt.Errorf"
}

func (AssignSourceCtor) isAssignSource() {}

// AssignSourceCall represents a function call producing error, with locality.
type AssignSourceCall struct {
	Callee string // rendered, params elided
	Local  bool   // true if in same module
}

func (AssignSourceCall) isAssignSource() {}

// AssignSourceSentinel represents a package-level error constant, with locality.
type AssignSourceSentinel struct {
	Symbol string // e.g., "os.ErrNotExist"
	Local  bool
}

func (AssignSourceSentinel) isAssignSource() {}

// AssignSourceAlias represents direct aliasing of another error variable: err := otherErr.
type AssignSourceAlias struct {
	Target string // identifier name of RHS
}

func (AssignSourceAlias) isAssignSource() {}

// AssignSourceTypeAssert represents a type assertion producing an error value.
type AssignSourceTypeAssert struct {
	Expr string // e.g., "t.(error)"
}

func (AssignSourceTypeAssert) isAssignSource() {}

type Assign struct {
	Pos  Pos
	Name string       // LHS variable or "@err" synthetic for direct returns (when no named return error)
	Src  AssignSource // ADT variant for RHS
}

func (Assign) isNode() {}

type Wrap struct {
	Pos  Pos
	Name string
	Msg  string // keep quotes in Pretty
	Via  string
}

func (Wrap) isNode() {}

type Return struct {
	Pos  Pos
	Name string
}

func (Return) isNode() {}

type Log struct {
	Pos   Pos
	Vars  []string
	Level string // warn|error|fatal|other
	Via   string
}

func (Log) isNode() {}

type Ref struct{ Package, Name string }

type Check struct {
	Pos   Pos
	Vars  []string
	Args  []string
	Name  Ref // predicate callee
	Class Ref // semantic class, e.g. os.ErrNotExist
}

func (Check) isNode() {}

type If struct {
	Pos  Pos
	Expr string // keep quotes in Pretty (raw condition)
	Then []Node
	Else []Node
}

func (If) isNode() {}

type CIRFunction struct {
	Name  string
	Nodes []Node
}

type CIRProgram struct {
	File      string
	Functions []CIRFunction
}

// ---------- Config ----------

type LoggerSpec struct {
	Package string
	Name    string
	Level   string
}

type CheckerSpec struct {
	Func  Ref
	Class Ref
}

type Config struct {
	Loggers      []LoggerSpec
	Checkers     []CheckerSpec
	Constructors []Ref // functions that construct errors (no %w semantics)
	// Future: map of function name -> error result index
}

func DefaultConfig() Config {
	return Config{
		Loggers: []LoggerSpec{
			{Package: "fmt", Name: "Println", Level: "warn"},
			{Package: "fmt", Name: "Printf", Level: "warn"},
			{Package: "log", Name: "Println", Level: "warn"},
			{Package: "log", Name: "Printf", Level: "warn"},
			{Package: "log", Name: "Fatal", Level: "fatal"},
			{Package: "log", Name: "Fatalf", Level: "fatal"},
			{Package: "log/slog", Name: "Error", Level: "error"},
			{Package: "log/slog", Name: "Warn", Level: "warn"},
			{Package: "testing", Name: "Log", Level: "warn"},
			{Package: "testing", Name: "Error", Level: "error"},
			{Package: "testing", Name: "Fatal", Level: "fatal"},
			{Package: "testing", Name: "Fatalf", Level: "fatal"},
		},
		Checkers: []CheckerSpec{
			{Func: Ref{"os", "IsExist"}, Class: Ref{"os", "ErrExist"}},
			{Func: Ref{"os", "IsNotExist"}, Class: Ref{"os", "ErrNotExist"}},
			{Func: Ref{"os", "IsTimeout"}, Class: Ref{"os", "ErrTimeout"}},
			{Func: Ref{"os", "IsPermission"}, Class: Ref{"os", "ErrPermission"}},
			{Func: Ref{"errors", "Is"}, Class: Ref{"errors", "Any"}},
		},
		Constructors: []Ref{
			{Package: "errors", Name: "New"},
			{Package: "fmt", Name: "Errorf"}, // only constructor when NO %w
		},
	}
}

// ---------- Translator ----------

type Translator struct {
	cfg        Config
	errIface   types.Type
	info       *types.Info
	fileSet    *token.FileSet
	pkgName    string
	modulePath string // from go.mod
}

func New(cfg Config) *Translator { return &Translator{cfg: cfg} }

func (t *Translator) TranslateFile(filename string, src []byte) (*CIRProgram, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, src, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	// try to locate module path from nearest go.mod
	t.modulePath = findModulePath(filename)

	conf := types.Config{Importer: importer.Default()}
	info := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
	}
	_, _ = conf.Check(file.Name.Name, fset, []*ast.File{file}, info)

	t.errIface = types.Universe.Lookup("error").Type()
	t.info = info
	t.fileSet = fset
	t.pkgName = file.Name.Name

	prog := &CIRProgram{File: filepath.Base(filename)}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		st := newState(fn.Name.Name)
		// register named error returns (signature only)
		if fn.Type.Results != nil {
			for _, res := range fn.Type.Results.List {
				if len(res.Names) > 0 {
					for _, id := range res.Names {
						if t.typeIsError(res.Type) {
							st.namedRet[id.Name] = true
						}
					}
				}
			}
		}
		nodes := t.walkBlock(fn.Body, st)
		prog.Functions = append(prog.Functions, CIRFunction{Name: fn.Name.Name, Nodes: nodes})
	}
	return prog, nil
}

func DemoTranslate(code string) (*CIRProgram, error) {
	tr := New(DefaultConfig())
	return tr.TranslateFile("snippet.go", []byte(code))
}

type state struct {
	// names in scope that are error-like (heuristic) — used for log/check capture
	errVars map[string]bool
	// only named return error parameters from the function signature
	namedRet map[string]bool
	funcName string
}

func newState(fn string) *state {
	return &state{
		errVars:  make(map[string]bool),
		namedRet: make(map[string]bool),
		funcName: fn,
	}
}

func (t *Translator) walkBlock(b *ast.BlockStmt, st *state) []Node {
	var out []Node
	for _, s := range b.List {
		switch s := s.(type) {
		case *ast.AssignStmt:
			out = append(out, t.onAssign(s, st)...)
		case *ast.ExprStmt:
			out = append(out, t.onExpr(s.X, st)...)
		case *ast.IfStmt:
			out = append(out, t.onIf(s, st)...)
		case *ast.ReturnStmt:
			out = append(out, t.onReturn(s, st)...)
		}
	}
	return out
}

// ---------- Handlers ----------

func (t *Translator) onAssign(as *ast.AssignStmt, st *state) []Node {
	var out []Node

	// Single LHS: err := X  /  err = X
	if len(as.Lhs) == 1 {
		if id, ok := as.Lhs[0].(*ast.Ident); ok {
			if len(as.Rhs) == 1 {
				// RHS classification (constructor / wrap / alias / sentinel / call / type-assert)
				if ce, ok := as.Rhs[0].(*ast.CallExpr); ok {
					if via, msg, okCtor, okWrap := t.classifyConstructorOrWrap(ce); okCtor {
						out = append(out, Assign{Pos: t.posOfNode(as), Name: id.Name, Src: AssignSourceCtor{Msg: msg, Via: via}})
						st.errVars[id.Name] = true
						return out
					} else if okWrap {
						// underlying error is last arg; bind it first, then Wrap
						var underlying ast.Expr
						if len(ce.Args) > 1 {
							underlying = ce.Args[len(ce.Args)-1]
						}
						src := t.classifyAssignSource(underlying)
						out = append(out, Assign{Pos: t.posOfNode(as), Name: id.Name, Src: src})
						st.errVars[id.Name] = true
						out = append(out, Wrap{Pos: t.posOfNode(as), Name: id.Name, Msg: normalizeWrapMsg(firstStringArg(ce)), Via: "fmt.Errorf"})
						return out
					}
				}

				// Type assertion to error: err := t.(error)
				if ta, ok := as.Rhs[0].(*ast.TypeAssertExpr); ok && t.exprIsErrorAssert(ta) {
					out = append(out, Assign{Pos: t.posOfNode(as), Name: id.Name, Src: AssignSourceTypeAssert{Expr: t.exprString(as.Rhs[0])}})
					st.errVars[id.Name] = true
					return out
				}

				// Alias: err := otherErr (single ident of error type)
				if rid, ok := as.Rhs[0].(*ast.Ident); ok {
					if t.identIsErrorVar(rid) {
						out = append(out, Assign{Pos: t.posOfNode(as), Name: id.Name, Src: AssignSourceAlias{Target: rid.Name}})
						st.errVars[id.Name] = true
						return out
					}
				}

				// Sentinel or call or (fallback) alias
				src := t.classifyAssignSource(as.Rhs[0])
				out = append(out, Assign{Pos: t.posOfNode(as), Name: id.Name, Src: src})
				// Heuristic: mark any LHS name ending with "err" as error-var for logging/checks
				if id.Name == "err" || strings.HasSuffix(strings.ToLower(id.Name), "err") {
					st.errVars[id.Name] = true
				}
				return out
			}
		}
	}

	// Multi-value call: _, err := fn()
	if len(as.Rhs) == 1 {
		if _, ok := as.Rhs[0].(*ast.CallExpr); ok {
			if len(as.Lhs) > 1 {
				last := as.Lhs[len(as.Lhs)-1]
				if id, ok := last.(*ast.Ident); ok {
					call := as.Rhs[0].(*ast.CallExpr)
					src := t.assignSourceForCall(call)
					out = append(out, Assign{Pos: t.posOfNode(as), Name: id.Name, Src: src})
					st.errVars[id.Name] = true
					return out
				}
			}
		}
	}

	// General multi-assign: best-effort per pair
	for i, lhs := range as.Lhs {
		id, ok := lhs.(*ast.Ident)
		if !ok {
			continue
		}
		var rhs ast.Expr
		if i < len(as.Rhs) {
			rhs = as.Rhs[i]
		}
		if rhs == nil {
			continue
		}
		var src AssignSource
		// special cases
		if ce, ok := rhs.(*ast.CallExpr); ok {
			if via, msg, okCtor, okWrap := t.classifyConstructorOrWrap(ce); okCtor {
				src = AssignSourceCtor{Msg: msg, Via: via}
			} else if okWrap {
				// Bind rhs last arg then wrap
				var underlying ast.Expr
				if len(ce.Args) > 1 {
					underlying = ce.Args[len(ce.Args)-1]
				}
				src = t.classifyAssignSource(underlying)
				out = append(out, Assign{Pos: t.posOfNode(as), Name: id.Name, Src: src})
				st.errVars[id.Name] = true
				out = append(out, Wrap{Pos: t.posOfNode(as), Name: id.Name, Msg: normalizeWrapMsg(firstStringArg(ce)), Via: "fmt.Errorf"})
				continue
			} else {
				src = t.assignSourceForCall(ce)
			}
		} else if ta, ok := rhs.(*ast.TypeAssertExpr); ok && t.exprIsErrorAssert(ta) {
			src = AssignSourceTypeAssert{Expr: t.exprString(rhs)}
		} else if rid, ok := rhs.(*ast.Ident); ok && t.identIsErrorVar(rid) {
			src = AssignSourceAlias{Target: rid.Name}
		} else {
			src = t.classifyAssignSource(rhs)
		}
		out = append(out, Assign{Pos: t.posOfNode(as), Name: id.Name, Src: src})
		if id.Name == "err" || strings.HasSuffix(strings.ToLower(id.Name), "err") {
			st.errVars[id.Name] = true
		}
	}
	return out
}

func (t *Translator) onExpr(e ast.Expr, st *state) []Node {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return nil
	}

	// panic(...)
	if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "panic" {
		if len(call.Args) == 0 {
			return nil
		}
		arg := call.Args[0]
		if ce, ok := arg.(*ast.CallExpr); ok {
			if _, msg, _, okWrap := t.classifyConstructorOrWrap(ce); okWrap {
				errName := "@err" // synthetic
				return []Node{
					Wrap{Pos: t.posOfNode(e), Name: errName, Msg: msg, Via: "fmt.Errorf"},
					Log{Pos: t.posOfNode(e), Vars: []string{errName}, Level: "fatal", Via: "panic"},
				}
			}
		}
		argStr := t.exprString(arg)
		return []Node{Log{Pos: t.posOfNode(e), Vars: []string{argStr}, Level: "fatal", Via: "panic"}}
	}

	// Checkers
	if cs, ok := t.matchChecker(t.exprString(call.Fun)); ok {
		args := make([]string, len(call.Args))
		var vars []string
		for i, a := range call.Args {
			args[i] = t.exprString(a)
			if id, ok := a.(*ast.Ident); ok && st.errVars[id.Name] {
				vars = append(vars, id.Name)
			}
		}
		return []Node{Check{
			Pos:   t.posOfNode(call),
			Vars:  vars,
			Args:  args,
			Name:  cs.Func,
			Class: cs.Class,
		}}
	}

	// Loggers
	callee := t.exprString(call.Fun)
	if ls, ok := t.matchLogger(callee); ok {
		var names []string
		for _, a := range call.Args {
			if id, ok := a.(*ast.Ident); ok && st.errVars[id.Name] {
				names = append(names, id.Name)
			}
		}
		if len(names) > 0 {
			return []Node{Log{Pos: t.posOfNode(call), Vars: names, Level: ls.Level, Via: callee}}
		}
	}

	return nil
}

func (t *Translator) onIf(s *ast.IfStmt, st *state) []Node {
	var out []Node
	// emit init before if
	if s.Init != nil {
		switch init := s.Init.(type) {
		case *ast.AssignStmt:
			out = append(out, t.onAssign(init, st)...)
		case *ast.ExprStmt:
			out = append(out, t.onExpr(init.X, st)...)
		}
	}
	expr := t.exprString(s.Cond)
	thenNodes := t.walkBlock(s.Body, st)
	var elseNodes []Node
	if s.Else != nil {
		if eb, ok := s.Else.(*ast.BlockStmt); ok {
			elseNodes = t.walkBlock(eb, st)
		}
	}
	out = append(out, If{Pos: t.posOfNode(s), Expr: expr, Then: thenNodes, Else: elseNodes})
	return out
}

func (t *Translator) onReturn(r *ast.ReturnStmt, st *state) []Node {
	var out []Node
	// Only process the last return expression — by rule, that's the error unless overridden (future).
	if len(r.Results) == 0 {
		return out
	}
	idx := len(r.Results) - 1
	res := r.Results[idx]

	// --- v18.3 success-path pruning ---
	if isNilLiteral(res) {
		// Success return (error is nil): omit entirely.
		return out
	}

	switch v := res.(type) {
	case *ast.CallExpr:
		// %w => Wrap; constructor => Ctor
		if via, msg, okCtor, okWrap := t.classifyConstructorOrWrap(v); okWrap {
			// choose name: prefer named return error (signature); else "@err"
			name := t.findNamedErrorReturn(st)
			if name == "" {
				name = "@err"
			}
			// underlying to assign first
			var underlying ast.Expr
			if len(v.Args) > 1 {
				underlying = v.Args[len(v.Args)-1]
			}
			src := t.classifyAssignSource(underlying)
			// If it's a plain alias to some var, no need to "rebind" into name before wrapping
			if _, isAlias := src.(AssignSourceAlias); !isAlias {
				out = append(out, Assign{Pos: t.posOfNode(r), Name: name, Src: src})
			}
			out = append(out, Wrap{Pos: t.posOfNode(r), Name: name, Msg: msg, Via: "fmt.Errorf"})
			out = append(out, Return{Pos: t.posOfNode(r), Name: name})
			return out
		} else if okCtor {
			name := t.findNamedErrorReturn(st)
			if name == "" {
				name = "@err"
			}
			out = append(out, Assign{Pos: t.posOfNode(r), Name: name, Src: AssignSourceCtor{Msg: msg, Via: via}})
			out = append(out, Return{Pos: t.posOfNode(r), Name: name})
			return out
		}
	case *ast.TypeAssertExpr:
		if t.exprIsErrorAssert(v) {
			name := t.findNamedErrorReturn(st)
			if name == "" {
				name = "@err"
			}
			out = append(out, Assign{Pos: t.posOfNode(r), Name: name, Src: AssignSourceTypeAssert{Expr: t.exprString(v)}})
			out = append(out, Return{Pos: t.posOfNode(r), Name: name})
			return out
		}
	case *ast.Ident, *ast.SelectorExpr:
		name := t.findNamedErrorReturn(st)
		if name == "" {
			name = "@err"
		}
		src := t.classifyAssignSource(v)
		out = append(out, Assign{Pos: t.posOfNode(r), Name: name, Src: src})
		out = append(out, Return{Pos: t.posOfNode(r), Name: name})
		return out
	}

	return out
}

// ---------- Classification & helpers ----------

func (t *Translator) classifyConstructorOrWrap(call *ast.CallExpr) (via string, msg string, isCtor bool, isWrap bool) {
	// callee
	pkg, name := t.calleePkgName(call.Fun)
	via = joinPkgName(pkg, name)
	// fmt.Errorf: dual role
	if pkg == "fmt" && name == "Errorf" {
		m := firstStringArg(call)
		if strings.Contains(m, "%w") {
			// wrap
			msg = normalizeWrapMsg(m)
			return via, msg, false, true
		}
		// constructor (no %w)
		msg = m
		return via, msg, true, false
	}
	// other registered constructors
	for _, c := range t.cfg.Constructors {
		if c.Package == pkg && c.Name == name {
			msg = firstStringArg(call)
			return via, msg, true, false
		}
	}
	return via, "", false, false
}

func (t *Translator) classifyAssignSource(e ast.Expr) AssignSource {
	// Calls
	if c, ok := e.(*ast.CallExpr); ok {
		return t.assignSourceForCall(c)
	}
	// Type assertion to error
	if ta, ok := e.(*ast.TypeAssertExpr); ok && t.exprIsErrorAssert(ta) {
		return AssignSourceTypeAssert{Expr: t.exprString(e)}
	}
	// Selector: possible sentinel pkg.ErrX
	if sel, ok := e.(*ast.SelectorExpr); ok {
		pkgName, full := t.selectorText(sel)
		typ := t.info.TypeOf(sel)
		if typ != nil && types.AssignableTo(typ, t.errIface) {
			// Determine package path via Uses
			if obj := t.info.Uses[sel.Sel]; obj != nil && obj.Pkg() != nil {
				local := t.isPkgLocal(obj.Pkg())
				return AssignSourceSentinel{Symbol: full, Local: local}
			}
			// Fallback: stdlib or unknown -> foreign
			if pkgName != "" && pkgName != t.pkgName {
				return AssignSourceSentinel{Symbol: full, Local: false}
			}
		}
	}
	// Ident: may be sentinel (pkg-level) or variable; alias if variable of error type
	if id, ok := e.(*ast.Ident); ok {
		typ := t.info.TypeOf(id)
		if typ != nil && types.AssignableTo(typ, t.errIface) {
			if obj := t.info.Uses[id]; obj != nil {
				// package-level sentinel if parent is package scope
				if pkg := obj.Pkg(); pkg != nil && obj.Parent() == pkg.Scope() {
					return AssignSourceSentinel{Symbol: id.Name, Local: t.isPkgLocal(pkg)}
				}
			}
			// variable or param
			return AssignSourceAlias{Target: id.Name}
		}
	}
	// Fallback: render as alias of expression text (very rare)
	txt := t.exprString(e)
	return AssignSourceAlias{Target: txt}
}

func (t *Translator) assignSourceForCall(c *ast.CallExpr) AssignSource {
	callee, locality := t.shortCall(c)
	return AssignSourceCall{Callee: callee, Local: locality == "local call"}
}

func (t *Translator) exprIsErrorAssert(ta *ast.TypeAssertExpr) bool {
	if ta == nil || ta.Type == nil {
		return false
	}
	typ := t.info.TypeOf(ta.Type)
	return typ != nil && types.AssignableTo(typ, t.errIface)
}

func (t *Translator) identIsErrorVar(id *ast.Ident) bool {
	if id == nil {
		return false
	}
	typ := t.info.TypeOf(id)
	if typ == nil {
		return false
	}
	return types.AssignableTo(typ, t.errIface)
}

func (t *Translator) typeIsError(expr ast.Expr) bool {
	typ := t.info.TypeOf(expr)
	return typ != nil && types.AssignableTo(typ, t.errIface)
}

func (t *Translator) findNamedErrorReturn(st *state) string {
	// Only named return params from the signature qualify here.
	for name := range st.namedRet {
		return name // if multiple, first is fine; could sort deterministically if needed
	}
	return ""
}

func (t *Translator) shortCall(c *ast.CallExpr) (string, string) {
	// Determine callee and locality by types info
	switch fun := c.Fun.(type) {
	case *ast.SelectorExpr:
		// pkg.Func(...) or recv.Method(...)
		if id, ok := fun.X.(*ast.Ident); ok {
			// Package func
			name := id.Name + "." + fun.Sel.Name + elideParams(len(c.Args))
			// Resolve package path: Sel use points to *types.Func, whose Pkg is the defining package
			if obj := t.info.Uses[fun.Sel]; obj != nil && obj.Pkg() != nil {
				if t.isPkgLocal(obj.Pkg()) {
					return name, "local call"
				}
				return name, "foreign call"
			}
			// Fallback: if package name differs from current package, treat as foreign
			if id.Name != t.pkgName {
				return name, "foreign call"
			}
			return name, "local call"
		}
		// Method call on receiver: use selection to find defining package
		if sel := t.info.Selections[fun]; sel != nil {
			name := t.exprString(fun.X) + "." + sel.Obj().Name() + elideParams(len(c.Args))
			if sel.Obj().Pkg() != nil && t.isPkgLocal(sel.Obj().Pkg()) {
				return name, "local call"
			}
			return name, "foreign call"
		}
		// Fallback
		return t.exprString(fun) + elideParams(len(c.Args)), "expr"
	case *ast.Ident:
		// Unqualified function in same package => local call
		return fun.Name + elideParams(len(c.Args)), "local call"
	default:
		return t.exprString(c.Fun) + elideParams(len(c.Args)), "expr"
	}
}

func elideParams(n int) string {
	if n == 0 {
		return "()"
	}
	return "(…)"
}

func (t *Translator) selectorText(s *ast.SelectorExpr) (pkg string, full string) {
	if id, ok := s.X.(*ast.Ident); ok {
		return id.Name, id.Name + "." + s.Sel.Name
	}
	return "", t.exprString(s)
}

func (t *Translator) calleePkgName(fun ast.Expr) (pkg, name string) {
	if se, ok := fun.(*ast.SelectorExpr); ok {
		if id, ok := se.X.(*ast.Ident); ok {
			return id.Name, se.Sel.Name
		}
	}
	if id, ok := fun.(*ast.Ident); ok {
		return "", id.Name
	}
	return "", t.exprString(fun)
}

func joinPkgName(pkg, name string) string {
	if pkg == "" {
		return name
	}
	return pkg + "." + name
}

// module-locality helpers
func findModulePath(startFile string) string {
	dir := filepath.Dir(startFile)
	for {
		mod := filepath.Join(dir, "go.mod")
		if f, err := os.Open(mod); err == nil {
			defer f.Close()
			sc := bufio.NewScanner(f)
			for sc.Scan() {
				line := strings.TrimSpace(sc.Text())
				if strings.HasPrefix(line, "module ") {
					return strings.TrimSpace(strings.TrimPrefix(line, "module "))
				}
			}
			return ""
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func (t *Translator) isPkgLocal(p *types.Package) bool {
	if p == nil {
		return false
	}
	pp := p.Path()
	if t.modulePath == "" {
		// No module context: consider same package name as local for unqualified calls only
		return p.Name() == t.pkgName
	}
	// stdlib packages do not contain a dot and won't start with module path
	if !strings.Contains(pp, ".") {
		return false
	}
	return strings.HasPrefix(pp, t.modulePath)
}

// ---------- Utilities ----------

func normalizeWrapMsg(m string) string {
	m = strings.TrimSpace(m)
	m = strings.TrimSuffix(m, ": %w")
	m = strings.TrimSuffix(m, " %w")
	m = strings.TrimSuffix(m, "(%w)")
	return m
}

func firstStringArg(call *ast.CallExpr) string {
	if len(call.Args) == 0 {
		return ""
	}
	if lit, ok := call.Args[0].(*ast.BasicLit); ok {
		return strings.Trim(lit.Value, "`\"")
	}
	return ""
}

func (t *Translator) matchLogger(callee string) (LoggerSpec, bool) {
	pkg, name := splitPkgIdent(callee)
	for _, l := range t.cfg.Loggers {
		if l.Package == pkg && l.Name == name {
			return l, true
		}
	}
	return LoggerSpec{}, false
}

func (t *Translator) matchChecker(callee string) (CheckerSpec, bool) {
	pkg, name := splitPkgIdent(callee)
	for _, c := range t.cfg.Checkers {
		if c.Func.Package == pkg && c.Func.Name == name {
			return c, true
		}
	}
	return CheckerSpec{}, false
}

func splitPkgIdent(full string) (string, string) {
	i := strings.LastIndex(full, ".")
	if i < 0 {
		return "", full
	}
	return full[:i], full[i+1:]
}

func (t *Translator) posOfNode(n ast.Node) Pos {
	pos := t.fileSet.Position(n.Pos())
	return Pos{File: filepath.Base(pos.Filename), Line: pos.Line, Col: pos.Column}
}

func (t *Translator) exprString(e ast.Expr) string {
	if e == nil {
		return ""
	}
	var b strings.Builder
	_ = printer.Fprint(&b, t.fileSet, e)
	return b.String()
}

// ---------- Pretty ----------

func (p *CIRProgram) Pretty(indentedBlocks bool) string {
	var b strings.Builder
	for _, fn := range p.Functions {
		if indentedBlocks {
			fmt.Fprintf(&b, "Function %s:\n", fn.Name)
		} else {
			fmt.Fprintf(&b, "Function %s {\n", fn.Name)
		}
		for _, n := range fn.Nodes {
			renderNode(&b, n, 1, indentedBlocks)
		}
		if indentedBlocks {
			b.WriteByte('\n')
		} else {
			b.WriteString("}\n\n")
		}
	}
	return b.String()
}

func renderNode(b *strings.Builder, n Node, indent int, indentedBlocks bool) {
	ind := strings.Repeat("  ", indent)
	switch x := n.(type) {
	case Assign:
		switch src := x.Src.(type) {
		case AssignSourceCtor:
			if src.Via != "" {
				fmt.Fprintf(b, "%sAssign [%s] <- NewError msg=%q (via %s)\n", ind, x.Name, src.Msg, src.Via)
			} else {
				fmt.Fprintf(b, "%sAssign [%s] <- NewError msg=%q\n", ind, x.Name, src.Msg)
			}
		case AssignSourceCall:
			loc := "foreign"
			if src.Local {
				loc = "local"
			}
			fmt.Fprintf(b, "%sAssign [%s] <- %s (%s call)\n", ind, x.Name, src.Callee, loc)
		case AssignSourceSentinel:
			loc := "foreign"
			if src.Local {
				loc = "local"
			}
			fmt.Fprintf(b, "%sAssign [%s] <- %s (%s sentinel)\n", ind, x.Name, src.Symbol, loc)
		case AssignSourceAlias:
			fmt.Fprintf(b, "%sAssign [%s] <- %s\n", ind, x.Name, src.Target)
		case AssignSourceTypeAssert:
			fmt.Fprintf(b, "%sAssign [%s] <- %s (type assertion)\n", ind, x.Name, src.Expr)
		default:
			fmt.Fprintf(b, "%sAssign [%s] <- <unknown>\n", ind, x.Name)
		}
	case Wrap:
		fmt.Fprintf(b, "%sWrap [%s] msg=%q (via %s)\n", ind, x.Name, x.Msg, x.Via)
	case Return:
		fmt.Fprintf(b, "%sReturn [%s]\n", ind, x.Name)
	case Log:
		fmt.Fprintf(b, "%sLog %v level=%s (via %s)\n", ind, x.Vars, x.Level, x.Via)
	case Check:
		class := x.Class.Package + "." + x.Class.Name
		name := x.Name.Package + "." + x.Name.Name
		if len(x.Vars) == 0 {
			fmt.Fprintf(b, "%sCheck %v class=%s (via %s)\n", ind, x.Args, class, name)
		} else {
			fmt.Fprintf(b, "%sCheck %v class=%s (via %s)\n", ind, "["+strings.Join(x.Vars, " ")+"]", class, name)
		}
	case If:
		if indentedBlocks {
			fmt.Fprintf(b, "%sIf %q:\n", ind, x.Expr)
			for _, t := range x.Then {
				renderNode(b, t, indent+1, indentedBlocks)
			}
			if len(x.Else) > 0 {
				fmt.Fprintf(b, "%sElse:\n", ind)
				for _, e := range x.Else {
					renderNode(b, e, indent+1, indentedBlocks)
				}
			}
		} else {
			fmt.Fprintf(b, "%sIf %q {\n", ind, x.Expr)
			for _, t := range x.Then {
				renderNode(b, t, indent+1, indentedBlocks)
			}
			if len(x.Else) > 0 {
				fmt.Fprintf(b, "%s} else {\n", ind)
				for _, e := range x.Else {
					renderNode(b, e, indent+1, indentedBlocks)
				}
			}
			fmt.Fprintf(b, "%s}\n", ind)
		}
	}
}

// ---------- tiny helpers ----------

func isNilLiteral(e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == "nil"
}
