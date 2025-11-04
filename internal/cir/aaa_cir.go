package cir

// Node is the base interface implemented by all CIR node types.
// Each node denotes a single error-handling construct identified in Go source code
// (e.g., assignment, creation, wrapping, check, propagation).
type Node interface {
	isNode()
}

// Statement marks nodes that represent Go statements
// participating in error-handling semantics.
type Statement interface {
	isStatement()
}

// Expr marks nodes representing error expressions,
// such as wraps, sentinels, constructor calls, etc.
type Expr interface {
	isExpr()
}

// ErrorVarNode marks nodes that identify variables participating in
// error-handling flow — e.g., `err` in `if err != nil` or `err := call()`.
type ErrorVarNode interface {
	isErrorVarNode()
}

// ErrorTypeGuess marks nodes responsible for deducing the type of error,
// when its nature cannot be directly inferred from syntax.
type ErrorTypeGuess interface {
	isErrorTypeGuess()
}

// Reference identifies a declared entity in Go source code, such as
// a function, type, variable, or constant. It is used to attribute
// CIR nodes to the symbols they relate to.
type Reference struct {
	// Package is the import path of the package that declares the entity
	// (e.g., "io", "fmt", or "example.com/project/module").
	Package string

	// Type is type package-local name. It is needed when some method of
	// a type should be referenced. Will be empty for free functions and
	// variables/constants.
	Type string

	// Name is the declared identifier of the entity within its package.
	Name string
}
