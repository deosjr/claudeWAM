package main

import (
	"testing"
)

// compileAndRun is the standard pipeline:
//   1. Compile the program clauses
//   2. Compile the query (appended to same Program so labels resolve)
//   3. Load the Program into a fresh WAM
//   4. Run from the query's start PC
func compileAndRun(t *testing.T, src string, query string) *WAM {
	t.Helper()
	comp := NewCompiler()
	comp.CompileProgram(ParseClauses(src))
	startPC, _ := comp.CompileQuery(MustParseQuery(query))

	m := NewWAM()
	m.Load(comp.Program())
	m.P = startPC
	m.CP = len(m.Code) // CALL returns here → exits Run loop
	m.Run()

	if m.Fail {
		t.Errorf("query %q failed", query)
	}
	return m
}

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

// TestCompileAndRun exercises the full pipeline including CompileQuery.
func TestCompileAndRun(t *testing.T) {
	src := `
		nat(zero).
		nat(s(X)) :- nat(X).
	`
	// nat(s(s(zero))) should succeed
	m := compileAndRun(t, src, "nat(s(s(zero)))")
	_ = m
}
