package cir

// ExprVar represents an explicit error variable reference,
// such as `err` in either a check (`if err != nil`) or an assignment (`err := call()`).
type ExprVar struct {
	// Name is the identifier of the error variable in the current scope.
	Name string
}

// ExprVarHidden represents an implicit or intentionally ignored error variable,
// such as `_` in an assignment like `data, _ := os.ReadFile(path)`.
//
// This node is used to track places where an error result is explicitly discarded.
type ExprVarHidden struct{}

// Interface markers.
func (*ExprVar) isNode()               {}
func (*ExprVar) isExpr()               {}
func (*ExprVar) isErrorVarNode()       {}
func (*ExprVarHidden) isNode()         {}
func (*ExprVarHidden) isExpr()         {}
func (*ExprVarHidden) isErrorVarNode() {}
