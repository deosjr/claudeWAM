package main

import (
	"testing"
)

// TestFigure2_4 replicates the worked example from Hassan Aït-Kaci Chapter 2.
//
// Query:   ?- p(Z, h(Z,W), f(W))
// Program: p(f(X), h(Y, f(a)), Y).
//
// Expected bindings: Z=f(f(a)), W=f(a), X=f(a), Y=f(f(a))
//
// Compiled query (M0 instructions, Figure 2.1):
//   put_structure h/2, X3  ; build h(Z,W): push FUN h/2, set STR in X3
//   set_variable X2        ; Z (first occurrence) → new unbound var
//   set_variable X5        ; W (first occurrence) → new unbound var
//   put_structure f/1, X4  ; build f(W): push FUN f/1, set STR in X4
//   set_value X5           ; W (second occurrence) → same var as above
//   put_structure p/3, X1  ; build top-level call p(Z, h(Z,W), f(W))
//   set_value X2           ; Z
//   set_value X3           ; h(Z,W)
//   set_value X4           ; f(W)
//
// Compiled program (M0 instructions, Figure 2.4):
//   get_structure p/3, X1  ; match/build outer p/3
//   unify_variable X2      ; first arg  → load ref to f(X) into X2
//   unify_variable X3      ; second arg → load ref to h(Y,f(a)) into X3
//   unify_variable X4      ; third arg  → load ref to Y into X4
//   get_structure f/1, X2  ; examine f(X)
//   unify_variable X5      ; X
//   get_structure h/2, X3  ; examine h(Y, f(a))
//   unify_value X4         ; Y = third arg of p (same variable)
//   unify_variable X6      ; f(a)
//   get_structure f/1, X6  ; examine f(a)
//   unify_constant a       ; literal atom a
//   proceed
func TestFigure2_4(t *testing.T) {
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
		{SET_VARIABLE, 2, 0},  // set_variable X2   (Z)
		{SET_VARIABLE, 5, 0},  // set_variable X5   (W)
		{PUT_STRUCTURE, f1, 4}, // put_structure f/1, X4
		{SET_VALUE, 5, 0},     // set_value X5      (W again)
		{PUT_STRUCTURE, p3, 1}, // put_structure p/3, X1
		{SET_VALUE, 2, 0},     // set_value X2      (Z)
		{SET_VALUE, 3, 0},     // set_value X3      (h(Z,W))
		{SET_VALUE, 4, 0},     // set_value X4      (f(W))
		// After these 9 instructions, X1 holds STR→p(...) on the heap.
		// We now "call" the procedure by falling through to the get_ instructions.

		// ── Program: p(f(X), h(Y, f(a)), Y) ───────────────────────────────
		{GET_STRUCTURE, p3, 1}, // get_structure p/3, X1
		{UNIFY_VARIABLE, 2, 0}, // unify_variable X2  (f(X))
		{UNIFY_VARIABLE, 3, 0}, // unify_variable X3  (h(Y,f(a)))
		{UNIFY_VARIABLE, 4, 0}, // unify_variable X4  (Y)
		{GET_STRUCTURE, f1, 2}, // get_structure f/1, X2
		{UNIFY_VARIABLE, 5, 0}, // unify_variable X5  (X)
		{GET_STRUCTURE, h2, 3}, // get_structure h/2, X3
		{UNIFY_VALUE, 4, 0},   // unify_value X4     (Y = third arg)
		{UNIFY_VARIABLE, 6, 0}, // unify_variable X6  (f(a))
		{GET_STRUCTURE, f1, 6}, // get_structure f/1, X6
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

// TestBacktracking checks TRY_ME_ELSE / TRUST_ME with two clauses for member/2:
//
//   member(X, [X|_]).
//   member(X, [_|T]) :- member(X, T).
//
// We hand-compile member and query member(b, [a,b,c]) expecting one answer.
func TestBacktracking(t *testing.T) {
	m := NewWAM()
	at := m.Atoms

	nilAtm := ATM(at.Atom("[]"))
	_ = nilAtm
	aAtm := ATM(at.Atom("a"))
	bAtm := ATM(at.Atom("b"))
	cAtm := ATM(at.Atom("c"))
	member2 := at.Func("member", 2)

	// Build list [a,b,c] on the heap directly.
	// LIS(addr) → [Heap[addr], Heap[addr+1]]
	//   cCell at 0: ATM c
	//   nilCell ... we store [] as ATM("[]")
	// Layout (heap addresses 0..5):
	//   0: ATM c
	//   1: ATM []   (tail of [c])
	//   2: ATM b
	//   3: LIS(0)   (tail of [b,...] = [c])
	//   4: ATM a
	//   5: LIS(2)   (tail of [a,...] = [b,c])
	// So [a,b,c] = LIS(4)
	m.Heap[0] = cAtm
	m.Heap[1] = ATM(at.Atom("[]"))
	m.Heap[2] = bAtm
	m.Heap[3] = LIS(0)
	m.Heap[4] = aAtm
	m.Heap[5] = LIS(2)
	m.H = 6
	list_abc := LIS(4)

	// A0 = b (what we search for), A1 = [a,b,c]
	m.X[0] = bAtm
	m.X[1] = list_abc

	// Hand-compiled code. Labels (PC offsets) must be set after laying out code.
	// We use placeholder offsets and fill them in.
	//
	// member/2 entry:
	//   0: try_me_else [clause2]    ; clause 1: member(X, [X|_])
	//   1: get_list A1              ;   match [X|_] in A1
	//   2: unify_value A0           ;   head = X (already in A0)
	//   3: unify_void 1             ;   tail = _
	//   4: proceed
	//
	//   5: trust_me                 ; clause 2: member(X, [_|T]) :- member(X,T)
	//   6: get_list A1              ;   match [_|T] in A1
	//   7: unify_void 1             ;   head = _
	//   8: unify_variable X2        ;   tail = T → X2
	//   9: put_value A0, A0         ;   X stays in A0
	//  10: put_value X2, A1         ;   T → A1
	//  11: execute member/2         ;   tail call

	clause2PC := 5
	code := []Instruction{
		/* 0 */ {TRY_ME_ELSE, clause2PC, 2},
		/* 1 */ {GET_LIST, 1, 0},
		/* 2 */ {UNIFY_VALUE, 0, 0},
		/* 3 */ {UNIFY_VOID, 1, 0},
		/* 4 */ {PROCEED, 0, 0},
		/* 5 */ {TRUST_ME, 0, 0},
		/* 6 */ {GET_LIST, 1, 0},
		/* 7 */ {UNIFY_VOID, 1, 0},
		/* 8 */ {UNIFY_VARIABLE, 2, 0},
		/* 9 */ {PUT_VALUE, 0, 0},
		/* 10 */ {PUT_VALUE, 2, 1},
		/* 11 */ {EXECUTE, member2, 2},
	}

	m.Code = code
	m.Labels[at.FuncEntry(member2).String()] = 0
	m.P = 0
	m.CP = len(code) // return to "past the end" = halt

	m.Run()

	if m.Fail {
		t.Error("member(b, [a,b,c]) should succeed but failed")
	} else {
		t.Log("member(b, [a,b,c]) succeeded")
	}
}
