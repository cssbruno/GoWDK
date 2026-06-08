// Package goblockgen turns captured go blocks into normal generated Go
// package source used by build and app generation.
package goblockgen

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/cssbruno/gowdk/internal/manifest"
)

const generatedImportBase = "gowdk-generated-app/gowdk_go"

// GeneratedImportPath returns the generated-app import path for go blocks from a
// GOWDK package.
func GeneratedImportPath(packageName string) string {
	return generatedImportBase + "/" + SafePackageDir(packageName)
}

// GeneratedRelPath returns the generated app relative path for one go block file.
func GeneratedRelPath(packageName string) string {
	return filepath.Join("gowdk_go", SafePackageDir(packageName), "go.go")
}

// SafePackageName returns a valid Go package identifier for generated go block
// files.
func SafePackageName(packageName string) string {
	name := strings.TrimSpace(packageName)
	if name == "" {
		return "goblocks"
	}
	out := make([]rune, 0, len(name))
	for index, char := range name {
		if char == '_' || unicode.IsLetter(char) || unicode.IsDigit(char) {
			if index == 0 && unicode.IsDigit(char) {
				out = append(out, 'p')
			}
			out = append(out, char)
			continue
		}
		out = append(out, '_')
	}
	result := strings.Trim(string(out), "_")
	if result == "" {
		return "goblocks"
	}
	return result
}

// SafePackageDir returns a stable generated directory name for a GOWDK package.
func SafePackageDir(packageName string) string {
	return SafePackageName(packageName)
}

// ParseFile parses one go block as a normal Go file in packageName.
func ParseFile(packageName string, block manifest.GoBlock) (*ast.File, error) {
	source := "package " + SafePackageName(packageName) + "\n" + block.Body
	file, err := parser.ParseFile(token.NewFileSet(), "go-block.gwdk.go", source, parser.AllErrors)
	if err != nil {
		line := block.Span.Start.Line
		if line > 0 {
			return nil, fmt.Errorf("go block starting on line %d has invalid Go: %w", line, err)
		}
		return nil, fmt.Errorf("go block has invalid Go: %w", err)
	}
	return file, nil
}

// ImportAliases returns Go import aliases from block-local imports plus used
// .gwdk imports.
func ImportAliases(file *ast.File, imports []manifest.Import) map[string]string {
	aliases := astImportAliases(file)
	used := usedIdentifiers(file)
	for _, item := range imports {
		importPath := strings.TrimSpace(item.Path)
		if importPath == "" {
			continue
		}
		alias := importAlias(item)
		if !used[alias] {
			continue
		}
		aliases[alias] = importPath
	}
	return aliases
}

// Source emits one formatted Go source file containing the selected go blocks.
func Source(packageName string, imports []manifest.Import, blocks []manifest.GoBlock) ([]byte, error) {
	var files []*ast.File
	for _, block := range blocks {
		file, err := ParseFile(packageName, block)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	if len(files) == 0 {
		return nil, nil
	}

	var decls []ast.Decl
	importDecl := generatedImportDecl(imports, files)
	if importDecl != nil {
		decls = append(decls, importDecl)
	}
	for _, file := range files {
		for _, declaration := range file.Decls {
			if gen, ok := declaration.(*ast.GenDecl); ok && gen.Tok == token.IMPORT {
				continue
			}
			decls = append(decls, declaration)
		}
	}
	out := &ast.File{Name: ast.NewIdent(SafePackageName(packageName)), Decls: decls}
	var buffer bytes.Buffer
	if err := printer.Fprint(&buffer, token.NewFileSet(), out); err != nil {
		return nil, fmt.Errorf("print generated go block source: %w", err)
	}
	formatted, err := format.Source(buffer.Bytes())
	if err != nil {
		return nil, fmt.Errorf("format generated go block source: %w", err)
	}
	return formatted, nil
}

func generatedImportDecl(imports []manifest.Import, files []*ast.File) ast.Decl {
	var specs []ast.Spec
	seen := map[string]bool{}
	used := map[string]bool{}
	for _, file := range files {
		for alias := range usedIdentifiers(file) {
			used[alias] = true
		}
		for _, spec := range importSpecs(file) {
			alias, importPath := importSpecAliasPath(spec)
			key := importKey(alias, importPath)
			if seen[key] {
				continue
			}
			seen[key] = true
			specs = append(specs, spec)
		}
	}
	for _, item := range imports {
		importPath := strings.TrimSpace(item.Path)
		if importPath == "" {
			continue
		}
		alias := importAlias(item)
		if !used[alias] {
			continue
		}
		key := importKey(strings.TrimSpace(item.Alias), importPath)
		if seen[key] {
			continue
		}
		seen[key] = true
		spec := &ast.ImportSpec{Path: &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(importPath)}}
		if strings.TrimSpace(item.Alias) != "" {
			spec.Name = ast.NewIdent(item.Alias)
		}
		specs = append(specs, spec)
	}
	if len(specs) == 0 {
		return nil
	}
	return &ast.GenDecl{Tok: token.IMPORT, Specs: specs}
}

func astImportAliases(file *ast.File) map[string]string {
	imports := map[string]string{}
	for _, spec := range importSpecs(file) {
		alias, importPath := importSpecAliasPath(spec)
		if alias == "" {
			alias = path.Base(importPath)
		}
		if alias != "" && alias != "." && alias != "_" {
			imports[alias] = importPath
		}
	}
	return imports
}

func importSpecs(file *ast.File) []*ast.ImportSpec {
	if file == nil {
		return nil
	}
	var specs []*ast.ImportSpec
	for _, declaration := range file.Decls {
		gen, ok := declaration.(*ast.GenDecl)
		if !ok || gen.Tok != token.IMPORT {
			continue
		}
		for _, spec := range gen.Specs {
			importSpec, ok := spec.(*ast.ImportSpec)
			if ok {
				specs = append(specs, importSpec)
			}
		}
	}
	return specs
}

func importSpecAliasPath(spec *ast.ImportSpec) (string, string) {
	importPath := strings.Trim(spec.Path.Value, `"`)
	if unquoted, err := strconv.Unquote(spec.Path.Value); err == nil {
		importPath = unquoted
	}
	alias := ""
	if spec.Name != nil {
		alias = spec.Name.Name
	}
	return alias, importPath
}

func importAlias(item manifest.Import) string {
	if strings.TrimSpace(item.Alias) != "" {
		return item.Alias
	}
	return path.Base(strings.TrimSpace(item.Path))
}

func usedIdentifiers(file *ast.File) map[string]bool {
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

func importKey(alias string, importPath string) string {
	return alias + "\x00" + importPath
}
