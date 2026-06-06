package buildgen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/printer"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cssbruno/gowdk/internal/manifest"
)

func parsePathDeclarations(body string) ([]map[string]string, error) {
	return parseLiteralDeclarations(body, "paths", "path param")
}

func parsePathParams(source string) (map[string]string, error) {
	return parseLiteralStringMap(source, "path param")
}

func parseBuildData(body string, routeParams map[string]string, imports []manifest.Import) (map[string]string, error) {
	lines := significantBuildLines(body)
	if len(lines) == 1 {
		if match := buildCallPattern.FindStringSubmatch(lines[0]); match != nil {
			return runBuildDataCall(match[1], match[2], imports)
		}
	}
	declarations, err := parseLiteralDeclarations(body, "build", "build field")
	if err != nil {
		return nil, err
	}
	if len(declarations) == 0 {
		return nil, nil
	}
	data := map[string]string{}
	for index, declaration := range declarations {
		for key, value := range declaration {
			if _, exists := data[key]; exists {
				return nil, fmt.Errorf("duplicate build field %q", key)
			}
			interpolated, err := interpolateBuildValue(value, routeParams)
			if err != nil {
				return nil, fmt.Errorf("build field %s: %w", key, err)
			}
			data[key] = interpolated
		}
		if len(declaration) == 0 && index == 0 {
			return nil, fmt.Errorf("build {} declaration must not be empty")
		}
	}
	return data, nil
}

func significantBuildLines(body string) []string {
	var lines []string
	for _, rawLine := range strings.Split(body, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func runBuildDataCall(alias, function string, imports []manifest.Import) (map[string]string, error) {
	item, ok := findBuildImport(alias, imports)
	if !ok {
		return nil, fmt.Errorf("build import %q is not declared", alias)
	}
	source, err := buildDataRunnerSource(alias, item.Path, function)
	if err != nil {
		return nil, err
	}
	file, err := os.CreateTemp("", "gowdk-build-data-*.go")
	if err != nil {
		return nil, err
	}
	path := file.Name()
	defer os.Remove(path)
	if _, err := file.WriteString(source); err != nil {
		file.Close()
		return nil, err
	}
	if err := file.Close(); err != nil {
		return nil, err
	}

	command := exec.Command("go", "run", path)
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("run build data function %s.%s: %w\n%s", alias, function, err, strings.TrimSpace(string(output)))
	}
	return parseBuildFunctionOutput(output)
}

func findBuildImport(alias string, imports []manifest.Import) (manifest.Import, bool) {
	for _, item := range imports {
		if item.Alias == alias {
			return item, true
		}
	}
	return manifest.Import{}, false
}

func buildDataRunnerSource(alias, importPath, function string) (string, error) {
	if !literalNamePattern.MatchString(alias) {
		return "", fmt.Errorf("invalid build import alias %q", alias)
	}
	if !literalNamePattern.MatchString(function) {
		return "", fmt.Errorf("invalid build function name %q", function)
	}
	if strings.TrimSpace(importPath) == "" {
		return "", fmt.Errorf("build import %q has an empty path", alias)
	}
	file := &ast.File{
		Name: ast.NewIdent("main"),
		Decls: []ast.Decl{
			&ast.GenDecl{Tok: token.IMPORT, Specs: []ast.Spec{
				&ast.ImportSpec{Path: buildDataStringLit("encoding/json")},
				&ast.ImportSpec{Path: buildDataStringLit("os")},
				&ast.ImportSpec{Name: ast.NewIdent(alias), Path: buildDataStringLit(importPath)},
			}},
			buildDataMainDecl(alias, function),
		},
	}
	var buffer bytes.Buffer
	if err := printer.Fprint(&buffer, token.NewFileSet(), file); err != nil {
		return "", fmt.Errorf("print build data runner: %w", err)
	}
	formatted, err := format.Source(buffer.Bytes())
	if err != nil {
		return "", fmt.Errorf("format build data runner: %w", err)
	}
	return string(formatted), nil
}

func buildDataMainDecl(alias, function string) ast.Decl {
	return &ast.FuncDecl{
		Name: ast.NewIdent("main"),
		Type: &ast.FuncType{Params: &ast.FieldList{}},
		Body: &ast.BlockStmt{List: []ast.Stmt{
			&ast.AssignStmt{
				Lhs: []ast.Expr{ast.NewIdent("value")},
				Tok: token.DEFINE,
				Rhs: []ast.Expr{&ast.CallExpr{Fun: &ast.SelectorExpr{X: ast.NewIdent(alias), Sel: ast.NewIdent(function)}}},
			},
			&ast.IfStmt{
				Init: &ast.AssignStmt{
					Lhs: []ast.Expr{ast.NewIdent("err")},
					Tok: token.DEFINE,
					Rhs: []ast.Expr{&ast.CallExpr{
						Fun: &ast.SelectorExpr{
							X: &ast.CallExpr{
								Fun:  &ast.SelectorExpr{X: ast.NewIdent("json"), Sel: ast.NewIdent("NewEncoder")},
								Args: []ast.Expr{&ast.SelectorExpr{X: ast.NewIdent("os"), Sel: ast.NewIdent("Stdout")}},
							},
							Sel: ast.NewIdent("Encode"),
						},
						Args: []ast.Expr{ast.NewIdent("value")},
					}},
				},
				Cond: &ast.BinaryExpr{X: ast.NewIdent("err"), Op: token.NEQ, Y: ast.NewIdent("nil")},
				Body: &ast.BlockStmt{List: []ast.Stmt{&ast.ExprStmt{X: &ast.CallExpr{
					Fun:  ast.NewIdent("panic"),
					Args: []ast.Expr{ast.NewIdent("err")},
				}}}},
			},
		}},
	}
}

func buildDataStringLit(value string) *ast.BasicLit {
	return &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(value)}
}

func parseBuildFunctionOutput(output []byte) (map[string]string, error) {
	var raw map[string]any
	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, fmt.Errorf("decode build data output: %w", err)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("build data function must return a non-empty JSON object")
	}
	data := map[string]string{}
	for key, value := range raw {
		if !literalNamePattern.MatchString(key) {
			return nil, fmt.Errorf("invalid build field name %q", key)
		}
		scalar, ok := buildScalarString(value)
		if !ok {
			return nil, fmt.Errorf("build field %s must be a string, number, boolean, or null", key)
		}
		data[key] = scalar
	}
	return data, nil
}

func buildScalarString(value any) (string, bool) {
	switch typed := value.(type) {
	case nil:
		return "", true
	case string:
		if strings.TrimSpace(typed) == "" {
			return "", false
		}
		return typed, true
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), true
	case bool:
		return strconv.FormatBool(typed), true
	default:
		return "", false
	}
}

func interpolateBuildValue(value string, routeParams map[string]string) (string, error) {
	if !strings.Contains(value, "{") {
		return value, nil
	}
	var out strings.Builder
	for {
		start := strings.Index(value, "{")
		if start < 0 {
			out.WriteString(value)
			return out.String(), nil
		}
		end := strings.Index(value[start:], "}")
		if end < 0 {
			return "", fmt.Errorf("unterminated interpolation")
		}
		end += start
		out.WriteString(value[:start])
		name := strings.TrimSpace(value[start+1 : end])
		if param, ok := buildRouteParamExpression(name); ok {
			name = param
		}
		resolved, ok := routeParams[name]
		if !ok {
			return "", fmt.Errorf("unknown route param %q", name)
		}
		out.WriteString(resolved)
		value = value[end+1:]
	}
}

func buildRouteParamExpression(value string) (string, bool) {
	if !strings.HasPrefix(value, `param("`) || !strings.HasSuffix(value, `")`) {
		return "", false
	}
	name := strings.TrimPrefix(strings.TrimSuffix(value, `")`), `param("`)
	if !literalNamePattern.MatchString(name) {
		return "", false
	}
	return name, true
}

func parseLiteralDeclarations(body, blockName, itemName string) ([]map[string]string, error) {
	var declarations []map[string]string
	for index, rawLine := range strings.Split(body, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		match := literalDeclarationPattern.FindStringSubmatch(line)
		if match == nil {
			return nil, fmt.Errorf("%s line %d must use `=> { name: \"value\" }`", blockName, index+1)
		}
		params, err := parseLiteralStringMap(match[1], itemName)
		if err != nil {
			return nil, fmt.Errorf("%s line %d: %w", blockName, index+1, err)
		}
		declarations = append(declarations, params)
	}
	return declarations, nil
}

func parseLiteralStringMap(source, itemName string) (map[string]string, error) {
	assignments, err := splitPathAssignments(source)
	if err != nil {
		return nil, err
	}
	if len(assignments) == 0 {
		return nil, fmt.Errorf("literal declaration must include values")
	}

	params := map[string]string{}
	for _, assignment := range assignments {
		name, rawValue, ok := strings.Cut(assignment, ":")
		if !ok {
			return nil, fmt.Errorf("%s %q must use name: \"value\"", itemName, strings.TrimSpace(assignment))
		}
		name = strings.TrimSpace(name)
		if !literalNamePattern.MatchString(name) {
			return nil, fmt.Errorf("invalid %s name %q", itemName, name)
		}
		if _, exists := params[name]; exists {
			return nil, fmt.Errorf("duplicate %s %q", itemName, name)
		}
		value, err := parsePathString(strings.TrimSpace(rawValue))
		if err != nil {
			return nil, fmt.Errorf("%s %s: %w", itemName, name, err)
		}
		params[name] = value
	}
	return params, nil
}

func splitPathAssignments(source string) ([]string, error) {
	var assignments []string
	start := 0
	inString := false
	escaped := false
	for index, char := range source {
		if escaped {
			escaped = false
			continue
		}
		if inString {
			switch char {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch char {
		case '"':
			inString = true
		case ',':
			part := strings.TrimSpace(source[start:index])
			if part == "" {
				return nil, fmt.Errorf("empty path param assignment")
			}
			assignments = append(assignments, part)
			start = index + 1
		}
	}
	if inString {
		return nil, fmt.Errorf("unterminated string")
	}
	part := strings.TrimSpace(source[start:])
	if part != "" {
		assignments = append(assignments, part)
	}
	return assignments, nil
}

func parsePathString(source string) (string, error) {
	if !strings.HasPrefix(source, `"`) {
		return "", fmt.Errorf("value must be a string literal")
	}
	value, err := strconv.Unquote(source)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("value must not be empty")
	}
	return value, nil
}

func validateRouteParamValue(name, value string) error {
	if strings.ContainsAny(value, "/?#") {
		return fmt.Errorf("route param %q value must not contain /, ?, or #", name)
	}
	if value == "." || value == ".." {
		return fmt.Errorf("route param %q value is unsafe", name)
	}
	return nil
}

func mergeBuildData(buildData, routeData map[string]string) (map[string]string, error) {
	merged := cloneStringMap(buildData)
	for key, value := range routeData {
		if _, exists := merged[key]; exists {
			return nil, fmt.Errorf("build data field %q conflicts with route param", key)
		}
		merged[key] = value
	}
	return merged, nil
}

func cloneStringMap(input map[string]string) map[string]string {
	output := map[string]string{}
	for key, value := range input {
		output[key] = value
	}
	return output
}

func sourcePathSet(paths []string) map[string]bool {
	set := map[string]bool{}
	for _, sourcePath := range paths {
		abs, err := filepath.Abs(sourcePath)
		if err != nil {
			continue
		}
		set[filepath.Clean(abs)] = true
	}
	return set
}

func sourcePathChanged(set map[string]bool, sourcePath string) bool {
	abs, err := filepath.Abs(sourcePath)
	if err != nil {
		return false
	}
	return set[filepath.Clean(abs)]
}
