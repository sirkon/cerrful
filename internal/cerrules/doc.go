// Package cerrules defines the canonical CER-series rule codes enforced by cerrful.
//
// Each rule in cerrful represents a verifiable invariant of error handling logic.
// The CER-series provides a stable numeric and textual identity for every rule,
// ensuring that violations can be reported, filtered, and traced consistently
// across analysis passes, log output, and research tools.
//
// # Purpose
//
// The cerrules package serves as the single source of truth for all rule codes.
// It is used by:
//   - the static analyzer (for classification of findings);
//   - the reporter (for consistent emission of diagnostics);
//   - and documentation tools (for cross-linking rule references).
//
// # Structure
//
// Rule codes follow the format “CER<NNN>: <Name>” and are grouped by functional area:
//
//	000–099  Structural propagation and wrapping rules
//	100–149  Text and formatting style rules
//	150–199  Logging and reporting discipline
//
// Example:
//
//	cerrules.CER010AnnotateExternal.String()     → "CER010: AnnotateExternal"
//	cerrules.CER010AnnotateExternal.Description() → "Wrap errors when crossing a semantic boundary."
//
// # Usage
//
// Typical use in the analyzer:
//
//	if fn.HasIgnoredError() {
//	    report(cerrules.NoSilentDrop())
//	}
//
// Typical output in reporter:
//
//	CER000: NoSilentDrop — Error must never be ignored.
//
// # Notes
//
//   - Rule identifiers are stable and versioned; never renumber existing codes.
//   - New rules must follow the next available CER-range slot.
//   - Unknown or invalid codes render as "<Unknown rule code>" in descriptions.
//
// # Future direction
//
// The CER-series is considered complete. Its evolution lies not in new rules,
// but in heuristic layers built atop it — ranking the quality and depth
// of annotations rather than their mere presence. Such heuristics may form
// a separate class (HEU-series) evaluating annotation clarity, contextuality,
// and trace precision.
//
// cerrules is part of the cerrful core and is imported implicitly by the analysis toolchain.
package cerrules
