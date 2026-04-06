package main

import (
	"testing"
)

// TestSection2_2QueryTerm tests the L0 query term compilation from section 2.2
// of Aït-Kaci's WAM tutorial.
//
// The example query ?- p(Z, h(Z,W), f(W)) compiles to exactly 9 instructions
// (Figure 2.1). After they run, three things must hold:
//  1. X1 carries the full p/3 structure, readable as "p(_1,h(_1,_2),f(_2))".
//  2. Z (X2) and W (X5) are unbound variables.
//  3. Variable sharing is preserved: Z appears with the same heap address in
//     both p's first argument and h's first argument; likewise W in h's second
//     argument and f's argument.
func TestSection2_2QueryTerm(t *testing.T) {
	m := NewWAM()
	at := m.Atoms

	h2 := at.Func("h", 2)
	f1 := at.Func("f", 1)
	p3 := at.Func("p", 3)

	// Figure 2.3: compiled query ?- p(Z, h(Z,W), f(W))
	// Inner structures are built first (bottom-up) so that registers X3/X4
	// hold STR pointers before they are referenced by set_value.
	m.Code = []Instruction{
		{PUT_STRUCTURE, h2, 3}, // put_structure h/2, X3  — start h(Z,W)
		{SET_VARIABLE, 2, 0},   // set_variable  X2       — Z (first occurrence)
		{SET_VARIABLE, 5, 0},   // set_variable  X5       — W (first occurrence)
		{PUT_STRUCTURE, f1, 4}, // put_structure f/1, X4  — start f(W)
		{SET_VALUE, 5, 0},      // set_value     X5       — W (second occurrence)
		{PUT_STRUCTURE, p3, 1}, // put_structure p/3, X1  — start p(Z,h(Z,W),f(W))
		{SET_VALUE, 2, 0},      // set_value     X2       — Z
		{SET_VALUE, 3, 0},      // set_value     X3       — h(Z,W)
		{SET_VALUE, 4, 0},      // set_value     X4       — f(W)
	}
	m.P = 0
	m.CP = len(m.Code)
	m.Run()

	if m.Fail {
		t.Fatal("building query term should not fail")
	}

	// X1 must read as p/3 with Z shared in two places and W shared in two places.
	got := m.ReadTerm(m.X[1])
	want := "p(_1,h(_1,_2),f(_2))"
	if got != want {
		t.Errorf("X1 = %q, want %q", got, want)
	}

	// Z (X2) and W (X5) must still be unbound.
	zCell := m.deref(m.X[2])
	wCell := m.deref(m.X[5])
	if !zCell.IsREF() {
		t.Errorf("Z should be unbound REF, got %s", zCell)
	}
	if !wCell.IsREF() {
		t.Errorf("W should be unbound REF, got %s", wCell)
	}

	// Sharing: Z in p's first arg must be the same heap cell as X2.
	if m.deref(m.Heap[m.X[1].Addr()+1]) != zCell {
		t.Error("Z is not shared between X2 and p's first argument")
	}
	// Sharing: W in f's arg must be the same heap cell as X5.
	if m.deref(m.Heap[m.X[4].Addr()+1]) != wCell {
		t.Error("W is not shared between X5 and f's argument")
	}
}

// TestSection2_3ProgramClause tests the L0 program clause compilation from
// section 2.3 of Aït-Kaci's WAM tutorial.
//
// Program: p(f(X), h(Y, f(a)), Y).
//
// The compiled instructions (Figure 2.4) begin with get_structure to enter READ
// mode on the existing query structure, then use unify_variable / unify_value /
// unify_constant to traverse and bind.  Key correctness properties:
//  1. get_structure p/3, X1 enters READ mode because X1 already holds the
//     p/3 structure built by the query instructions.
//  2. Shared variables are handled correctly: Y appears as both the fourth
//     register loaded by unify_variable and the third argument matched by
//     unify_value inside h/2.
//  3. After proceed the bindings Z=f(f(a)) and W=f(a) are established.
func TestSection2_3ProgramClause(t *testing.T) {
	m := NewWAM()
	at := m.Atoms

	h2 := at.Func("h", 2)
	f1 := at.Func("f", 1)
	p3 := at.Func("p", 3)
	aAtm := ATM(at.Atom("a"))

	// Phase 1: build the query term ?- p(Z, h(Z,W), f(W)) on the heap.
	// This is identical to TestSection2_2QueryTerm; it leaves X1=STR→p/3
	// with Z in X2 and W in X5 as unbound variables.
	m.Code = []Instruction{
		{PUT_STRUCTURE, h2, 3}, // put_structure h/2, X3
		{SET_VARIABLE, 2, 0},   // set_variable  X2       — Z
		{SET_VARIABLE, 5, 0},   // set_variable  X5       — W
		{PUT_STRUCTURE, f1, 4}, // put_structure f/1, X4
		{SET_VALUE, 5, 0},      // set_value     X5       — W
		{PUT_STRUCTURE, p3, 1}, // put_structure p/3, X1
		{SET_VALUE, 2, 0},      // set_value     X2       — Z
		{SET_VALUE, 3, 0},      // set_value     X3       — h(Z,W)
		{SET_VALUE, 4, 0},      // set_value     X4       — f(W)
	}
	m.P = 0
	m.CP = len(m.Code)
	m.Run()
	if m.Fail {
		t.Fatal("query setup failed unexpectedly")
	}

	// Phase 2: run the program clause p(f(X), h(Y, f(a)), Y) against X1.
	// Figure 2.4: get_structure enters READ mode because X1 already holds
	// a p/3 structure; all subsequent unify_* instructions traverse it.
	m.Code = []Instruction{
		{GET_STRUCTURE, p3, 1},         // get_structure p/3, X1   — READ mode
		{UNIFY_VARIABLE, 2, 0},         // unify_variable X2       — f(X)
		{UNIFY_VARIABLE, 3, 0},         // unify_variable X3       — h(Y,f(a))
		{UNIFY_VARIABLE, 4, 0},         // unify_variable X4       — Y
		{GET_STRUCTURE, f1, 2},         // get_structure f/1, X2   — into f(X); X2=REF so WRITE mode, binds Z
		{UNIFY_VARIABLE, 5, 0},         // unify_variable X5       — X (fresh var)
		{GET_STRUCTURE, h2, 3},         // get_structure h/2, X3   — READ mode
		{UNIFY_VALUE, 4, 0},            // unify_value    X4       — Y = 3rd arg of p (shared)
		{UNIFY_VARIABLE, 6, 0},         // unify_variable X6       — f(a)
		{GET_STRUCTURE, f1, 6},         // get_structure f/1, X6   — into f(a); X6=REF so WRITE, binds W
		{UNIFY_CONSTANT, int(aAtm), 0}, // unify_constant a
		{PROCEED, 0, 0},
	}
	m.P = 0
	m.CP = len(m.Code)
	m.Run()

	if m.Fail {
		t.Fatal("program clause matching failed unexpectedly")
	}

	// Z (X2 at the time of the query) is bound through a chain:
	// Z → f(X), X → W, W → f(a) ⟹ Z = f(f(a))
	Z := m.ReadTerm(m.X[2])
	W := m.ReadTerm(m.X[6]) // X6 holds the W (program's f(a) binding)
	if Z != "f(f(a))" {
		t.Errorf("Z: got %q, want %q", Z, "f(f(a))")
	}
	if W != "f(a)" {
		t.Errorf("W: got %q, want %q", W, "f(a)")
	}
	t.Logf("Z = %s", Z)
	t.Logf("W = %s", W)
}

// TestChapter2L0 replicates the worked example from Hassan Aït-Kaci Chapter 2.
//
// Query:   ?- p(Z, h(Z,W), f(W))
// Program: p(f(X), h(Y, f(a)), Y).
//
// Expected bindings: Z=f(f(a)), W=f(a), X=f(a), Y=f(f(a))
//
// Compiled query (M0 instructions, Figure 2.1):
//
//	put_structure h/2, X3  ; build h(Z,W): push FUN h/2, set STR in X3
//	set_variable X2        ; Z (first occurrence) → new unbound var
//	set_variable X5        ; W (first occurrence) → new unbound var
//	put_structure f/1, X4  ; build f(W): push FUN f/1, set STR in X4
//	set_value X5           ; W (second occurrence) → same var as above
//	put_structure p/3, X1  ; build top-level call p(Z, h(Z,W), f(W))
//	set_value X2           ; Z
//	set_value X3           ; h(Z,W)
//	set_value X4           ; f(W)
//
// Compiled program (M0 instructions, Figure 2.4):
//
//	get_structure p/3, X1  ; match/build outer p/3
//	unify_variable X2      ; first arg  → load ref to f(X) into X2
//	unify_variable X3      ; second arg → load ref to h(Y,f(a)) into X3
//	unify_variable X4      ; third arg  → load ref to Y into X4
//	get_structure f/1, X2  ; examine f(X)
//	unify_variable X5      ; X
//	get_structure h/2, X3  ; examine h(Y, f(a))
//	unify_value X4         ; Y = third arg of p (same variable)
//	unify_variable X6      ; f(a)
//	get_structure f/1, X6  ; examine f(a)
//	unify_constant a       ; literal atom a
//	proceed
func TestChapter2L0(t *testing.T) {
	m := NewWAM()
	at := m.Atoms

	// Intern all atoms and functors we need.
	h2 := at.Func("h", 2)
	f1 := at.Func("f", 1)
	p3 := at.Func("p", 3)
	aAtm := ATM(at.Atom("a"))

	code := []Instruction{
		// ── Query: ?- p(Z, h(Z,W), f(W)) ──────────────────────────────────
		{PUT_STRUCTURE, h2, 3}, // put_structure h/2, X3
		{SET_VARIABLE, 2, 0},   // set_variable X2   (Z)
		{SET_VARIABLE, 5, 0},   // set_variable X5   (W)
		{PUT_STRUCTURE, f1, 4}, // put_structure f/1, X4
		{SET_VALUE, 5, 0},      // set_value X5      (W again)
		{PUT_STRUCTURE, p3, 1}, // put_structure p/3, X1
		{SET_VALUE, 2, 0},      // set_value X2      (Z)
		{SET_VALUE, 3, 0},      // set_value X3      (h(Z,W))
		{SET_VALUE, 4, 0},      // set_value X4      (f(W))
		// After these 9 instructions, X1 holds STR→p(...) on the heap.
		// We now "call" the procedure by falling through to the get_ instructions.

		// ── Program: p(f(X), h(Y, f(a)), Y) ───────────────────────────────
		{GET_STRUCTURE, p3, 1},         // get_structure p/3, X1
		{UNIFY_VARIABLE, 2, 0},         // unify_variable X2  (f(X))
		{UNIFY_VARIABLE, 3, 0},         // unify_variable X3  (h(Y,f(a)))
		{UNIFY_VARIABLE, 4, 0},         // unify_variable X4  (Y)
		{GET_STRUCTURE, f1, 2},         // get_structure f/1, X2
		{UNIFY_VARIABLE, 5, 0},         // unify_variable X5  (X)
		{GET_STRUCTURE, h2, 3},         // get_structure h/2, X3
		{UNIFY_VALUE, 4, 0},            // unify_value X4     (Y = third arg)
		{UNIFY_VARIABLE, 6, 0},         // unify_variable X6  (f(a))
		{GET_STRUCTURE, f1, 6},         // get_structure f/1, X6
		{UNIFY_CONSTANT, int(aAtm), 0}, // unify_constant a
		{PROCEED, 0, 0},
	}

	m.Code = code
	m.P = 0
	m.CP = len(code) // PROCEED will set P here, which exits the Run() loop
	m.Run()

	if m.Fail {
		t.Fatal("execution failed unexpectedly")
	}

	// Read back and check the bindings.
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
