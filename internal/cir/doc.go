// Package cir defines structural types used to describe and locate
// error-handling semantics within Go source code.
//
// The entities in this package provide a consistent vocabulary for
// representing error-related constructs—such as creation, wrapping,
// checking, and propagation—within concrete code spans. Higher-level
// analyzers may use these definitions to identify and relate such
// fragments during source analysis.
package cir
