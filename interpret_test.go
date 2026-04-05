package main

import (
	"testing"
)

// TestInterpretAppend mirrors the first query in bowenProlog/main.go:
//
//	append(nil, L, L).
//	append(cons(X,L1), L2, cons(X,L3)) :- append(L1, L2, L3).
//	?- append(cons(a,cons(b,nil)), cons(c,nil), L)
//	   → L = cons(a,cons(b,cons(c,nil)))
func TestInterpretAppend(t *testing.T) {
	src := `
		append(nil, L, L).
		append(cons(X,L1), L2, cons(X,L3)) :- append(L1, L2, L3).
	`
	results := Interpret(src, "append(cons(a,cons(b,nil)),cons(c,nil),L)")

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	got := results[0]["L"]
	want := "cons(a,cons(b,cons(c,nil)))"
	if got != want {
		t.Errorf("L = %q, want %q", got, want)
	}
	t.Logf("L = %s", got)
}

// TestInterpretAppendMulti mirrors the second query in bowenProlog/main.go:
//
//	?- append(L, X, cons(a,cons(b,cons(c,nil))))
//	   → four solutions: L=nil X=cons(a,...), L=cons(a,nil) X=cons(b,...), ...
func TestInterpretAppendMulti(t *testing.T) {
	src := `
		append(nil, L, L).
		append(cons(X,L1), L2, cons(X,L3)) :- append(L1, L2, L3).
	`
	results := Interpret(src, "append(L,X,cons(a,cons(b,cons(c,nil))))")

	// Expect exactly 4 solutions (splitting a 3-element list).
	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d: %v", len(results), results)
	}

	expected := []struct{ l, x string }{
		{"nil", "cons(a,cons(b,cons(c,nil)))"},
		{"cons(a,nil)", "cons(b,cons(c,nil))"},
		{"cons(a,cons(b,nil))", "cons(c,nil)"},
		{"cons(a,cons(b,cons(c,nil)))", "nil"},
	}
	for i, e := range expected {
		if results[i]["L"] != e.l || results[i]["X"] != e.x {
			t.Errorf("solution %d: L=%q X=%q, want L=%q X=%q",
				i, results[i]["L"], results[i]["X"], e.l, e.x)
		}
	}
	for _, r := range results {
		t.Logf("L = %s,  X = %s", r["L"], r["X"])
	}
}
