package main

func (m *WAM) Run() {
	m.Fail = false
	for m.P >= 0 && m.P < len(m.Code) && !m.Fail {
		instr := m.Code[m.P]
		m.P++
		m.exec(instr)
		if m.Fail {
			m.backtrack()
		}
	}
}

func (m *WAM) exec(i Instruction) {
	r1 := Reg(i.Arg1)
	r2 := Reg(i.Arg2)

	switch i.Op {

	// ── Chapter 2: L0 ─────────────────────────────────────────────────────

	case PUT_STRUCTURE:
		// Push a FUN cell and point Xi at it via STR.
		// This begins a structure block on the heap; subsequent SET_* instructions
		// fill in the arguments.
		//
		// The paper's pseudocode first writes a STR cell to the heap at HEAP[H]
		// pointing forward to HEAP[H+1] (the FUN cell), then copies that STR cell
		// into Xi, consuming two heap slots per structure header. Here we skip the
		// intermediate heap write: only the FUN cell is pushed, and STR(addr) is
		// constructed directly into Xi. The invariant is the same — Xi holds
		// STR(address_of_FUN) — but the heap is one cell more compact.
		m.setReg(r2, m.newStr(i.Arg1))
		m.Mode = WRITE

	case SET_VARIABLE:
		// First occurrence of a variable in a query: allocate fresh unbound cell.
		v := m.newVar()
		m.setReg(r1, v)

	case SET_VALUE:
		// Subsequent occurrence: push the already-known value.
		m.push(m.getReg(r1))

	case GET_STRUCTURE:
		// The key instruction: either we're matching an existing structure (READ)
		// or binding an unbound variable to a new one (WRITE).
		d := m.deref(m.getReg(r2))
		switch d.Tag() {
		case TagREF:
			// Unbound variable: push new structure on heap and bind.
			m.bind(d, m.newStr(i.Arg1))
			m.Mode = WRITE
		case TagSTR:
			funCell := m.Heap[d.Addr()]
			if funCell.Addr() == i.Arg1 {
				m.S = d.Addr() + 1 // point S at first argument cell
				m.Mode = READ
			} else {
				m.Fail = true
			}
		default:
			m.Fail = true
		}

	case UNIFY_VARIABLE:
		switch m.Mode {
		case READ:
			m.setReg(r1, m.Heap[m.S])
			m.S++
		case WRITE:
			v := m.newVar()
			m.setReg(r1, v)
		}

	case UNIFY_VALUE:
		switch m.Mode {
		case READ:
			if !m.unify(m.getReg(r1), m.Heap[m.S]) {
				m.Fail = true
			}
			m.S++
		case WRITE:
			m.push(m.getReg(r1))
		}

	// ── Chapter 3: L1 ─────────────────────────────────────────────────────

	case PUT_VARIABLE:
		// First occurrence of a variable in a clause body: push a fresh unbound
		// var onto the heap and store it in BOTH the X register AND the arg register.
		v := m.newVar()
		m.setReg(r1, v)
		m.setReg(r2, v)

	case PUT_VALUE:
		m.setReg(r2, m.getReg(r1))

	case PUT_UNSAFE_VALUE:
		// Like PUT_VALUE but if the Y variable is still local to this environment
		// (above HB) we must globalise it by pushing a copy onto the heap.
		// This prevents a dangling reference if the environment is deallocated
		// before the variable is used.
		v := m.deref(m.getReg(r1))
		if v.IsREF() && v.Addr() >= m.HB {
			globalised := m.newVar()
			m.bind(v, globalised)
			v = globalised
		}
		m.setReg(r2, v)

	case GET_VARIABLE:
		m.setReg(r1, m.getReg(r2))

	case GET_VALUE:
		if !m.unify(m.getReg(r1), m.getReg(r2)) {
			m.Fail = true
		}

	// ── Chapter 4: L2 ─────────────────────────────────────────────────────

	case ALLOCATE:
		// Create an environment frame. Permanent variables (those that span a
		// CALL) live here, not in X registers.
		frame := EnvironmentFrame{
			CE: m.E,
			CP: m.CP,
			Y:  make([]Cell, i.Arg1),
		}
		// Initialise permanent variables as unbound heap vars.
		for k := range frame.Y {
			frame.Y[k] = m.newVar()
		}
		m.envFrames = append(m.envFrames, frame)
		m.E = len(m.envFrames) - 1

	case DEALLOCATE:
		frame := m.envFrames[m.E]
		m.CP = frame.CP
		m.E = frame.CE

	case CALL:
		// Arg1 = functor id (procedure to call), Arg2 = arity.
		// Save the return address (current P, already incremented past CALL).
		m.CP = m.P
		label, ok := m.procLabel(i.Arg1)
		if !ok {
			m.Fail = true
			return
		}
		m.P = label

	case EXECUTE:
		// Last-call optimisation: CALL + DEALLOCATE in one instruction.
		// The environment frame is popped before jumping, so it can be reused.
		if m.E >= 0 {
			frame := m.envFrames[m.E]
			m.CP = frame.CP
			m.E = frame.CE
		}
		label, ok := m.procLabel(i.Arg1)
		if !ok {
			m.Fail = true
			return
		}
		m.P = label

	case PROCEED:
		// Return: restore P from CP.
		m.P = m.CP

	// ── Chapter 5: backtracking ────────────────────────────────────────────

	case TRY_ME_ELSE:
		// Save current state in a new choice point. On failure, backtrack() will
		// restore everything and jump to Arg1.
		cp := ChoicePoint{
			N:          i.Arg2,
			CE:         m.E,
			CP:         m.CP,
			B:          m.B,
			TR:         m.TR,
			H:          m.H,
			B0:         m.B0,
			NextClause: i.Arg1,
			Args:       make([]Cell, i.Arg2),
		}
		for k := 0; k < i.Arg2; k++ {
			cp.Args[k] = m.X[k]
		}
		m.choicePoints = append(m.choicePoints, cp)
		m.B = len(m.choicePoints) - 1
		m.HB = m.H

	case RETRY_ME_ELSE:
		m.choicePoints[m.B].NextClause = i.Arg1

	case TRUST_ME:
		// Last alternative: pop the choice point entirely.
		m.choicePoints = m.choicePoints[:m.B]
		if m.B > 0 {
			m.B = m.B - 1
		} else {
			m.B = -1
		}
		m.HB = 0
		if m.B >= 0 {
			m.HB = m.choicePoints[m.B].H
		}

	case TRY:
		cp := ChoicePoint{
			N:          i.Arg2,
			CE:         m.E,
			CP:         m.CP,
			B:          m.B,
			TR:         m.TR,
			H:          m.H,
			B0:         m.B0,
			NextClause: m.P,
			Args:       make([]Cell, i.Arg2),
		}
		for k := 0; k < i.Arg2; k++ {
			cp.Args[k] = m.X[k]
		}
		cp.NextClause = i.Arg1
		m.choicePoints = append(m.choicePoints, cp)
		m.B = len(m.choicePoints) - 1
		m.HB = m.H
		m.P = i.Arg1

	case RETRY:
		m.restoreChoicePoint()
		m.choicePoints[m.B].NextClause = i.Arg1
		m.P = i.Arg1

	case TRUST:
		m.restoreChoicePoint()
		m.choicePoints = m.choicePoints[:m.B]
		m.B--
		if m.B < 0 {
			m.HB = 0
		} else {
			m.HB = m.choicePoints[m.B].H
		}
		m.P = i.Arg1

	case NECK_CUT:
		if m.B > m.B0 {
			m.B = m.B0
			m.tidyTrail()
		}

	case GET_LEVEL:
		// Store the current choice point index in a permanent variable.
		m.setReg(r1, Cell(m.B+1)) // +1 so 0 means "no choice point"

	case CUT:
		cutB := int(m.getReg(r1)) - 1
		if m.B > cutB {
			m.B = cutB
			m.tidyTrail()
		}

	// ── Chapter 5, Figure 5.2: constants ──────────────────────────────────

	case SET_CONSTANT:
		m.push(Cell(i.Arg1))

	case UNIFY_CONSTANT:
		switch m.Mode {
		case READ:
			if !m.unify(Cell(i.Arg1), m.Heap[m.S]) {
				m.Fail = true
			}
			m.S++
		case WRITE:
			m.push(Cell(i.Arg1))
		}

	case GET_CONSTANT:
		d := m.deref(m.getReg(r2))
		if !m.unify(Cell(i.Arg1), d) {
			m.Fail = true
		}

	case PUT_CONSTANT:
		m.setReg(r2, Cell(i.Arg1))

	// ── Chapter 5, Figure 5.3: lists ──────────────────────────────────────

	case PUT_LIST:
		// List cell's value is the heap address of the head; tail is at addr+1.
		m.setReg(r1, LIS(m.H))
		m.Mode = WRITE

	case SET_LIST:
		// Push a LIS cell pointing at the current heap top (head will follow).
		// Used inside a SET_* sequence (no register target).
		m.push(LIS(m.H + 1))

	case GET_LIST:
		d := m.deref(m.getReg(r1))
		switch d.Tag() {
		case TagREF:
			m.bind(d, LIS(m.H))
			m.Mode = WRITE
		case TagLIS:
			m.S = d.Addr()
			m.Mode = READ
		default:
			m.Fail = true
		}

	// ── Chapter 5, Figure 5.6: void variables ─────────────────────────────

	case SET_VOID:
		for k := 0; k < i.Arg1; k++ {
			m.newVar()
		}

	case UNIFY_VOID:
		switch m.Mode {
		case READ:
			m.S += i.Arg1
		case WRITE:
			for k := 0; k < i.Arg1; k++ {
				m.newVar()
			}
		}
	}
}

// backtrack restores the machine state from the most recent choice point.
func (m *WAM) backtrack() {
	if m.B < 0 {
		m.P = -1 // no choice point: halt with failure
		return
	}
	m.restoreChoicePoint()
	m.P = m.choicePoints[m.B].NextClause
	m.Fail = false
}

func (m *WAM) restoreChoicePoint() {
	cp := m.choicePoints[m.B]
	for k := 0; k < cp.N; k++ {
		m.X[k] = cp.Args[k]
	}
	m.E = cp.CE
	m.CP = cp.CP
	m.unwindTrail(cp.TR)
	m.H = cp.H  // discard heap cells above the saved top
	m.HB = cp.H
	m.B0 = cp.B0
	m.TR = cp.TR
}

// tidyTrail removes trail entries that are now above HB (no longer conditional).
func (m *WAM) tidyTrail() {
	newTR := m.TR
	for i := m.TR - 1; i >= 0; i-- {
		if m.Trail[i] < m.HB {
			newTR = i + 1
			break
		}
		newTR = i
	}
	m.TR = newTR
}

// procLabel looks up the entry point for a procedure by its functor id.
// Returns (-1, false) if not found (triggers failure, not panic).
func (m *WAM) procLabel(functorID int) (int, bool) {
	f := m.Atoms.FuncEntry(functorID)
	key := f.String()
	label, ok := m.Labels[key]
	return label, ok
}
