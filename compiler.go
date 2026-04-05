package main

// workItem is used by the BFS queue in compileHead / emitUnify.
type workItem struct {
	term Term
	reg  Reg
}

// Compiler translates parsed Prolog clauses into WAM instruction sequences.
//
// The structure mirrors the paper's Figure 4.4 closely:
//   - compile_clause  → CompileClause
//   - compile_head    → compileHead (emits GET_* instructions)
//   - compile_body    → compileGoal (emits PUT_* instructions per goal)
//   - compile_arg     → compileArg  (recursive, handles nested terms)
//
// The key insight: HEAD compilation is top-down (BFS order, using a work queue
// of subterms), so that the argument registers A0..An are consumed in order.
// BODY compilation is bottom-up (DFS, building inner structures before outer),
// so that inner terms are ready in X registers before PUT_STRUCTURE is emitted.
//
// The compiler has no dependency on the WAM. It produces a Program which the
// WAM loads separately via WAM.Load.

type Compiler struct {
	prog      *Program
	usedTemps map[int]bool // registers in use during current clause compilation
}

func NewCompiler() *Compiler {
	return &Compiler{prog: NewProgram()}
}

func (c *Compiler) Program() *Program {
	return c.prog
}

// resetTemps initialises the used-register set for a new clause. Argument
// registers 0..arity-1 and all classified temp-var registers are pre-marked
// so that freshReg never returns a register that already holds something.
func (c *Compiler) resetTemps(arity int, vars map[string]VarInfo) {
	c.usedTemps = map[int]bool{}
	for i := 0; i < arity; i++ {
		c.usedTemps[i] = true
	}
	for _, info := range vars {
		if info.Class == Temp {
			c.usedTemps[int(info.Reg)] = true
		}
	}
}

// CompileProgram compiles a slice of clauses grouped by functor/arity, wrapping
// each procedure in TRY_ME_ELSE / RETRY_ME_ELSE / TRUST_ME guards.
func (c *Compiler) CompileProgram(clauses []Clause) {
	type key struct {
		name  string
		arity int
	}
	order := []key{}
	groups := map[key][]Clause{}
	for _, cl := range clauses {
		k := key{cl.Head.Functor, len(cl.Head.Args)}
		if _, ok := groups[k]; !ok {
			order = append(order, k)
		}
		groups[k] = append(groups[k], cl)
	}
	for _, k := range order {
		c.compileProcedure(k.name, k.arity, groups[k])
	}
}

func (c *Compiler) compileProcedure(name string, arity int, clauses []Clause) {
	fid := c.prog.Atoms.Func(name, arity)
	c.prog.Labels[c.prog.Atoms.FuncEntry(fid).String()] = len(c.prog.Code)

	for i, cl := range clauses {
		clauseStart := len(c.prog.Code)
		switch {
		case len(clauses) == 1:
			// single clause: no choice point needed
		case i == 0:
			c.emit(TRY_ME_ELSE, 0, arity) // Arg1 patched below
		case i < len(clauses)-1:
			c.emit(RETRY_ME_ELSE, 0, arity) // Arg1 patched below
		default:
			c.emit(TRUST_ME, 0, 0)
		}

		c.CompileClause(cl)

		// Patch the TRY_ME_ELSE or RETRY_ME_ELSE emitted before this clause
		// to jump to the instruction just emitted (start of next clause).
		if i == 0 && len(clauses) > 1 {
			c.prog.Code[clauseStart].Arg1 = len(c.prog.Code)
		} else if i > 0 && i < len(clauses)-1 {
			c.prog.Code[clauseStart].Arg1 = len(c.prog.Code)
		}
	}
}

// CompileClause compiles one clause into the program's code slice.
func (c *Compiler) CompileClause(cl Clause) {
	vars, numPerm := classifyVars(cl)

	// Assign X registers to temporary variables, starting after the argument
	// registers (A0..A(arity-1) = X0..X(arity-1)).
	nextX := len(cl.Head.Args)
	for name, info := range vars {
		if info.Class == Temp {
			info.Reg = X(nextX)
			vars[name] = info
			nextX++
		}
	}

	c.resetTemps(len(cl.Head.Args), vars)
	seen := map[string]bool{} // tracks first vs subsequent occurrence

	if numPerm > 0 {
		c.emit(ALLOCATE, numPerm, 0)
	}

	c.compileHead(cl.Head, vars, seen)

	for i, goal := range cl.Body {
		isLast := i == len(cl.Body)-1
		c.compileGoal(goal, vars, seen, isLast && numPerm > 0)
	}

	if numPerm > 0 {
		c.emit(DEALLOCATE, 0, 0)
	}
	if len(cl.Body) == 0 {
		c.emit(PROCEED, 0, 0)
	}
}

// CompileQuery compiles a single query goal into the program. Returns the PC
// where execution should start and a map from variable name to the register
// (always a Y register) that holds each variable's heap reference.
//
// Query variables are made permanent (Y registers) so their heap references
// survive the CALL instruction and can be dereferenced after Run() returns.
// Without ALLOCATE + permanent registers, the X register holding the variable
// would be overwritten by the callee's own temporaries, losing the binding.
func (c *Compiler) CompileQuery(goal Compound) (startPC int, varRegs map[string]Reg) {
	startPC = len(c.prog.Code)

	// Collect distinct named variables from the goal and assign each a Y register.
	vars := map[string]VarInfo{}
	permCount := 0
	for _, name := range collectVarNames(Term(goal)) {
		if name == "_" {
			continue
		}
		if _, already := vars[name]; already {
			continue
		}
		vars[name] = VarInfo{Class: Perm, Reg: Y(permCount)}
		permCount++
	}

	// Emit ALLOCATE only when there are variables to preserve.
	if permCount > 0 {
		c.emit(ALLOCATE, permCount, 0)
	}

	c.resetTemps(len(goal.Args), vars) // marks arg regs used; no Temp vars to mark
	seen := map[string]bool{}
	for i, arg := range goal.Args {
		c.compileArg(arg, X(i), vars, seen)
	}
	fid := c.prog.Atoms.Func(goal.Functor, len(goal.Args))
	c.emit(CALL, fid, len(goal.Args))
	// Intentionally no DEALLOCATE: the env frame must stay alive so that the
	// caller can read back the Y registers after Run() returns.

	varRegs = map[string]Reg{}
	for name, info := range vars {
		varRegs[name] = info.Reg
	}
	return startPC, varRegs
}

// compileHead emits GET_* instructions for each argument of the head.
// BFS: argument registers are consumed left-to-right before recursing into
// substructures, which mirrors the order the WAM expects to see them.
func (c *Compiler) compileHead(head Compound, vars map[string]VarInfo, seen map[string]bool) {
	queue := []workItem{}
	for i, arg := range head.Args {
		queue = append(queue, workItem{arg, X(i)})
	}
	for len(queue) > 0 {
		w := queue[0]
		queue = queue[1:]
		switch t := w.term.(type) {
		case Var:
			if t.Name == "_" {
				break
			}
			info := vars[t.Name]
			if !seen[t.Name] {
				seen[t.Name] = true
				c.emit(GET_VARIABLE, int(info.Reg), int(w.reg))
			} else {
				c.emit(GET_VALUE, int(info.Reg), int(w.reg))
			}
		case Atom:
			c.emit(GET_CONSTANT, int(ATM(c.prog.Atoms.Atom(t.Name))), int(w.reg))
		case Integer:
			c.emit(GET_CONSTANT, int(INT(t.Val)), int(w.reg))
		case Compound:
			if t.Functor == "." && len(t.Args) == 2 {
				c.emit(GET_LIST, int(w.reg), 0)
				for _, sub := range t.Args {
					c.emitUnify(sub, vars, seen, &queue)
				}
			} else {
				fid := c.prog.Atoms.Func(t.Functor, len(t.Args))
				c.emit(GET_STRUCTURE, fid, int(w.reg))
				for _, sub := range t.Args {
					c.emitUnify(sub, vars, seen, &queue)
				}
			}
		}
	}
}

// emitUnify emits a UNIFY_* instruction for one subterm inside a GET_STRUCTURE
// or GET_LIST. Compound subterms get a fresh X register and are enqueued for
// later GET_STRUCTURE processing (still BFS).
func (c *Compiler) emitUnify(t Term, vars map[string]VarInfo, seen map[string]bool, queue *[]workItem) {
	switch v := t.(type) {
	case Var:
		if v.Name == "_" {
			c.emit(UNIFY_VOID, 1, 0)
			return
		}
		info := vars[v.Name]
		if !seen[v.Name] {
			seen[v.Name] = true
			c.emit(UNIFY_VARIABLE, int(info.Reg), 0)
		} else {
			c.emit(UNIFY_VALUE, int(info.Reg), 0)
		}
	case Atom:
		c.emit(UNIFY_CONSTANT, int(ATM(c.prog.Atoms.Atom(v.Name))), 0)
	case Integer:
		c.emit(UNIFY_CONSTANT, int(INT(v.Val)), 0)
	case Compound:
		xreg := c.freshReg(vars)
		if !seen[v.Functor] {
			c.emit(UNIFY_VARIABLE, int(xreg), 0)
		} else {
			c.emit(UNIFY_VALUE, int(xreg), 0)
		}
		*queue = append(*queue, workItem{v, xreg})
	}
}

// compileGoal emits PUT_* instructions for one body goal, then CALL or EXECUTE.
// Arguments are built bottom-up (DFS) so inner structures land in registers
// before the outer PUT_STRUCTURE or CALL that references them.
func (c *Compiler) compileGoal(goal Compound, vars map[string]VarInfo, seen map[string]bool, isLast bool) {
	for i, arg := range goal.Args {
		c.compileArg(arg, X(i), vars, seen)
	}
	fid := c.prog.Atoms.Func(goal.Functor, len(goal.Args))
	if isLast {
		c.emit(EXECUTE, fid, len(goal.Args))
	} else {
		c.emit(CALL, fid, len(goal.Args))
	}
}

// compileArg recursively builds a term into a target register using PUT_*.
func (c *Compiler) compileArg(t Term, target Reg, vars map[string]VarInfo, seen map[string]bool) {
	switch v := t.(type) {
	case Var:
		if v.Name == "_" {
			c.emit(PUT_VARIABLE, int(target), int(target))
			return
		}
		info := vars[v.Name]
		if !seen[v.Name] {
			seen[v.Name] = true
			c.emit(PUT_VARIABLE, int(info.Reg), int(target))
		} else {
			if info.Class == Perm {
				c.emit(PUT_UNSAFE_VALUE, int(info.Reg), int(target))
			} else {
				c.emit(PUT_VALUE, int(info.Reg), int(target))
			}
		}
	case Atom:
		c.emit(PUT_CONSTANT, int(ATM(c.prog.Atoms.Atom(v.Name))), int(target))
	case Integer:
		c.emit(PUT_CONSTANT, int(INT(v.Val)), int(target))
	case Compound:
		if v.Functor == "." && len(v.Args) == 2 {
			c.emit(PUT_LIST, int(target), 0)
			for _, sub := range v.Args {
				c.emitSet(sub, vars, seen)
			}
		} else {
			fid := c.prog.Atoms.Func(v.Functor, len(v.Args))
			argRegs := make([]Reg, len(v.Args))
			for i, sub := range v.Args {
				argRegs[i] = c.freshReg(vars)
				c.compileArg(sub, argRegs[i], vars, seen)
			}
			c.emit(PUT_STRUCTURE, fid, int(target))
			for _, r := range argRegs {
				c.emit(SET_VALUE, int(r), 0)
			}
		}
	}
}

// emitSet emits a SET_* instruction for use inside PUT_LIST / PUT_STRUCTURE.
func (c *Compiler) emitSet(t Term, vars map[string]VarInfo, seen map[string]bool) {
	switch v := t.(type) {
	case Var:
		if v.Name == "_" {
			c.emit(SET_VOID, 1, 0)
			return
		}
		info := vars[v.Name]
		if !seen[v.Name] {
			seen[v.Name] = true
			c.emit(SET_VARIABLE, int(info.Reg), 0)
		} else {
			c.emit(SET_VALUE, int(info.Reg), 0)
		}
	case Atom:
		c.emit(SET_CONSTANT, int(ATM(c.prog.Atoms.Atom(v.Name))), 0)
	case Integer:
		c.emit(SET_CONSTANT, int(INT(v.Val)), 0)
	case Compound:
		r := c.freshReg(vars)
		c.compileArg(v, r, vars, seen)
		c.emit(SET_VALUE, int(r), 0)
	}
}

// freshReg picks an X register not in usedTemps, marks it used, and returns it.
// Because usedTemps is updated on each call, repeated calls within one clause
// return different registers — fixing the bug where nested PUT_STRUCTURE calls
// with no named variables all returned X(0) and overwrote each other.
func (c *Compiler) freshReg(_ map[string]VarInfo) Reg {
	for r := 0; ; r++ {
		if !c.usedTemps[r] {
			c.usedTemps[r] = true
			return X(r)
		}
	}
}

func (c *Compiler) emit(op Opcode, a1, a2 int) {
	c.prog.emit(op, a1, a2)
}
