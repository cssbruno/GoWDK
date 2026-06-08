package buildgen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cssbruno/gowdk/internal/manifest"
)

func runBuildDataCallRef(ref buildCallRef, imports []manifest.Import, scripts []manifest.GoBlock, source string) (map[string]string, error) {
	if ref.Alias == "" {
		if script, ok := packageScriptWithFunction(scripts, ref.Function); ok {
			return runInlineBuildDataCall(script, imports, source, ref.Function)
		}
		importPath, err := samePackageImportPath(source)
		if err != nil {
			return nil, err
		}
		return runBuildDataCall("gowdkbuilddata", importPath, ref.Function)
	}
	item, ok := findBuildImport(ref.Alias, imports)
	if !ok {
		return nil, fmt.Errorf("build import %q is not declared", ref.Alias)
	}
	return runBuildDataCall(ref.Alias, item.Path, ref.Function)
}

func packageScriptWithFunction(scripts []manifest.GoBlock, function string) (manifest.GoBlock, bool) {
	for _, script := range scripts {
		if !isStaticPackageGoBlockTarget(script.Target) {
			continue
		}
		file, err := parseInlineGoBlockFile(script, "gowdkinline")
		if err != nil {
			return manifest.GoBlock{}, false
		}
		for _, declaration := range file.Decls {
			functionDeclaration, ok := declaration.(*ast.FuncDecl)
			if ok && functionDeclaration.Name.Name == function {
				return script, true
			}
		}
	}
	return manifest.GoBlock{}, false
}

func isStaticPackageGoBlockTarget(target string) bool {
	switch strings.TrimSpace(target) {
	case "":
		return true
	default:
		return false
	}
}

func runInlineBuildDataCall(script manifest.GoBlock, imports []manifest.Import, source string, function string) (map[string]string, error) {
	runnerSource, err := inlineBuildDataRunnerSource(script, imports, source, function)
	if err != nil {
		return nil, err
	}
	file, err := os.CreateTemp("", "gowdk-inline-build-data-*.go")
	if err != nil {
		return nil, err
	}
	path := file.Name()
	defer os.Remove(path)
	if err := file.Close(); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, []byte(runnerSource), 0o600); err != nil {
		return nil, err
	}

	command := exec.Command("go", "run", path)
	command.Dir = sourceDir(source)
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("run inline build data function %s: %w\n%s", function, err, strings.TrimSpace(string(output)))
	}
	return parseBuildFunctionOutput(output)
}

func inlineBuildDataRunnerSource(script manifest.GoBlock, imports []manifest.Import, source string, function string) (string, error) {
	if !literalNamePattern.MatchString(function) {
		return "", fmt.Errorf("invalid build function name %q", function)
	}
	file, err := parseInlineGoBlockFile(script, packageNameForInlineScript(source))
	if err != nil {
		return "", err
	}

	decls := []ast.Decl{inlineBuildDataImportDecl(imports, file)}
	for _, declaration := range file.Decls {
		if gen, ok := declaration.(*ast.GenDecl); ok && gen.Tok == token.IMPORT {
			continue
		}
		decls = append(decls, declaration)
	}
	decls = append(decls, inlineBuildDataMainDecl(function))

	runner := &ast.File{Name: ast.NewIdent("main"), Decls: decls}
	var buffer bytes.Buffer
	if err := printer.Fprint(&buffer, token.NewFileSet(), runner); err != nil {
		return "", fmt.Errorf("print inline build data runner: %w", err)
	}
	formatted, err := format.Source(buffer.Bytes())
	if err != nil {
		return "", fmt.Errorf("format inline build data runner: %w", err)
	}
	return string(formatted), nil
}

func parseInlineGoBlockFile(script manifest.GoBlock, packageName string) (*ast.File, error) {
	source := "package " + packageName + "\n" + script.Body
	file, err := parser.ParseFile(token.NewFileSet(), "inline-script.gwdk.go", source, parser.AllErrors)
	if err != nil {
		line := script.Span.Start.Line
		if line > 0 {
			return nil, fmt.Errorf("go block starting on line %d has invalid Go: %w", line, err)
		}
		return nil, fmt.Errorf("go block has invalid Go: %w", err)
	}
	return file, nil
}

func packageNameForInlineScript(source string) string {
	base := filepath.Base(sourceDir(source))
	if literalNamePattern.MatchString(base) {
		return base
	}
	return "gowdkinline"
}

func inlineBuildDataImportDecl(imports []manifest.Import, scriptFile *ast.File) ast.Decl {
	specs := []ast.Spec{
		&ast.ImportSpec{Name: ast.NewIdent("gowdkjson"), Path: buildDataStringLit("encoding/json")},
		&ast.ImportSpec{Name: ast.NewIdent("gowdkos"), Path: buildDataStringLit("os")},
	}
	seen := map[string]bool{
		importKey("gowdkjson", "encoding/json"): true,
		importKey("gowdkos", "os"):              true,
	}
	for _, item := range imports {
		path := strings.TrimSpace(item.Path)
		if path == "" {
			continue
		}
		alias := strings.TrimSpace(item.Alias)
		key := importKey(alias, path)
		if seen[key] {
			continue
		}
		seen[key] = true
		spec := &ast.ImportSpec{Path: buildDataStringLit(path)}
		if alias != "" {
			spec.Name = ast.NewIdent(alias)
		}
		specs = append(specs, spec)
	}
	for _, declaration := range scriptFile.Decls {
		gen, ok := declaration.(*ast.GenDecl)
		if !ok || gen.Tok != token.IMPORT {
			continue
		}
		for _, spec := range gen.Specs {
			importSpec, ok := spec.(*ast.ImportSpec)
			if !ok {
				continue
			}
			path, err := strconv.Unquote(importSpec.Path.Value)
			if err != nil {
				path = importSpec.Path.Value
			}
			alias := ""
			if importSpec.Name != nil {
				alias = importSpec.Name.Name
			}
			key := importKey(alias, path)
			if seen[key] {
				continue
			}
			seen[key] = true
			specs = append(specs, importSpec)
		}
	}
	return &ast.GenDecl{Tok: token.IMPORT, Specs: specs}
}

func importKey(alias string, path string) string {
	return alias + "\x00" + path
}

func inlineBuildDataMainDecl(function string) ast.Decl {
	return &ast.FuncDecl{
		Name: ast.NewIdent("main"),
		Type: &ast.FuncType{Params: &ast.FieldList{}},
		Body: &ast.BlockStmt{List: []ast.Stmt{
			&ast.AssignStmt{
				Lhs: []ast.Expr{ast.NewIdent("value")},
				Tok: token.DEFINE,
				Rhs: []ast.Expr{&ast.CallExpr{Fun: ast.NewIdent(function)}},
			},
			&ast.IfStmt{
				Init: &ast.AssignStmt{
					Lhs: []ast.Expr{ast.NewIdent("err")},
					Tok: token.DEFINE,
					Rhs: []ast.Expr{&ast.CallExpr{
						Fun: &ast.SelectorExpr{
							X: &ast.CallExpr{
								Fun:  &ast.SelectorExpr{X: ast.NewIdent("gowdkjson"), Sel: ast.NewIdent("NewEncoder")},
								Args: []ast.Expr{&ast.SelectorExpr{X: ast.NewIdent("gowdkos"), Sel: ast.NewIdent("Stdout")}},
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

func samePackageImportPath(source string) (string, error) {
	dir := sourceDir(source)
	info := goListDir(dir)
	if strings.TrimSpace(info.ImportPath) == "" {
		return "", fmt.Errorf("same-package build data function requires a buildable Go package for %s", dir)
	}
	return info.ImportPath, nil
}

type goListDirInfo struct {
	ImportPath string
}

func goListDir(dir string) goListDirInfo {
	command := exec.Command("go", "list", "-json", ".")
	command.Dir = dir
	output, err := command.Output()
	if err != nil {
		return goListDirInfo{}
	}
	var info goListDirInfo
	if err := json.Unmarshal(output, &info); err != nil {
		return goListDirInfo{}
	}
	return info
}

func sourceDir(source string) string {
	if strings.TrimSpace(source) == "" {
		return "."
	}
	return filepath.Dir(source)
}

func runBuildDataCall(alias, importPath, function string) (map[string]string, error) {
	source, err := buildDataRunnerSource(alias, importPath, function)
	if err != nil {
		return nil, err
	}
	file, err := os.CreateTemp("", "gowdk-build-data-*.go")
	if err != nil {
		return nil, err
	}
	path := file.Name()
	defer os.Remove(path)
	if err := file.Close(); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
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
