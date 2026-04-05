package main

// deref follows a chain of REF pointers until it hits an unbound variable
// (a REF pointing to itself) or a non-REF cell. This is the WAM's walk().
// Bowen did this lazily via the AVL substitution; here it's a pointer chase
// through a flat array, which is O(chain length) but cache-friendly.
func (m *WAM) deref(c Cell) Cell {
	for c.IsREF() {
		next := m.Heap[c.Addr()]
		if next == c { // unbound: self-referential REF
			return c
		}
		c = next
	}
	return c
}

// push appends a cell to the heap and returns its address.
func (m *WAM) push(c Cell) int {
	addr := m.H
	m.Heap[m.H] = c
	m.H++
	return addr
}

// bind binds one unbound variable to a value. The WAM convention is to bind
// the newer variable (higher heap address) to the older one (lower address),
// avoiding dangling forward references.
func (m *WAM) bind(a, b Cell) {
	aAddr, bAddr := a.Addr(), b.Addr()
	if a.IsREF() && (!b.IsREF() || bAddr < aAddr) {
		m.Heap[aAddr] = b
		m.trailIfNeeded(aAddr)
	} else {
		m.Heap[bAddr] = a
		m.trailIfNeeded(bAddr)
	}
}

// trailIfNeeded records addr on the trail only if it is a *conditional*
// binding — i.e. the cell existed before the current choice point (addr < HB).
// Cells above HB will be reclaimed by resetting H on backtrack, so no explicit
// undo is needed for them. This is the key insight behind the trail.
func (m *WAM) trailIfNeeded(addr int) {
	if addr < m.HB {
		m.Trail[m.TR] = addr
		m.TR++
	}
}

// unwindTrail undoes all bindings recorded since trailMark, restoring each
// cell to an unbound self-referential REF. Called on backtracking.
func (m *WAM) unwindTrail(trailMark int) {
	for i := trailMark; i < m.TR; i++ {
		addr := m.Trail[i]
		m.Heap[addr] = REF(addr) // reset to unbound
	}
	m.TR = trailMark
}

// newVar pushes a new unbound variable onto the heap.
func (m *WAM) newVar() Cell {
	addr := m.H
	m.push(REF(addr)) // points to itself = unbound
	return REF(addr)
}
