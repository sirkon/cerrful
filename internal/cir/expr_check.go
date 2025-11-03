package cir

// ErrorTypeIsCheck represents `errors.Is` (and equivalents) usage in a source code. The type is being dug
// is stored in Type, the [errors.Is] thing used referenced in Ref. The source of an error is in Src.
//
//	errors.Is(err, io.EOF) // Src: <ExprFor>(err), Type: "io"."EOF", Ref: "errors"."IO"
type ErrorTypeIsCheck struct {
	Src  Expr
	Type Reference
	Ref  Reference
}

// ErrorTypeIsSomeCheck represents registered or detected functions like `os.IsExist`.
//
//	os.IsExists(err) // Src: <ExprFor>(err), Type: "os"."IsExists".
type ErrorTypeIsSomeCheck struct {
	Src  Expr
	Type Reference
}

// ErrorValueIsNotNil represents "err != nil"
//
//	err != nil // Src: <ExprFor>(err)
type ErrorValueIsNotNil struct {
	Src Expr
}

// ErrorValueIsNil represents "err == nil"
//
//	err == nil // Src: <ExprFor>(err)
type ErrorValueIsNil struct {
	Src Expr
}

// ErrorValueEQ represents "err == io.EOF" or whatever type routine with.
//
//	err == io.EOF // Src: <ExprFor>(err), Type: "io"."EOF"
type ErrorValueEQ struct {
	Src  Expr
	Type Reference
}

// ErrorValueNEQ represents an inequality comparison between an error and
// a specific sentinel or typed value.
//
//	err != io.EOF // Src: <ExprFor>(err), Type: "io.EOF"
type ErrorValueNEQ struct {
	Src  Expr
	Type Reference
}

// ErrorTypeExtract represents `errors.As` (and equivalents) usage in a source code. The target
// is stored in Type, the [errors.As] thing used referenced in Ref. The source of an error is in Src.
//
//	errors.As(err, &target) // Src: <ExprFor>(err), Target: "target", Ref: "errors"."As"
//
// And these will be reported:
//
//	errors.As(err, io.EOF)
//	errors.As(err, os.Rename(…))
type ErrorTypeExtract struct {
	Src    Expr
	Target *ExprVar
	Ref    Reference
}

func (*ErrorTypeIsCheck) isNode()               {}
func (*ErrorTypeIsSomeCheck) isErrorTypeGuess() {}
func (*ErrorTypeIsSomeCheck) isNode()           {}
func (*ErrorTypeIsCheck) isErrorTypeGuess()     {}
func (*ErrorValueIsNotNil) isNode()             {}
func (*ErrorValueIsNil) isNode()                {}
func (*ErrorValueEQ) isNode()                   {}
func (*ErrorValueNEQ) isNode()                  {}
func (*ErrorValueIsNotNil) isExpr()             {}
func (*ErrorValueIsNil) isExpr()                {}
func (*ErrorValueEQ) isExpr()                   {}
func (*ErrorValueNEQ) isExpr()                  {}
func (*ErrorValueIsNotNil) isCheck()            {}
func (*ErrorValueIsNil) isCheck()               {}
func (*ErrorValueEQ) isCheck()                  {}
func (*ErrorValueNEQ) isCheck()                 {}
func (*ErrorTypeExtract) isNode()               {}
func (*ErrorTypeExtract) isErrorTypeGuess()     {}
