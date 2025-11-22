package cir

// Return represents a return statement with an error expression.
//
// Examples:
//
//	return err                              // Val: "err"
//	return fmt.Errorf("open file: %w", err) // Val: Wrap("err", "open file")
type Return struct {
	Val Expr
}

func (*Return) isNode()      {}
func (*Return) isStatement() {}
