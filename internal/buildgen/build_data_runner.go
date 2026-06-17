package buildgen

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/cssbruno/gowdk/internal/gwdkir"
)

func runBuildDataCallRef(ref buildCallRef, imports []gwdkir.Import, scripts []gwdkir.GoBlock, source string, routeParams map[string]string) (map[string]string, error) {
	if ref.Alias == "" {
		if script, ok := packageScriptWithFunction(scripts, ref.Function); ok {
			return runInlineBuildDataCall(script, imports, source, ref.Function, routeParams)
		}
		importPath, err := samePackageImportPath(source)
		if err != nil {
			return nil, err
		}
		return runBuildDataCall("gowdkbuilddata", importPath, ref.Function, sourceDir(source), routeParams)
	}
	item, ok := findBuildImport(ref.Alias, imports)
	if !ok {
		return nil, fmt.Errorf("build import %q is not declared", ref.Alias)
	}
	return runBuildDataCall(ref.Alias, item.Path, ref.Function, sourceDir(source), routeParams)
}

func packageScriptWithFunction(scripts []gwdkir.GoBlock, function string) (gwdkir.GoBlock, bool) {
	for _, script := range scripts {
		if !isStaticPackageGoBlockTarget(script.Target) {
			continue
		}
		file, err := parseInlineGoBlockFile(script, "gowdkinline")
		if err != nil {
			return gwdkir.GoBlock{}, false
		}
		for _, declaration := range file.Decls {
			functionDeclaration, ok := declaration.(*ast.FuncDecl)
			if ok && functionDeclaration.Name.Name == function {
				return script, true
			}
		}
	}
	return gwdkir.GoBlock{}, false
}

func isStaticPackageGoBlockTarget(target string) bool {
	switch strings.TrimSpace(target) {
	case "":
		return true
	default:
		return false
	}
}

func runInlineBuildDataCall(script gwdkir.GoBlock, imports []gwdkir.Import, source string, function string, routeParams map[string]string) (map[string]string, error) {
	var lastErr error
	for _, candidate := range buildDataRunnerCandidates(routeParams) {
		runnerSource, err := inlineBuildDataRunnerSource(script, imports, source, function, candidate, routeParams)
		if err != nil {
			return nil, err
		}
		data, err := runBuildDataRunner(runnerSource, sourceDir(source), "inline build data function "+function)
		if err == nil || !isBuildDataSignatureMismatch(err) {
			return data, err
		}
		lastErr = err
	}
	return nil, lastErr
}

type buildDataRunnerCandidate struct {
	returnsError bool
	withParams   bool
}

func buildDataRunnerCandidates(routeParams map[string]string) []buildDataRunnerCandidate {
	return []buildDataRunnerCandidate{
		{returnsError: true},
		{returnsError: false},
		{returnsError: true, withParams: true},
		{returnsError: false, withParams: true},
	}
}

func inlineBuildDataRunnerSource(script gwdkir.GoBlock, imports []gwdkir.Import, source string, function string, candidate buildDataRunnerCandidate, routeParams map[string]string) (string, error) {
	if !isLiteralName(function) {
		return "", fmt.Errorf("invalid build function name %q", function)
	}
	file, err := parseInlineGoBlockFile(script, packageNameForInlineScript(source))
	if err != nil {
		return "", err
	}

	decls := []ast.Decl{inlineBuildDataImportDecl(imports, file, candidate)}
	for _, declaration := range file.Decls {
		if gen, ok := declaration.(*ast.GenDecl); ok && gen.Tok == token.IMPORT {
			continue
		}
		decls = append(decls, declaration)
	}
	decls = append(decls, inlineBuildDataMainDecl(function, candidate, routeParams))

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

func parseInlineGoBlockFile(script gwdkir.GoBlock, packageName string) (*ast.File, error) {
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
	if isLiteralName(base) {
		return base
	}
	return "gowdkinline"
}

func inlineBuildDataImportDecl(imports []gwdkir.Import, scriptFile *ast.File, candidate buildDataRunnerCandidate) ast.Decl {
	specs := []ast.Spec{
		&ast.ImportSpec{Name: ast.NewIdent("gowdkjson"), Path: buildDataStringLit("encoding/json")},
		&ast.ImportSpec{Name: ast.NewIdent("gowdkos"), Path: buildDataStringLit("os")},
	}
	seen := map[string]bool{
		importKey("gowdkjson", "encoding/json"): true,
		importKey("gowdkos", "os"):              true,
	}
	if candidate.returnsError {
		specs = append(specs, &ast.ImportSpec{Name: ast.NewIdent("gowdkfmt"), Path: buildDataStringLit("fmt")})
		seen[importKey("gowdkfmt", "fmt")] = true
	}
	if candidate.withParams {
		specs = append(specs, &ast.ImportSpec{Name: ast.NewIdent("gowdkbuildparams"), Path: buildDataStringLit("github.com/cssbruno/gowdk")})
		seen[importKey("gowdkbuildparams", "github.com/cssbruno/gowdk")] = true
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

func inlineBuildDataMainDecl(function string, candidate buildDataRunnerCandidate, routeParams map[string]string) ast.Decl {
	args := buildDataCallArgs(candidate, routeParams)
	var statements []ast.Stmt
	if candidate.returnsError {
		statements = append(statements,
			&ast.AssignStmt{
				Lhs: []ast.Expr{ast.NewIdent("value"), ast.NewIdent("err")},
				Tok: token.DEFINE,
				Rhs: []ast.Expr{&ast.CallExpr{Fun: ast.NewIdent(function), Args: args}},
			},
			buildDataErrorExitStmt("gowdkfmt", "gowdkos"),
		)
	} else {
		statements = append(statements, &ast.AssignStmt{
			Lhs: []ast.Expr{ast.NewIdent("value")},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{&ast.CallExpr{Fun: ast.NewIdent(function), Args: args}},
		})
	}
	statements = append(statements, buildDataEncodeStmt("gowdkjson", "gowdkos"))
	return &ast.FuncDecl{
		Name: ast.NewIdent("main"),
		Type: &ast.FuncType{Params: &ast.FieldList{}},
		Body: &ast.BlockStmt{List: statements},
	}
}

func samePackageImportPath(source string) (string, error) {
	dir := sourceDir(source)
	info, err := goListDir(dir)
	if err != nil {
		return "", fmt.Errorf("same-package build data function requires a buildable Go package for %s: %w", dir, err)
	}
	if strings.TrimSpace(info.ImportPath) == "" {
		return "", fmt.Errorf("same-package build data function requires a buildable Go package for %s", dir)
	}
	return info.ImportPath, nil
}

type goListDirInfo struct {
	ImportPath string
}

func goListDir(dir string) (goListDirInfo, error) {
	command := exec.Command("go", "list", "-json", ".")
	command.Dir = dir
	output, err := command.Output()
	if err != nil {
		return goListDirInfo{}, goListError(err)
	}
	var info goListDirInfo
	if err := json.Unmarshal(output, &info); err != nil {
		return goListDirInfo{}, fmt.Errorf("parse go list output: %w", err)
	}
	return info, nil
}

// goListError surfaces the underlying go list failure, including its stderr,
// instead of collapsing it into a generic message that hides the cause (for
// example a sibling package compile error or a missing go.mod).
func goListError(err error) error {
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		if stderr := strings.TrimSpace(string(exit.Stderr)); stderr != "" {
			return fmt.Errorf("%w\n%s", err, stderr)
		}
	}
	return err
}

func sourceDir(source string) string {
	if strings.TrimSpace(source) == "" {
		return "."
	}
	return filepath.Dir(source)
}

func runBuildDataCall(alias, importPath, function string, workDir string, routeParams map[string]string) (map[string]string, error) {
	var lastErr error
	for _, candidate := range buildDataRunnerCandidates(routeParams) {
		source, err := buildDataRunnerSource(alias, importPath, function, candidate, routeParams)
		if err != nil {
			return nil, err
		}
		data, err := runBuildDataRunner(source, workDir, "build data function "+alias+"."+function)
		if err == nil || !isBuildDataSignatureMismatch(err) {
			return data, err
		}
		lastErr = err
	}
	return nil, lastErr
}

func findBuildImport(alias string, imports []gwdkir.Import) (gwdkir.Import, bool) {
	for _, item := range imports {
		if item.Alias == alias {
			return item, true
		}
	}
	return gwdkir.Import{}, false
}

func buildDataRunnerSource(alias, importPath, function string, candidate buildDataRunnerCandidate, routeParams map[string]string) (string, error) {
	if !isLiteralName(alias) {
		return "", fmt.Errorf("invalid build import alias %q", alias)
	}
	if !isLiteralName(function) {
		return "", fmt.Errorf("invalid build function name %q", function)
	}
	if strings.TrimSpace(importPath) == "" {
		return "", fmt.Errorf("build import %q has an empty path", alias)
	}
	file := &ast.File{
		Name: ast.NewIdent("main"),
		Decls: []ast.Decl{
			buildDataImportDecl(alias, importPath, candidate),
			buildDataMainDecl(alias, function, candidate, routeParams),
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

func buildDataImportDecl(alias, importPath string, candidate buildDataRunnerCandidate) ast.Decl {
	specs := []ast.Spec{
		&ast.ImportSpec{Path: buildDataStringLit("encoding/json")},
		&ast.ImportSpec{Path: buildDataStringLit("os")},
		&ast.ImportSpec{Name: ast.NewIdent(alias), Path: buildDataStringLit(importPath)},
	}
	if candidate.returnsError {
		specs = append(specs, &ast.ImportSpec{Path: buildDataStringLit("fmt")})
	}
	if candidate.withParams {
		specs = append(specs, &ast.ImportSpec{Name: ast.NewIdent("gowdkbuildparams"), Path: buildDataStringLit("github.com/cssbruno/gowdk")})
	}
	return &ast.GenDecl{Tok: token.IMPORT, Specs: specs}
}

func buildDataMainDecl(alias, function string, candidate buildDataRunnerCandidate, routeParams map[string]string) ast.Decl {
	args := buildDataCallArgs(candidate, routeParams)
	var statements []ast.Stmt
	if candidate.returnsError {
		statements = append(statements,
			&ast.AssignStmt{
				Lhs: []ast.Expr{ast.NewIdent("value"), ast.NewIdent("err")},
				Tok: token.DEFINE,
				Rhs: []ast.Expr{&ast.CallExpr{Fun: &ast.SelectorExpr{X: ast.NewIdent(alias), Sel: ast.NewIdent(function)}, Args: args}},
			},
			buildDataErrorExitStmt("fmt", "os"),
		)
	} else {
		statements = append(statements, &ast.AssignStmt{
			Lhs: []ast.Expr{ast.NewIdent("value")},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{&ast.CallExpr{Fun: &ast.SelectorExpr{X: ast.NewIdent(alias), Sel: ast.NewIdent(function)}, Args: args}},
		})
	}
	statements = append(statements, buildDataEncodeStmt("json", "os"))
	return &ast.FuncDecl{
		Name: ast.NewIdent("main"),
		Type: &ast.FuncType{Params: &ast.FieldList{}},
		Body: &ast.BlockStmt{List: statements},
	}
}

func buildDataCallArgs(candidate buildDataRunnerCandidate, routeParams map[string]string) []ast.Expr {
	if !candidate.withParams {
		return nil
	}
	return []ast.Expr{buildDataParamsExpr(routeParams)}
}

func buildDataParamsExpr(routeParams map[string]string) ast.Expr {
	return &ast.CompositeLit{
		Type: &ast.SelectorExpr{X: ast.NewIdent("gowdkbuildparams"), Sel: ast.NewIdent("BuildParams")},
		Elts: []ast.Expr{&ast.KeyValueExpr{
			Key:   ast.NewIdent("Route"),
			Value: buildDataStringMapLit(routeParams),
		}},
	}
}

func buildDataStringMapLit(values map[string]string) ast.Expr {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	elts := make([]ast.Expr, 0, len(keys))
	for _, key := range keys {
		elts = append(elts, &ast.KeyValueExpr{
			Key:   buildDataStringLit(key),
			Value: buildDataStringLit(values[key]),
		})
	}
	return &ast.CompositeLit{
		Type: &ast.MapType{Key: ast.NewIdent("string"), Value: ast.NewIdent("string")},
		Elts: elts,
	}
}

func buildDataErrorExitStmt(fmtAlias, osAlias string) ast.Stmt {
	return &ast.IfStmt{
		Cond: &ast.BinaryExpr{X: ast.NewIdent("err"), Op: token.NEQ, Y: ast.NewIdent("nil")},
		Body: &ast.BlockStmt{List: []ast.Stmt{
			&ast.ExprStmt{X: &ast.CallExpr{
				Fun: &ast.SelectorExpr{X: ast.NewIdent(fmtAlias), Sel: ast.NewIdent("Fprintln")},
				Args: []ast.Expr{
					&ast.SelectorExpr{X: ast.NewIdent(osAlias), Sel: ast.NewIdent("Stderr")},
					ast.NewIdent("err"),
				},
			}},
			&ast.ExprStmt{X: &ast.CallExpr{
				Fun:  &ast.SelectorExpr{X: ast.NewIdent(osAlias), Sel: ast.NewIdent("Exit")},
				Args: []ast.Expr{&ast.BasicLit{Kind: token.INT, Value: "1"}},
			}},
		}},
	}
}

func buildDataEncodeStmt(jsonAlias, osAlias string) ast.Stmt {
	return &ast.IfStmt{
		Init: &ast.AssignStmt{
			Lhs: []ast.Expr{ast.NewIdent("err")},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{&ast.CallExpr{
				Fun: &ast.SelectorExpr{
					X: &ast.CallExpr{
						Fun:  &ast.SelectorExpr{X: ast.NewIdent(jsonAlias), Sel: ast.NewIdent("NewEncoder")},
						Args: []ast.Expr{&ast.SelectorExpr{X: ast.NewIdent(osAlias), Sel: ast.NewIdent("Stdout")}},
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
	}
}

func buildDataStringLit(value string) *ast.BasicLit {
	return &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(value)}
}

func runBuildDataRunner(source string, workDir string, label string) (map[string]string, error) {
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
	if strings.TrimSpace(workDir) != "" {
		command.Dir = workDir
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	stderrText := strings.TrimSpace(stderr.String())
	if err != nil {
		if stderrText != "" {
			return nil, fmt.Errorf("run %s: %w\n%s", label, err, stderrText)
		}
		return nil, fmt.Errorf("run %s: %w", label, err)
	}
	data, err := parseBuildFunctionOutput(output)
	if err != nil && stderrText != "" {
		return nil, fmt.Errorf("%w\nstderr:\n%s", err, stderrText)
	}
	return data, err
}

func isBuildDataSignatureMismatch(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	for _, fragment := range []string{
		"assignment mismatch",
		"too many arguments in call",
		"not enough arguments in call",
		"cannot use gowdkbuildparams.BuildParams",
	} {
		if strings.Contains(message, fragment) {
			return true
		}
	}
	return false
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
		if !isLiteralName(key) {
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
