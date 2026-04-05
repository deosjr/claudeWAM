package main

// Variable classification is the step that decides whether each variable in a
// clause gets a temporary X register or a permanent Y register (slot in the
// environment frame).
//
// The rule (Hassan §4.1):
//   A variable is PERMANENT if it appears in more than one body goal, OR in
//   both the head and at least one body goal.
//   Everything else is TEMPORARY.
//
// Permanent variables must survive across CALL instructions, so they live in
// the environment frame allocated by ALLOCATE.  Temporary variables die when
// the instruction stream reaches the next CALL.

type VarClass int

const (
	Temp VarClass = iota
	Perm
)

type VarInfo struct {
	Class VarClass
	Reg   Reg  // assigned X or Y register
}

// classifyVars analyses a clause and returns a map from variable name to VarInfo.
// It also returns the number of permanent variables (the N for ALLOCATE N).
func classifyVars(c Clause) (map[string]VarInfo, int) {
	// Count which "segments" each variable appears in.
	// Segment 0 = head; segment k = body goal k.
	// A variable appearing in segments {i, j} with i != j is permanent —
	// except if both are 0 and some body goal (i.e. head-only vars are temp).
	type segSet map[int]bool
	segs := map[string]segSet{}

	record := func(name string, seg int) {
		if name == "_" {
			return // anonymous: never share, never classify
		}
		if segs[name] == nil {
			segs[name] = segSet{}
		}
		segs[name][seg] = true
	}

	for _, name := range collectVarNames(c.Head) {
		record(name, 0)
	}
	for i, goal := range c.Body {
		for _, name := range collectVarNames(Term(goal)) {
			record(name, i+1) // body goals are segments 1..n
		}
	}

	info := map[string]VarInfo{}
	permCount := 0
	tempCount := 0

	for name, s := range segs {
		// Permanent if the variable spans more than one body goal segment,
		// OR appears in the head AND any body goal.
		spansBodyGoals := false
		appearsInBody := false
		appearsInHead := s[0]
		bodySegsCount := 0
		for seg := range s {
			if seg > 0 {
				appearsInBody = true
				bodySegsCount++
			}
		}
		if bodySegsCount > 1 {
			spansBodyGoals = true
		}

		isPerm := spansBodyGoals || (appearsInHead && appearsInBody && len(c.Body) > 0)

		if isPerm {
			info[name] = VarInfo{Class: Perm, Reg: Y(permCount)}
			permCount++
		} else {
			info[name] = VarInfo{Class: Temp, Reg: X(tempCount)}
			tempCount++
		}
	}

	return info, permCount
}
