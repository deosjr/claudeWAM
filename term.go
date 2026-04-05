package main

import (
	"fmt"
	"strings"
)

// Term is the AST node type for parsed Prolog terms.
type Term interface{ isTerm() }

type Var struct{ Name string }      // uppercase or _ prefix
type Atom struct{ Name string }     // lowercase
type Integer struct{ Val int }      // numeric literal
type Compound struct {              // f(t1, ..., tn)
	Functor string
	Args    []Term
}

func (Var) isTerm()      {}
func (Atom) isTerm()     {}
func (Integer) isTerm()  {}
func (Compound) isTerm() {}

// Clause is a compiled Prolog clause: head :- body (body may be empty for facts).
type Clause struct {
	Head Compound
	Body []Compound
}

func (v Var) String() string { return v.Name }
func (a Atom) String() string { return a.Name }
func (n Integer) String() string { return fmt.Sprintf("%d", n.Val) }
func (c Compound) String() string {
	if len(c.Args) == 0 {
		return c.Functor
	}
	args := make([]string, len(c.Args))
	for i, a := range c.Args {
		args[i] = fmt.Sprintf("%v", a)
	}
	return fmt.Sprintf("%s(%s)", c.Functor, strings.Join(args, ","))
}
func (cl Clause) String() string {
	if len(cl.Body) == 0 {
		return cl.Head.String() + "."
	}
	body := make([]string, len(cl.Body))
	for i, b := range cl.Body {
		body[i] = b.String()
	}
	return fmt.Sprintf("%s :- %s.", cl.Head.String(), strings.Join(body, ", "))
}

// collectVarNames returns every variable name that appears in a term (may have duplicates).
func collectVarNames(t Term) []string {
	switch v := t.(type) {
	case Var:
		return []string{v.Name}
	case Compound:
		var out []string
		for _, arg := range v.Args {
			out = append(out, collectVarNames(arg)...)
		}
		return out
	}
	return nil
}
