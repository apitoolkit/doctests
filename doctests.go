package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/TobiasYin/go-lsp/logs"
	"github.com/TobiasYin/go-lsp/lsp"
	"github.com/TobiasYin/go-lsp/lsp/defines"
	"github.com/gookit/color"
	"github.com/kr/pretty"
	"github.com/samber/lo"
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

func init() {
	// -----
	var logger *log.Logger
	defer func() {
		logs.Init(logger)
	}()
	logPath = flag.String("logs", "", "logs file path")
	// logPath = strPtr("~/doctest.log")
	if logPath == nil || *logPath == "" {
		logger = log.New(os.Stderr, "", 0)
		return
	}
	p := *logPath
	f, err := os.Open(p)
	if err == nil {
		logger = log.New(f, "", 0)
		return
	}
	f, err = os.Create(p)
	if err == nil {
		logger = log.New(f, "", 0)
		return
	}
	panic(fmt.Sprintf("logs init error: %v", *logPath))
}

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
						// Last comment line
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

			// fmt.Printf("%s", buf.Bytes())

			err = os.WriteFile(fileName, buf.Bytes(), 0666)
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

func initialize(context *glsp.Context, params *protocol.InitializeParams) (interface{}, error) {
	protocol.SetTraceValue(protocol.TraceValueVerbose)

	capabilities := handler.CreateServerCapabilities()
	t := true
	cc := protocol.CodeActionOptions{
		CodeActionKinds: []string{"QuickFix"},
		ResolveProvider: &t,
	}
	cc.WorkDoneProgress = &t

	capabilities.CodeActionProvider = cc

	capabilities.CodeLensProvider.ResolveProvider = &t
	capabilities.CodeLensProvider.WorkDoneProgress = &t

	capabilities.ExecuteCommandProvider.Commands = append(capabilities.ExecuteCommandProvider.Commands, "codelens.evaluate")
	capabilities.ExecuteCommandProvider.WorkDoneProgress = &t

	result := protocol.InitializeResult{
		Capabilities: capabilities,
		ServerInfo: &protocol.InitializeResultServerInfo{
			Name:    lsName,
			Version: &version,
		},
	}
	if m := logging.NewMessage([]string{"engine", "parser"}, logging.Info, 0); m != nil {
		m.Set("message", "Initialize").
			Set("req", pretty.Sprint(result)).
			Set("params", pretty.Sprint(params)).
			Send()
	}
	return result, nil
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "doctest",
		Short: "Doctest will execute doctest blocks in your comments and update their results",
		Run: func(cmd *cobra.Command, args []string) {
			ParseCommentsForFileGlob(args)
		},
	}

	fmtCmd := &cobra.Command{
		Use:   "fmt",
		Short: "fmt",
		Run: func(cmd *cobra.Command, args []string) {
			ParseCommentsForFileGlob(args)
		},
	}
	rootCmd.AddCommand(fmtCmd)

	lspCmd := &cobra.Command{
		Use:   "lsp",
		Short: "lsp new",
		Run: func(cmd *cobra.Command, args []string) {
			const lsName = "doctests"
			backend := simple.NewBackend()
			backend.Configure(1, strPtr("/Users/tonyalaribe/doctest.log"))
			logging.SetBackend(backend)

			handler = protocol.Handler{
				Initialize:  initialize,
				Initialized: initialized,
				Shutdown:    shutdown,
				SetTrace:    setTrace,
				TextDocumentDidSave: func(context *glsp.Context, params *protocol.DidSaveTextDocumentParams) error {
					// if m := logging.NewMessage([]string{"engine", "parser"}, logging.Info, 0); m != nil {
					// 	m.Set("message", "Did Save").
					// 		Set("req", pretty.Sprint(params)).
					// 		Send()
					// }
					// str := strings.Replace((string)(params.TextDocument.URI), "file://", "", 1)
					// ParseCommentsForFileGlob2([]string{str})
					return nil
				},
				TextDocumentCodeLens: func(context *glsp.Context, params *protocol.CodeLensParams) ([]protocol.CodeLens, error) {
					if m := logging.NewMessage([]string{"engine", "parser"}, logging.Info, 0); m != nil {
						m.Set("message", "CodeLens").
							Set("req", pretty.Sprint(params)).
							Send()
					}
					str := strings.Replace((string)(params.TextDocument.URI), "file://", "", 1)
					return parseFileAndReturnCodeLenses(str), nil
				},
				TextDocumentCodeAction: func(context *glsp.Context, params *protocol.CodeActionParams) (interface{}, error) {
					if m := logging.NewMessage([]string{"engine", "parser"}, logging.Info, 0); m != nil {
						m.Set("message", "CodeAction").
							Set("req", pretty.Sprint(params)).
							Send()
					}
					str := strings.Replace((string)(params.TextDocument.URI), "file://", "", 1)
					return parseFileAndReturnCodeActions(str), nil
				},
				CodeActionResolve: func(context *glsp.Context, params *protocol.CodeAction) (*protocol.CodeAction, error) {
					params = resolveEditCodeAction(params)
					if m := logging.NewMessage([]string{"engine", "parser"}, logging.Info, 0); m != nil {
						m.Set("message", "CodeActionResolve").
							Set("req", pretty.Sprint(params)).
							Set("params", pretty.Sprint(params)).
							Send()
					}

					return params, nil
				},
				CodeLensResolve: func(context *glsp.Context, params *protocol.CodeLens) (*protocol.CodeLens, error) {
					if m := logging.NewMessage([]string{"engine", "parser"}, logging.Info, 0); m != nil {
						m.Set("message", "CodeLensResolve").
							Set("req", pretty.Sprint(params)).
							Send()
					}
					return nil, nil
				},
				WorkspaceExecuteCommand: func(context *glsp.Context, params *protocol.ExecuteCommandParams) (interface{}, error) {
					if m := logging.NewMessage([]string{"engine", "parser"}, logging.Info, 0); m != nil {
						m.Set("message", "WorkspaceExecuteCommand").
							Set("req", pretty.Sprint(params)).
							Send()
					}
					// str := strings.Replace((string)(params.TextDocument.URI), "file://", "", 1)
					if params.Command == "codelens.evaluate" {
						ParseCommentsForFileGlob2([]string{params.Arguments[0].(string)})
					}
					t := "Testing sending a random value"
					return &t, nil
				},
			}

			server := server.NewServer(&handler, lsName, true)

			server.RunStdio()
		},
	}

	lspCmd2 := &cobra.Command{
		Use:   "lsp2",
		Short: "startup the lsp server",
		Run: func(cmd *cobra.Command, args []string) {
			server := lsp.NewServer(
				&lsp.Options{
					CompletionProvider: &defines.CompletionOptions{
						TriggerCharacters: &[]string{"."},
					},
					// Network: "tcp",
				},
			)

			// server.OnWillSaveTextDocument(func(ctx context.Context, req *defines.WillSaveTextDocumentParams) (err error) {
			// 	// fmt.Println("🔥 WILL SAVE CALLED")
			// 	str := strings.Replace((string)(req.TextDocument.Uri), "file://", "", 1)
			// 	ParseCommentsForFileGlob2([]string{str})
			// 	return nil
			// })

			server.OnDidSaveTextDocument(func(ctx context.Context, req *defines.DidSaveTextDocumentParams) (err error) {
				// fmt.Println("🔥 DID SAVE CALLED")
				str := strings.Replace((string)(req.TextDocument.Uri), "file://", "", 1)
				ParseCommentsForFileGlob2([]string{str})
				return nil
			})

			server.Run()
		},
	}
	rootCmd.AddCommand(lspCmd2)
	rootCmd.AddCommand(lspCmd)
	rootCmd.Execute()
}

func initialized(context *glsp.Context, params *protocol.InitializedParams) error {
	return nil
}

func shutdown(context *glsp.Context) error {
	protocol.SetTraceValue(protocol.TraceValueOff)
	return nil
}

func setTrace(context *glsp.Context, params *protocol.SetTraceParams) error {
	protocol.SetTraceValue(params.Value)
	return nil
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
							Line:      protocol.UInteger(pos.Line),
							Character: protocol.UInteger(pos.Column),
						},
						End: protocol.Position{
							Line:      protocol.UInteger(end.Line),
							Character: protocol.UInteger(end.Column),
						},
					},
					Command: &protocol.Command{
						Title:     commandTitle,
						Command:   "codelens.evaluate",
						Arguments: []interface{}{file, comment.Text},
					},
				}
				codelenses = append(codelenses, codeL)
			}
		}
	}
	return codelenses
}

func parseFileAndReturnCodeActions(file string) []protocol.CodeAction {
	fset := token.NewFileSet() // positions are relative to fset
	d, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
	if err != nil {
		panic(err)
	}
	codelenses := []protocol.CodeAction{}
	for _, commentG := range d.Comments {
		for idx, comment := range commentG.List {
			if strings.HasPrefix(comment.Text, "// >>>") {
				commandTitle := "Evaluate"
				if len(commentG.List) > idx+1 {
					commandTitle = "Refresh"
				}

				pos := fset.Position(comment.Pos())
				end := fset.Position(comment.End())
				severity := protocol.DiagnosticSeverityHint
				code := protocol.IntegerOrString{Value: protocol.Integer(1)}
				kind := protocol.CodeActionKind("QuickFix")
				codeL := protocol.CodeAction{
					Title: commandTitle,
					Kind:  &kind,
					Diagnostics: []protocol.Diagnostic{
						{
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
							Source:   strPtr("doctest"),
							Severity: &severity,
							Code:     &code,
							Message:  commandTitle,
						},
					},
					Data: []interface{}{file, comment.Text},
					// Command: &protocol.Command{
					// 	Title:     commandTitle,
					// 	Command:   "codelens.evaluate",
					// 	Arguments: []interface{}{file},
					// },
				}
				codelenses = append(codelenses, codeL)
			}
		}
	}
	return codelenses
}

func resolveEditCodeAction(ca *protocol.CodeAction) *protocol.CodeAction {
	data := lo.Map(ca.Data.([]interface{}), func(i interface{}, _ int) string {
		return i.(string)
	})
	file := data[0]
	fset := token.NewFileSet() // positions are relative to fset
	d, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
	if err != nil {
		panic(err)
	}

	intp := interp.New(interp.Options{
		GoPath:               os.Getenv("GOPATH"),
		SourcecodeFilesystem: os.DirFS(path.Dir(file)),
	})
	intp.Use(stdlib.Symbols)

	_, err = intp.EvalPath(path.Base(file))
	if err != nil {
		panic(err)
	}

	textEdits := []protocol.TextEdit{}
	for _, commentG := range d.Comments {
		for idx, comment := range commentG.List {
			if strings.HasPrefix(comment.Text, "// >>>") && (comment.Text == data[1]) {
				if len(commentG.List) > idx+1 {
				} else {
					pos := fset.Position(comment.Pos())

					expr := strings.TrimPrefix(comment.Text, "// >>> ")
					resp, err := intp.Eval(expr)
					if err != nil {
						panic(err)
					}
					resp2 := "// " + fmt.Sprint(resp) + "\n"

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
				}

				// end := fset.Position(comment.End())
			}

			// pos := fset.Position(comment.Pos())
			// end := fset.Position(comment.End())
			// severity := protocol.DiagnosticSeverityHint
			// code := protocol.IntegerOrString{Value: protocol.Integer(1)}
			// kind := protocol.CodeActionKind("QuickFix")

			// Command: &protocol.Command{
			// 	Title:     commandTitle,
			// 	Command:   "codelens.evaluate",
			// 	Arguments: []interface{}{file},
			// },
		}
	}
	edit := protocol.WorkspaceEdit{
		Changes: map[protocol.DocumentUri][]protocol.TextEdit{
			"file://" + file: textEdits,
		},
	}
	ca.Edit = &edit
	return ca
}
