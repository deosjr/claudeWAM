package main

import (
	"fmt"
	"strings"
)

// ReadTerm reconstructs a human-readable Prolog term from a heap cell.
func (m *WAM) ReadTerm(c Cell) string {
	return m.readTerm(m.deref(c), map[int]bool{})
}

func (m *WAM) readTerm(c Cell, seen map[int]bool) string {
	switch c.Tag() {
	case TagREF:
		return fmt.Sprintf("_%d", c.Addr())
	case TagATM:
		return m.Atoms.AtomName(c.Addr())
	case TagINT:
		return fmt.Sprintf("%d", int(int64(c.Val())))
	case TagSTR:
		addr := c.Addr()
		if seen[addr] {
			return "..."
		}
		seen[addr] = true
		fun := m.Atoms.FuncEntry(m.Heap[addr].Addr())
		if fun.Arity == 0 {
			return fun.Name
		}
		args := make([]string, fun.Arity)
		for i := range args {
			args[i] = m.readTerm(m.deref(m.Heap[addr+1+i]), seen)
		}
		return fmt.Sprintf("%s(%s)", fun.Name, strings.Join(args, ","))
	case TagLIS:
		return m.readList(c, seen)
	}
	return "?"
}

func (m *WAM) readList(c Cell, seen map[int]bool) string {
	parts := []string{}
	for {
		d := m.deref(c)
		if d == ATM(m.Atoms.Atom("[]")) || d == ATM(m.Atoms.Atom("nil")) {
			break
		}
		if !d.IsLIS() {
			parts = append(parts, "|"+m.readTerm(d, seen))
			break
		}
		parts = append(parts, m.readTerm(m.deref(m.Heap[d.Addr()]), seen))
		c = m.Heap[d.Addr()+1]
	}
	return "[" + strings.Join(parts, ",") + "]"
}
