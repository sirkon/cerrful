package cir

// Assign represents all assignments having error value at its expectable position – the last one.
// Here are assignments what would be described this way:
//
//	data, err := os.ReadFile(…)         // Dst: &ExprVar{Name: "err"}, Var: &ExprCall{…}
//	err := os.Rename(oldname, newname)
//	_, err := writer.Write(…)
//
// And these will not be represented as such:
//
//	myErr, ok := err.(*MyError) // Because the error is not the last value.
//	xp2 := math.Pow(x, 2)       // No errors at all in return values.
type Assign struct {
	Dst ExprVar
	Src Expr
}

// AssignCheckFlag represents an assignment where the Dest holds the variable and the Source holds RHS
// for these kind of statements:
//
//	ok := errors.Is(errExpr, someErrType)   // Dst: "ok", Var: <ExprFor>(errors.Is(…))
//	ok := errors.As(errExpr, &targetErrVar)
//	ok := errors.IsExists(errExpr)
//
// and so on.
type AssignCheckFlag struct {
	Dst string
	Src ErrorTypeGuess
}

// AssignAssert represents type assertion over error in a source code. This node may vary
// depending on whether the assertion guard was used or not:
//
//   - v, ok := expr.(someErrorType) // Dst: "v", Guard: "ok", Src: "expr", Type: "someErrorType"
//   - v := expr.(someErrorType)     // … Guard:"" …
type AssignAssert struct {
	Dst   ErrorVarNode
	Guard string
	Src   Expr
	Type  Reference
}

func (*Assign) isNode()               {}
func (*Assign) isStatement()          {}
func (*AssignCheckFlag) isNode()      {}
func (*AssignCheckFlag) isStatement() {}
func (*AssignAssert) isNode()         {}
func (*AssignAssert) isStatement()    {}
