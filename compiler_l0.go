package main

import "fmt"

// compiler_l0.go implements an explicit L0 compiler following the three-step
// process described in Aït-Kaci's "Warren's Abstract Machine: A Tutorial
// Reconstruction", Chapter 2.
//
// The three steps are made explicit as separate functions:
//
//   Step 1 — Flatten  (FlattenL0)
//     Assign registers to every distinct subterm, producing a flat list of
//     equations of the form  Xi = f(Xj, Xk, ...).
//     The root term receives register X1. Subterms are numbered outward in
//     BFS (breadth-first, left-to-right) order: the root's compound arguments
//     get X2, X3, ... in argument order; their compound arguments follow; and
//     so on level by level.  Every occurrence of the same variable name maps
//     to the same register.  Atoms and integers are treated as zero-arity
//     structures (a/0, etc.) and consume a register like any other compound.
//     Anonymous variables (_) each receive a fresh register.
//
//   Step 2 — Reorder  (ReorderQueryL0 / ReorderProgramL0)
//     Sort the equations into the order required by the WAM:
//
//     • Queries (PUT_* instructions): inner structures must be emitted before
//       the outer structures that reference them.  A PUT_STRUCTURE instruction
//       for f/n is immediately followed by SET_VALUE instructions that push the
//       already-computed argument registers; those registers must hold STR
//       pointers built by earlier put_structure calls.  The required order is
//       DFS post-order: recurse into each compound argument before emitting
//       the parent.
//
//     • Programs (GET_* instructions): outer structures must be processed
//       before inner ones.  GET_STRUCTURE on an outer structure sets the read
//       cursor S to point at the structure's argument slots; subsequent
//       UNIFY_VARIABLE instructions load inner-structure addresses into
//       registers, which GET_STRUCTURE can then match.  The required order is
//       BFS top-down — the same order Flatten already produces.
//
//   Step 3 — Translate  (TranslateQueryL0 / TranslateProgramL0)
//     Convert each sorted equation to WAM instructions.  A "seen" set tracks
//     which registers have already been introduced:
//
//     Queries:   Xi = f(...)  →  put_structure f/n, Xi
//                                set_variable  Xj   (first use of Xj)
//                                set_value     Xj   (subsequent uses)
//
//     Programs:  Xi = f(...)  →  get_structure f/n, Xi
//                                unify_variable Xj  (first use of Xj)
//                                unify_value    Xj  (subsequent uses)
//
// L0 handles only a single flat term with no CALL/PROCEED, no
// ALLOCATE/DEALLOCATE, no choice points, and no constant shorthand
// (SET_CONSTANT / UNIFY_CONSTANT are Chapter 5 optimisations not present in
// L0).  compiler.go folds all three steps into a single recursive traversal;
// this file makes each step a separate data transformation to match the
// paper's presentation.

// ── Step 1: Flatten ────────────────────────────────────────────────────────

// FlatArg is one argument in a flattened equation.
// In L0, every argument position holds a register reference: atoms and
// integers are compiled as zero-arity structures and assigned their own
// registers, just like compound subterms.
type FlatArg struct {
	Reg int // register index n (for Xn)
}

// FlatEq is one equation in the flattened form:  Reg = f/Arity(Args...).
// A zero-arity equation (Arity == 0, Args == nil) represents a constant or
// atom encoded as a zero-arity structure.
// Variables are tracked implicitly via VarRegs; they do not produce equations.
type FlatEq struct {
	Reg   int      // LHS register
	FID   int      // functor ID in the AtomTable
	Arity int      // functor arity (equals len(Args))
	Args  []FlatArg // flattened arguments (nil for zero-arity)
}

// FlatTerm is the output of the Flatten step.
type FlatTerm struct {
	Eqs     []FlatEq       // compound equations in BFS (top-down) order
	Root    int            // register that holds the root term (always 1)
	VarRegs map[string]int // variable name → assigned register
}

// FlattenL0 is Step 1: assign registers to all subterms of t.
//
// The root gets register X1; subsequent subterms are numbered outward in BFS
// (left-to-right, top-down) order.  Every named variable maps to the same
// register across all occurrences.  Atoms and integers are treated as
// zero-arity structures and receive registers.  Anonymous variables (_) each
// get a fresh register that is never reused.
//
// The returned Eqs slice is in BFS order (root equation first).  This is the
// natural order for program (GET_*) compilation; query (PUT_*) compilation
// reorders it to DFS post-order in Step 2.
func FlattenL0(t Term, atoms *AtomTable) FlatTerm {
	varRegs := map[string]int{}
	nextReg := 2 // X1 is the root; subterms start at X2

	getVarReg := func(name string) int {
		if r, ok := varRegs[name]; ok {
			return r
		}
		r := nextReg
		nextReg++
		varRegs[name] = r
		return r
	}

	type work struct {
		t   Term
		reg int
	}
	queue := []work{{t, 1}}
	var eqs []FlatEq

	for len(queue) > 0 {
		w := queue[0]
		queue = queue[1:]

		switch v := w.t.(type) {
		case Compound:
			args := make([]FlatArg, len(v.Args))
			for i, a := range v.Args {
				switch av := a.(type) {
				case Var:
					if av.Name == "_" {
						// Anonymous: fresh register, not tracked, never shared.
						r := nextReg
						nextReg++
						args[i] = FlatArg{Reg: r}
					} else {
						args[i] = FlatArg{Reg: getVarReg(av.Name)}
					}
				case Atom, Integer, Compound:
					// All non-variable terms (including atoms and integers treated as
					// zero-arity structures) get a fresh register and are enqueued.
					r := nextReg
					nextReg++
					args[i] = FlatArg{Reg: r}
					queue = append(queue, work{a, r})
				}
			}
			fid := atoms.Func(v.Functor, len(v.Args))
			eqs = append(eqs, FlatEq{
				Reg:   w.reg,
				FID:   fid,
				Arity: len(v.Args),
				Args:  args,
			})

		case Atom:
			// Atom as a zero-arity structure: a/0.
			fid := atoms.Func(v.Name, 0)
			eqs = append(eqs, FlatEq{Reg: w.reg, FID: fid, Arity: 0})

		case Integer:
			// Integer as a zero-arity structure: "42"/0 etc.
			fid := atoms.Func(fmt.Sprintf("%d", v.Val), 0)
			eqs = append(eqs, FlatEq{Reg: w.reg, FID: fid, Arity: 0})
		}
		// Var at work-item level: variable only, no equation.
	}

	return FlatTerm{Eqs: eqs, Root: 1, VarRegs: varRegs}
}

// ── Step 2: Reorder ────────────────────────────────────────────────────────

// ReorderQueryL0 is Step 2 for queries: sort equations into DFS post-order so
// that every inner structure appears before the outer structure that references
// it.
//
// Post-order is computed by DFS on the equation graph starting from the root:
// for each compound equation, all compound sub-equations reachable through its
// register arguments are visited recursively before the equation itself is
// appended.  This is the same traversal order the paper's register-assignment
// table exhibits for query terms (Figure 2.1).
func ReorderQueryL0(ft FlatTerm) []FlatEq {
	byReg := make(map[int]FlatEq, len(ft.Eqs))
	for _, eq := range ft.Eqs {
		byReg[eq.Reg] = eq
	}

	visited := make(map[int]bool)
	result := make([]FlatEq, 0, len(ft.Eqs))

	var visit func(reg int)
	visit = func(reg int) {
		if visited[reg] {
			return
		}
		visited[reg] = true
		eq, hasEq := byReg[reg]
		if !hasEq {
			return // variable: no compound equation to emit
		}
		// Visit all sub-arguments first (post-order: children before parent).
		for _, arg := range eq.Args {
			visit(arg.Reg)
		}
		result = append(result, eq)
	}

	visit(ft.Root)
	return result
}

// ReorderProgramL0 is Step 2 for programs: equations stay in BFS (top-down)
// order, which FlattenL0 already produces.
//
// GET_STRUCTURE on an outer structure sets the read cursor S to the first
// argument slot of that structure.  The UNIFY_VARIABLE instructions that
// follow load inner-structure addresses into registers.  Only after those
// registers are populated can subsequent GET_STRUCTURE instructions match
// the inner structures — so outer-first (BFS) order is required.
func ReorderProgramL0(ft FlatTerm) []FlatEq {
	return ft.Eqs // BFS order from Flatten is already correct for programs
}

// ── Step 3: Translate ──────────────────────────────────────────────────────

// TranslateQueryL0 is Step 3 for queries: convert the sorted equations to
// PUT_STRUCTURE / SET_VARIABLE / SET_VALUE instructions.
//
// For each equation  Xi = f(args...):
//   put_structure f/n, Xi   — push FUN cell, store STR(addr) in Xi; mark Xi seen
//   for each argument register Xj:
//     first use  → set_variable Xj  (push fresh unbound var, store in Xj)
//     seen       → set_value    Xj  (push Xj's current value onto heap)
func TranslateQueryL0(eqs []FlatEq) []Instruction {
	seen := make(map[int]bool)
	var instrs []Instruction

	for _, eq := range eqs {
		instrs = append(instrs, Instruction{Op: PUT_STRUCTURE, Arg1: eq.FID, Arg2: eq.Reg})
		seen[eq.Reg] = true

		for _, arg := range eq.Args {
			if !seen[arg.Reg] {
				seen[arg.Reg] = true
				instrs = append(instrs, Instruction{Op: SET_VARIABLE, Arg1: arg.Reg})
			} else {
				instrs = append(instrs, Instruction{Op: SET_VALUE, Arg1: arg.Reg})
			}
		}
	}
	return instrs
}

// TranslateProgramL0 is Step 3 for programs: convert the sorted equations to
// GET_STRUCTURE / UNIFY_VARIABLE / UNIFY_VALUE instructions.
//
// For each equation  Xi = f(args...):
//   get_structure f/n, Xi   — match (READ) or build (WRITE); mark Xi seen
//   for each argument register Xj:
//     first use  → unify_variable Xj  (READ: load heap slot into Xj;
//                                       WRITE: push fresh unbound var)
//     seen       → unify_value    Xj  (READ: unify heap slot with Xj;
//                                       WRITE: push Xj's value)
func TranslateProgramL0(eqs []FlatEq) []Instruction {
	seen := make(map[int]bool)
	var instrs []Instruction

	for _, eq := range eqs {
		instrs = append(instrs, Instruction{Op: GET_STRUCTURE, Arg1: eq.FID, Arg2: eq.Reg})
		seen[eq.Reg] = true

		for _, arg := range eq.Args {
			if !seen[arg.Reg] {
				seen[arg.Reg] = true
				instrs = append(instrs, Instruction{Op: UNIFY_VARIABLE, Arg1: arg.Reg})
			} else {
				instrs = append(instrs, Instruction{Op: UNIFY_VALUE, Arg1: arg.Reg})
			}
		}
	}
	return instrs
}

// ── Convenience wrappers ───────────────────────────────────────────────────

// CompileQueryL0 runs all three steps for a query term.
// The root term is placed in register X1 (matching the paper's convention).
func CompileQueryL0(t Term, atoms *AtomTable) []Instruction {
	ft := FlattenL0(t, atoms)
	eqs := ReorderQueryL0(ft)
	return TranslateQueryL0(eqs)
}

// CompileProgramL0 runs all three steps for a program term.
// The term to match must already be in register X1 before the instructions
// execute.  L0 has no PROCEED; the caller halts execution by setting CP past
// the end of the code.
func CompileProgramL0(t Term, atoms *AtomTable) []Instruction {
	ft := FlattenL0(t, atoms)
	eqs := ReorderProgramL0(ft)
	return TranslateProgramL0(eqs)
}
