package main

import (
	"flag"
	"fmt"
	"sort"
	"strings"
)

// src is the example program used when no source file is provided.
// It mirrors the append example from bowenProlog/main.go.
const exampleSrc = `
	append(nil, L, L).
	append(cons(X,L1), L2, cons(X,L3)) :- append(L1, L2, L3).
`

// exampleQueries are run when no query is given on the command line.
var exampleQueries = []string{
	"append(cons(a,cons(b,nil)),cons(c,nil),L)",
	"append(L,X,cons(a,cons(b,cons(c,nil))))",
}

func main() {
	all := flag.Bool("all", false, "enumerate all solutions via backtracking (default: stop at first)")
	flag.Parse()

	queries := exampleQueries
	if flag.NArg() > 0 {
		queries = flag.Args()
	}

	for _, q := range queries {
		fmt.Printf("?- %s\n", q)
		runQuery(exampleSrc, q, *all)
		fmt.Println()
	}
}

// runQuery compiles src, executes query, and prints the results.
// When findAll is false only the first solution is shown; when true every
// solution is printed separated by ";" in the classic Prolog REPL style.
func runQuery(src, query string, findAll bool) {
	q := NewQuery(src, query)

	bindings, ok := q.Next()
	if !ok {
		fmt.Println("   false.")
		return
	}
	printBindings(bindings)

	if !findAll {
		if q.MayHaveMore() {
			fmt.Println("   (more solutions possible; use -all)")
		} else {
			fmt.Println("   (1 solution)")
		}
		return
	}

	// -all: keep asking for the next answer until the machine says no.
	for {
		bindings, ok = q.Next()
		if !ok {
			fmt.Println(".")
			break
		}
		fmt.Println(";")
		printBindings(bindings)
	}
}

// printBindings prints one solution's variable bindings in alphabetical order.
func printBindings(bindings map[string]string) {
	if len(bindings) == 0 {
		fmt.Println("   true.")
		return
	}
	names := make([]string, 0, len(bindings))
	for n := range bindings {
		names = append(names, n)
	}
	sort.Strings(names)
	parts := make([]string, len(names))
	for i, n := range names {
		parts[i] = fmt.Sprintf("%s = %s", n, bindings[n])
	}
	fmt.Printf("   %s\n", strings.Join(parts, ",  "))
}
