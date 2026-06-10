package compiler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

type packageDeclaration struct {
	Source        string
	Label         string
	PageID        string
	ComponentName string
	Package       string
	Imports       []gwdkir.Import
	GoBlocks      []gwdkir.GoBlock
	Span          source.SourceSpan
}

type goPackageInfo struct {
	Name        string
	Source      string
	Diagnostics []ValidationError
}

func validatePackages(ir gwdkir.Program) []ValidationError {
	declarations := packageDeclarations(ir)
	var diagnostics []ValidationError
	byDir := map[string][]packageDeclaration{}
	for _, declaration := range declarations {
		if !shouldValidatePackageSource(declaration.Source) {
			continue
		}
		if strings.TrimSpace(declaration.Package) == "" {
			diagnostics = append(diagnostics, ValidationError{
				Code:          "missing_package_declaration",
				PageID:        declaration.PageID,
				ComponentName: declaration.ComponentName,
				Source:        declaration.Source,
				Span:          declaration.Span,
				Message:       fmt.Sprintf("%s is missing a package declaration; add package <name> as the first non-comment declaration", declaration.Label),
			})
			continue
		}
		dir := packageSourceDir(declaration.Source)
		byDir[dir] = append(byDir[dir], declaration)
	}

	for dir, group := range byDir {
		diagnostics = append(diagnostics, validateGOWDKPackageGroup(dir, group)...)
		goInfo := inspectGoPackageForValidation(dir, group)
		diagnostics = append(diagnostics, goInfo.Diagnostics...)
		if goInfo.Name == "" {
			continue
		}
		for _, declaration := range group {
			if declaration.Package == goInfo.Name {
				continue
			}
			diagnostics = append(diagnostics, ValidationError{
				Code:          "package_mismatch",
				PageID:        declaration.PageID,
				ComponentName: declaration.ComponentName,
				Source:        declaration.Source,
				Span:          declaration.Span,
				Message: fmt.Sprintf(
					"%s declares package %s, but sibling Go file %s declares package %s",
					declaration.Label,
					declaration.Package,
					goInfo.Source,
					goInfo.Name,
				),
			})
		}
	}
	return diagnostics
}

func shouldValidatePackageSource(sourcePath string) bool {
	sourcePath = strings.TrimSpace(sourcePath)
	if sourcePath == "" {
		return false
	}
	if _, err := os.Stat(sourcePath); err == nil {
		return true
	}
	return false
}

func packageDeclarations(ir gwdkir.Program) []packageDeclaration {
	var declarations []packageDeclaration
	for _, page := range ir.Pages {
		declarations = append(declarations, packageDeclaration{
			Source:   page.Source,
			Label:    sourceLabel(page.Source, page.ID+".page.gwdk"),
			PageID:   page.ID,
			Package:  page.Package,
			Imports:  page.Imports,
			GoBlocks: page.Blocks.GoBlocks,
			Span:     firstSpan(page.Spans.Package, page.Spans.Page, page.Spans.Route),
		})
	}
	for _, component := range ir.Components {
		declarations = append(declarations, packageDeclaration{
			Source:        component.Source,
			Label:         sourceLabel(component.Source, component.Name+".cmp.gwdk"),
			ComponentName: component.Name,
			Package:       component.Package,
			Imports:       component.Imports,
			GoBlocks:      component.Blocks.GoBlocks,
			Span:          firstSpan(component.PackageSpan, component.Span),
		})
	}
	for _, layout := range ir.Layouts {
		declarations = append(declarations, packageDeclaration{
			Source:   layout.Source,
			Label:    sourceLabel(layout.Source, layout.ID+".layout.gwdk"),
			Package:  layout.Package,
			GoBlocks: layout.Blocks.GoBlocks,
			Span:     firstSpan(layout.PackageSpan, layout.Span),
		})
	}
	return declarations
}

func validateGOWDKPackageGroup(dir string, group []packageDeclaration) []ValidationError {
	if len(group) < 2 {
		return nil
	}
	sort.Slice(group, func(i, j int) bool {
		return group[i].Source < group[j].Source
	})
	expected := group[0].Package
	expectedSource := group[0].Source
	var diagnostics []ValidationError
	for _, declaration := range group[1:] {
		if declaration.Package == expected {
			continue
		}
		diagnostics = append(diagnostics, ValidationError{
			Code:          "package_mismatch",
			PageID:        declaration.PageID,
			ComponentName: declaration.ComponentName,
			Source:        declaration.Source,
			Span:          declaration.Span,
			Message: fmt.Sprintf(
				"%s declares package %s, but sibling GOWDK file %s in %s declares package %s",
				declaration.Label,
				declaration.Package,
				expectedSource,
				dir,
				expected,
			),
		})
	}
	return diagnostics
}

func inspectGoPackageForValidation(dir string, group []packageDeclaration) goPackageInfo {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return goPackageInfo{}
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})
	fileSet := token.NewFileSet()
	var info goPackageInfo
	var files []*ast.File
	for _, entry := range entries {
		if entry.IsDir() || !isGoPackageSource(entry.Name()) {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		file, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			info.Diagnostics = append(info.Diagnostics, ValidationError{
				Code:    "go_package_error",
				Source:  path,
				Message: fmt.Sprintf("Go package file %s cannot be parsed: %v", path, err),
			})
			continue
		}
		if file.Name == nil {
			info.Diagnostics = append(info.Diagnostics, ValidationError{
				Code:    "go_package_error",
				Source:  path,
				Message: fmt.Sprintf("Go package file %s is missing a package declaration", path),
			})
			continue
		}
		files = append(files, file)
		name := file.Name.Name
		if info.Name == "" {
			info.Name = name
			info.Source = path
			continue
		}
		if name != info.Name {
			info.Diagnostics = append(info.Diagnostics, ValidationError{
				Code:    "go_package_error",
				Source:  path,
				Message: fmt.Sprintf("Go package directory %s contains packages %s and %s", dir, info.Name, name),
			})
		}
	}
	for _, declaration := range group {
		for _, block := range declaration.GoBlocks {
			if !isPackageGoBlockTarget(block.Target) {
				continue
			}
			file, err := parseGoBlockPackageFileForValidation(fileSet, declaration, block)
			if err != nil {
				info.Diagnostics = append(info.Diagnostics, *err)
				continue
			}
			if file == nil {
				continue
			}
			files = append(files, file)
			name := file.Name.Name
			if info.Name == "" {
				info.Name = name
				info.Source = declaration.Source
				continue
			}
			if name != info.Name {
				info.Diagnostics = append(info.Diagnostics, ValidationError{
					Code:    "go_package_error",
					Source:  declaration.Source,
					Span:    block.Span,
					Message: fmt.Sprintf("GOWDK go block in %s declares package %s, but sibling Go package declares package %s", declaration.Source, name, info.Name),
				})
			}
		}
	}
	if len(info.Diagnostics) == 0 && len(files) > 0 {
		info.Diagnostics = append(info.Diagnostics, typeCheckGoPackageForValidation(dir, info.Name, fileSet, files)...)
	}
	return info
}

func parseGoBlockPackageFileForValidation(fileSet *token.FileSet, declaration packageDeclaration, block gwdkir.GoBlock) (*ast.File, *ValidationError) {
	src, err := goBlockPackageSourceForValidation(declaration, block)
	if err != nil {
		return nil, nil
	}
	file, parseErr := parser.ParseFile(fileSet, declaration.Source, src, parser.AllErrors|parser.ParseComments)
	if parseErr != nil {
		diagnostic := ValidationError{
			Code:          "invalid_go_block",
			PageID:        declaration.PageID,
			ComponentName: declaration.ComponentName,
			Source:        declaration.Source,
			Span:          block.Span,
			Message:       fmt.Sprintf("go %s contains invalid Go: %v", goBlockLabel(block.Target), parseErr),
		}
		return nil, &diagnostic
	}
	return file, nil
}

func goBlockPackageSourceForValidation(declaration packageDeclaration, block gwdkir.GoBlock) (string, error) {
	packageName := strings.TrimSpace(declaration.Package)
	if packageName == "" {
		return "", fmt.Errorf("go block package is missing")
	}
	body := strings.TrimSpace(block.Body)
	if body == "" {
		return "package " + packageName + "\n", nil
	}
	bodyFileSet := token.NewFileSet()
	bodyFile, err := parser.ParseFile(bodyFileSet, declaration.Source, "package "+packageName+"\n\n"+block.Body, parser.AllErrors)
	if err != nil {
		return "", err
	}

	file := &ast.File{
		Package: bodyFile.Package,
		Name:    bodyFile.Name,
	}
	if specs := goBlockGOWDKImportSpecsForValidation(declaration.Imports, bodyFile); len(specs) > 0 {
		file.Decls = append(file.Decls, &ast.GenDecl{Tok: token.IMPORT, Specs: specs})
	}
	line := block.Span.Start.Line
	if line <= 0 {
		line = 1
	}
	if directive := addGoBlockLineDirectiveForValidation(bodyFileSet, bodyFile, filepath.ToSlash(declaration.Source), line); directive != nil {
		file.Comments = append(file.Comments, directive)
	}
	file.Decls = append(file.Decls, bodyFile.Decls...)
	file.Comments = append(file.Comments, bodyFile.Comments...)

	var buffer bytes.Buffer
	if err := printer.Fprint(&buffer, bodyFileSet, file); err != nil {
		return "", fmt.Errorf("print go block package source: %w", err)
	}
	return buffer.String(), nil
}

func goBlockGOWDKImportSpecsForValidation(imports []gwdkir.Import, bodyFile *ast.File) []ast.Spec {
	used := usedScriptIdentifiersForValidation(bodyFile)
	localImports := goBlockImportAliasesForValidation(bodyFile)
	var specs []ast.Spec
	for _, item := range imports {
		importPath := strings.TrimSpace(item.Path)
		if importPath == "" {
			continue
		}
		alias := goBlockImportAliasForValidation(item)
		if !used[alias] || localImports[alias] {
			continue
		}
		spec := &ast.ImportSpec{Path: &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(importPath)}}
		if strings.TrimSpace(item.Alias) != "" {
			spec.Name = ast.NewIdent(item.Alias)
		}
		specs = append(specs, spec)
	}
	sort.Slice(specs, func(left, right int) bool {
		return goBlockImportSpecSortKey(specs[left]) < goBlockImportSpecSortKey(specs[right])
	})
	return specs
}

func goBlockImportSpecSortKey(spec ast.Spec) string {
	importSpec, ok := spec.(*ast.ImportSpec)
	if !ok {
		return ""
	}
	alias := ""
	if importSpec.Name != nil {
		alias = importSpec.Name.Name
	}
	path := ""
	if importSpec.Path != nil {
		path = importSpec.Path.Value
	}
	return alias + "\x00" + path
}

func addGoBlockLineDirectiveForValidation(fileSet *token.FileSet, file *ast.File, sourcePath string, line int) *ast.CommentGroup {
	if len(file.Decls) == 0 {
		return nil
	}
	directive := &ast.Comment{
		Slash: goBlockLineDirectivePosition(fileSet, file.Decls[0].Pos()),
		Text:  "//line " + sourcePath + ":" + strconv.Itoa(line),
	}
	group := &ast.CommentGroup{List: []*ast.Comment{directive}}
	switch decl := file.Decls[0].(type) {
	case *ast.GenDecl:
		if decl.Doc == nil {
			decl.Doc = group
			return group
		}
		decl.Doc.List = append(group.List, decl.Doc.List...)
	case *ast.FuncDecl:
		if decl.Doc == nil {
			decl.Doc = group
			return group
		}
		decl.Doc.List = append(group.List, decl.Doc.List...)
	default:
		file.Comments = append([]*ast.CommentGroup{group}, file.Comments...)
	}
	return group
}

func goBlockLineDirectivePosition(fileSet *token.FileSet, declaration token.Pos) token.Pos {
	tokenFile := fileSet.File(declaration)
	if tokenFile == nil {
		return declaration
	}
	position := fileSet.Position(declaration)
	if position.Line > 1 {
		return tokenFile.LineStart(position.Line) - 1
	}
	return declaration
}

func usedScriptIdentifiersForValidation(file *ast.File) map[string]bool {
	used := map[string]bool{}
	ast.Inspect(file, func(node ast.Node) bool {
		ident, ok := node.(*ast.Ident)
		if ok {
			used[ident.Name] = true
		}
		return true
	})
	return used
}

func goBlockImportAliasesForValidation(file *ast.File) map[string]bool {
	aliases := map[string]bool{}
	for _, spec := range file.Imports {
		importPath := ""
		if spec.Path != nil {
			if unquoted, err := strconv.Unquote(spec.Path.Value); err == nil {
				importPath = unquoted
			}
		}
		alias := ""
		if spec.Name != nil {
			alias = spec.Name.Name
		}
		if alias == "" {
			alias = filepath.Base(importPath)
		}
		if alias != "" {
			aliases[alias] = true
		}
	}
	return aliases
}

func goBlockImportAliasForValidation(item gwdkir.Import) string {
	if strings.TrimSpace(item.Alias) != "" {
		return item.Alias
	}
	return filepath.Base(strings.TrimSpace(item.Path))
}

func isPackageGoBlockTarget(target string) bool {
	switch strings.TrimSpace(target) {
	case "":
		return true
	default:
		return false
	}
}

func typeCheckGoPackageForValidation(packageDir string, packageName string, fileSet *token.FileSet, files []*ast.File) []ValidationError {
	var typeErrors []error
	config := types.Config{
		Importer: goTypeImporterForValidation(packageDir, fileSet, files),
		Error: func(err error) {
			typeErrors = append(typeErrors, err)
		},
	}
	_, err := config.Check(packageName, fileSet, files, nil)
	if len(typeErrors) == 0 && err != nil {
		typeErrors = append(typeErrors, err)
	}
	if len(typeErrors) == 0 {
		return nil
	}
	diagnostics := make([]ValidationError, 0, len(typeErrors))
	for _, err := range typeErrors {
		diagnostics = append(diagnostics, goTypeCheckDiagnostic(fileSet, err))
	}
	return diagnostics
}

func goTypeImporterForValidation(packageDir string, fileSet *token.FileSet, files []*ast.File) types.Importer {
	importPaths := importedGoPaths(files)
	if len(importPaths) == 0 {
		return importer.Default()
	}
	exports, err := goListExportFiles(packageDir, importPaths)
	if err != nil || len(exports) == 0 {
		return importer.Default()
	}
	return importer.ForCompiler(fileSet, "gc", func(path string) (io.ReadCloser, error) {
		exportPath := exports[path]
		if exportPath == "" {
			return nil, fmt.Errorf("missing export data for %s", path)
		}
		return os.Open(exportPath)
	})
}

func importedGoPaths(files []*ast.File) []string {
	seen := map[string]bool{}
	var paths []string
	for _, file := range files {
		for _, spec := range file.Imports {
			if spec.Path == nil {
				continue
			}
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil || path == "" || seen[path] {
				continue
			}
			seen[path] = true
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	return paths
}

func goListExportFiles(packageDir string, importPaths []string) (map[string]string, error) {
	args := append([]string{"list", "-deps", "-export", "-json"}, importPaths...)
	command := exec.Command("go", args...)
	command.Dir = packageDir
	output, err := command.Output()
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(strings.NewReader(string(output)))
	exports := map[string]string{}
	for {
		var item struct {
			ImportPath string
			Export     string
		}
		if err := decoder.Decode(&item); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if item.ImportPath == "" || item.Export == "" {
			continue
		}
		exports[item.ImportPath] = item.Export
	}
	return exports, nil
}

func goTypeCheckDiagnostic(fileSet *token.FileSet, err error) ValidationError {
	diagnostic := ValidationError{
		Code:    "go_package_error",
		Message: fmt.Sprintf("Go package failed type-check: %v", err),
	}
	var typeError types.Error
	switch typed := err.(type) {
	case types.Error:
		typeError = typed
	case *types.Error:
		if typed == nil {
			return diagnostic
		}
		typeError = *typed
	default:
		return diagnostic
	}
	position := fileSet.PositionFor(typeError.Pos, true)
	if position.IsValid() {
		diagnostic.Source = position.Filename
		diagnostic.Span = source.SourceSpan{
			Start: source.SourcePosition{Line: position.Line, Column: position.Column},
			End:   source.SourcePosition{Line: position.Line, Column: position.Column + 1},
		}
	}
	return diagnostic
}

func isGoPackageSource(name string) bool {
	if name == "gowdk.config.go" {
		return false
	}
	return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
}

func packageSourceDir(sourcePath string) string {
	dir := filepath.Dir(sourcePath)
	abs, err := filepath.Abs(dir)
	if err != nil {
		return dir
	}
	return abs
}

func sourceLabel(sourcePath string, fallback string) string {
	if strings.TrimSpace(sourcePath) == "" {
		return fallback
	}
	return filepath.Base(sourcePath)
}
