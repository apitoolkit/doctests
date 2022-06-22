package main

import (
	"bytes"
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/gookit/color"
	"github.com/kr/pretty"
	"github.com/spf13/cobra"
	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

type ReportItem struct {
	Expr     string
	Failed   bool
	Previous string
	Current  string
}

func ParseCommentsForFileGlob(files []string) {
	reports := ParseCommentForFileGlob2(files)
	var failed bool
	for _, r := range reports {
		fmt.Printf("// >>> %s\n", r.Expr)
		if r.Failed {
			failed = true
			color.Error.Printf("// WAS %s\n// NOW %s", r.Previous, r.Current)
		} else {
			fmt.Printf("// %s\n", r.Current)
		}
		fmt.Println("\n\n")
	}
	if failed {
		color.Error.Println("DOCTESTS FAILED")
	} else {
		color.Green.Println("DOCTESTS SUCCEEDED WITH NO FAILURES")
	}
}

func ParseCommentForFileGlob2(files []string) []ReportItem {
	pathToFiles := map[string][]string{}
	for _, f := range files {
		dir := filepath.Dir(f)
		base := filepath.Base(f)
		existingL, exists := pathToFiles[dir]
		if exists {
			existingL = append(existingL, base)
			pathToFiles[dir] = existingL
		} else {
			pathToFiles[dir] = []string{base}
		}
	}
	reports := []ReportItem{}
	for k, v := range pathToFiles {
		reports_ := ParseComments(k, v)
		reports = append(reports, reports_...)
	}
	return reports
}

func ParseComments(rootPath string, files []string) []ReportItem {
	reports := []ReportItem{}

	if rootPath == "." {
		rootPath = "./"
	}
	fset := token.NewFileSet() // positions are relative to fset
	d, err := parser.ParseDir(fset, rootPath, nil, parser.ParseComments)
	if err != nil {
		panic(err)
	}

	intp := interp.New(interp.Options{
		GoPath:               os.Getenv("GOPATH"),
		SourcecodeFilesystem: os.DirFS(rootPath),
	})
	intp.Use(stdlib.Symbols)

	for pkgName, packageNode := range d {
		_ = pkgName
		for fileName, astFile := range packageNode.Files {
			// Evaluate the current file so that the comments can refer to the package
			pretty.Println(strings.Split(fileName, rootPath), fileName, rootPath)

			fileToEvalList := strings.Split(fileName, rootPath)
			fileToEval := filepath.Base(fileToEvalList[0])
			if len(fileToEvalList) > 1 {
				fileToEval = filepath.Base(fileToEvalList[1])
			}

			fmt.Println("FILE TO EVAL", fileToEval)
			_, err := intp.EvalPath(fileToEval)
			if err != nil {
				panic(err)
			}

			for _, comment := range astFile.Comments {
				for currCommentLineIndex, commentLine := range comment.List {
					if !strings.HasPrefix(commentLine.Text, "// >>>") {
						continue
					}

					report := ReportItem{}

					expr := strings.TrimPrefix(commentLine.Text, "// >>> ")
					report.Expr = expr

					resp, err := intp.Eval(expr)
					if err != nil {
						panic(err)
					}

					report.Current = fmt.Sprint(resp)
					nextLineResponse := "// " + report.Current

					if len(comment.List) > (currCommentLineIndex + 1) {
						nextCommentLine := comment.List[currCommentLineIndex+1]
						if nextCommentLine.Text == nextLineResponse {
							continue
						}
						report.Previous = strings.TrimPrefix(nextCommentLine.Text, "// ")
						report.Failed = true
						nextCommentLine.Text = fmt.Sprintf("// WAS %s \n // NOW %s", report.Previous, report.Current)
					} else {
						// Last comment line
						commentLine.Text = commentLine.Text + "\n" + nextLineResponse
					}

					reports = append(reports, report)
				}
			}

			var buf bytes.Buffer
			err = format.Node(&buf, fset, astFile)
			if err != nil {
				panic(err)
			}

			// fmt.Printf("%s", buf.Bytes())

			err = os.WriteFile(fileName, buf.Bytes(), 0666)
			if err != nil {
				panic(err)
			}
		}
	}
	return reports
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "doctest",
		Short: "Doctest will execute doctest blocks in your comments and update their results",
		Run: func(cmd *cobra.Command, args []string) {
			ParseCommentsForFileGlob(args)
		},
	}
	rootCmd.Execute()
}
