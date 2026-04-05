package main

import "fmt"

// Functor is a (name, arity) pair, e.g. cons/2.
type Functor struct {
	Name  string
	Arity int
}

func (f Functor) String() string { return fmt.Sprintf("%s/%d", f.Name, f.Arity) }

// AtomTable interns atom names and functors into integer IDs so that a Cell
// can represent either with a single uint64 value field.
type AtomTable struct {
	atoms    []string
	atomIdx  map[string]int
	functors []Functor
	funcIdx  map[Functor]int
}

func NewAtomTable() *AtomTable {
	return &AtomTable{
		atomIdx: map[string]int{},
		funcIdx: map[Functor]int{},
	}
}

func (t *AtomTable) Atom(name string) int {
	if id, ok := t.atomIdx[name]; ok {
		return id
	}
	id := len(t.atoms)
	t.atoms = append(t.atoms, name)
	t.atomIdx[name] = id
	return id
}

func (t *AtomTable) AtomName(id int) string { return t.atoms[id] }

func (t *AtomTable) Func(name string, arity int) int {
	f := Functor{name, arity}
	if id, ok := t.funcIdx[f]; ok {
		return id
	}
	id := len(t.functors)
	t.functors = append(t.functors, f)
	t.funcIdx[f] = id
	return id
}

func (t *AtomTable) FuncEntry(id int) Functor { return t.functors[id] }
