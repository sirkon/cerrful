package cir

// ExprVar represents an explicit error variable reference,
// such as `err` in either a check (`if err != nil`) or an assignment (`err := call()`).
type ExprVar struct {
	// Name is the identifier of the error variable in the current scope.
	Name string
}

// Interface markers.
func (*ExprVar) isNode()         {}
func (*ExprVar) isExpr()         {}
func (*ExprVar) isErrorVarNode() {}
