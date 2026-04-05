package main

// Program is the output of the compiler and the input to the WAM.
// The compiler has no dependency on the WAM; it only produces Programs.
// The WAM has no dependency on the compiler; it only consumes Programs.
type Program struct {
	Atoms  *AtomTable
	Code   []Instruction
	Labels map[string]int // functor string "f/n" → index into Code
}

func NewProgram() *Program {
	return &Program{
		Atoms:  NewAtomTable(),
		Labels: map[string]int{},
	}
}

func (p *Program) emit(op Opcode, a1, a2 int) {
	p.Code = append(p.Code, Instruction{op, a1, a2})
}
