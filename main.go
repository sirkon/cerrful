package main

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const doc = `mycustomlint is a linter that checks for proper error wrapping and logging`

// Analyzer is the main entry point for the linter
var Analyzer = &analysis.Analyzer{
	Name:     "cerrful",
	Doc:      doc,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

func run(pass *analysis.Pass) (any, error) {
	pector := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{
		(*ast.FuncDecl)(nil),
	}

	pector.Preorder(nodeFilter, func(node ast.Node) {
		n := node.(*ast.FuncDecl) // No need to assert check since we only get func decls.

		checkErrProcessing(n, pector, pass)
	})

	return nil, nil
}

// checkErrProcessing we are doing:
//
//   - Count every call expression returning err. Not doing errcheck job, so only count what wasn't explicitly ignored.
//   - Count every new error spawn.
//   - Looking for places having <errVar> != nil yet making new … errVar = … assignment without known
//     log call.
//   - We demand every error got from calls to be properly annotated with any known annotation variant in
//     case the count exceeded 1.
func checkErrProcessing(f *ast.FuncDecl, pector *inspector.Inspector, pass *analysis.Pass) {
}
