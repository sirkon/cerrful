package main

import (
	"go/ast"
	"go/types"
	"maps"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/types/typeutil"
)

type knownErrWrapChecker struct {
	known map[packagedFunc]SigWrapType
	pass  *analysis.Pass
}

func newKnownErrWrapChecker(custom map[packagedFunc]SigWrapType) *knownErrWrapChecker {
	predefined := map[packagedFunc]SigWrapType{
		{pkgPath: "fmt", name: "Errorf"}: SigWrapTypeErrorf,

		// I have my bias!
		{pkgPath: "github.com/sirkon/errors", name: "Wrap"}:  SigWrapTypeWrap,
		{pkgPath: "github.com/sirkon/errors", name: "Wrapf"}: SigWrapTypeWrap,

		// For my job.
		{pkgPath: "gitlab.corp.mail.ru/infra/hotbox/library/go/errors", name: "Wrap"}:  SigWrapTypeWrap,
		{pkgPath: "gitlab.corp.mail.ru/infra/hotbox/library/go/errors", name: "Wrapf"}: SigWrapTypeWrap,

		// Were widely used before. I am sure they still are, at least in older codebases.
		{pkgPath: "github.com/pkg/errors", name: "Wrap"}:         SigWrapTypeWrap,
		{pkgPath: "github.com/pkg/errors", name: "Wrapf"}:        SigWrapTypeWrap,
		{pkgPath: "github.com/pkg/errors", name: "WithMessage"}:  SigWrapTypeWrap,
		{pkgPath: "github.com/pkg/errors", name: "WithMessagef"}: SigWrapTypeWrap,

		// Some more…
		{pkgPath: "golang.org/x/xerrors", name: "Errorf"}: SigWrapTypeErrorf,

		// TODO add more predefines for repos with enough stars/users.
	}

	if custom == nil {
		custom = make(map[packagedFunc]SigWrapType)
	} else {
		custom = maps.Clone(custom)
	}

	// Merge custom and predefined defs.
	maps.Insert(custom, maps.All(predefined))

	return &knownErrWrapChecker{known: custom}
}

// isErrorWrap checks if given call expression wraps given error.
func (c *knownErrWrapChecker) isErrorWrap(call *ast.CallExpr, err *ast.Ident) bool {
	// Check if this call expression uses supported function.
	sigType, ok := c.getSupportFunctionSigType(call)
	if !ok {
		// This is not a supported function.
		return false
	}

	// Uses rule for given signature type to check proper error wrapping.
	switch sigType {
	case SigWrapTypeWrap:
		return c.checkWrapSignatureCall(call, err)
	case SigWrapTypeErrorf:
		return c.checkErrorfSignatureCall(call, err)
	default:
		return false
	}
}

func (c *knownErrWrapChecker) getSupportFunctionSigType(call *ast.CallExpr) (SigWrapType, bool) {
	fn := typeutil.Callee(c.pass.TypesInfo, call)
	if fn == nil {
		// Because using "raw" closures to handle error processing is a huge overcomplication.
		return SigWrapTypeInvalid, false
	}

	fnType, ok := fn.(*types.Func)
	if !ok {
		// Same here.
		return SigWrapTypeInvalid, false
	}

	pkg := fnType.Pkg()
	if pkg == nil {
		// Not what we are looking for.
		return SigWrapTypeInvalid, false
	}

	si, ok := c.known[packagedFunc{
		pkgPath: pkg.Path(),
		name:    fnType.Name(),
	}]
	if ok {
		return si, true
	}

	return SigWrapTypeInvalid, false
}

func (c *knownErrWrapChecker) checkErrorfSignatureCall(call *ast.CallExpr, err *ast.Ident) bool {
	if len(call.Args) == 0 {
		return false
	}

	for _, arg := range call.Args {
		switch v := arg.(type) {
		case *ast.Ident:
			if v.Name == err.Name {
				return true
			}

		case *ast.CallExpr:
			// An error can be wrapped. Checking…
			sigType, ok := c.getSupportFunctionSigType(v)
			if !ok {
				continue
			}

			switch sigType {
			case SigWrapTypeWrap:
				if c.checkWrapSignatureCall(call, err) {
					return true
				}
			case SigWrapTypeErrorf:
				if c.checkErrorfSignatureCall(call, err) {
					return true
				}
			default:
				continue
			}
		}
	}

	return false
}

func (c *knownErrWrapChecker) checkWrapSignatureCall(call *ast.CallExpr, err *ast.Ident) bool {
	if len(call.Args) < 2 {
		return false
	}

	switch v := call.Args[0].(type) {
	case *ast.Ident:
		return v.Name == err.Name

	case *ast.CallExpr:
		// An error can be wrapped. Let's do deep dive.
		sigType, ok := c.getSupportFunctionSigType(call)
		if !ok {
			return false
		}

		switch sigType {
		case SigWrapTypeWrap:
			return c.checkWrapSignatureCall(call, err)
		case SigWrapTypeErrorf:
			return c.checkErrorfSignatureCall(call, err)
		default:
			return false
		}

	default:
		return false
	}
}
