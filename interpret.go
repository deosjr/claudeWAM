package main

// Query is a compiled, ready-to-run Prolog query.
// Call Next() to advance to the next solution one at a time.
//
// Typical usage:
//
//	q := NewQuery(src, "append(L, X, cons(a,cons(b,nil)))")
//	for {
//	    bindings, ok := q.Next()
//	    if !ok {
//	        break          // no more solutions
//	    }
//	    fmt.Println(bindings["L"])
//	}
type Query struct {
	m       *WAM
	varRegs map[string]Reg // name → Y register holding the variable's heap ref
	started bool           // false until the first Next() call
	done    bool           // true once the machine has reported failure
}

// NewQuery compiles src, appends the compiled query, loads everything into a
// fresh WAM, and returns a Query ready for the first Next() call.
func NewQuery(src, query string) *Query {
	comp := NewCompiler()
	comp.CompileProgram(ParseClauses(src))
	startPC, varRegs := comp.CompileQuery(MustParseQuery(query))

	m := NewWAM()
	m.Load(comp.Program())
	m.P = startPC
	m.CP = len(m.Code) // returning here causes Run() to exit cleanly

	return &Query{m: m, varRegs: varRegs}
}

// Next advances the machine to the next solution and returns the variable
// bindings for that solution.  Returns (nil, false) when no further solutions
// exist (equivalent to Prolog's "false" / "fail").
//
// After the first successful call, subsequent calls trigger backtracking
// before running, so each call corresponds to pressing ";" in a Prolog REPL.
func (q *Query) Next() (map[string]string, bool) {
	if q.done {
		return nil, false
	}

	if q.started {
		// Trigger backtracking to search for the next clause.
		if q.m.B < 0 {
			// No choice points left: the previous answer was the last one.
			q.done = true
			return nil, false
		}
		q.m.Fail = true
		q.m.backtrack()
		if q.m.P < 0 || q.m.P >= len(q.m.Code) {
			q.done = true
			return nil, false
		}
	}
	q.started = true

	q.m.Run()

	if q.m.Fail {
		q.done = true
		return nil, false
	}

	// Read variable bindings from the permanent Y registers in the query's
	// environment frame; they survive the CALL and any inner backtracking.
	bindings := make(map[string]string, len(q.varRegs))
	for name, reg := range q.varRegs {
		bindings[name] = q.m.ReadTerm(q.m.getReg(reg))
	}
	return bindings, true
}

// MayHaveMore reports whether there are live choice points that could yield
// another answer.  It returns true even when all remaining branches will fail;
// calling Next() is the only way to confirm whether another solution exists.
//
// Useful for displaying a hint like "(more solutions possible; use -all)"
// without actually consuming the next answer.
func (q *Query) MayHaveMore() bool {
	return !q.done && q.m.B >= 0
}

// Interpret is a convenience wrapper that collects every solution into a slice.
// For large or infinite solution sets prefer Query.Next() directly.
func Interpret(src, query string) []map[string]string {
	q := NewQuery(src, query)
	var results []map[string]string
	for {
		bindings, ok := q.Next()
		if !ok {
			break
		}
		results = append(results, bindings)
	}
	return results
}
