package main

import (
	"bytes"
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/cosmos72/gomacro/fast"
	"github.com/gookit/color"
	"github.com/kr/pretty"
	"github.com/spf13/cobra"
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
	"github.com/tliron/glsp/server"
	"github.com/tliron/kutil/logging"
	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"

	// Must include a backend implementation. See kutil's logging/ for other options.
	simple "github.com/tliron/kutil/logging/simple"
)

func strPtr(str string) *string {
	return &str
}

var logPath *string

type ReportItem struct {
	Expr     string
	Failed   bool
	Previous string
	Current  string
}

func ParseCommentsForFileGlob(files []string) {
	reports := ParseCommentsForFileGlob2(files)
	var failed bool
	for _, r := range reports {
		fmt.Printf("// >>> %s\n", r.Expr)
		if r.Failed {
			failed = true
			color.Error.Printf("// WAS %s\n// NOW %s", r.Previous, r.Current)
		} else {
			fmt.Printf("// %s\n", r.Current)
		}
		fmt.Print("\n\n")
	}
	if failed {
		color.Error.Println("DOCTESTS FAILED")
	} else {
		color.Green.Println("DOCTESTS SUCCEEDED WITH NO FAILURES")
	}
}

func ParseCommentsForFileGlob2(files []string) []ReportItem {
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
			// pretty.Println(strings.Split(fileName, rootPath), fileName, rootPath)

			fileToEvalList := strings.Split(fileName, rootPath)
			fileToEval := filepath.Base(fileToEvalList[0])
			if len(fileToEvalList) > 1 {
				fileToEval = filepath.Base(fileToEvalList[1])
			}

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

					if len(comment.List) <= (currCommentLineIndex + 1) {
						commentLine.Text = commentLine.Text + "\n" + nextLineResponse
						reports = append(reports, report)
						continue
					}

					nextCommentLine := comment.List[currCommentLineIndex+1]
					if nextCommentLine.Text == nextLineResponse {
						continue
					}
					report.Previous = strings.TrimPrefix(nextCommentLine.Text, "// ")
					report.Failed = true
					if strings.HasPrefix(nextCommentLine.Text, "// WAS ") {
						if len(comment.List) > (currCommentLineIndex + 2) {
							nowLine := comment.List[currCommentLineIndex+2]
							if strings.HasPrefix(nowLine.Text, "// NOW ") {
								report.Previous = strings.TrimPrefix(nextCommentLine.Text, "// WAS ")
								// report.Previous = strings.TrimPrefix(nowLine.Text, "// NOW ")
								// Maybe we shouldnt change the WAS
								// nextCommentLine.Text = "// WAS " + report.Previous
								nowLine.Text = "// NOW " + report.Current
							}
						}
					} else {
						nextCommentLine.Text = fmt.Sprintf("// WAS %s \n// NOW %s", report.Previous, report.Current)
					}

					reports = append(reports, report)
				}
			}

			var buf bytes.Buffer
			err = format.Node(&buf, fset, astFile)
			if err != nil {
				panic(err)
			}

			err = os.WriteFile(fileName, buf.Bytes(), 0o666)
			if err != nil {
				panic(err)
			}
		}
	}
	return reports
}

var (
	lsName         = "doctest_lsp"
	version string = "0.0.1"
	handler protocol.Handler
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "doctest",
		Short: "Doctest will execute doctest blocks in your comments and update their results",
	}

	fmtCmd := &cobra.Command{
		Use:   "fmt",
		Short: "fmt",
		Run: func(_ *cobra.Command, args []string) {
			ParseCommentsForFileGlob(args)
		},
	}
	rootCmd.AddCommand(fmtCmd)

	interpreter := &GoMacroInterpreter{}

	lspCmd := &cobra.Command{
		Use:   "lsp",
		Short: "lsp new",
		Run: func(cmd *cobra.Command, args []string) {
			const lsName = "doctests"
			backend := simple.NewBackend()
			backend.Configure(1, strPtr("/Users/tonyalaribe/doctest.log"))
			logging.SetBackend(backend)

			handler = protocol.Handler{
				Initialize: func(context *glsp.Context, params *protocol.InitializeParams) (interface{}, error) {
					protocol.SetTraceValue(protocol.TraceValueVerbose)

					capabilities := handler.CreateServerCapabilities()
					t := true

					capabilities.CodeLensProvider.ResolveProvider = &t
					// capabilities.CodeLensProvider.WorkDoneProgress = &t

					capabilities.ExecuteCommandProvider.Commands = append(capabilities.ExecuteCommandProvider.Commands, "codelens.evaluate")
					// capabilities.ExecuteCommandProvider.WorkDoneProgress = &t

					result := protocol.InitializeResult{
						Capabilities: capabilities,
						ServerInfo: &protocol.InitializeResultServerInfo{
							Name:    lsName,
							Version: &version,
						},
					}
					return result, nil
				},
				Initialized: func(context *glsp.Context, params *protocol.InitializedParams) error {
					// context.Notify(protocol.ServerWorkspaceCodeLensRefresh, nil)
					return nil
				},
				Shutdown: func(_ *glsp.Context) error {
					protocol.SetTraceValue(protocol.TraceValueVerbose)
					return nil
				},
				SetTrace: func(_ *glsp.Context, params *protocol.SetTraceParams) error {
					protocol.SetTraceValue(params.Value)
					return nil
				},
				TextDocumentCodeLens: func(context *glsp.Context, params *protocol.CodeLensParams) ([]protocol.CodeLens, error) {
					str := strings.Replace((string)(params.TextDocument.URI), "file://", "", 1)
					return parseFileAndReturnCodeLenses(str), nil
				},
				WorkspaceExecuteCommand: func(context *glsp.Context, params *protocol.ExecuteCommandParams) (interface{}, error) {
					if m := logging.NewMessage([]string{"engine", "parser"}, logging.Info, 0); m != nil {
						m.Set("message", "WorkspaceExecuteCommand").
							Set("req", pretty.Sprint(params)).
							Send()
					}
					if params.Command == "codelens.evaluate" {
						if m := logging.NewMessage([]string{"engine", "parser"}, logging.Info, 0); m != nil {
							m.Set("message", "PRE - RESOLVE LensEdit  WorkspaceExecuteCommand").
								Set("req", pretty.Sprint(params)).
								Send()
						}
						edits := resolveLensEdit(params.Arguments[0].(string), params.Arguments[1].(string), interpreter)
						if m := logging.NewMessage([]string{"engine", "parser"}, logging.Info, 0); m != nil {
							m.Set("message", "POST WorkspaceExecuteCommand").
								Set("edits", pretty.Sprint(edits)).
								Send()
						}
						if m := logging.NewMessage([]string{"engine", "parser"}, logging.Info, 0); m != nil {
							m.Set("message", "RESOLVE LensEdit  WorkspaceExecuteCommand").
								Set("req", pretty.Sprint(params)).
								Set("edits", edits).
								Send()
						}
						context.Notify(protocol.ServerWorkspaceApplyEdit, protocol.ApplyWorkspaceEditParams{
							Label: strPtr("evaluated doctest"),
							Edit:  edits,
						})
						context.Notify(protocol.ServerWorkspaceCodeLensRefresh, nil)
					}
					return nil, nil
				},
			}

			server := server.NewServer(&handler, lsName, true)
			server.RunStdio()
		},
	}

	rootCmd.AddCommand(lspCmd)
	rootCmd.Execute()
}

func parseFileAndReturnCodeLenses(file string) []protocol.CodeLens {
	fset := token.NewFileSet() // positions are relative to fset
	d, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
	if err != nil {
		panic(err)
	}
	codelenses := []protocol.CodeLens{}
	for _, commentG := range d.Comments {
		for idx, comment := range commentG.List {
			if strings.HasPrefix(comment.Text, "// >>>") {
				commandTitle := "Evaluate"
				if len(commentG.List) > idx+1 {
					commandTitle = "Refresh"
				}

				pos := fset.Position(comment.Pos())
				end := fset.Position(comment.End())
				codeL := protocol.CodeLens{
					Range: protocol.Range{
						Start: protocol.Position{
							Line:      protocol.UInteger(pos.Line - 1),
							Character: protocol.UInteger(pos.Column),
						},
						End: protocol.Position{
							Line:      protocol.UInteger(end.Line - 1),
							Character: protocol.UInteger(end.Column),
						},
					},
					Command: &protocol.Command{
						Title:     commandTitle,
						Command:   "codelens.evaluate",
						Arguments: []interface{}{file, comment.Text, strings.TrimPrefix(comment.Text, "// >>> ")},
					},
				}
				codelenses = append(codelenses, codeL)
			}
		}
	}
	return codelenses
}

func resolveLensEdit(file string, cmdLine string, intp interpreter) protocol.WorkspaceEdit {
	fset := token.NewFileSet() // positions are relative to fset
	d, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
	if err != nil {
		panic(err)
	}

	textEdits := []protocol.TextEdit{}
	for _, commentG := range d.Comments {
		for idx, comment := range commentG.List {
			if strings.HasPrefix(comment.Text, "// >>>") && (comment.Text == cmdLine) {
				pos := fset.Position(comment.Pos())
				_, err := intp.InitFile(file)
				if err != nil {
					panic(err)
				}

				expr := strings.TrimPrefix(comment.Text, "// >>> ")
				resp, err := intp.Eval(expr)
				if err != nil {
					panic(err)
				}

				if len(commentG.List) <= idx+1 || strings.TrimSpace(commentG.List[idx+1].Text) == "//" || strings.TrimSpace(commentG.List[idx+1].Text) == "// -" {
					resp2 := "// " + resp + "\n"

					start := protocol.Position{
						Line:      protocol.UInteger(pos.Line),
						Character: protocol.UInteger(0),
					}
					tE := protocol.TextEdit{
						Range: protocol.Range{
							Start: start,
							End:   start,
						},
						NewText: resp2,
					}

					textEdits = append(textEdits, tE)
					continue
				}
				nextCommentLine := commentG.List[idx+1]
				if strings.HasPrefix(nextCommentLine.Text, "// WAS") && len(commentG.List) >= (idx+2) {
					nowLine := commentG.List[idx+2]
					if strings.HasPrefix(nowLine.Text, "// NOW ") {
						pos := fset.Position(nowLine.Pos())
						start := protocol.Position{
							Line:      protocol.UInteger(pos.Line - 1),
							Character: protocol.UInteger(0),
						}

						end := fset.Position(nowLine.End())
						tE := protocol.TextEdit{
							Range: protocol.Range{
								Start: start,
								End: protocol.Position{
									Line:      protocol.UInteger(end.Line - 1),
									Character: protocol.UInteger(end.Column),
								},
							},
							NewText: "// NOW " + resp,
						}

						textEdits = append(textEdits, tE)
					}
				} else {
					prev := strings.TrimPrefix(nextCommentLine.Text, "// ")
					if strings.TrimSpace(prev) == strings.TrimSpace(resp) {
						continue
					}

					pos := fset.Position(nextCommentLine.Pos())
					start := protocol.Position{
						Line:      protocol.UInteger(pos.Line - 1),
						Character: protocol.UInteger(0),
					}

					end := fset.Position(nextCommentLine.End())
					tE := protocol.TextEdit{
						Range: protocol.Range{
							Start: start,
							End: protocol.Position{
								Line:      protocol.UInteger(end.Line - 1),
								Character: protocol.UInteger(end.Column),
							},
						},
						NewText: fmt.Sprintf("// WAS %s \n// NOW %s", prev, resp),
					}

					textEdits = append(textEdits, tE)
				}
			}
		}
	}
	edit := protocol.WorkspaceEdit{
		Changes: map[protocol.DocumentUri][]protocol.TextEdit{
			"file://" + file: textEdits,
		},
	}
	return edit
}

// *********************************
// Interpreter interface
// *********************************
type interpreter interface {
	InitFile(file string) (interpreter, error)
	Eval(expr string) (string, error)
}

// *********************************
// Interpreter interface implementation for yaegi
// *********************************
type YaegiInterpreter struct {
	intp *interp.Interpreter
}

func (y *YaegiInterpreter) InitFile(file string) (interpreter, error) {
	intp := interp.New(interp.Options{
		GoPath:               os.Getenv("GOPATH"),
		SourcecodeFilesystem: os.DirFS(path.Dir(file)),
	})
	intp.Use(stdlib.Symbols)

	_, err := intp.EvalPath(path.Base(file))
	if err != nil {
		return y, err
	}

	y.intp = intp
	return y, nil
}

func (y *YaegiInterpreter) Eval(expr string) (string, error) {
	resp_, err := y.intp.Eval(expr)
	resp := fmt.Sprintf("%+v", resp_)
	return resp, err
}

// *********************************
// Interpreter interface implementation for gomacro
// *********************************
type GoMacroInterpreter struct {
	intp *fast.Interp
}

func (i *GoMacroInterpreter) InitFile(file string) (interpreter, error) {
	intp := fast.New()
	intp.ImportPackage(".", path.Dir(file))
	i.intp = intp
	return i, nil
}

func (i *GoMacroInterpreter) Eval(expr string) (string, error) {
	resp_, _ := i.intp.Eval(expr)

	rStr := []string{}
	for _, v := range resp_ {
		rStr = append(rStr, fmt.Sprintf("%+v", v.ReflectValue()))
	}

	if len(rStr) > 1 {
		return fmt.Sprintf("(%s)", strings.Join(rStr, ",")), nil
	}
	return strings.Join(rStr, ","), nil
}
