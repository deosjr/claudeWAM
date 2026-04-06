package main

import "fmt"

type Opcode int

const (
	// ── Chapter 2: L0 ── structure unification only ─────────────────────────

	// Figure 2.2: query compilation instructions.

	PUT_STRUCTURE Opcode = iota // put_structure f/n, Xi — push FUN cell, load STR into Xi
	SET_VARIABLE                // set_variable Xi        — push new unbound var, load into Xi
	SET_VALUE                   // set_value Xi           — push Xi's value onto heap

	// Figure 2.6: program compilation instructions.

	GET_STRUCTURE  // get_structure f/n, Xi — match/build structure in Xi
	UNIFY_VARIABLE // unify_variable Xi     — READ: load subterm into Xi; WRITE: push new var
	UNIFY_VALUE    // unify_value Xi        — READ: unify subterm with Xi; WRITE: push Xi

	// ── Chapter 2.4: L1 ── adds top-level argument registers ──────────────────

	// Figure 2.8: variable argument instructions

	PUT_VARIABLE // put_variable Xn, Ai   — new unbound var in Xn AND Ai (first body occurrence)
	PUT_VALUE    // put_value Xn, Ai      — copy Xn to Ai (subsequent occurrence)

	GET_VARIABLE // get_variable Xn, Ai  — copy Ai to Xn (first head occurrence)
	GET_VALUE    // get_value Xn, Ai     — unify Xn with Ai (subsequent occurrence)

	// ── Chapter 3: L2 ── adds environments (permanent variables) ─────────────

	ALLOCATE   // allocate N     — push environment frame with N permanent variable slots
	DEALLOCATE // deallocate     — pop environment frame (restore CE and CP)
	CALL       // call p/n, N   — save CP, jump to procedure (N = env slots still live)
	EXECUTE    // execute p/n   — like CALL but also deallocates (last-call optimisation)
	PROCEED    // proceed        — return: jump to CP

	// ── Chapter 4: L3 or Pure Prolog ─ disjunctions and backtracking ─────────

	TRY_ME_ELSE   // try_me_else L  — push choice point; on fail jump to L
	RETRY_ME_ELSE // retry_me_else L — update choice point's next-clause to L
	TRUST_ME      // trust_me       — pop choice point (last clause, no more alternatives)

	TRY   // try L   — like TRY_ME_ELSE but inline
	RETRY // retry L — like RETRY_ME_ELSE but inline
	TRUST // trust L — like TRUST_ME but inline

	NECK_CUT  // neck_cut   — cut at neck of clause (first instruction after head)
	GET_LEVEL // get_level Yn — save current choice point B into Yn
	CUT       // cut Yn      — cut to choice point saved in Yn

	// ── Chapter 5: optimisations ─────────────────────────────────────────────

	// ── Chapter 5, Figure 5.2: specialized instructions for constants ────────
	// Optimisation: treat atoms/integers as flat tagged values rather than
	// zero-arity structures, avoiding a heap cell and a register per constant.

	SET_CONSTANT   // set_constant c    — push atom/int cell
	UNIFY_CONSTANT // unify_constant c  — READ: unify subterm with c; WRITE: push c
	GET_CONSTANT   // get_constant c, Ai — unify Ai with constant c
	PUT_CONSTANT   // put_constant c, Ai — load constant into Ai

	// ── Chapter 5, Figure 5.3: specialized instructions for lists ────────────
	// Optimisation: dedicated LIS cells for lists instead of encoding as ./2.

	PUT_LIST // put_list Ai      — start building a list, load LIS ptr into Ai
	SET_LIST // set_list         — push a LIS cell (used inside SET_* sequences)
	GET_LIST // get_list Ai      — match/build list in Ai

	// ── Chapter 5, Figure 5.6: specialized instructions for void variables ───
	// Optimisation: skip anonymous variables in bulk rather than allocating
	// a fresh register per _.

	SET_VOID   // set_void n   — push n anonymous unbound vars
	UNIFY_VOID // unify_void n — READ: skip n subterms; WRITE: push n vars

	// Chapter 5.8.2: Unsafe variables

	PUT_UNSAFE_VALUE // put_unsafe_value Yn, Ai — like put_value but globalises if var is local
)

// Instruction is one WAM instruction. Arg1 and Arg2 are multipurpose:
// register index (Reg), functor id (int), label (int), or constant cell (Cell).
type Instruction struct {
	Op   Opcode
	Arg1 int // primary operand
	Arg2 int // secondary operand (if any)
}

func (i Instruction) String() string {
	switch i.Op {
	case PUT_STRUCTURE:
		return fmt.Sprintf("put_structure %d, X%d", i.Arg1, i.Arg2)
	case GET_STRUCTURE:
		return fmt.Sprintf("get_structure %d, X%d", i.Arg1, i.Arg2)
	case SET_VARIABLE:
		return fmt.Sprintf("set_variable X%d", i.Arg1)
	case SET_VALUE:
		return fmt.Sprintf("set_value X%d", i.Arg1)
	case UNIFY_VARIABLE:
		return fmt.Sprintf("unify_variable X%d", i.Arg1)
	case UNIFY_VALUE:
		return fmt.Sprintf("unify_value X%d", i.Arg1)
	case CALL:
		return fmt.Sprintf("call %d/%d", i.Arg1, i.Arg2)
	case PROCEED:
		return "proceed"
	case ALLOCATE:
		return fmt.Sprintf("allocate %d", i.Arg1)
	case DEALLOCATE:
		return "deallocate"
	case TRY_ME_ELSE:
		return fmt.Sprintf("try_me_else %d", i.Arg1)
	case RETRY_ME_ELSE:
		return fmt.Sprintf("retry_me_else %d", i.Arg1)
	case TRUST_ME:
		return "trust_me"
	}
	return fmt.Sprintf("op(%d, %d, %d)", i.Op, i.Arg1, i.Arg2)
}
