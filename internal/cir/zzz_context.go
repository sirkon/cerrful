package cir

// Context holds all CIR statements collected for a single analysis scope.
// It serves as a lightweight container for statement nodes before tracing.
type Context struct {
	nodes []Statement
}
