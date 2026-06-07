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
	"math"
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

func parseBuildData(body string, routeParams map[string]string, imports []manifest.Import, scripts []manifest.GoBlock, source string) (map[string]string, error) {
	lines := significantBuildLines(body)
	if len(lines) == 1 {
		call, ok, err := parseBuildDataCallLine(lines[0])
		if err != nil {
			return nil, err
		}
		if ok {
			return runBuildDataCallRef(call, imports, scripts, source)
		}
	}
	data := map[string]buildValue{}
	declarations := 0
	for index, line := range lines {
		declaration, ok, err := parseBuildLiteralLine(line)
		if err != nil {
			return nil, fmt.Errorf("build line %d: %w", index+1, err)
		}
		if !ok {
			return nil, fmt.Errorf("build line %d must use `=> { name: value }` or `=> BuildData()`", index+1)
		}
		declarations++
		if len(declaration.Elts) == 0 && index == 0 {
			return nil, fmt.Errorf("build {} declaration must not be empty")
		}
		for _, element := range declaration.Elts {
			key, value, err := buildFieldValue(element, routeParams, data)
			if err != nil {
				return nil, fmt.Errorf("build line %d: %w", index+1, err)
			}
			if _, exists := data[key]; exists {
				return nil, fmt.Errorf("duplicate build field %q", key)
			}
			data[key] = value
		}
	}
	if declarations == 0 {
		return nil, nil
	}
	return buildValueStrings(data), nil
}

type buildCallRef struct {
	Alias    string
	Function string
}

type buildValueKind int

const (
	buildValueString buildValueKind = iota
	buildValueNumber
	buildValueBool
	buildValueNil
)

type buildValue struct {
	kind    buildValueKind
	text    string
	number  float64
	boolean bool
}

func buildStringValue(value string) buildValue {
	return buildValue{kind: buildValueString, text: value}
}

func buildNumberValue(value float64) buildValue {
	return buildValue{kind: buildValueNumber, text: strconv.FormatFloat(value, 'f', -1, 64), number: value}
}

func buildBoolValue(value bool) buildValue {
	return buildValue{kind: buildValueBool, text: strconv.FormatBool(value), boolean: value}
}

func buildNilValue() buildValue {
	return buildValue{kind: buildValueNil}
}

func buildValueStrings(data map[string]buildValue) map[string]string {
	out := make(map[string]string, len(data))
	for key, value := range data {
		out[key] = value.text
	}
	return out
}

func parseBuildDataCallLine(line string) (buildCallRef, bool, error) {
	expr, ok := strings.CutPrefix(strings.TrimSpace(line), "=>")
	if !ok {
		return buildCallRef{}, false, nil
	}
	expr = strings.TrimSpace(expr)
	if strings.HasPrefix(expr, "{") {
		return buildCallRef{}, false, nil
	}
	parsed, err := parser.ParseExpr(expr)
	if err != nil {
		return buildCallRef{}, false, fmt.Errorf("parse build call: %w", err)
	}
	call, ok := parsed.(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return buildCallRef{}, false, nil
	}
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return buildCallRef{Function: fun.Name}, true, nil
	case *ast.SelectorExpr:
		alias, ok := fun.X.(*ast.Ident)
		if !ok {
			return buildCallRef{}, false, fmt.Errorf("build data call receiver must be an import alias")
		}
		return buildCallRef{Alias: alias.Name, Function: fun.Sel.Name}, true, nil
	default:
		return buildCallRef{}, false, nil
	}
}

func parseBuildLiteralLine(line string) (*ast.CompositeLit, bool, error) {
	body, ok := strings.CutPrefix(strings.TrimSpace(line), "=>")
	if !ok {
		return nil, false, nil
	}
	body = strings.TrimSpace(body)
	if !strings.HasPrefix(body, "{") || !strings.HasSuffix(body, "}") {
		return nil, false, nil
	}
	expr, err := parser.ParseExpr("struct{}" + body)
	if err != nil {
		return nil, true, fmt.Errorf("parse build literal: %w", err)
	}
	literal, ok := expr.(*ast.CompositeLit)
	if !ok {
		return nil, true, fmt.Errorf("build literal must be an object")
	}
	return literal, true, nil
}

func buildFieldValue(expr ast.Expr, routeParams map[string]string, data map[string]buildValue) (string, buildValue, error) {
	kv, ok := expr.(*ast.KeyValueExpr)
	if !ok {
		return "", buildValue{}, fmt.Errorf("build field must use name: value")
	}
	key, ok := kv.Key.(*ast.Ident)
	if !ok || !literalNamePattern.MatchString(key.Name) {
		return "", buildValue{}, fmt.Errorf("invalid build field name")
	}
	value, err := buildValueFromExpr(kv.Value, routeParams, data)
	if err != nil {
		return "", buildValue{}, fmt.Errorf("build field %s: %w", key.Name, err)
	}
	return key.Name, value, nil
}

func buildValueFromExpr(expr ast.Expr, routeParams map[string]string, data map[string]buildValue) (buildValue, error) {
	switch typed := expr.(type) {
	case *ast.BasicLit:
		switch typed.Kind {
		case token.STRING:
			value, err := strconv.Unquote(typed.Value)
			if err != nil {
				return buildValue{}, err
			}
			if strings.TrimSpace(value) == "" {
				return buildValue{}, fmt.Errorf("value must not be empty")
			}
			interpolated, err := interpolateBuildValue(value, routeParams, buildValueStrings(data))
			if err != nil {
				return buildValue{}, err
			}
			return buildStringValue(interpolated), nil
		case token.INT, token.FLOAT:
			number, err := strconv.ParseFloat(strings.ReplaceAll(typed.Value, "_", ""), 64)
			if err != nil {
				return buildValue{}, fmt.Errorf("invalid numeric literal %q", typed.Value)
			}
			value := buildNumberValue(number)
			if typed.Kind == token.INT {
				value.text = strings.ReplaceAll(typed.Value, "_", "")
			}
			return value, nil
		default:
			return buildValue{}, fmt.Errorf("unsupported scalar literal")
		}
	case *ast.Ident:
		switch typed.Name {
		case "true":
			return buildBoolValue(true), nil
		case "false":
			return buildBoolValue(false), nil
		case "nil", "null":
			return buildNilValue(), nil
		default:
			value, ok := data[typed.Name]
			if !ok {
				return buildValue{}, fmt.Errorf("unknown build field reference %q", typed.Name)
			}
			return value, nil
		}
	case *ast.CallExpr:
		return buildCallValue(typed, routeParams, data)
	case *ast.ParenExpr:
		return buildValueFromExpr(typed.X, routeParams, data)
	case *ast.UnaryExpr:
		value, err := buildValueFromExpr(typed.X, routeParams, data)
		if err != nil {
			return buildValue{}, err
		}
		return buildUnaryValue(typed.Op, value)
	case *ast.BinaryExpr:
		left, err := buildValueFromExpr(typed.X, routeParams, data)
		if err != nil {
			return buildValue{}, err
		}
		right, err := buildValueFromExpr(typed.Y, routeParams, data)
		if err != nil {
			return buildValue{}, err
		}
		return buildBinaryValue(typed.Op, left, right)
	default:
		return buildValue{}, fmt.Errorf("value must be a string, number, boolean, nil, expression, param(), field(), or earlier field reference")
	}
}

func buildCallValue(call *ast.CallExpr, routeParams map[string]string, data map[string]buildValue) (buildValue, error) {
	name, ok := call.Fun.(*ast.Ident)
	if !ok || len(call.Args) != 1 {
		return buildValue{}, fmt.Errorf("unsupported build value call")
	}
	arg, ok := call.Args[0].(*ast.BasicLit)
	if !ok || arg.Kind != token.STRING {
		return buildValue{}, fmt.Errorf("%s argument must be a string literal", name.Name)
	}
	key, err := strconv.Unquote(arg.Value)
	if err != nil {
		return buildValue{}, err
	}
	switch name.Name {
	case "param":
		value, ok := routeParams[key]
		if !ok {
			return buildValue{}, fmt.Errorf("unknown route param %q", key)
		}
		return buildStringValue(value), nil
	case "field":
		value, ok := data[key]
		if !ok {
			return buildValue{}, fmt.Errorf("unknown build field %q", key)
		}
		return value, nil
	default:
		return buildValue{}, fmt.Errorf("unsupported build value call %s", name.Name)
	}
}

func buildUnaryValue(op token.Token, value buildValue) (buildValue, error) {
	switch op {
	case token.ADD:
		if value.kind != buildValueNumber {
			return buildValue{}, fmt.Errorf("unary + requires a number")
		}
		return value, nil
	case token.SUB:
		if value.kind != buildValueNumber {
			return buildValue{}, fmt.Errorf("unary - requires a number")
		}
		return buildNumberValue(-value.number), nil
	case token.NOT:
		if value.kind != buildValueBool {
			return buildValue{}, fmt.Errorf("unary ! requires a boolean")
		}
		return buildBoolValue(!value.boolean), nil
	default:
		return buildValue{}, fmt.Errorf("unsupported unary operator %s", op)
	}
}

func buildBinaryValue(op token.Token, left, right buildValue) (buildValue, error) {
	switch op {
	case token.ADD:
		if left.kind == buildValueString || right.kind == buildValueString {
			return buildStringValue(left.text + right.text), nil
		}
		return buildNumericBinaryValue(op, left, right)
	case token.SUB, token.MUL, token.QUO, token.REM:
		return buildNumericBinaryValue(op, left, right)
	case token.EQL, token.NEQ:
		equal, err := buildValuesEqual(left, right)
		if err != nil {
			return buildValue{}, err
		}
		if op == token.NEQ {
			equal = !equal
		}
		return buildBoolValue(equal), nil
	case token.LSS, token.LEQ, token.GTR, token.GEQ:
		return buildOrderedComparisonValue(op, left, right)
	case token.LAND, token.LOR:
		if left.kind != buildValueBool || right.kind != buildValueBool {
			return buildValue{}, fmt.Errorf("logical operator %s requires booleans", op)
		}
		if op == token.LAND {
			return buildBoolValue(left.boolean && right.boolean), nil
		}
		return buildBoolValue(left.boolean || right.boolean), nil
	default:
		return buildValue{}, fmt.Errorf("unsupported binary operator %s", op)
	}
}

func buildNumericBinaryValue(op token.Token, left, right buildValue) (buildValue, error) {
	if left.kind != buildValueNumber || right.kind != buildValueNumber {
		return buildValue{}, fmt.Errorf("operator %s requires numbers", op)
	}
	switch op {
	case token.ADD:
		return buildNumberValue(left.number + right.number), nil
	case token.SUB:
		return buildNumberValue(left.number - right.number), nil
	case token.MUL:
		return buildNumberValue(left.number * right.number), nil
	case token.QUO:
		if right.number == 0 {
			return buildValue{}, fmt.Errorf("division by zero")
		}
		return buildNumberValue(left.number / right.number), nil
	case token.REM:
		if right.number == 0 {
			return buildValue{}, fmt.Errorf("division by zero")
		}
		return buildNumberValue(math.Mod(left.number, right.number)), nil
	default:
		return buildValue{}, fmt.Errorf("unsupported numeric operator %s", op)
	}
}

func buildValuesEqual(left, right buildValue) (bool, error) {
	if left.kind != right.kind {
		return false, nil
	}
	switch left.kind {
	case buildValueString, buildValueNil:
		return left.text == right.text, nil
	case buildValueNumber:
		return left.number == right.number, nil
	case buildValueBool:
		return left.boolean == right.boolean, nil
	default:
		return false, fmt.Errorf("unsupported equality operands")
	}
}

func buildOrderedComparisonValue(op token.Token, left, right buildValue) (buildValue, error) {
	if left.kind != right.kind {
		return buildValue{}, fmt.Errorf("operator %s requires matching operand types", op)
	}
	var result bool
	switch left.kind {
	case buildValueNumber:
		result = compareOrdered(op, left.number, right.number)
	case buildValueString:
		result = compareOrdered(op, left.text, right.text)
	default:
		return buildValue{}, fmt.Errorf("operator %s requires strings or numbers", op)
	}
	return buildBoolValue(result), nil
}

func compareOrdered[T ~float64 | ~string](op token.Token, left, right T) bool {
	switch op {
	case token.LSS:
		return left < right
	case token.LEQ:
		return left <= right
	case token.GTR:
		return left > right
	case token.GEQ:
		return left >= right
	default:
		return false
	}
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
	case "", "spa":
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
	if _, err := file.WriteString(runnerSource); err != nil {
		file.Close()
		return nil, err
	}
	if err := file.Close(); err != nil {
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

func interpolateBuildValue(value string, routeParams map[string]string, data map[string]string) (string, error) {
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
			resolved, ok := routeParams[name]
			if !ok {
				return "", fmt.Errorf("unknown route param %q", name)
			}
			out.WriteString(resolved)
			value = value[end+1:]
			continue
		}
		if field, ok := buildFieldExpression(name); ok {
			name = field
		}
		resolved, ok := data[name]
		if !ok {
			resolved, ok = routeParams[name]
		}
		if !ok {
			return "", fmt.Errorf("unknown route param %q", name)
		}
		out.WriteString(resolved)
		value = value[end+1:]
	}
}

func buildFieldExpression(value string) (string, bool) {
	if !strings.HasPrefix(value, `field("`) || !strings.HasSuffix(value, `")`) {
		return "", false
	}
	name := strings.TrimPrefix(strings.TrimSuffix(value, `")`), `field("`)
	if !literalNamePattern.MatchString(name) {
		return "", false
	}
	return name, true
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
