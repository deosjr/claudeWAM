# hassanWAM

A Go implementation of the Warren Abstract Machine, following Hassan Aït-Kaci's
tutorial reconstruction of the WAM step by step.

This project is the second in a pair of learning exercises on Prolog compilation.

## Background

### Step 1 — Bowen's Portable Prolog Compiler

[bowenProlog](https://github.com/deosjr/bowenProlog) is a Go implementation of
the compiler described in:

> D.L. Bowen, L.M. Byrd, W.F. Clocksin —
> **A Portable Prolog Compiler** (1983)
> DAI Research Paper 173, University of Edinburgh

Bowen's design is intentionally high-level and machine-independent.  Its
central idea is an *immutable AVL-tree substitution* (a persistent mapping from
variables to terms) that is passed around and never destructively updated.
Backtracking is free: just discard the substitution from the failed branch and
try the next clause.  The bytecode is tiny — seven instructions
(`CONST`, `VAR`, `FUNCTOR`, `POP`, `ENTER`, `CALL`, `EXIT`) — and the
interpreter is correspondingly simple.

The price is performance.  Every unification allocates new tree nodes; there is
no mutable heap; structure sharing requires copying; and the substitution grows
without bound across deep recursions.

### Step 2 — Warren's Abstract Machine

This repository implements the machine described in:

> Hassan Aït-Kaci —
> **Warren's Abstract Machine: A Tutorial Reconstruction** (1991)
> MIT Press

The WAM is the foundation of virtually every serious Prolog system.  It trades
the clean immutability of Bowen's approach for raw speed:

| Concept | Bowen | WAM |
|---|---|---|
| Term storage | Immutable AVL substitution | Mutable heap array |
| Backtracking | Discard substitution branch | Unwind trail, restore registers |
| Variable binding | Tree node allocation | In-place heap cell rewrite |
| Structure sharing | Copy on unify | Pointer into heap |
| Instruction set | 7 instructions | ~35 instructions |
| Variable lifetime | All variables live forever | Temporary (X) vs permanent (Y) |
| Call convention | Continuation list | `CP` register + environment frames |

The key WAM ideas, in the order Aït-Kaci introduces them:

1. **Tagged words** — every heap cell carries a 3-bit tag (`REF`, `STR`, `ATM`,
   `INT`, `LIS`, `FUN`) so the runtime can inspect terms without a separate type
   field.
2. **Heap + trail** — the heap is a flat array; the trail records which cells
   were bound so they can be reset on backtracking.
3. **READ / WRITE mode** — `GET_STRUCTURE` detects whether it is matching an
   existing structure (READ) or building a new one (WRITE), allowing unification
   and construction to share instruction sequences.
4. **Argument registers** — `X0..X(n-1)` are both argument registers and
   temporaries; permanent variables that span a `CALL` boundary live in
   environment frames (`Y` registers) allocated by `ALLOCATE`.
5. **Choice points** — `TRY_ME_ELSE` / `RETRY_ME_ELSE` / `TRUST_ME` save and
   restore machine state for backtracking, replacing Bowen's implicit
   substitution branching.
6. **Last-call optimisation** — `EXECUTE` combines `CALL` + `DEALLOCATE` so
   tail-recursive predicates run in constant stack space.

## Repository layout

| File | Purpose |
|---|---|
| `cell.go` | Tagged 64-bit word type and constructors (`REF`, `STR`, `ATM`, …) |
| `atoms.go` | Atom/functor intern table |
| `machine.go` | WAM state: heap, trail, registers, environment frames, choice points |
| `heap.go` | `deref`, `bind`, trail management, `unwindTrail` |
| `unify.go` | Iterative PDL-based unification |
| `instructions.go` | Opcode definitions and `Instruction` struct |
| `exec.go` | `Run` loop and full instruction dispatch |
| `print.go` | `ReadTerm`: walk the heap and reconstruct a printable term |
| `program.go` | `Program` struct — compiler output, WAM input |
| `term.go` | Prolog AST (`Var`, `Atom`, `Integer`, `Compound`, `Clause`) |
| `parse.go` | Tokeniser + recursive-descent parser |
| `classify.go` | Variable classification (temporary vs permanent) |
| `compiler.go` | Clause compiler: `CompileProgram`, `CompileClause`, `CompileQuery` |
| `interpret.go` | `Query` struct with `Next()` / `MayHaveMore()` for incremental solution generation; `Interpret` convenience wrapper that collects all answers |
| `main.go` | Interactive driver |

## Running

```
go run . [-all] [query]
```

With no arguments the bundled `append` example is run.  Pass `-all` to
enumerate every solution via backtracking instead of stopping at the first.

```
$ go run .
?- append(cons(a,cons(b,nil)),cons(c,nil),L)
   L = cons(a,cons(b,cons(c,nil)))
   (more solutions possible; use -all)

$ go run . -all
?- append(L,X,cons(a,cons(b,cons(c,nil))))
   L = nil,  X = cons(a,cons(b,cons(c,nil)))
;
   L = cons(a,nil),  X = cons(b,cons(c,nil))
;
   L = cons(a,cons(b,nil)),  X = cons(c,nil)
;
   L = cons(a,cons(b,cons(c,nil))),  X = nil
.
```
