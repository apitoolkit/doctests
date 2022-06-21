package main

import (
	"bytes"
	"fmt"
	"go/doc"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"strings"

	"github.com/kr/pretty"
	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

func ParseComments() {
	rootPath := "../doctester/"
	fset := token.NewFileSet() // positions are relative to fset
	d, err := parser.ParseDir(fset, rootPath, nil, parser.ParseComments)
	if err != nil {
		fmt.Println(err)
		return
	}

	intp := interp.New(interp.Options{
		GoPath:               os.Getenv("GOPATH"),
		SourcecodeFilesystem: os.DirFS(rootPath),
	})
	intp.Use(stdlib.Symbols)

	for pkgName, packageNode := range d {
		_ = pkgName
		_ = packageNode
		for fileName, astFile := range packageNode.Files {
			// fmt.Println(fileName)
			_ = fileName
			_ = astFile

			// Evaluate the current file so that the comments can refer to the package
			_, err := intp.EvalPath(strings.Split(fileName, rootPath)[1])
			if err != nil {
				panic(err)
			}
			for _, comment := range astFile.Comments {
				pretty.Println(comment)
				for currCommentLineIndex, commentLine := range comment.List {
					if strings.HasPrefix(commentLine.Text, "// >>>") {
						expr := strings.TrimPrefix(commentLine.Text, "// >>> ")
						resp, err := intp.Eval(expr)
						if err != nil {
							panic(err)
						}
						newRespValue := fmt.Sprint(resp)
						nextLineResponse := "// " + newRespValue
						fmt.Println("RESP=", nextLineResponse)

						if len(comment.List) > (currCommentLineIndex + 1) {
							nextCommentLine := comment.List[currCommentLineIndex+1]
							if nextCommentLine.Text == nextLineResponse {
								continue
							}
							oldValue := strings.TrimPrefix(nextCommentLine.Text, "// ")
							nextCommentLine.Text = fmt.Sprintf("// WAS %s \n // NOW %s", oldValue, newRespValue)
						} else {
							// Last comment line
							commentLine.Text = commentLine.Text + "\n" + nextLineResponse
						}
					}
				}
			}

			// pretty.Println(astFile)
			var buf bytes.Buffer
			if err := format.Node(&buf, fset, astFile); err != nil {
				panic(err)
			}

			fmt.Printf("%s", buf.Bytes())

			err = os.WriteFile(fileName, buf.Bytes(), 0666)
			if err != nil {
				panic(err)
			}
		}
	}
}

func ParseComments2() {
	fset := token.NewFileSet() // positions are relative to fset
	d, err := parser.ParseDir(fset, "../doctester/", nil, parser.ParseComments)
	if err != nil {
		fmt.Println(err)
		return
	}

	for k, packageNode := range d {
		fmt.Println("package", k)
		p := doc.New(packageNode, "../doctester/", 3)

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

					exports := intp.Symbols("/Users/tonyalaribe/Projects/doctester/")
					pretty.Println("Exports", exports)

					_ = intp
					intp.ImportUsed()

					_, err := intp.EvalPath("dc.go")
					if err != nil {
						fmt.Println("error ", err)
					}

					r, err := intp.Eval("doctester.AddTest()")
					if err != nil {
						fmt.Println("error ", err)
					}

					fmt.Println(fmt.Sprint(r))

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
