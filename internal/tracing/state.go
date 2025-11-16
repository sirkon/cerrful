package tracing

import (
	"go/token"
)

// State for tracking interpretation states.
type State struct {
	errors map[string]*StateErrorFacts
	exits  map[token.Pos]*StateErrorFacts
}

// NewState is [State] constructor.
func NewState() *State {
	return &State{}
}

// Var access an errors controller for the given variable.
func (s *State) Var(name string) *StateErrorFacts {
	v, ok := s.errors[name]
	if !ok {
		v = &StateErrorFacts{}
		s.errors[name] = v
	}

	return v
}

func (s *State) Clone() *State {
	ns := NewState()

	ns.errors = make(map[string]*StateErrorFacts, len(s.errors))
	for k, v := range s.errors {
		ns.errors[k] = v.Clone()
	}

	ns.exits = make(map[token.Pos]*StateErrorFacts, len(s.exits))
	for k, v := range s.exits {
		ns.exits[k] = v.Clone()
	}

	return ns
}
