package compiler

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk/internal/diagnostics"
)

type compilerDiagnosticEmission struct {
	code     string
	severity diagnostics.Severity
	source   string
}

type compilerDiagnosticHelper struct {
	codeParam int
	severity  diagnostics.Severity
}

func TestCompilerEmittedDiagnosticSeveritiesMatchRegistry(t *testing.T) {
	emissions := compilerDiagnosticEmissions(t)
	if len(emissions) == 0 {
		t.Fatal("expected compiler diagnostic emissions")
	}
	var mismatches []string
	for _, emission := range emissions {
		expected, ok := diagnostics.DefaultSeverity(emission.code)
		if !ok {
			mismatches = append(mismatches, emission.source+": unregistered diagnostic code "+emission.code)
			continue
		}
		if emission.severity == expected || compilerSeverityOverrideAllowed(emission.code, emission.severity) {
			continue
		}
		mismatches = append(mismatches, emission.source+": "+emission.code+" severity "+string(emission.severity)+" differs from registry default "+string(expected))
	}
	sort.Strings(mismatches)
	if len(mismatches) > 0 {
		t.Fatalf("compiler diagnostic severity drift:\n%s", strings.Join(mismatches, "\n"))
	}
}

func compilerSeverityOverrideAllowed(code string, severity diagnostics.Severity) bool {
	// A missing page guard is a warning by default, but pages that expose backend
	// endpoints fail closed as errors because public-by-omission would expose
	// request-time behavior.
	return code == "missing_page_guard" && severity == diagnostics.SeverityError
}

func compilerDiagnosticEmissions(t *testing.T) []compilerDiagnosticEmission {
	t.Helper()
	fileSet := token.NewFileSet()
	files := parseCompilerFiles(t, fileSet)
	helpers := compilerDiagnosticHelpers(files)
	var emissions []compilerDiagnosticEmission
	for _, file := range files {
		parents := parentNodes(file.file)
		ast.Inspect(file.file, func(node ast.Node) bool {
			switch typed := node.(type) {
			case *ast.CompositeLit:
				if emission, ok := compilerCompositeDiagnosticEmission(fileSet, file.path, typed, parents); ok {
					emissions = append(emissions, emission)
				}
			case *ast.CallExpr:
				if emission, ok := compilerHelperDiagnosticEmission(fileSet, file.path, typed, helpers); ok {
					emissions = append(emissions, emission)
				}
			}
			return true
		})
	}
	return emissions
}

type parsedCompilerFile struct {
	path string
	file *ast.File
}

func parseCompilerFiles(t *testing.T, fileSet *token.FileSet) []parsedCompilerFile {
	t.Helper()
	var files []parsedCompilerFile
	err := filepath.WalkDir(".", func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			return err
		}
		files = append(files, parsedCompilerFile{path: path, file: file})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func compilerDiagnosticHelpers(files []parsedCompilerFile) map[string]compilerDiagnosticHelper {
	helpers := map[string]compilerDiagnosticHelper{}
	for _, file := range files {
		parents := parentNodes(file.file)
		for _, declaration := range file.file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}
			params := functionParamIndexes(function.Type.Params)
			ast.Inspect(function.Body, func(node ast.Node) bool {
				lit, ok := node.(*ast.CompositeLit)
				if !ok || !isValidationErrorComposite(lit, parents) {
					return true
				}
				codeExpr := compositeField(lit, "Code")
				codeIdent, ok := codeExpr.(*ast.Ident)
				if !ok {
					return true
				}
				paramIndex, ok := params[codeIdent.Name]
				if !ok {
					return true
				}
				helpers[function.Name.Name] = compilerDiagnosticHelper{
					codeParam: paramIndex,
					severity:  validationErrorCompositeSeverity(lit),
				}
				return true
			})
		}
	}
	return helpers
}

func compilerCompositeDiagnosticEmission(fileSet *token.FileSet, path string, lit *ast.CompositeLit, parents map[ast.Node]ast.Node) (compilerDiagnosticEmission, bool) {
	code := stringLiteralValue(compositeField(lit, "Code"))
	if code == "" {
		return compilerDiagnosticEmission{}, false
	}
	switch {
	case isValidationErrorComposite(lit, parents):
		return compilerDiagnosticEmission{code: code, severity: validationErrorCompositeSeverity(lit), source: nodeLocation(fileSet, path, lit)}, true
	case isRouteInfoComposite(lit):
		return compilerDiagnosticEmission{code: code, severity: diagnostics.SeverityInfo, source: nodeLocation(fileSet, path, lit)}, true
	default:
		return compilerDiagnosticEmission{}, false
	}
}

func compilerHelperDiagnosticEmission(fileSet *token.FileSet, path string, call *ast.CallExpr, helpers map[string]compilerDiagnosticHelper) (compilerDiagnosticEmission, bool) {
	name := calledFunctionName(call.Fun)
	helper, ok := helpers[name]
	if !ok || helper.codeParam >= len(call.Args) {
		return compilerDiagnosticEmission{}, false
	}
	code := stringLiteralValue(call.Args[helper.codeParam])
	if code == "" {
		return compilerDiagnosticEmission{}, false
	}
	return compilerDiagnosticEmission{code: code, severity: helper.severity, source: nodeLocation(fileSet, path, call)}, true
}

func parentNodes(root ast.Node) map[ast.Node]ast.Node {
	parents := map[ast.Node]ast.Node{}
	var stack []ast.Node
	ast.Inspect(root, func(node ast.Node) bool {
		if node == nil {
			stack = stack[:len(stack)-1]
			return false
		}
		if len(stack) > 0 {
			parents[node] = stack[len(stack)-1]
		}
		stack = append(stack, node)
		return true
	})
	return parents
}

func functionParamIndexes(fields *ast.FieldList) map[string]int {
	indexes := map[string]int{}
	if fields == nil {
		return indexes
	}
	index := 0
	for _, field := range fields.List {
		if len(field.Names) == 0 {
			index++
			continue
		}
		for _, name := range field.Names {
			indexes[name.Name] = index
			index++
		}
	}
	return indexes
}

func isValidationErrorComposite(lit *ast.CompositeLit, parents map[ast.Node]ast.Node) bool {
	if typeName(lit.Type) == "ValidationError" {
		return true
	}
	parent, ok := parents[lit].(*ast.CompositeLit)
	if !ok {
		return false
	}
	return compositeTypeIsValidationErrorSlice(parent.Type)
}

func compositeTypeIsValidationErrorSlice(expr ast.Expr) bool {
	if typeName(expr) == "ValidationErrors" {
		return true
	}
	array, ok := expr.(*ast.ArrayType)
	return ok && typeName(array.Elt) == "ValidationError"
}

func isRouteInfoComposite(lit *ast.CompositeLit) bool {
	return typeName(lit.Type) == "RouteInfo"
}

func validationErrorCompositeSeverity(lit *ast.CompositeLit) diagnostics.Severity {
	switch typeName(compositeField(lit, "Severity")) {
	case "SeverityWarning":
		return diagnostics.SeverityWarning
	case "SeverityError":
		return diagnostics.SeverityError
	default:
		return diagnostics.SeverityError
	}
}

func compositeField(lit *ast.CompositeLit, name string) ast.Expr {
	for _, element := range lit.Elts {
		kv, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if typeName(kv.Key) == name {
			return kv.Value
		}
	}
	return nil
}

func typeName(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.SelectorExpr:
		return typed.Sel.Name
	default:
		return ""
	}
}

func calledFunctionName(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.Ident:
		return typed.Name
	default:
		return ""
	}
}

func stringLiteralValue(expr ast.Expr) string {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return ""
	}
	value, err := strconv.Unquote(lit.Value)
	if err != nil {
		return ""
	}
	return value
}

func nodeLocation(fileSet *token.FileSet, path string, node ast.Node) string {
	position := fileSet.Position(node.Pos())
	return path + ":" + strconv.Itoa(position.Line)
}
