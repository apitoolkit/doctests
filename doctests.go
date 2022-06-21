package main

import (
	"fmt"
	"go/doc"
	"go/parser"
	"go/token"
	"os"
	"strings"

	"github.com/kr/pretty"
	"github.com/traefik/yaegi/interp"
)

// >>> AddTest
// -- 3
func AddTest() int {
	return 2 + 3
}

// ParseComments xxx
func ParseComments() {
	fset := token.NewFileSet() // positions are relative to fset
	d, err := parser.ParseDir(fset, "./", nil, parser.ParseComments)
	if err != nil {
		fmt.Println(err)
		return
	}

	for k, f := range d {
		fmt.Println("package", k)
		p := doc.New(f, "./", 2)
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
					fs := os.DirFS("./")
					intp := interp.New(interp.Options{
						SourcecodeFilesystem: fs,
						Unrestricted:         true,
					})
					if err != nil {
						fmt.Println(err)
					}

					_ = intp
					intp

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
