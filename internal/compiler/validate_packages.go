package compiler

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/cssbruno/gowdk/internal/manifest"
)

type packageDeclaration struct {
	Source        string
	Label         string
	PageID        string
	ComponentName string
	Package       string
	Span          manifest.SourceSpan
}

type goPackageInfo struct {
	Name        string
	Source      string
	Diagnostics []ValidationError
}

func validatePackages(app manifest.Manifest) []ValidationError {
	declarations := packageDeclarations(app)
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
		goInfo := inspectGoPackageForValidation(dir)
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

func shouldValidatePackageSource(source string) bool {
	source = strings.TrimSpace(source)
	if source == "" {
		return false
	}
	if _, err := os.Stat(source); err == nil {
		return true
	}
	return false
}

func packageDeclarations(app manifest.Manifest) []packageDeclaration {
	var declarations []packageDeclaration
	for _, page := range app.Pages {
		declarations = append(declarations, packageDeclaration{
			Source:  page.Source,
			Label:   sourceLabel(page.Source, page.ID+".page.gwdk"),
			PageID:  page.ID,
			Package: page.Package,
			Span:    firstSpan(page.Spans.Package, page.Spans.Page, page.Spans.Route),
		})
	}
	for _, component := range app.Components {
		declarations = append(declarations, packageDeclaration{
			Source:        component.Source,
			Label:         sourceLabel(component.Source, component.Name+".cmp.gwdk"),
			ComponentName: component.Name,
			Package:       component.Package,
			Span:          firstSpan(component.PackageSpan, component.Span),
		})
	}
	for _, layout := range app.Layouts {
		declarations = append(declarations, packageDeclaration{
			Source:  layout.Source,
			Label:   sourceLabel(layout.Source, layout.ID+".layout.gwdk"),
			Package: layout.Package,
			Span:    firstSpan(layout.PackageSpan, layout.Span),
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

func inspectGoPackageForValidation(dir string) goPackageInfo {
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
	if len(info.Diagnostics) == 0 && len(files) > 0 {
		info.Diagnostics = append(info.Diagnostics, typeCheckGoPackageForValidation(dir, info.Name, fileSet, files)...)
	}
	return info
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
	position := fileSet.Position(typeError.Pos)
	if position.IsValid() {
		diagnostic.Source = position.Filename
		diagnostic.Span = manifest.SourceSpan{
			Start: manifest.SourcePosition{Line: position.Line, Column: position.Column},
			End:   manifest.SourcePosition{Line: position.Line, Column: position.Column + 1},
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

func packageSourceDir(source string) string {
	dir := filepath.Dir(source)
	abs, err := filepath.Abs(dir)
	if err != nil {
		return dir
	}
	return abs
}

func sourceLabel(source string, fallback string) string {
	if strings.TrimSpace(source) == "" {
		return fallback
	}
	return filepath.Base(source)
}
