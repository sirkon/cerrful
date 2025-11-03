package tmp

// State allow to control facts over error variable.
type State struct {
	facts map[string]*Facts
}

// NewState is [State] constructor.
func NewState() *State {
	return &State{}
}

// Var access a facts controller for the given variable.
func (s *State) Var(name string) *Facts {
	v, ok := s.facts[name]
	if !ok {
		v = &Facts{}
		s.facts[name] = v
	}

	return v
}
