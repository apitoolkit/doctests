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

	"github.com/kr/pretty"
	"github.com/spf13/cobra"
	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

func ParseCommentForFileGlob(files []string) {
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
	for k, v := range pathToFiles {
		ParseComments(k, v)
	}
}

func ParseComments(rootPath string, files []string) {
	if rootPath == "." {
		rootPath = "./"
	}
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
		for fileName, astFile := range packageNode.Files {
			// Evaluate the current file so that the comments can refer to the package
			pretty.Println(strings.Split(fileName, rootPath), fileName, rootPath)

			fileToEvalList := strings.Split(fileName, rootPath)
			fileToEval := fileToEvalList[0]
			if len(fileToEvalList) > 1 {
				fileToEval = fileToEvalList[1]
			}

			_, err := intp.EvalPath(fileToEval)
			if err != nil {
				panic(err)
			}

			for _, comment := range astFile.Comments {
				for currCommentLineIndex, commentLine := range comment.List {
					if strings.HasPrefix(commentLine.Text, "// >>>") {
						expr := strings.TrimPrefix(commentLine.Text, "// >>> ")
						resp, err := intp.Eval(expr)
						if err != nil {
							panic(err)
						}
						newRespValue := fmt.Sprint(resp)
						nextLineResponse := "// " + newRespValue

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

			var buf bytes.Buffer
			err = format.Node(&buf, fset, astFile)
			if err != nil {
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

func main() {
	rootCmd := &cobra.Command{
		Use:   "doctest",
		Short: "Doctest will execute doctest blocks in your comments and update their results",
		Run: func(cmd *cobra.Command, args []string) {
			ParseCommentForFileGlob(args)
		},
	}
	rootCmd.Execute()
}
