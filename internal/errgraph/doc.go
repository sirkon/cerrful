// Package errgraph provides means to build an error processing graph of a function.
//
// Nodes here are:
//
//   - Function start.
//   - Calls that can return errors and that are not error wraps or constructors.
//   - Defers.
//   - End of execution of any kind – return statements, panics, fatals, whatever.
//
// Edges are:
//
//   - Any code, that won't call for functions with error returns.
//
// So, you see, this graph is acyclic and looks very much like diamond.
package errgraph
