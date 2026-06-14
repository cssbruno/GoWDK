package compiler

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

// DiscoverGoEndpoints merges optional //gowdk:act and //gowdk:api comments
// from selected feature-package Go files into the program as standalone Go
// endpoints.
func DiscoverGoEndpoints(config gowdk.Config, ir *gwdkir.Program) error {
	dirs := endpointSourceDirs(*ir)
	if len(dirs) == 0 {
		return nil
	}
	var endpoints []gwdkir.GoEndpoint
	var diagnostics []ValidationError
	for _, dir := range dirs {
		discovered, found := discoverGoEndpointsInDir(dir)
		endpoints = append(endpoints, discovered...)
		diagnostics = append(diagnostics, found...)
	}
	if len(diagnostics) > 0 {
		return ValidationErrors(diagnostics)
	}
	sort.Slice(endpoints, func(i, j int) bool {
		if endpoints[i].Source == endpoints[j].Source {
			return endpoints[i].Name < endpoints[j].Name
		}
		return endpoints[i].Source < endpoints[j].Source
	})
	gwdkanalysis.AddStandaloneEndpoints(config, ir, endpoints)
	return nil
}

func endpointSourceDirs(ir gwdkir.Program) []string {
	seen := map[string]bool{}
	var dirs []string
	add := func(sourcePath string) {
		if strings.TrimSpace(sourcePath) == "" {
			return
		}
		dir := sourceDir(sourcePath)
		abs, err := filepath.Abs(dir)
		if err == nil {
			dir = abs
		}
		if seen[dir] {
			return
		}
		seen[dir] = true
		dirs = append(dirs, dir)
	}
	for _, page := range ir.Pages {
		add(page.Source)
	}
	for _, component := range ir.Components {
		add(component.Source)
	}
	for _, layout := range ir.Layouts {
		add(layout.Source)
	}
	sort.Strings(dirs)
	return dirs
}

func discoverGoEndpointsInDir(dir string) ([]gwdkir.GoEndpoint, []ValidationError) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		// A source directory that simply does not exist is not an error - the
		// program may declare no Go endpoints there. Any other read failure
		// (permissions, I/O) is surfaced so it is not mistaken for "no
		// endpoints found".
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, []ValidationError{goEndpointDiagnostic(token.NewFileSet(), dir, nil, "go_endpoint_read_error", fmt.Sprintf("read Go endpoint directory: %v", err))}
	}
	fileSet := token.NewFileSet()
	var endpoints []gwdkir.GoEndpoint
	var diagnostics []ValidationError
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		file, err := parser.ParseFile(fileSet, path, nil, parser.ParseComments)
		if err != nil {
			diagnostics = append(diagnostics, goEndpointDiagnostic(fileSet, path, nil, "go_endpoint_parse_error", fmt.Sprintf("parse Go endpoint comments: %v", err)))
			continue
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Doc == nil {
				continue
			}
			found, foundDiagnostics := endpointCommentsForFunction(fileSet, path, file.Name.Name, function)
			diagnostics = append(diagnostics, foundDiagnostics...)
			if len(found) > 1 {
				diagnostics = append(diagnostics, goEndpointDiagnostic(fileSet, path, function, "duplicate_go_endpoint_comment", fmt.Sprintf("Go handler %s has multiple gowdk endpoint comments; declare at most one", function.Name.Name)))
				continue
			}
			if len(found) == 0 {
				continue
			}
			endpoint := found[0]
			if function.Recv != nil || function.Name == nil || !function.Name.IsExported() {
				diagnostics = append(diagnostics, ValidationError{
					Code:    "invalid_go_endpoint_handler",
					Source:  endpoint.Source,
					Span:    endpoint.Span,
					Message: fmt.Sprintf("Go endpoint comment on %s must annotate an exported package-level function", function.Name.Name),
				})
				continue
			}
			endpoints = append(endpoints, endpoint)
		}
	}
	return endpoints, diagnostics
}

func endpointCommentsForFunction(fileSet *token.FileSet, path string, packageName string, function *ast.FuncDecl) ([]gwdkir.GoEndpoint, []ValidationError) {
	var endpoints []gwdkir.GoEndpoint
	var diagnostics []ValidationError
	for _, comment := range function.Doc.List {
		text := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
		text = strings.TrimSpace(strings.TrimPrefix(text, "/*"))
		text = strings.TrimSpace(strings.TrimSuffix(text, "*/"))
		kind, methodText, routeText, matched, err := parseGoEndpointComment(text)
		if !matched {
			continue
		}
		span := goTokenSpan(fileSet, comment.Pos(), comment.End())
		if err != nil {
			diagnostics = append(diagnostics, ValidationError{
				Code:    "malformed_go_endpoint_comment",
				Source:  path,
				Span:    span,
				Message: err.Error(),
			})
			continue
		}
		method := strings.ToUpper(methodText)
		route := strings.Trim(routeText, `"`)
		endpoints = append(endpoints, gwdkir.GoEndpoint{
			Kind:        kind,
			SourceKind:  gwdkir.EndpointSourceGo,
			Package:     packageName,
			Source:      path,
			Name:        function.Name.Name,
			Method:      method,
			Route:       route,
			Span:        span,
			RouteSpan:   span,
			RouteParams: routeParamSpansFallback(route, span),
		})
	}
	return endpoints, diagnostics
}

func parseGoEndpointComment(text string) (string, string, string, bool, error) {
	rest, ok := strings.CutPrefix(text, "gowdk:")
	if !ok {
		return "", "", "", false, nil
	}
	fields := strings.Fields(rest)
	if len(fields) != 3 {
		return "", "", "", true, fmt.Errorf("malformed Go endpoint comment %q; expected //gowdk:act METHOD /path or //gowdk:api METHOD /path", text)
	}
	kind := fields[0]
	if kind != "act" && kind != "api" {
		return "", "", "", true, fmt.Errorf("malformed Go endpoint comment %q; supported endpoint kinds are act and api", text)
	}
	if !isASCIILetters(fields[1]) {
		return "", "", "", true, fmt.Errorf("malformed Go endpoint comment %q; method must contain only ASCII letters", text)
	}
	return kind, fields[1], fields[2], true, nil
}

func isASCIILetters(value string) bool {
	if value == "" {
		return false
	}
	for index := 0; index < len(value); index++ {
		char := value[index]
		if (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') {
			continue
		}
		return false
	}
	return true
}

func goEndpointDiagnostic(fileSet *token.FileSet, path string, node ast.Node, code string, message string) ValidationError {
	var span source.SourceSpan
	if node != nil {
		span = goTokenSpan(fileSet, node.Pos(), node.End())
	}
	return ValidationError{Code: code, Source: path, Span: span, Message: message}
}

func goTokenSpan(fileSet *token.FileSet, start token.Pos, end token.Pos) source.SourceSpan {
	startPos := fileSet.Position(start)
	endPos := fileSet.Position(end)
	return source.SourceSpan{
		Start: source.SourcePosition{Line: startPos.Line, Column: startPos.Column},
		End:   source.SourcePosition{Line: endPos.Line, Column: endPos.Column},
	}
}

func routeParamSpansFallback(route string, fallback source.SourceSpan) []source.NamedSpan {
	info, issues := parseRoute(route)
	if len(issues) > 0 {
		return nil
	}
	out := make([]source.NamedSpan, 0, len(info.Params))
	for _, param := range info.Params {
		out = append(out, source.NamedSpan{Name: param, Span: fallback})
	}
	return out
}
