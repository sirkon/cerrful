// Package cerrful implements a Go AST -> CIR translator per the Cerrful Rulebook.
// v17: constructors (config-driven), fmt.Errorf dual role, invented error names for return-only constructors,
//
//	keep module-aware local/foreign, msg quotes only, typed refs everywhere.
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
	"unicode/utf8"
)

// ---------- CIR model ----------

type Node interface{ isNode() }

type Pos struct {
	File string
	Line int
	Col  int
}

type Assign struct {
	Pos  Pos
	Name string
	// One of the following RHS modes is used:
	RHS    string // for normal expressions/calls/sentinels (params elided); no quotes
	Flavor string // "local call" | "foreign call" | "local sentinel" | "foreign sentinel" | "expr"
	// Constructor mode:
	IsCtor  bool   // true => this Assign renders as: NewError msg="..." (via <pkg>.<name>)
	CtorMsg string // message (quoted in Pretty)
	CtorVia string // callee for constructors, like "errors.New" or "fmt.Errorf"
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
		// register named error returns
		if fn.Type.Results != nil {
			for _, res := range fn.Type.Results.List {
				if len(res.Names) > 0 {
					for _, id := range res.Names {
						if t.typeIsError(res.Type) {
							st.errVars[id.Name] = true
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
	errVars   map[string]bool
	funcName  string
	usedNames map[string]int // track invented names per function
}

func newState(fn string) *state {
	return &state{
		errVars:   make(map[string]bool),
		funcName:  fn,
		usedNames: make(map[string]int),
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

	// Single LHS
	if len(as.Lhs) == 1 {
		if id, ok := as.Lhs[0].(*ast.Ident); ok {
			if len(as.Rhs) == 1 {
				// Detect constructor calls first
				if ce, ok := as.Rhs[0].(*ast.CallExpr); ok {
					if via, msg, okCtor, okWrap := t.classifyConstructorOrWrap(ce); okCtor {
						// Constructor: Assign <- NewError msg="..." (via X.Y)
						out = append(out, Assign{
							Pos: t.posOfNode(as), Name: id.Name,
							IsCtor: true, CtorMsg: msg, CtorVia: via,
						})
						st.errVars[id.Name] = true
						return out
					} else if okWrap {
						// Wrap: Assign underlying + Wrap
						// underlying error is last arg
						var underlying ast.Expr
						if len(ce.Args) > 1 {
							underlying = ce.Args[len(ce.Args)-1]
						}
						rhsText, flavor := t.classifyExpr(underlying)
						out = append(out, Assign{Pos: t.posOfNode(as), Name: id.Name, RHS: rhsText, Flavor: flavor})
						st.errVars[id.Name] = true
						out = append(out, Wrap{Pos: t.posOfNode(as), Name: id.Name, Msg: msg, Via: "fmt.Errorf"})
						return out
					}
				}
			}
		}
	}

	// Multi-value: _, err := fn()
	if len(as.Rhs) == 1 {
		if _, ok := as.Rhs[0].(*ast.CallExpr); ok {
			if len(as.Lhs) > 1 {
				last := as.Lhs[len(as.Lhs)-1]
				if id, ok := last.(*ast.Ident); ok {
					rhsText, flavor := t.classifyExpr(as.Rhs[0])
					out = append(out, Assign{Pos: t.posOfNode(as), Name: id.Name, RHS: rhsText, Flavor: flavor})
					st.errVars[id.Name] = true
					return out
				}
			}
		}
	}

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
		rhsText, flavor := t.classifyExpr(rhs)
		out = append(out, Assign{Pos: t.posOfNode(as), Name: id.Name, RHS: rhsText, Flavor: flavor})
		// track likely error vars
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

	// panic
	if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "panic" {
		if len(call.Args) == 0 {
			return nil
		}
		arg := call.Args[0]
		if ce, ok := arg.(*ast.CallExpr); ok {
			if _, msg, _, okWrap := t.classifyConstructorOrWrap(ce); okWrap {
				errName := "err" // best-effort
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
	for _, res := range r.Results {
		switch v := res.(type) {
		case *ast.CallExpr:
			// %w => Wrap; constructor => NewError Assign+Return
			if via, msg, okCtor, okWrap := t.classifyConstructorOrWrap(v); okWrap {
				errName := t.inventErrName(st)
				out = append(out, Wrap{Pos: t.posOfNode(r), Name: errName, Msg: msg, Via: "fmt.Errorf"})
				out = append(out, Return{Pos: t.posOfNode(r), Name: errName})
				return out
			} else if okCtor {
				var name string
				if retName := t.findNamedErrorReturn(st); retName != "" {
					name = retName
				} else {
					name = t.inventErrName(st)
				}
				out = append(out, Assign{
					Pos: t.posOfNode(r), Name: name,
					IsCtor: true, CtorMsg: msg, CtorVia: via,
				})
				out = append(out, Return{Pos: t.posOfNode(r), Name: name})
				return out
			}
		case *ast.Ident:
			if st.errVars[v.Name] {
				out = append(out, Return{Pos: t.posOfNode(r), Name: v.Name})
				return out
			}
		}
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

func (t *Translator) classifyExpr(e ast.Expr) (short string, flavor string) {
	switch v := e.(type) {
	case *ast.CallExpr:
		return t.shortCall(v)
	case *ast.SelectorExpr:
		// Possible sentinel: pkg.ErrX
		pkgName, full := t.selectorText(v)
		typ := t.info.TypeOf(v)
		if typ != nil && types.AssignableTo(typ, t.errIface) {
			// Determine package path via Uses
			if obj := t.info.Uses[v.Sel]; obj != nil && obj.Pkg() != nil {
				if t.isPkgLocal(obj.Pkg()) {
					return full, "local sentinel"
				}
				return full, "foreign sentinel"
			}
			// Fallback: stdlib or unknown -> foreign
			if pkgName != "" && pkgName != t.pkgName {
				return full, "foreign sentinel"
			}
			return full, "expr"
		}
		return full, "expr"
	case *ast.Ident:
		// IDENT classification: distinguish package-level sentinel vs local variable
		typ := t.info.TypeOf(v)
		if typ != nil && types.AssignableTo(typ, t.errIface) {
			if obj := t.info.Uses[v]; obj != nil {
				// package-level sentinel if parent is package scope
				if pkg := obj.Pkg(); pkg != nil {
					if obj.Parent() == pkg.Scope() {
						if t.isPkgLocal(pkg) {
							return v.Name, "local sentinel"
						}
						return v.Name, "foreign sentinel"
					}
				}
				// local variable/param/captured
				return v.Name, "expr"
			}
			return v.Name, "expr"
		}
		return v.Name, "expr"
	default:
		// Render and attempt to simplify (strip spaces)
		txt := t.exprString(e)
		txt = strings.ReplaceAll(txt, " ", "")
		return txt, "expr"
	}
}

func (t *Translator) shortCall(c *ast.CallExpr) (string, string) {
	// Determine callee and locality by types info
	switch fun := c.Fun.(type) {
	case *ast.SelectorExpr:
		// pkg.Func(...) or recv.Method(...)
		if id, ok := fun.X.(*ast.Ident); ok {
			// Package func
			name := id.Name + "." + fun.Sel.Name + t.elideParams(len(c.Args))
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
			name := t.exprString(fun.X) + "." + sel.Obj().Name() + t.elideParams(len(c.Args))
			if sel.Obj().Pkg() != nil && t.isPkgLocal(sel.Obj().Pkg()) {
				return name, "local call"
			}
			return name, "foreign call"
		}
		// Fallback
		return t.exprString(fun) + t.elideParams(len(c.Args)), "expr"
	case *ast.Ident:
		// Unqualified function in same package => local call
		return fun.Name + t.elideParams(len(c.Args)), "local call"
	default:
		return t.exprString(c.Fun) + t.elideParams(len(c.Args)), "expr"
	}
}

func (t *Translator) elideParams(n int) string {
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

func (t *Translator) isFmtErrorf(call *ast.CallExpr) bool {
	if fun, ok := call.Fun.(*ast.SelectorExpr); ok {
		if pkg, ok := fun.X.(*ast.Ident); ok && pkg.Name == "fmt" && fun.Sel.Name == "Errorf" {
			return true
		}
	}
	return false
}

func (t *Translator) parseFmtErrorf(call *ast.CallExpr) (string, string) {
	if len(call.Args) == 0 {
		return "", ""
	}
	if lit, ok := call.Args[0].(*ast.BasicLit); ok && strings.Contains(lit.Value, "%w") {
		msg := strings.Trim(lit.Value, "`\"")
		msg = normalizeWrapMsg(msg)
		var errName string
		if len(call.Args) > 1 {
			if id, ok := call.Args[len(call.Args)-1].(*ast.Ident); ok {
				errName = id.Name
			}
		}
		if errName == "" {
			errName = "err"
		}
		return msg, errName
	}
	return "", ""
}

// normalize wrap label variations
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

// name invention: err, errFuncName, errFuncName2, ...
func (t *Translator) inventErrName(st *state) string {
	// prefer "err" if unused
	if st.usedNames["err"] == 0 && !st.errVars["err"] {
		st.usedNames["err"] = 1
		st.errVars["err"] = true
		return "err"
	}
	base := "err" + toCamel(st.funcName)
	n := st.usedNames[base]
	if n == 0 {
		st.usedNames[base] = 1
		st.errVars[base] = true
		return base
	}
	n++
	st.usedNames[base] = n
	name := fmt.Sprintf("%s%d", base, n)
	st.errVars[name] = true
	return name
}

func toLowerCamel(s string) string {
	if s == "" {
		return ""
	}
	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError && size == 0 {
		return s
	}
	return strings.ToLower(string(r)) + s[size:]
}

func (t *Translator) typeIsError(expr ast.Expr) bool {
	typ := t.info.TypeOf(expr)
	return typ != nil && types.AssignableTo(typ, t.errIface)
}

func (t *Translator) findNamedErrorReturn(st *state) string {
	for name := range st.errVars {
		if name != "err" && strings.HasSuffix(strings.ToLower(name), "err") {
			return name
		}
	}
	return ""
}

func toCamel(s string) string {
	if s == "" {
		return ""
	}
	r, size := utf8.DecodeRuneInString(s)
	return strings.ToUpper(string(r)) + s[size:]
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
		if x.IsCtor {
			if x.CtorVia != "" {
				fmt.Fprintf(b, "%sAssign [%s] <- NewError msg=%q (via %s)\n", ind, x.Name, x.CtorMsg, x.CtorVia)
			} else {
				fmt.Fprintf(b, "%sAssign [%s] <- NewError msg=%q\n", ind, x.Name, x.CtorMsg)
			}
			return
		}
		if x.Flavor != "" && x.Flavor != "expr" {
			fmt.Fprintf(b, "%sAssign [%s] <- %s (%s)\n", ind, x.Name, x.RHS, x.Flavor)
		} else {
			fmt.Fprintf(b, "%sAssign [%s] <- %s\n", ind, x.Name, x.RHS)
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
			fmt.Fprintf(b, "%sCheck %v class=%s (via %s)\n", ind, bracketVars(x.Vars), class, name)
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

func bracketVars(v []string) string {
	if len(v) == 0 {
		return "[]"
	}
	return "[" + strings.Join(v, " ") + "]"
}
