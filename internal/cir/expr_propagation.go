package cir

// ExprNil represents an assignment of nil to an error variable.
//
//	err := nil
//	err = nil
type ExprNil struct{}

// ExprAlias represents error aliasing — a direct assignment from
// another error variable.
//
//	err := oldErr // Target: "oldErr"
//
// Only cases where Target refers to an error defined in the current
// function are classified as aliases. Assignments from an outer
// scope are considered ExprSentinel.
type ExprAlias struct {
	Target string
}

// ExprSentinel represents a reference to a sentinel error.
//
//	err := io.EOF // Ref: "io"."EOF"
type ExprSentinel struct {
	Ref Reference
}

// ExprType represents an error value constructed as a type
// implementing the [error] interface.
//
//	err := myerrs.MyError{…} // Ref: "path/to/myerrs"."MyError"
type ExprType struct {
	Ref Reference
}

// ExprCall represents a function call that produces an error value.
// Unregistered error constructors and wrappers belong to this category.
//
//	json.Unmarshal(data) // HasArgs: true, Ref: "json"."Unmarshal"
type ExprCall struct {
	HasArgs bool
	Ref     Reference
}

// ExprWrap represents a wrapping operation combining an error source
// and a message.
//
//	errors.Wrap(err, "do something")    // Src: <ExprFor>(err), Msg: "do something", Ref: "custom/errs/pkg"."Wrap"
//	fmt.Errorf("do something: %w", err) // Src: <ExprFor>(err), Msg: "do something", Ref: "fmt"."Errorf"
type ExprWrap struct {
	Src Statement
	Msg string
	Ref Reference
}

// ExprNew represents creation of a new error instance.
//
//	errors.New("error")  // Ref: "errors"."New"
//	fmt.Errorf("errorf") // Ref: "fmt"."Errorf"
type ExprNew struct {
	Ref Reference
}

func (*ExprNil) isNode()      {}
func (*ExprNil) isExpr()      {}
func (*ExprAlias) isNode()    {}
func (*ExprAlias) isExpr()    {}
func (*ExprSentinel) isNode() {}
func (*ExprSentinel) isExpr() {}
func (*ExprType) isNode()     {}
func (*ExprType) isExpr()     {}
func (*ExprCall) isNode()     {}
func (*ExprCall) isExpr()     {}
func (*ExprWrap) isNode()     {}
func (*ExprWrap) isExpr()     {}
func (*ExprNew) isNode()      {}
func (*ExprNew) isExpr()      {}
