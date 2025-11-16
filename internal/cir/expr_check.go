package cir

// ErrorTypeIsCheck represents `errors.Is` (and equivalents) usage in a source code. The type is being dug
// is stored in Type, the [errors.Is] thing used referenced in Ref. The source of an error is in Src.
//
//	errors.Is(err, io.EOF) // Var: "err", Type: "io"."EOF", Ref: "errors"."IO"
type ErrorTypeIsCheck struct {
	Src  *ExprVar
	Type Reference

	Ref Reference
}

// ErrorTypeIsHelperCheck represents recognized and registered helpers that check for specific error types,
// such as os.IsNotExist.
//
//	os.IsExists(err) // Var: "err", Type: "os"."IsExists".
type ErrorTypeIsHelperCheck struct {
	Src *ExprVar
	Ref Reference
}

// ErrorValueIsNotNil represents "err != nil"
//
//	err != nil // Var: "err"
type ErrorValueIsNotNil struct {
	Src *ExprVar
}

// ErrorValueIsNil represents "err == nil"
//
//	err == nil // Var: "err"
type ErrorValueIsNil struct {
	Src *ExprVar
}

// ErrorValueEQ represents an equality comparison between an error and
// a specific sentinel or typed value.
//
//	err == io.EOF // Var: "err", RHS: "io"."EOF"
type ErrorValueEQ struct {
	Src *ExprVar
	RHS Expr
}

// ErrorValueNEQ represents an inequality comparison between an error and
// a specific sentinel or typed value.
//
//	err != io.EOF // Var: "err", Type: "io.EOF"
type ErrorValueNEQ struct {
	Src *ExprVar
	RHS Expr
}

// ErrorTypeExtract represents `errors.As` (and equivalents) usage in a source code. The target
// is stored in Type, the [errors.As] thing used referenced in Ref. The source of an error is in Src.
// Non-variable targets (calls, literals) are reported as semantic violations.
//
//	errors.As(err, &target) // Var: "err", Target: "target", Ref: "errors"."As"
//
// And these will be reported:
//
//	errors.As(err, io.EOF)
//	errors.As(err, os.Rename(…))
type ErrorTypeExtract struct {
	Src    *ExprVar
	Target *ExprVar
	Ref    Reference
}

func (*ErrorTypeIsCheck) isNode()                 {}
func (*ErrorTypeIsHelperCheck) isErrorTypeGuess() {}
func (*ErrorTypeIsHelperCheck) isNode()           {}
func (*ErrorTypeIsCheck) isErrorTypeGuess()       {}
func (*ErrorValueIsNotNil) isNode()               {}
func (*ErrorValueIsNil) isNode()                  {}
func (*ErrorValueEQ) isNode()                     {}
func (*ErrorValueNEQ) isNode()                    {}
func (*ErrorValueIsNotNil) isExpr()               {}
func (*ErrorValueIsNil) isExpr()                  {}
func (*ErrorValueEQ) isExpr()                     {}
func (*ErrorValueNEQ) isExpr()                    {}
func (*ErrorValueIsNotNil) isCheck()              {}
func (*ErrorValueIsNil) isCheck()                 {}
func (*ErrorValueEQ) isCheck()                    {}
func (*ErrorValueNEQ) isCheck()                   {}
func (*ErrorTypeExtract) isNode()                 {}
func (*ErrorTypeExtract) isErrorTypeGuess()       {}
