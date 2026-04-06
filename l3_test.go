package main

import "testing"

// TestBacktracking checks TRY_ME_ELSE / TRUST_ME with two clauses for member/2:
//
//	member(X, [X|_]).
//	member(X, [_|T]) :- member(X, T).
//
// We hand-compile member and query member(b, [a,b,c]) expecting one answer.
func TestBacktracking(t *testing.T) {
	m := NewWAM()
	at := m.Atoms

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
