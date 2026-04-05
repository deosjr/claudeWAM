package main

// unify is the WAM's iterative unification algorithm (Section 2.2).
// It uses an explicit work-stack (PDL) rather than Go call-stack recursion.
// Pairs of cells to unify are pushed; each iteration pops one pair and
// either succeeds trivially, binds a variable, or pushes subterm pairs.
//
// Bowen's substitution.unify did the same thing recursively through the
// AVL tree. Here the "heap IS the substitution" — bindings are in-place.
func (m *WAM) unify(a1, a2 Cell) bool {
	pdl := []Cell{a1, a2}
	for len(pdl) >= 2 {
		d1 := m.deref(pdl[len(pdl)-2])
		d2 := m.deref(pdl[len(pdl)-1])
		pdl = pdl[:len(pdl)-2]

		if d1 == d2 {
			continue // identical cells (same unbound var, same atom, etc.)
		}

		// At least one is an unbound variable: bind and continue.
		if d1.IsREF() || d2.IsREF() {
			m.bind(d1, d2)
			continue
		}

		// Both are non-variables. Tags must match.
		if d1.Tag() != d2.Tag() {
			return false
		}

		switch d1.Tag() {
		case TagATM, TagINT:
			// Atoms and integers must be identical (already checked d1 != d2).
			return false

		case TagSTR:
			// Both are structure pointers. The FUN cell at the pointed-to address
			// encodes functor+arity. They must match; if so push all argument pairs.
			f1 := m.Heap[d1.Addr()]
			f2 := m.Heap[d2.Addr()]
			if f1 != f2 {
				return false
			}
			arity := m.Atoms.FuncEntry(f1.Addr()).Arity
			for i := 1; i <= arity; i++ {
				pdl = append(pdl, m.Heap[d1.Addr()+i], m.Heap[d2.Addr()+i])
			}

		case TagLIS:
			// List cell points to [head, tail] pair. Push both.
			pdl = append(pdl,
				m.Heap[d1.Addr()], m.Heap[d2.Addr()],
				m.Heap[d1.Addr()+1], m.Heap[d2.Addr()+1],
			)

		default:
			return false
		}
	}
	return true
}
