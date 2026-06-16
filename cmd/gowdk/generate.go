package main

import (
	"bytes"
	"fmt"
	"go/ast"
	goformat "go/format"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

const generateUsage = "usage: gowdk generate stubs [--config <file>] [--env-file <file>] [--module <name>] [--ssr] [files...]"

func generate(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf(generateUsage)
	}
	switch args[0] {
	case "stubs":
		return generateStubs(args[1:])
	default:
		return fmt.Errorf("unknown generate target %q", args[0])
	}
}

func generateStubs(args []string) error {
	_, ir, err := commandProgram(args, "generate stubs", false)
	if err != nil {
		return err
	}
	files := stubFilesFromIR(ir)
	if len(files) == 0 {
		fmt.Println("no missing action/API handler stubs to generate")
		return nil
	}
	paths := make([]string, 0, len(files))
	for _, file := range files {
		path, err := writeStubFile(file)
		if err != nil {
			return err
		}
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		fmt.Println(path)
	}
	return nil
}

type stubFile struct {
	dir         string
	packageName string
	targets     []stubTarget
}

type stubTarget struct {
	kind   gwdkir.EndpointKind
	symbol string
}

func stubFilesFromIR(ir gwdkir.Program) []stubFile {
	byDir := map[string]*stubFile{}
	for _, endpoint := range ir.Endpoints {
		if endpoint.Kind != gwdkir.EndpointAction && endpoint.Kind != gwdkir.EndpointAPI {
			continue
		}
		if endpoint.Binding.Status != source.BackendBindingMissing {
			continue
		}
		symbol := endpoint.Binding.FunctionName
		if strings.TrimSpace(symbol) == "" {
			symbol = endpoint.Symbol
		}
		if !isExportedGoIdentifier(symbol) {
			continue
		}
		dir := "."
		if strings.TrimSpace(endpoint.SourceFile) != "" {
			dir = filepath.Dir(endpoint.SourceFile)
		}
		file := byDir[dir]
		if file == nil {
			packageName := endpoint.Binding.PackageName
			if strings.TrimSpace(packageName) == "" {
				packageName = endpoint.Package
			}
			if !isGoIdentifier(packageName) {
				packageName = "app"
			}
			file = &stubFile{dir: dir, packageName: packageName}
			byDir[dir] = file
		}
		if !hasStubTarget(file.targets, endpoint.Kind, symbol) {
			file.targets = append(file.targets, stubTarget{kind: endpoint.Kind, symbol: symbol})
		}
	}
	files := make([]stubFile, 0, len(byDir))
	for _, file := range byDir {
		sort.Slice(file.targets, func(i, j int) bool {
			if file.targets[i].kind == file.targets[j].kind {
				return file.targets[i].symbol < file.targets[j].symbol
			}
			return file.targets[i].kind < file.targets[j].kind
		})
		files = append(files, *file)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].dir < files[j].dir })
	return files
}

func hasStubTarget(targets []stubTarget, kind gwdkir.EndpointKind, symbol string) bool {
	for _, target := range targets {
		if target.kind == kind && target.symbol == symbol {
			return true
		}
	}
	return false
}

func writeStubFile(file stubFile) (string, error) {
	if err := os.MkdirAll(file.dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(file.dir, "gowdk_stubs.go")
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("%s already exists; refusing to overwrite handler stubs", path)
	} else if !os.IsNotExist(err) {
		return "", err
	}
	source, err := stubFileSource(file)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, source, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func stubFileSource(file stubFile) ([]byte, error) {
	decls := []ast.Decl{stubImportDecl(file.targets)}
	for _, target := range file.targets {
		decls = append(decls, stubFuncDecl(target))
	}
	astFile := &ast.File{
		Name:  ast.NewIdent(file.packageName),
		Decls: decls,
	}
	var buffer bytes.Buffer
	if err := printer.Fprint(&buffer, token.NewFileSet(), astFile); err != nil {
		return nil, err
	}
	return goformat.Source(buffer.Bytes())
}

func stubImportDecl(targets []stubTarget) ast.Decl {
	specs := []ast.Spec{
		&ast.ImportSpec{Name: ast.NewIdent("gowdkcontext"), Path: stubStringLit("context")},
		&ast.ImportSpec{Name: ast.NewIdent("gowdkresponse"), Path: stubStringLit("github.com/cssbruno/gowdk/runtime/response")},
	}
	if hasAPIStub(targets) {
		specs = append(specs, &ast.ImportSpec{Name: ast.NewIdent("gowdkhttp"), Path: stubStringLit("net/http")})
	}
	return &ast.GenDecl{Tok: token.IMPORT, Specs: specs}
}

func hasAPIStub(targets []stubTarget) bool {
	for _, target := range targets {
		if target.kind == gwdkir.EndpointAPI {
			return true
		}
	}
	return false
}

func stubFuncDecl(target stubTarget) ast.Decl {
	params := []*ast.Field{{Type: stubSelector("gowdkcontext", "Context")}}
	if target.kind == gwdkir.EndpointAPI {
		params = append(params, &ast.Field{Type: &ast.StarExpr{X: stubSelector("gowdkhttp", "Request")}})
	}
	return &ast.FuncDecl{
		Name: ast.NewIdent(target.symbol),
		Type: &ast.FuncType{
			Params: &ast.FieldList{List: params},
			Results: &ast.FieldList{List: []*ast.Field{
				{Type: stubSelector("gowdkresponse", "Response")},
				{Type: ast.NewIdent("error")},
			}},
		},
		Body: &ast.BlockStmt{List: []ast.Stmt{
			&ast.ReturnStmt{Results: []ast.Expr{
				&ast.CallExpr{
					Fun: stubSelector("gowdkresponse", "HTMLBody"),
					Args: []ast.Expr{
						&ast.BasicLit{Kind: token.INT, Value: "501"},
						stubStringLit("GOWDK generated stub: implement " + target.symbol),
					},
				},
				ast.NewIdent("nil"),
			}},
		}},
	}
}

func stubSelector(pkg, name string) ast.Expr {
	return &ast.SelectorExpr{X: ast.NewIdent(pkg), Sel: ast.NewIdent(name)}
}

func stubStringLit(value string) *ast.BasicLit {
	return &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(value)}
}

func isExportedGoIdentifier(value string) bool {
	if !isGoIdentifier(value) {
		return false
	}
	first, _ := utf8.DecodeRuneInString(value)
	return unicode.IsUpper(first)
}

func isGoIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for index, r := range value {
		if index == 0 {
			if r != '_' && !unicode.IsLetter(r) {
				return false
			}
			continue
		}
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
