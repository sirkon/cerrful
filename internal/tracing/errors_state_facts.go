package tracing

import (
	"github.com/sirkon/cerrful/internal/cir"
)

// Facts keeps facts about an error variable.
type Facts struct {
	notNil    *bool
	takenCare *bool
	wrapped   bool
	classOf   map[cir.Reference]bool
}

// --- Setters --------------------------------------------------------------------------------------------------------

// SetNotNil adds a view for the error of being not nil.
//
// Possible issues are:
//
//   - Contradictory checks ("if err != nil" in the scope of "if err == nil").
//   - Duplicate checks. Something like "if err != nil" within another "if err != nil" scope for the same "err".
func (f *Facts) SetNotNil(isnotnil bool) FactSetNotNilStatus {
	if f.notNil == nil {
		f.notNil = &isnotnil
		return FactSetNotNilStatusOK
	}

	v := *f.notNil
	if v == isnotnil {
		return FactSetNotNilStatusDuplicate
	} else {
		return FactSetNotNilStatusContradict
	}
}

// SetTakenCare sets a variable as it was taken care of. The isReturned thing
// sets it to returned (true), or logged (false).
//
// Possible issues are:
//
//   - Logging and returning must not intermix.
//   - Logging can be done only once.
func (f *Facts) SetTakenCare(isReturned bool) FactSetTakenCareStatus {
	if f.takenCare != nil {
		if *f.takenCare {
			// It was returned before.
			return FactSetTakeCareStatusAlreadyReturned
		}

		return FactSetTakenCareStatusAlreadyLogged
	}

	f.takenCare = &isReturned
	return FactSetTakenCareStatusOK
}

// SetClass adds a new class error variable can represent.
//
// We can face the following issues in here:
//
//   - One error value cannot belong to multiple classes and be exact of any at the same time.
//   - Duplication of "belong to X" or "is X" is an issue. Here also:
//   - Upgrading "belong to X" into "is X" has dedicated report.
//   - Downgrading "is X" to "belong to X" also reported explicitly.
func (f *Facts) SetClass(class cir.Reference, exact bool) FactSetClassStatus {
	v, ok := f.classOf[class]
	if !ok {
		if exact {
			// We don't allow multiple exacts.
			for _, isExact := range f.classOf {
				if isExact {
					return FactSetClassStatusExactImpossible
				}
			}
		}

		f.classOf[class] = exact
		return FactSetClassStatusOK
	}

	switch {
	case !v && exact:
		// It was not exact and is turning into exact. Upgrade.
		return FactSetClassStatusDuplicateUpgrade
	case v && !exact:
		// It was exact and is turning into not exact. Downgrade.
		return FactSetClassStatusDuplicateDowngrade
	default:
		return FactSetClassStatusDuplicate
	}
}

// SetWrapped mark variable as wrapped.
//
// Multiple wraps are no issue.
func (f *Facts) SetWrapped() {
	f.wrapped = true
}

// --- Getters --------------------------------------------------------------------------------------------------------

// IsNotNil returns if the variable is known to be nil (false) or not nil (true). It returns nil
// if it is known at all – no "if err =/!= nil" checks were done.
func (f *Facts) IsNotNil() *bool {
	return f.notNil
}

// IsTakenCare returns if this variable has been taken care already, no matter the method.
func (f *Facts) IsTakenCare() bool {
	if f.takenCare == nil {
		return false
	}

	return true
}

// IsLogged returns true if this variable has been logged already.
func (f *Facts) IsLogged() bool {
	if f.takenCare == nil {
		return false
	}

	return !*f.takenCare
}

// IsReturned returns true if this variable has been returned already.
func (f *Facts) IsReturned() bool {
	if f.takenCare == nil {
		return false
	}

	return *f.takenCare
}

// IsClassOf returns true if the variable is known to belong to the class or being an exact value
// of this class.
func (f *Facts) IsClassOf(ref cir.Reference) bool {
	_, ok := f.classOf[ref]
	return ok
}

// Is returns true if the variable is known to have this exact type.
func (f *Facts) Is(ref cir.Reference) bool {
	v, ok := f.classOf[ref]
	if !ok {
		return false
	}

	return v
}

// IsWrapped returns true if this variable has been marked as wrapped.
func (f *Facts) IsWrapped() bool {
	return f.wrapped
}

// --- Types for status setting part ----------------------------------------------------------------------------------

// FactSetNotNilStatus represents possible issues that can be arisen when NotNil status was being set.
type FactSetNotNilStatus int

const (
	factSetNotNilStatusInvalid FactSetNotNilStatus = iota

	// FactSetNotNilStatusOK everyting is OK.
	FactSetNotNilStatusOK

	// FactSetNotNilStatusDuplicate expresses a situation in a source code when we do err != nil
	// check while being in a scope where it has been established already. Something like
	//
	//    if err != nil {
	//        if err != nil {
	//
	// Clearly report-worthy.
	FactSetNotNilStatusDuplicate

	// FactSetNotNilStatusContradict refers to a situation where we have established an
	// err == nil, and then we do err != nil check. Or vice versa. Needs to be reported this.
	FactSetNotNilStatusContradict
)

type FactSetTakenCareStatus int

const (
	factSetTakenCareStatusInvalid FactSetTakenCareStatus = iota

	// FactSetTakenCareStatusOK everything is intact.
	FactSetTakenCareStatusOK

	// FactSetTakeCareStatusAlreadyReturned describes a situation where an error was
	// returned already. Clearly indicated an unused code this. Needs to be reported.
	FactSetTakeCareStatusAlreadyReturned

	// FactSetTakenCareStatusAlreadyLogged refers to a state of error processing where
	// this error was already logged. Returns and further logging is prohibited thereafter
	// and this return state will trigger that "either log or return" enforcing policy.
	FactSetTakenCareStatusAlreadyLogged
)

type FactSetClassStatus int

const (
	factSetClassStatusInvalid FactSetClassStatus = iota

	// FactSetClassStatusOK a new class
	FactSetClassStatusOK

	// FactSetClassStatusDuplicate represents a case with multiple errors.Is digging for the same error type.
	FactSetClassStatusDuplicate

	// FactSetClassStatusDuplicateUpgrade represents a case where we have errors.Is first and go for
	// comparison/type assert/errors.As after. Why not to use errors.As from the start?
	FactSetClassStatusDuplicateUpgrade

	// FactSetClassStatusDuplicateDowngrade represents a case where we already have an exact type of the
	// variable computed. No need to dig with errors.Is after errors.As did this over the same type.
	// It is NOP semantically in the current context.
	FactSetClassStatusDuplicateDowngrade

	// FactSetClassStatusExactImpossible points to the fact it is impossible to have meaningful decomposition
	// for an error to have different exact types at the same time. Like, an error can't be both exact io.EOF
	// and io.ErrNoProgress at the same time. It can be originated from both (i.e. "belong to X"), but
	// not to be an exact match for both.
	FactSetClassStatusExactImpossible
)
