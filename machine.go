package main

// Mode is READ (matching an existing heap term) or WRITE (building a new one).
// Bowen detected this implicitly from whether args was empty; the WAM makes it
// an explicit register toggled by get_structure / put_structure.
type Mode int

const (
	READ  Mode = iota
	WRITE Mode = iota
)

const (
	HeapSize  = 65536
	TrailSize = 65536
	NumXRegs  = 256
)

// Reg addresses a register. Non-negative = X (temporary), negative = Y (permanent).
// Y0 = -1, Y1 = -2, etc. Permanent variables live in the current environment frame,
// not in CPU registers, so they survive across CALL boundaries.
type Reg int

func X(n int) Reg { return Reg(n) }
func Y(n int) Reg { return Reg(-n - 1) }

// EnvironmentFrame holds the permanent variables of one clause activation.
// Pushed by ALLOCATE, popped by DEALLOCATE.
type EnvironmentFrame struct {
	CE int    // previous environment frame index (-1 = none)
	CP int    // continuation program counter
	Y  []Cell // permanent variables Y0..Y(n-1)
}

// ChoicePoint saves everything needed to retry a clause on backtracking.
// Pushed by TRY_ME_ELSE, updated by RETRY_ME_ELSE, popped by TRUST_ME.
type ChoicePoint struct {
	N          int    // number of saved argument registers
	CE         int    // saved environment frame index
	CP         int    // saved continuation PC
	B          int    // previous choice point index (-1 = none)
	TR         int    // trail top at point of creation
	H          int    // heap top at point of creation
	B0         int    // cut barrier at point of creation
	NextClause int    // label (PC) to jump to on retry
	Args       []Cell // saved A1..AN
}

// WAM is the machine state. Compare to Bowen's interpreter struct: that had
// procedures + an immutable substitution. Here we have a mutable heap, a
// trail, explicit environments, and choice points instead.
//
// Stack (Appendix B.3): the paper gives environments and choice points a single
// contiguous Stack region, with E and B as offsets into it. Frames of both
// kinds are interleaved in creation order, and deallocating an environment
// frame reclaims stack space that may sit below a live choice point. Here the
// two kinds are kept in separate Go slices (envFrames, choicePoints); E and B
// are indices into their respective slices. Lifetimes are managed by Go's GC
// rather than by stack trimming. The semantics are identical but the memory
// layout differs from the paper.
//
// PDL (Appendix B.3): the paper lists the PDL (Push-Down List used by unify)
// as a named region of machine memory. Here it is a local variable inside
// unify(), allocated fresh on each call and discarded on return. This is valid
// because the PDL never needs to persist between instructions; a local slice
// makes its transient nature explicit.
type WAM struct {
	Heap  [HeapSize]Cell
	Trail [TrailSize]int

	X [NumXRegs]Cell // temporary / argument registers

	H  int // heap top (next free slot)
	S  int // structure pointer (cursor during READ mode unification)
	HB int // heap top at last choice point (for trail condition)
	TR int // trail top
	P  int // program counter
	CP int // continuation PC (= return address)
	E  int // current environment frame index (-1 = none)
	B  int // current choice point index (-1 = none)
	B0 int // cut barrier

	Mode Mode

	Fail bool // failure flag; checked after each instruction

	Atoms  *AtomTable
	Code   []Instruction
	Labels map[string]int // named labels → PC offsets

	envFrames    []EnvironmentFrame
	choicePoints []ChoicePoint
}

func NewWAM() *WAM {
	return &WAM{
		Atoms:  NewAtomTable(),
		Labels: map[string]int{},
		E:      -1,
		B:      -1,
		B0:     -1,
	}
}

// Load installs a compiled Program into the machine, replacing any previously
// loaded code. The WAM adopts the program's atom table so that Cell values
// (which embed atom IDs) remain consistent at runtime.
func (m *WAM) Load(p *Program) {
	m.Code = p.Code
	m.Labels = p.Labels
	m.Atoms = p.Atoms
}

// getReg / setReg dispatch between X and Y registers.
func (m *WAM) getReg(r Reg) Cell {
	if r >= 0 {
		return m.X[r]
	}
	return m.envFrames[m.E].Y[-r-1]
}

func (m *WAM) setReg(r Reg, c Cell) {
	if r >= 0 {
		m.X[r] = c
		return
	}
	m.envFrames[m.E].Y[-r-1] = c
}
