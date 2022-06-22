# Doctests: Test interactive Golang examples in your code comments

`doctest` is a tool that checks code examples and properties in Go comments.
It is similar in spirit to the [popular Python module with the same name](https://docs.python.org/3/library/doctest.html) and [Haskell library with same name](https://github.com/sol/doctest).

## Getting Started

### Installation

You can install Doctests via go modules via:

```go
go install github.com/apitoolkit/doctests
```

### Running Doctest

The easiest way to run a doctest is via the CLI. To execute Doctest in the current Directory, simply run the doctest command with no arguments in the given directory. 

```go
doctests
```

Or give it a path or list of file paths or filepath glob

```go
doctests ./main.go
```
OR

```go
doctests ./main.go ./abc.go
```
OR (For every file in a project tree) 
```go
doctests ./**/*.go
```

### Wrting Doctests

`Doctest` comment lines always start with `// >>>`. The 3 greater than signs allows Doctest to detect that those lines are to be executed. The result is always inserted into the next line.

```go
package adder

// Add adds two numbers
// >>> adder.Add(1, 2)
// 3
func Add(a int, b int) int {
  return a+b
}
```
Notice that it's important to include the package name when calling functions.

## Contributors
- Anthony Alaribe
- Arne Wielding
- Mohamed Nabil 
- Omar Ahmed
