package compiler

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cssbruno/gowdk/internal/manifest"
	"github.com/cssbruno/gowdk/internal/source"
)

// DiscoverGoEndpointComments merges optional //gowdk:act and //gowdk:api
// comments from selected feature-package Go files into the manifest.
func DiscoverGoEndpointComments(app manifest.Manifest) (manifest.Manifest, error) {
	dirs := endpointSourceDirs(app)
	if len(dirs) == 0 {
		return app, nil
	}
	var endpoints []manifest.EndpointDeclaration
	var diagnostics []ValidationError
	for _, dir := range dirs {
		discovered, found := discoverGoEndpointsInDir(dir)
		endpoints = append(endpoints, discovered...)
		diagnostics = append(diagnostics, found...)
	}
	if len(diagnostics) > 0 {
		return app, ValidationErrors(diagnostics)
	}
	sort.Slice(endpoints, func(i, j int) bool {
		if endpoints[i].Source == endpoints[j].Source {
			return endpoints[i].Name < endpoints[j].Name
		}
		return endpoints[i].Source < endpoints[j].Source
	})
	app.Endpoints = append(app.Endpoints, endpoints...)
	return app, nil
}

func endpointSourceDirs(app manifest.Manifest) []string {
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
	for _, page := range app.Pages {
		add(page.Source)
	}
	for _, component := range app.Components {
		add(component.Source)
	}
	for _, layout := range app.Layouts {
		add(layout.Source)
	}
	sort.Strings(dirs)
	return dirs
}

func discoverGoEndpointsInDir(dir string) ([]manifest.EndpointDeclaration, []ValidationError) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil
	}
	fileSet := token.NewFileSet()
	var endpoints []manifest.EndpointDeclaration
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
			found := endpointCommentsForFunction(fileSet, path, file.Name.Name, function)
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

func endpointCommentsForFunction(fileSet *token.FileSet, path string, packageName string, function *ast.FuncDecl) []manifest.EndpointDeclaration {
	var endpoints []manifest.EndpointDeclaration
	for _, comment := range function.Doc.List {
		text := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
		text = strings.TrimSpace(strings.TrimPrefix(text, "/*"))
		text = strings.TrimSpace(strings.TrimSuffix(text, "*/"))
		kind, methodText, routeText, ok := parseGoEndpointComment(text)
		if !ok {
			continue
		}
		method := strings.ToUpper(methodText)
		route := strings.Trim(routeText, `"`)
		span := goTokenSpan(fileSet, comment.Pos(), comment.End())
		endpoints = append(endpoints, manifest.EndpointDeclaration{
			Kind:        kind,
			SourceKind:  manifest.EndpointSourceGo,
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
	return endpoints
}

func parseGoEndpointComment(text string) (string, string, string, bool) {
	rest, ok := strings.CutPrefix(text, "gowdk:")
	if !ok {
		return "", "", "", false
	}
	fields := strings.Fields(rest)
	if len(fields) != 3 {
		return "", "", "", false
	}
	kind := fields[0]
	if kind != "act" && kind != "api" {
		return "", "", "", false
	}
	if !isASCIILetters(fields[1]) {
		return "", "", "", false
	}
	return kind, fields[1], fields[2], true
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
