package main

import "testing"

// ── Step 1: Flatten ────────────────────────────────────────────────────────

// TestFlattenL0 verifies the register-assignment equations produced by
// FlattenL0 for the query term p(Z, h(Z,W), f(W)) from section 2.2 of the
// paper.
//
// Expected assignment (Figure 2.1, 1-indexed registers):
//
//	X1 ← p(X2, X3, X4)
//	X3 ← h(X2, X5)
//	X4 ← f(X5)
//	Z → X2,  W → X5
//
// The Eqs slice must be in BFS (top-down) order — p first, then h, then f —
// which is the natural order for program (GET_*) compilation.
func TestFlattenL0(t *testing.T) {
	at := NewAtomTable()
	ft := FlattenL0(MustParseQuery("p(Z, h(Z,W), f(W))"), at)

	if ft.Root != 1 {
		t.Errorf("Root = %d, want 1", ft.Root)
	}
	if len(ft.Eqs) != 3 {
		t.Fatalf("len(Eqs) = %d, want 3 (p, h, f)", len(ft.Eqs))
	}

	// Variable register assignments
	if ft.VarRegs["Z"] != 2 {
		t.Errorf("Z → X%d, want X2", ft.VarRegs["Z"])
	}
	if ft.VarRegs["W"] != 5 {
		t.Errorf("W → X%d, want X5", ft.VarRegs["W"])
	}

	// BFS order: p/3 at X1, then h/2 at X3, then f/1 at X4
	cases := []struct {
		reg   int
		arity int
		label string
	}{
		{1, 3, "p/3"},
		{3, 2, "h/2"},
		{4, 1, "f/1"},
	}
	for i, c := range cases {
		eq := ft.Eqs[i]
		if eq.Reg != c.reg {
			t.Errorf("Eqs[%d].Reg = %d, want %d (%s)", i, eq.Reg, c.reg, c.label)
		}
		if eq.Arity != c.arity {
			t.Errorf("Eqs[%d].Arity = %d, want %d (%s)", i, eq.Arity, c.arity, c.label)
		}
	}

	// p/3 args: X2 (Z), X3 (h), X4 (f)
	p := ft.Eqs[0]
	if p.Args[0].Reg != 2 {
		t.Errorf("p arg0 reg = %d, want 2 (Z)", p.Args[0].Reg)
	}
	if p.Args[1].Reg != 3 {
		t.Errorf("p arg1 reg = %d, want 3 (h)", p.Args[1].Reg)
	}
	if p.Args[2].Reg != 4 {
		t.Errorf("p arg2 reg = %d, want 4 (f)", p.Args[2].Reg)
	}

	// h/2 args: X2 (Z shared), X5 (W)
	h := ft.Eqs[1]
	if h.Args[0].Reg != 2 {
		t.Errorf("h arg0 reg = %d, want 2 (Z shared)", h.Args[0].Reg)
	}
	if h.Args[1].Reg != 5 {
		t.Errorf("h arg1 reg = %d, want 5 (W)", h.Args[1].Reg)
	}

	// f/1 args: X5 (W shared)
	f := ft.Eqs[2]
	if f.Args[0].Reg != 5 {
		t.Errorf("f arg0 reg = %d, want 5 (W shared)", f.Args[0].Reg)
	}
}

// TestFlattenL0Atom verifies that atoms are treated as zero-arity structures
// and assigned their own registers.  For h(f(a), b):
//
//	BFS: h/2 at X1, f/1 at X2, b/0 at X3, a/0 at X4
func TestFlattenL0Atom(t *testing.T) {
	at := NewAtomTable()
	ft := FlattenL0(MustParseQuery("h(f(a), b)"), at)

	if len(ft.Eqs) != 4 {
		t.Fatalf("len(Eqs) = %d, want 4 (h, f, b, a)", len(ft.Eqs))
	}
	if ft.Eqs[0].Reg != 1 || ft.Eqs[0].Arity != 2 {
		t.Errorf("eq[0] = reg %d arity %d, want reg 1 arity 2 (h/2)", ft.Eqs[0].Reg, ft.Eqs[0].Arity)
	}
	if ft.Eqs[1].Reg != 2 || ft.Eqs[1].Arity != 1 {
		t.Errorf("eq[1] = reg %d arity %d, want reg 2 arity 1 (f/1)", ft.Eqs[1].Reg, ft.Eqs[1].Arity)
	}
	// b and a are enqueued left-to-right from h's args, then a from f's args
	// BFS: h (X1), f (X2), b (X3), a (X4)
	if ft.Eqs[2].Reg != 3 || ft.Eqs[2].Arity != 0 {
		t.Errorf("eq[2] = reg %d arity %d, want reg 3 arity 0 (b/0)", ft.Eqs[2].Reg, ft.Eqs[2].Arity)
	}
	if ft.Eqs[3].Reg != 4 || ft.Eqs[3].Arity != 0 {
		t.Errorf("eq[3] = reg %d arity %d, want reg 4 arity 0 (a/0)", ft.Eqs[3].Reg, ft.Eqs[3].Arity)
	}
}

// ── Step 2: Reorder ────────────────────────────────────────────────────────

// TestReorderQueryL0 verifies that ReorderQueryL0 converts BFS order to
// DFS post-order for p(Z, h(Z,W), f(W)):
//
//	BFS:        p, h, f
//	Post-order: h, f, p   (inner structures before outer)
func TestReorderQueryL0(t *testing.T) {
	at := NewAtomTable()
	ft := FlattenL0(MustParseQuery("p(Z, h(Z,W), f(W))"), at)
	eqs := ReorderQueryL0(ft)

	if len(eqs) != 3 {
		t.Fatalf("len = %d, want 3", len(eqs))
	}

	// Post-order: h (X3) first, f (X4) second, p (X1) last
	wantRegs := []int{3, 4, 1}
	wantLabels := []string{"h/2", "f/1", "p/3"}
	for i, want := range wantRegs {
		if eqs[i].Reg != want {
			t.Errorf("eqs[%d].Reg = %d, want %d (%s)", i, eqs[i].Reg, want, wantLabels[i])
		}
	}
}

// TestReorderProgramL0 verifies that ReorderProgramL0 leaves the BFS order
// unchanged for p(Z, h(Z,W), f(W)):
//
//	BFS (and program order): p, h, f
func TestReorderProgramL0(t *testing.T) {
	at := NewAtomTable()
	ft := FlattenL0(MustParseQuery("p(Z, h(Z,W), f(W))"), at)
	eqs := ReorderProgramL0(ft)

	if len(eqs) != 3 {
		t.Fatalf("len = %d, want 3", len(eqs))
	}

	// BFS order: p (X1) first, h (X3) second, f (X4) last
	wantRegs := []int{1, 3, 4}
	wantLabels := []string{"p/3", "h/2", "f/1"}
	for i, want := range wantRegs {
		if eqs[i].Reg != want {
			t.Errorf("eqs[%d].Reg = %d, want %d (%s)", i, eqs[i].Reg, want, wantLabels[i])
		}
	}
}

// ── Step 3: Translate (via full pipeline) ─────────────────────────────────

// TestCompileQueryL0 verifies that CompileQueryL0 produces the exact
// instruction sequence from Figure 2.1 of the paper for ?- p(Z, h(Z,W), f(W)).
//
// Expected (register numbers match the paper's 1-indexed convention):
//
//	put_structure h/2, X3
//	set_variable X2        (Z, first occurrence)
//	set_variable X5        (W, first occurrence)
//	put_structure f/1, X4
//	set_value X5           (W, second occurrence)
//	put_structure p/3, X1
//	set_value X2           (Z)
//	set_value X3           (h/2 reference)
//	set_value X4           (f/1 reference)
func TestCompileQueryL0(t *testing.T) {
	at := NewAtomTable()
	h2 := at.Func("h", 2)
	f1 := at.Func("f", 1)
	p3 := at.Func("p", 3)

	got := CompileQueryL0(MustParseQuery("p(Z, h(Z,W), f(W))"), at)

	want := []Instruction{
		{PUT_STRUCTURE, h2, 3},
		{SET_VARIABLE, 2, 0},
		{SET_VARIABLE, 5, 0},
		{PUT_STRUCTURE, f1, 4},
		{SET_VALUE, 5, 0},
		{PUT_STRUCTURE, p3, 1},
		{SET_VALUE, 2, 0},
		{SET_VALUE, 3, 0},
		{SET_VALUE, 4, 0},
	}

	checkInstructions(t, got, want)
}

// TestCompileProgramL0 verifies that CompileProgramL0 produces the correct
// instruction sequence for p(f(X), h(Y, f(a)), Y).
//
// In L0, the atom a is treated as the zero-arity structure a/0, so it
// receives its own register (X7) and a get_structure a/0, X7 instruction
// rather than the unify_constant shorthand introduced in Chapter 5.
//
//	get_structure p/3, X1
//	unify_variable X2      (f(X))
//	unify_variable X3      (h(Y, f(a)))
//	unify_variable X4      (Y)
//	get_structure f/1, X2
//	unify_variable X5      (X)
//	get_structure h/2, X3
//	unify_value X4         (Y, second occurrence)
//	unify_variable X6      (f(a))
//	get_structure f/1, X6
//	unify_variable X7      (a, first occurrence)
//	get_structure a/0, X7  (a as zero-arity structure)
func TestCompileProgramL0(t *testing.T) {
	at := NewAtomTable()
	h2 := at.Func("h", 2)
	f1 := at.Func("f", 1)
	p3 := at.Func("p", 3)
	a0 := at.Func("a", 0)

	got := CompileProgramL0(MustParseQuery("p(f(X), h(Y, f(a)), Y)"), at)

	want := []Instruction{
		{GET_STRUCTURE, p3, 1},
		{UNIFY_VARIABLE, 2, 0},
		{UNIFY_VARIABLE, 3, 0},
		{UNIFY_VARIABLE, 4, 0},
		{GET_STRUCTURE, f1, 2},
		{UNIFY_VARIABLE, 5, 0},
		{GET_STRUCTURE, h2, 3},
		{UNIFY_VALUE, 4, 0},
		{UNIFY_VARIABLE, 6, 0},
		{GET_STRUCTURE, f1, 6},
		{UNIFY_VARIABLE, 7, 0},
		{GET_STRUCTURE, a0, 7},
	}

	checkInstructions(t, got, want)
}

// ── End-to-end correctness ─────────────────────────────────────────────────

// TestL0CompilerCorrectness runs the compiled instructions for the Chapter 2
// example end-to-end and verifies that the expected bindings are established:
//
//	Query:   ?- p(Z, h(Z,W), f(W))
//	Program:  p(f(X), h(Y, f(a)), Y).
//
//	Expected: Z = f(f(a)),  W = f(a)
//
// The query instructions build the term on the heap; the program instructions
// unify against it.  After execution, Z is in X2 and W is in X5 (per the
// register assignment from FlattenL0, matching the paper's convention).
// L0 has no PROCEED; execution halts because CP is set past the end of code.
func TestL0CompilerCorrectness(t *testing.T) {
	m := NewWAM()
	at := m.Atoms

	queryInstrs := CompileQueryL0(MustParseQuery("p(Z, h(Z,W), f(W))"), at)
	programInstrs := CompileProgramL0(MustParseQuery("p(f(X), h(Y, f(a)), Y)"), at)

	m.Code = append(queryInstrs, programInstrs...)
	m.P = 0
	m.CP = len(m.Code)
	m.Run()

	if m.Fail {
		t.Fatal("execution failed unexpectedly")
	}

	// Z → X2, W → X5 per the flatten register assignment (matches the paper).
	Z := m.ReadTerm(m.X[2])
	W := m.ReadTerm(m.X[5])

	if Z != "f(f(a))" {
		t.Errorf("Z: got %q, want %q", Z, "f(f(a))")
	}
	if W != "f(a)" {
		t.Errorf("W: got %q, want %q", W, "f(a)")
	}
	t.Logf("Z = %s", Z)
	t.Logf("W = %s", W)
}

// checkInstructions is a helper that compares two instruction slices element
// by element and reports differences.
func checkInstructions(t *testing.T, got, want []Instruction) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("len(got)=%d, len(want)=%d\ngot:  %v\nwant: %v", len(got), len(want), got, want)
	}
	for i, g := range got {
		if g != want[i] {
			t.Errorf("instr[%d]: got {%v,%d,%d}, want {%v,%d,%d}",
				i, g.Op, g.Arg1, g.Arg2, want[i].Op, want[i].Arg1, want[i].Arg2)
		}
	}
}
