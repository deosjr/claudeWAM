package main

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// ── Tokeniser ────────────────────────────────────────────────────────────────

type tokenKind int

const (
	tokAtom tokenKind = iota
	tokVar
	tokInt
	tokLParen
	tokRParen
	tokLBrack
	tokRBrack
	tokComma
	tokPipe
	tokDot
	tokTurnstile // :-
	tokEOF
)

type tok struct {
	kind tokenKind
	text string
}

func tokenise(src string) []tok {
	var out []tok
	i := 0
	for i < len(src) {
		// skip whitespace and comments
		if unicode.IsSpace(rune(src[i])) {
			i++
			continue
		}
		if src[i] == '%' {
			for i < len(src) && src[i] != '\n' {
				i++
			}
			continue
		}
		// two-char tokens
		if i+1 < len(src) && src[i:i+2] == ":-" {
			out = append(out, tok{tokTurnstile, ":-"})
			i += 2
			continue
		}
		switch src[i] {
		case '(':
			out = append(out, tok{tokLParen, "("})
		case ')':
			out = append(out, tok{tokRParen, ")"})
		case '[':
			out = append(out, tok{tokLBrack, "["})
		case ']':
			out = append(out, tok{tokRBrack, "]"})
		case ',':
			out = append(out, tok{tokComma, ","})
		case '|':
			out = append(out, tok{tokPipe, "|"})
		case '.':
			out = append(out, tok{tokDot, "."})
		default:
			// integer
			if unicode.IsDigit(rune(src[i])) {
				j := i
				for j < len(src) && unicode.IsDigit(rune(src[j])) {
					j++
				}
				out = append(out, tok{tokInt, src[i:j]})
				i = j
				continue
			}
			// atom (lowercase) or operator symbols
			if unicode.IsLower(rune(src[i])) || src[i] == '+' || src[i] == '-' || src[i] == '*' || src[i] == '/' || src[i] == '=' || src[i] == '<' || src[i] == '>' || src[i] == '\\' {
				j := i
				for j < len(src) && (unicode.IsLetter(rune(src[j])) || unicode.IsDigit(rune(src[j])) || src[j] == '_') {
					j++
				}
				if j == i {
					j++ // single non-alphanumeric symbol char
				}
				out = append(out, tok{tokAtom, src[i:j]})
				i = j
				continue
			}
			// variable (uppercase or _)
			if unicode.IsUpper(rune(src[i])) || src[i] == '_' {
				j := i
				for j < len(src) && (unicode.IsLetter(rune(src[j])) || unicode.IsDigit(rune(src[j])) || src[j] == '_') {
					j++
				}
				out = append(out, tok{tokVar, src[i:j]})
				i = j
				continue
			}
			// quoted atom
			if src[i] == '\'' {
				j := i + 1
				for j < len(src) && src[j] != '\'' {
					j++
				}
				out = append(out, tok{tokAtom, src[i+1 : j]})
				i = j + 1
				continue
			}
			panic(fmt.Sprintf("unexpected char %q at %d", src[i], i))
		}
		i++
	}
	out = append(out, tok{tokEOF, ""})
	return out
}

// ── Parser ───────────────────────────────────────────────────────────────────

type parser struct {
	toks []tok
	pos  int
}

func newParser(src string) *parser {
	return &parser{toks: tokenise(src)}
}

func (p *parser) peek() tok  { return p.toks[p.pos] }
func (p *parser) next() tok  { t := p.toks[p.pos]; p.pos++; return t }
func (p *parser) expect(k tokenKind) tok {
	t := p.next()
	if t.kind != k {
		panic(fmt.Sprintf("expected token kind %d got %q", k, t.text))
	}
	return t
}

// ParseClauses parses a full Prolog source string into a slice of Clauses.
func ParseClauses(src string) []Clause {
	p := newParser(src)
	var clauses []Clause
	for p.peek().kind != tokEOF {
		clauses = append(clauses, p.parseClause())
	}
	return clauses
}

func (p *parser) parseClause() Clause {
	head := p.parseCompound()
	if p.peek().kind == tokDot {
		p.next()
		return Clause{Head: head}
	}
	p.expect(tokTurnstile)
	var body []Compound
	for {
		body = append(body, p.parseCompound())
		if p.peek().kind == tokDot {
			p.next()
			break
		}
		p.expect(tokComma)
	}
	return Clause{Head: head, Body: body}
}

// parseCompound parses f(args) or a bare atom as a 0-arity compound.
func (p *parser) parseCompound() Compound {
	name := p.expect(tokAtom).text
	if p.peek().kind != tokLParen {
		return Compound{Functor: name}
	}
	p.next() // consume (
	var args []Term
	for p.peek().kind != tokRParen {
		args = append(args, p.parseTerm())
		if p.peek().kind == tokComma {
			p.next()
		}
	}
	p.next() // consume )
	return Compound{Functor: name, Args: args}
}

// parseTerm parses any term (variable, integer, atom, compound, list).
func (p *parser) parseTerm() Term {
	switch p.peek().kind {
	case tokVar:
		return Var{p.next().text}
	case tokInt:
		n, _ := strconv.Atoi(p.next().text)
		return Integer{n}
	case tokLBrack:
		return p.parseList()
	case tokAtom:
		// may be f(...) or bare atom
		name := p.next().text
		if p.peek().kind == tokLParen {
			p.next()
			var args []Term
			for p.peek().kind != tokRParen {
				args = append(args, p.parseTerm())
				if p.peek().kind == tokComma {
					p.next()
				}
			}
			p.next()
			return Compound{Functor: name, Args: args}
		}
		return Atom{name}
	}
	panic(fmt.Sprintf("unexpected token %q in term", p.peek().text))
}

// parseList handles [], [H|T], [a,b,c], [a,b|T].
func (p *parser) parseList() Term {
	p.next() // consume [
	if p.peek().kind == tokRBrack {
		p.next()
		return Atom{"[]"}
	}
	var heads []Term
	for {
		heads = append(heads, p.parseTerm())
		if p.peek().kind == tokPipe {
			p.next()
			tail := p.parseTerm()
			p.expect(tokRBrack)
			return buildList(heads, tail)
		}
		if p.peek().kind == tokRBrack {
			p.next()
			return buildList(heads, Atom{"[]"})
		}
		p.expect(tokComma)
	}
}

func buildList(heads []Term, tail Term) Term {
	out := tail
	for i := len(heads) - 1; i >= 0; i-- {
		out = Compound{Functor: ".", Args: []Term{heads[i], out}}
	}
	return out
}

// MustParseQuery parses a single compound (for queries, no :- or .).
func MustParseQuery(src string) Compound {
	src = strings.TrimSpace(src)
	if !strings.HasSuffix(src, ".") {
		src += "."
	}
	p := newParser(src)
	c := p.parseCompound()
	return c
}
