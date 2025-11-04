// Package cerrules defines the canonical rule codes (CER-series) enforced by cerrful.
// Each rule represents a distinct verification invariant in the analysis pipeline.
//
// Rule numbering scheme:
//
//	000–099  Structural propagation and wrapping
//	100–149  Message text and formatting rules
//	150–199  Logging and reporting discipline
package cerrules

import "fmt"

// Rule represents a cerrful rule code (CER-series).
type Rule int

const (
	ruleInvalid Rule = iota

	CER000NoSilentDrop
	CER010AnnotateExternal
	CER020SingleLocalPassthrough
	CER030MultiReturnMustAnnotate
	CER040AnnotationRequiredForExternalAndMultiLocal
	CER050HandleInNonErrorFunc
	CER060NoShadowingOrAliasing
	CER065FixBeforeUse
	CER100TextAndStyleRules
	CER150NoLogAndReturn
)

// String returns the canonical code and short name of the rule.
// Example: "CER000: NoSilentDrop"
func (r Rule) String() string {
	switch r {
	case CER000NoSilentDrop:
		return "CER000: NoSilentDrop"
	case CER010AnnotateExternal:
		return "CER010: AnnotateExternal"
	case CER020SingleLocalPassthrough:
		return "CER020: SingleLocalPassthrough"
	case CER030MultiReturnMustAnnotate:
		return "CER030: MultiReturnMustAnnotate"
	case CER040AnnotationRequiredForExternalAndMultiLocal:
		return "CER040: AnnotationRequiredForExternalAndMultiLocal"
	case CER050HandleInNonErrorFunc:
		return "CER050: HandleInNonErrorFunc"
	case CER060NoShadowingOrAliasing:
		return "CER060: NoShadowingOrAliasing"
	case CER065FixBeforeUse:
		return "CER065: FixBeforeUse"
	case CER100TextAndStyleRules:
		return "CER100–CER145: TextAndStyleRules"
	case CER150NoLogAndReturn:
		return "CER150: NoLogAndReturn"
	default:
		return fmt.Sprintf("unknown-rule(%d)", r)
	}
}

// Description returns the human-readable explanation of the rule.
func (r Rule) Description() string {
	switch r {
	case CER000NoSilentDrop:
		return "Error must never be ignored."
	case CER010AnnotateExternal:
		return "Wrap errors when crossing a semantic boundary."
	case CER020SingleLocalPassthrough:
		return "Bare return allowed only for single-path locals."
	case CER030MultiReturnMustAnnotate:
		return "Multi-return functions must annotate propagated errors."
	case CER040AnnotationRequiredForExternalAndMultiLocal:
		return "Enforce annotation for externals and multi-propagation locals."
	case CER050HandleInNonErrorFunc:
		return "Errors in non-error-returning funcs must be logged or panicked."
	case CER060NoShadowingOrAliasing:
		return "Reassigning or aliasing tracked errors is forbidden."
	case CER065FixBeforeUse:
		return "Fix error expression into a variable before control use."
	case CER100TextAndStyleRules:
		return "Message formatting and forbidden terms."
	case CER150NoLogAndReturn:
		return "Error must be either logged or returned, never both."
	default:
		return "<Unknown rule code>"
	}
}

// Canonical constructors — for readability and stable call sites.

func NoSilentDrop() Rule            { return CER000NoSilentDrop }
func AnnotateExternal() Rule        { return CER010AnnotateExternal }
func SingleLocalPassthrough() Rule  { return CER020SingleLocalPassthrough }
func MultiReturnMustAnnotate() Rule { return CER030MultiReturnMustAnnotate }
func AnnotationRequiredForExternalAndMultiLocal() Rule {
	return CER040AnnotationRequiredForExternalAndMultiLocal
}
func HandleInNonErrorFunc() Rule  { return CER050HandleInNonErrorFunc }
func NoShadowingOrAliasing() Rule { return CER060NoShadowingOrAliasing }
func FixBeforeUse() Rule          { return CER065FixBeforeUse }
func TextAndStyleRules() Rule     { return CER100TextAndStyleRules }
func NoLogAndReturn() Rule        { return CER150NoLogAndReturn }
