// Package tracing provides contextual and semantic tracing facilities
// built as an overlay on top of SSA form.
//
// It connects CIR-level structures with their SSA interpretation,
// allowing analyzers to reason about error states, propagation, and
// control flow without executing the program.
//
// The package maintains contextual information for each relevant AST
// node and collects facts about how errors are used, logged, or
// returned within a given scope.
//
// Core components:
//
//   - Context
//     Maintains associations between source nodes and their error
//     handling state. Tracks spans, ownership, and positional data.
//
//   - Error state tracker
//     Aggregates semantic facts about each error value: creation,
//     propagation, wrapping, logging, and return. Enables reasoning
//     about complete error lifecycles.
//
//   - CIR extraction
//     Identifies CIR nodes that correspond to error-handling
//     constructs and prepares them for SSA interpretation.
//
//   - SSA interpreter
//     Provides a symbolic tracer over SSA instructions, resolving
//     logical relationships and effects related to error states.
//
// The tracing layer acts as a bridge between CIR representation
// and the SSA form. It formalizes how error-related semantics are
// recovered, tracked, and reasoned about without program execution.
//
// This package is used internally by cerrful analyzers to reconstruct
// the logical flow of errors and validate handling discipline.
package tracing
