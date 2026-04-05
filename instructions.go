package main

import "fmt"

type Opcode int

const (
	// ── Chapter 2: L0 ── structure unification only ─────────────────────────
	//
	// PUT instructions build query terms onto the heap (WRITE mode).
	// GET instructions match against terms already on the heap (READ/WRITE mode).
	// SET/UNIFY instructions handle the arguments *inside* a structure;
	//   in WRITE mode they push new cells; in READ mode they match existing ones.

	PUT_STRUCTURE Opcode = iota // put_structure f/n, Xi — push FUN cell, load STR into Xi
	SET_VARIABLE               // set_variable Xi      — push new unbound var, load into Xi
	SET_VALUE                  // set_value Xi         — push Xi's value onto heap
	SET_CONSTANT               // set_constant c, Xi   — push atom/int cell
	SET_VOID                   // set_void n           — push n anonymous unbound vars
	SET_LIST                   // set_list             — like PUT_LIST but used inside SET_* sequences: push a LIS cell

	GET_STRUCTURE   // get_structure f/n, Xi — match/build structure in Xi
	UNIFY_VARIABLE  // unify_variable Xi     — READ: load subterm into Xi; WRITE: push new var
	UNIFY_VALUE     // unify_value Xi        — READ: unify subterm with Xi; WRITE: push Xi
	UNIFY_CONSTANT  // unify_constant c      — READ: unify subterm with c;  WRITE: push c
	UNIFY_VOID      // unify_void n          — READ: skip n subterms;       WRITE: push n vars

	// ── Chapter 3: L1 ── adds top-level registers and constants ─────────────
	//
	// PUT/GET without structure: move values between X/Y registers and A regs.
	// These replace Bowen's single VAR/CONST instructions which used a mode flag.

	PUT_VARIABLE    // put_variable Xn, Ai  — new unbound var in Xn AND Ai (first body occurrence)
	PUT_VALUE       // put_value Xn, Ai     — copy Xn to Ai (subsequent occurrence)
	PUT_UNSAFE_VALUE // put_unsafe_value Yn, Ai — like put_value but globalises if var is local
	PUT_CONSTANT    // put_constant c, Ai   — load constant into Ai
	PUT_LIST        // put_list Ai          — start building a list, load LIS ptr into Ai

	GET_VARIABLE    // get_variable Xn, Ai  — copy Ai to Xn (first head occurrence)
	GET_VALUE       // get_value Xn, Ai     — unify Xn with Ai (subsequent occurrence)
	GET_CONSTANT    // get_constant c, Ai   — unify Ai with constant c
	GET_LIST        // get_list Ai          — match/build list in Ai

	// ── Chapter 4: L2 ── adds environments (permanent variables) ─────────────
	//
	// ALLOCATE creates an environment frame for permanent variables (those that
	// span a CALL boundary). DEALLOCATE removes it. This is what Bowen's ENTER
	// implicitly did, but only for the current clause — the WAM separates the
	// concerns of frame allocation from the head/body boundary.

	ALLOCATE   // allocate N     — push environment frame with N permanent variable slots
	DEALLOCATE // deallocate     — pop environment frame (restore CE and CP)
	CALL       // call p/n, N   — save CP, jump to procedure (N = env slots still live)
	EXECUTE    // execute p/n   — like CALL but also deallocates (last-call optimisation)
	PROCEED    // proceed        — return: jump to CP (≈ Bowen's EXIT with no continuation)

	// ── Chapter 5: Full WAM ── backtracking ──────────────────────────────────
	//
	// These replace Bowen's "try every clause, collect all states" with
	// destructive update + explicit undo via the trail.

	TRY_ME_ELSE   // try_me_else L  — push choice point; on fail jump to L
	RETRY_ME_ELSE // retry_me_else L — update choice point's next-clause to L
	TRUST_ME      // trust_me       — pop choice point (last clause, no more alternatives)

	// Indexing shortcuts (same semantics as above but for compiled index tables):
	TRY   // try L   — like TRY_ME_ELSE but inline
	RETRY // retry L — like RETRY_ME_ELSE but inline
	TRUST // trust L — like TRUST_ME but inline

	// Cut:
	NECK_CUT  // neck_cut   — cut at neck of clause (first instruction after head)
	GET_LEVEL // get_level Yn — save current choice point B into Yn
	CUT       // cut Yn      — cut to choice point saved in Yn
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
