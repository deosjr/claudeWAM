package main

import (
	"testing"
)

// TestCompilerAppend checks that append/3 compiled from source produces the
// right list.
//   append([], L, L).
//   append([H|T], L, [H|R]) :- append(T, L, R).
//   ?- append([a,b], [c], L)   →  L = [a,b,c]
func TestCompilerAppend(t *testing.T) {
	src := `
		append([], L, L).
		append([H|T], L, [H|R]) :- append(T, L, R).
	`
	// We need to read back L after the query. The query compiler puts its
	// variables into X registers, but we need to know which one is L.
	// For now, compile manually so we can track the register for L.
	comp := NewCompiler()
	comp.CompileProgram(ParseClauses(src))

	// Compile query: append([a,b], [c], L)
	// We hand-emit the PUT_* sequence so we know L lands in X[10].
	prog := comp.Program()
	queryStart := len(prog.Code)

	lReg := 10

	// A0 = [a,b]
	prog.emit(PUT_LIST, 0, 0)
	prog.emit(SET_CONSTANT, int(ATM(prog.Atoms.Atom("a"))), 0)
	prog.emit(SET_LIST, 0, 0)
	prog.emit(SET_CONSTANT, int(ATM(prog.Atoms.Atom("b"))), 0)
	prog.emit(SET_CONSTANT, int(ATM(prog.Atoms.Atom("[]"))), 0)

	// A1 = [c]
	prog.emit(PUT_LIST, 1, 0)
	prog.emit(SET_CONSTANT, int(ATM(prog.Atoms.Atom("c"))), 0)
	prog.emit(SET_CONSTANT, int(ATM(prog.Atoms.Atom("[]"))), 0)

	// A2 = L (fresh unbound var, also stored in X[lReg] so we can read it back)
	prog.emit(PUT_VARIABLE, lReg, 2)

	fid := prog.Atoms.Func("append", 3)
	prog.emit(CALL, fid, 3)

	m := NewWAM()
	m.Load(prog)
	m.P = queryStart
	m.CP = len(m.Code)
	m.Run()

	if m.Fail {
		t.Fatal("append([a,b],[c],L) failed")
	}
	L := m.ReadTerm(m.X[lReg])
	t.Logf("L = %s", L)
	if L != "[a,b,c]" {
		t.Errorf("got %q want %q", L, "[a,b,c]")
	}
}

// TestCompilerMember checks member/2 compiled from source.
//   member(X, [X|_]).
//   member(X, [_|T]) :- member(X, T).
//   ?- member(b, [a,b,c])  → success
func TestCompilerMember(t *testing.T) {
	src := `
		member(X, [X|_]).
		member(X, [_|T]) :- member(X, T).
	`
	comp := NewCompiler()
	comp.CompileProgram(ParseClauses(src))
	prog := comp.Program()
	queryStart := len(prog.Code)

	// A0 = b
	prog.emit(PUT_CONSTANT, int(ATM(prog.Atoms.Atom("b"))), 0)
	// A1 = [a,b,c]
	prog.emit(PUT_LIST, 1, 0)
	prog.emit(SET_CONSTANT, int(ATM(prog.Atoms.Atom("a"))), 0)
	prog.emit(SET_LIST, 0, 0)
	prog.emit(SET_CONSTANT, int(ATM(prog.Atoms.Atom("b"))), 0)
	prog.emit(SET_LIST, 0, 0)
	prog.emit(SET_CONSTANT, int(ATM(prog.Atoms.Atom("c"))), 0)
	prog.emit(SET_CONSTANT, int(ATM(prog.Atoms.Atom("[]"))), 0)

	fid := prog.Atoms.Func("member", 2)
	prog.emit(CALL, fid, 2)

	m := NewWAM()
	m.Load(prog)
	m.P = queryStart
	m.CP = len(m.Code)
	m.Run()

	if m.Fail {
		t.Error("member(b,[a,b,c]) should succeed but failed")
	} else {
		t.Log("member(b,[a,b,c]) succeeded")
	}
}

// TestCompileNat checks that nat/1 compiled from source accepts nat(s(s(zero))).
func TestCompileNat(t *testing.T) {
	src := `
		nat(zero).
		nat(s(X)) :- nat(X).
	`
	comp := NewCompiler()
	comp.CompileProgram(ParseClauses(src))
	startPC, _ := comp.CompileQuery(MustParseQuery("nat(s(s(zero)))"))

	m := NewWAM()
	m.Load(comp.Program())
	m.P = startPC
	m.CP = len(m.Code)
	m.Run()

	if m.Fail {
		t.Error("nat(s(s(zero))) should succeed but failed")
	}
}

// TestCompileQuerySection2_2 checks the instruction sequence the compiler emits
// for the section 2.2 example query ?- p(Z, h(Z,W), f(W)).
//
// Query variables become permanent (Y registers) so they survive the CALL.
// The full output is:
//
//	allocate 2
//	put_variable     Y0, X0   — Z first occurrence
//	put_unsafe_value Y0, X3   — Z second occurrence (inside h/2)
//	put_variable     Y1, X4   — W first occurrence (inside h/2)
//	put_structure    h/2, X1
//	set_value        X3
//	set_value        X4
//	put_unsafe_value Y1, X5   — W second occurrence (inside f/1)
//	put_structure    f/1, X2
//	set_value        X5
//	call             p/3
func TestCompileQuerySection2_2(t *testing.T) {
	comp := NewCompiler()
	startPC, _ := comp.CompileQuery(MustParseQuery("p(Z, h(Z,W), f(W))"))
	prog := comp.Program()

	h2 := prog.Atoms.Func("h", 2)
	f1 := prog.Atoms.Func("f", 1)
	p3 := prog.Atoms.Func("p", 3)

	got := prog.Code[startPC:]
	want := []Instruction{
		{ALLOCATE, 2, 0},
		{PUT_VARIABLE, int(Y(0)), int(X(0))},
		{PUT_UNSAFE_VALUE, int(Y(0)), int(X(3))},
		{PUT_VARIABLE, int(Y(1)), int(X(4))},
		{PUT_STRUCTURE, h2, int(X(1))},
		{SET_VALUE, int(X(3)), 0},
		{SET_VALUE, int(X(4)), 0},
		{PUT_UNSAFE_VALUE, int(Y(1)), int(X(5))},
		{PUT_STRUCTURE, f1, int(X(2))},
		{SET_VALUE, int(X(5)), 0},
		{CALL, p3, 3},
	}

	if len(got) != len(want) {
		t.Fatalf("got %d instructions, want %d\ngot:  %v\nwant: %v", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("instr[%d]: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestCompileProgramSection2_3 checks the instruction sequence the compiler
// emits for the section 2.3 example clause p(f(X), h(Y, f(a)), Y).
//
// In L1+ the outer functor p/3 is implicit (the callee receives its args in
// X0..X2), so the compiled head begins with get_structure on the individual
// argument registers rather than on p/3 itself. Temporary variable registers
// for X, Y, and the f(a) placeholder are assigned via map iteration and are
// therefore non-deterministic, so only the opcodes, the deterministic operands
// (functor IDs, argument positions, the constant), and cross-instruction
// register consistency are checked.
//
//	get_structure f/1, X0   — match first arg f(X)
//	unify_variable Xn       — X
//	get_structure h/2, X1   — match second arg h(Y, f(a))
//	unify_variable Xm       — Y (first occurrence)
//	unify_variable Xk       — f(a) placeholder
//	get_value      Xm, X2   — Y = third arg (shared)
//	get_structure  f/1, Xk  — expand f(a)
//	unify_constant a
//	proceed
func TestCompileProgramSection2_3(t *testing.T) {
	comp := NewCompiler()
	comp.CompileProgram(ParseClauses("p(f(X), h(Y, f(a)), Y)."))
	prog := comp.Program()

	f1 := prog.Atoms.Func("f", 1)
	h2 := prog.Atoms.Func("h", 2)
	aAtm := int(ATM(prog.Atoms.Atom("a")))

	got := prog.Code[prog.Labels["p/3"]:]

	wantOps := []Opcode{
		GET_STRUCTURE, UNIFY_VARIABLE,                   // f(X)
		GET_STRUCTURE, UNIFY_VARIABLE, UNIFY_VARIABLE,   // h(Y, f(a))
		GET_VALUE,                                       // Y = third arg
		GET_STRUCTURE, UNIFY_CONSTANT,                   // expand f(a)
		PROCEED,
	}
	if len(got) != len(wantOps) {
		t.Fatalf("got %d instructions, want %d\n%v", len(got), len(wantOps), got)
	}
	for i, op := range wantOps {
		if got[i].Op != op {
			t.Errorf("instr[%d]: got op %v, want %v", i, got[i].Op, op)
		}
	}

	if got[0].Arg1 != f1 || got[0].Arg2 != int(X(0)) {
		t.Errorf("instr[0]: want get_structure f/1 X0, got %+v", got[0])
	}
	if got[2].Arg1 != h2 || got[2].Arg2 != int(X(1)) {
		t.Errorf("instr[2]: want get_structure h/2 X1, got %+v", got[2])
	}
	if got[5].Arg2 != int(X(2)) {
		t.Errorf("instr[5]: want get_value ?, X2, got %+v", got[5])
	}
	if got[6].Arg1 != f1 {
		t.Errorf("instr[6]: want get_structure f/1 ?, got %+v", got[6])
	}
	if got[7].Arg1 != aAtm {
		t.Errorf("instr[7]: want unify_constant a, got %+v", got[7])
	}

	// Y's register must be consistent across its two occurrences.
	yReg := got[3].Arg1
	if got[5].Arg1 != yReg {
		t.Errorf("Y register inconsistent: unify_variable uses X%d but get_value uses X%d", yReg, got[5].Arg1)
	}

	// f(a) placeholder register must be consistent between unify_variable and get_structure.
	faReg := got[4].Arg1
	if got[6].Arg2 != faReg {
		t.Errorf("f(a) register inconsistent: unify_variable uses X%d but get_structure uses X%d", faReg, got[6].Arg2)
	}
}
