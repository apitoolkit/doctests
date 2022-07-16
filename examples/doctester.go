// Package examples is an example file to allow manually testing doctest in different scenarios.
// Feel free to open this file with the doctest lsp installed, and see it in action
package examples

import (
	"errors"

	"github.com/kr/pretty"
)

// Add adds two numbers
// >>> Add(2,3)
// 5
//
// >>> Add(-1,5)
// 4
//
// >>> Add(1,5)
// 6
func Add(a int, b int) int {
	return a + b
}

// XWithError ...
// >>> XWithError("hello world")
// (hello world,hello world)
func XWithError(a string) (string, error) {
	return a, errors.New(a)
}

// UseExternalImport ...
// >>> UseExternalImport("world", 25)
// doctester.T{V:"world", I:50}
//
func UseExternalImport(bl string, i int) string {
	return pretty.Sprint(SubtractT(bl, i))
}

type T struct {
	V string
	I int
}

// >>> SubtractT("hello", 25)
// {V:hello I:50}
//
// >>> SubtractT("world", 2)
// {V:world I:4}
//
// >>> SubtractT("Goer", 223)
// {V:Goer I:446}
func SubtractT(bla string, i int) T {
	return T{V: bla, I: i + i}
}
