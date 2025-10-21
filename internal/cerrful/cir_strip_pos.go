package cerrful

// StripPos returns a deep copy of prog with all Pos fields zeroed.
// This is useful for equality testing (ignoring source positions).
func StripPos(p *CIRProgram) *CIRProgram {
	if p == nil {
		return nil
	}
	cp := *p
	cp.Functions = make([]CIRFunction, len(p.Functions))
	for i, fn := range p.Functions {
		cp.Functions[i] = CIRFunction{Name: fn.Name}
		for _, n := range fn.Nodes {
			cp.Functions[i].Nodes = append(cp.Functions[i].Nodes, stripNodePos(n))
		}
	}
	return &cp
}

func stripNodePos(n Node) Node {
	switch x := n.(type) {
	case Assign:
		x.Pos = Pos{}
		return x
	case Wrap:
		x.Pos = Pos{}
		return x
	case Return:
		x.Pos = Pos{}
		return x
	case Log:
		x.Pos = Pos{}
		return x
	case Check:
		x.Pos = Pos{}
		return x
	case If:
		x.Pos = Pos{}
		newThen := make([]Node, len(x.Then))
		for i, t := range x.Then {
			newThen[i] = stripNodePos(t)
		}
		newElse := make([]Node, len(x.Else))
		for i, e := range x.Else {
			newElse[i] = stripNodePos(e)
		}
		x.Then = newThen
		x.Else = newElse
		return x
	default:
		return n
	}
}
