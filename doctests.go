package main

import (
	"fmt"
	// "go/build"
	"go/doc"
	"go/parser"
	"go/token"
	"os"
	"strings"

	"github.com/kr/pretty"
	// "github.com/switchupcb/yaegi/interp"
	// "github.com/switchupcb/yaegi/stdlib"
	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

// >>> AddTest
// -- 3
func AddTest() int {
	return 2 + 3
}

// ParseComments xxx
func ParseComments() {
	fset := token.NewFileSet() // positions are relative to fset
	d, err := parser.ParseDir(fset, "../doctester/", nil, parser.ParseComments)
	if err != nil {
		fmt.Println(err)
		return
	}

	for k, packageNode := range d {
		fmt.Println("package", k)
		p := doc.New(packageNode, "../doctester/", 2)
		// p.Filter(doc.Filter(func(s string) bool {
		// 	fmt.Println("🔥", s)
		// 	return strings.HasPrefix(s, ">>>")
		// }))

		pretty.Println(p)
		for _, t := range p.Types {
			fmt.Println("type", t.Name)
			fmt.Println("docs:", t.Doc)

			docs := strings.Split(t.Doc, "\n")
			pretty.Println("🔥", docs)

		}

		for _, v := range p.Vars {
			fmt.Println("type", v.Names)
			fmt.Println("docs:", v.Doc)
		}

		for _, f := range p.Funcs {
			fmt.Println("type", f.Name)
			fmt.Println("docs:", f.Doc)

			docs := strings.Split(f.Doc, "\n")
			pretty.Println("🔥", docs)

			for i, v := range docs {
				_ = i
				if strings.HasPrefix(">>>", v) {
					cmd := strings.TrimPrefix(">>>", v)
					_ = cmd
					// f.Dec
					fs := os.DirFS("../doctester/")
					// cacheDir, err := os.UserCacheDir()
					// if err != nil {
					// 	fmt.Println("cacheDir", err)
					// }

					intp := interp.New(interp.Options{
						GoPath: os.Getenv("GOPATH"),
						// GoCache: cacheDir,
						// GoPath:               "/Users/tonyalaribe/go/",
						SourcecodeFilesystem: fs,
						// GoToolDir:            build.ToolDir,
						// Stdout:               &stdout, Stderr: &stderr,
						// Unrestricted:         true,
					})
					if err != nil {
						fmt.Println(err)
					}

					intp.Use(stdlib.Symbols)

					_ = intp
					intp.ImportUsed()

					// prog, err := intp.CompileAST(packageNode)
					// if err != nil {
					// 	fmt.Println("error compile AST ", err)
					// }
					// _ = prog

					// r, err := intp.Eval("fmt.Println(\"hello world \")")
					r, err := intp.Eval("fmt.Println(\"hello world \")")
					if err != nil {
						fmt.Println("error ", err)
					}
					pretty.Println("RESP bla bla", r)

					_, err = intp.EvalPath("./")
					if err != nil {
						fmt.Println("error ", err)
					}

					r, err = intp.Eval("AddTest()")
					if err != nil {
						fmt.Println("error ", err)
					}
					_ = r

				}
			}
		}

		for _, n := range p.Notes {
			fmt.Println("body", n[0].Body)
		}
	}
}

func main() {
	ParseComments()
}
