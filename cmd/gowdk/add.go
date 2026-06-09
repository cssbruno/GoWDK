package main

import (
	"bytes"
	"fmt"
	"go/ast"
	goformat "go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/cssbruno/gowdk/internal/project"
)

const addUsage = "usage: gowdk add <addon> [--config <file>] | gowdk add --list"

// addonSpec describes a built-in addon that `gowdk add` can wire into a
// project's gowdk.config.go. The selector package name is the import alias used
// in generated source; it matches the final path segment of ImportPath.
type addonSpec struct {
	Name       string
	ImportPath string
	Package    string
	Summary    string
}

// addonRegistry lists the addons `gowdk add <name>` understands. Each maps to a
// `<package>.Addon()` call appended to Config.Addons. Keep this sorted by name
// so `gowdk add --list` output is stable.
var addonRegistry = map[string]addonSpec{
	"actions": {
		Name:       "actions",
		ImportPath: "github.com/cssbruno/gowdk/addons/actions",
		Package:    "actions",
		Summary:    "backend form/action handlers",
	},
	"api": {
		Name:       "api",
		ImportPath: "github.com/cssbruno/gowdk/addons/api",
		Package:    "api",
		Summary:    "request-time API endpoints",
	},
	"auth": {
		Name:       "auth",
		ImportPath: "github.com/cssbruno/gowdk/addons/auth",
		Package:    "auth",
		Summary:    "batteries-included auth: PBKDF2, signed sessions, RBAC guards (no external deps)",
	},
	"contracts": {
		Name:       "contracts",
		ImportPath: "github.com/cssbruno/gowdk/addons/contracts",
		Package:    "contracts",
		Summary:    "contract-driven command/event metadata",
	},
	"css": {
		Name:       "css",
		ImportPath: "github.com/cssbruno/gowdk/addons/css",
		Package:    "css",
		Summary:    "build-time CSS processing",
	},
	"db": {
		Name:       "db",
		ImportPath: "github.com/cssbruno/gowdk/addons/db",
		Package:    "db",
		Summary:    "sqlc + database/sql plumbing helper (no domain, no driver dep)",
	},
	"embed": {
		Name:       "embed",
		ImportPath: "github.com/cssbruno/gowdk/addons/embed",
		Package:    "embed",
		Summary:    "embed build output into the binary",
	},
	"partial": {
		Name:       "partial",
		ImportPath: "github.com/cssbruno/gowdk/addons/partial",
		Package:    "partial",
		Summary:    "fragment/partial responses",
	},
	"ratelimit": {
		Name:       "ratelimit",
		ImportPath: "github.com/cssbruno/gowdk/addons/ratelimit",
		Package:    "ratelimit",
		Summary:    "request-time rate limiting",
	},
	"ssr": {
		Name:       "ssr",
		ImportPath: "github.com/cssbruno/gowdk/addons/ssr",
		Package:    "ssr",
		Summary:    "server-side rendering",
	},
}

func addAddon(args []string) error {
	configPath := project.DefaultConfigFile
	var names []string
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--list":
			return listAddons()
		case arg == "--config":
			if index+1 >= len(args) {
				return fmt.Errorf("%s\n--config requires a value", addUsage)
			}
			index++
			configPath = args[index]
		case strings.HasPrefix(arg, "--config="):
			configPath = strings.TrimPrefix(arg, "--config=")
		case strings.HasPrefix(arg, "-"):
			return fmt.Errorf("unknown add flag %q", arg)
		default:
			names = append(names, arg)
		}
	}

	if len(names) == 0 {
		return fmt.Errorf("%s\nrun `gowdk add --list` to see available addons", addUsage)
	}

	specs := make([]addonSpec, 0, len(names))
	for _, name := range names {
		spec, ok := addonRegistry[name]
		if !ok {
			return fmt.Errorf("unknown addon %q\nrun `gowdk add --list` to see available addons", name)
		}
		specs = append(specs, spec)
	}

	source, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s not found; run `gowdk init` first", configPath)
		}
		return err
	}

	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, configPath, source, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parse %s: %w", configPath, err)
	}

	configLit, ok := findConfigLiteral(file)
	if !ok {
		return fmt.Errorf("could not find `var Config = gowdk.Config{...}` in %s", configPath)
	}

	var added []string
	for _, spec := range specs {
		if addonPresent(configLit, file, spec) {
			fmt.Printf("addon %q already present\n", spec.Name)
			continue
		}
		ensureImport(file, spec.ImportPath)
		if err := appendAddon(configLit, spec); err != nil {
			return err
		}
		added = append(added, spec.Name)
	}

	if len(added) == 0 {
		return nil
	}

	formatted, err := formatFile(fileSet, file)
	if err != nil {
		return err
	}
	if err := os.WriteFile(configPath, formatted, 0o644); err != nil {
		return err
	}

	for _, name := range added {
		fmt.Printf("added addon %q to %s\n", name, configPath)
	}
	fmt.Println("Run: gowdk build")
	return nil
}

func listAddons() error {
	names := make([]string, 0, len(addonRegistry))
	for name := range addonRegistry {
		names = append(names, name)
	}
	sort.Strings(names)
	fmt.Println("Available addons:")
	for _, name := range names {
		fmt.Printf("  %-12s %s\n", name, addonRegistry[name].Summary)
	}
	return nil
}

// findConfigLiteral returns the composite literal for `var Config = ...`.
func findConfigLiteral(file *ast.File) (*ast.CompositeLit, bool) {
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.VAR {
			continue
		}
		for _, spec := range genDecl.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for index, name := range valueSpec.Names {
				if name.Name != "Config" || index >= len(valueSpec.Values) {
					continue
				}
				if literal, ok := valueSpec.Values[index].(*ast.CompositeLit); ok {
					return literal, true
				}
			}
		}
	}
	return nil, false
}

// addonPresent reports whether the config already wires in this addon, matching
// `<alias>.Addon()` where alias is imported with the addon's import path.
func addonPresent(configLit *ast.CompositeLit, file *ast.File, spec addonSpec) bool {
	field, ok := addonsField(configLit)
	if !ok {
		return false
	}
	literal, ok := field.Value.(*ast.CompositeLit)
	if !ok {
		return false
	}
	aliases := importAliases(file, spec.ImportPath)
	for _, element := range literal.Elts {
		call, ok := element.(*ast.CallExpr)
		if !ok {
			continue
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Addon" {
			continue
		}
		ident, ok := selector.X.(*ast.Ident)
		if !ok {
			continue
		}
		if aliases[ident.Name] {
			return true
		}
	}
	return false
}

// appendAddon adds `<package>.Addon()` to Config.Addons, creating the field
// when it does not yet exist.
func appendAddon(configLit *ast.CompositeLit, spec addonSpec) error {
	call := &ast.CallExpr{Fun: &ast.SelectorExpr{
		X:   ast.NewIdent(spec.Package),
		Sel: ast.NewIdent("Addon"),
	}}

	if field, ok := addonsField(configLit); ok {
		if literal, ok := field.Value.(*ast.CompositeLit); ok {
			literal.Elts = append(literal.Elts, call)
			return nil
		}
		return fmt.Errorf("Config.Addons must be a []gowdk.Addon literal")
	}

	configLit.Elts = append(configLit.Elts, &ast.KeyValueExpr{
		Key: ast.NewIdent("Addons"),
		Value: &ast.CompositeLit{
			Type: &ast.ArrayType{Elt: &ast.SelectorExpr{
				X:   ast.NewIdent("gowdk"),
				Sel: ast.NewIdent("Addon"),
			}},
			Elts: []ast.Expr{call},
		},
	})
	return nil
}

func addonsField(configLit *ast.CompositeLit) (*ast.KeyValueExpr, bool) {
	for _, element := range configLit.Elts {
		keyValue, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := keyValue.Key.(*ast.Ident)
		if ok && key.Name == "Addons" {
			return keyValue, true
		}
	}
	return nil, false
}

// importAliases returns the local names a path is imported under (the package
// name when unaliased), so addon detection works regardless of import alias.
func importAliases(file *ast.File, importPath string) map[string]bool {
	aliases := map[string]bool{}
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil || path != importPath {
			continue
		}
		if spec.Name != nil {
			aliases[spec.Name.Name] = true
			continue
		}
		aliases[importBaseName(path)] = true
	}
	return aliases
}

// ensureImport adds importPath to the file's first import block if absent.
func ensureImport(file *ast.File, importPath string) {
	for _, spec := range file.Imports {
		if path, err := strconv.Unquote(spec.Path.Value); err == nil && path == importPath {
			return
		}
	}

	newImport := &ast.ImportSpec{Path: &ast.BasicLit{
		Kind:  token.STRING,
		Value: strconv.Quote(importPath),
	}}

	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if ok && genDecl.Tok == token.IMPORT {
			genDecl.Specs = append(genDecl.Specs, newImport)
			file.Imports = append(file.Imports, newImport)
			return
		}
	}

	genDecl := &ast.GenDecl{Tok: token.IMPORT, Specs: []ast.Spec{newImport}}
	file.Decls = append([]ast.Decl{genDecl}, file.Decls...)
	file.Imports = append(file.Imports, newImport)
}

func importBaseName(importPath string) string {
	for index := len(importPath) - 1; index >= 0; index-- {
		if importPath[index] == '/' {
			return importPath[index+1:]
		}
	}
	return importPath
}

// formatFile prints and gofmt-formats the modified AST. It drops position
// information so go/printer re-lays-out the imports and the appended field
// cleanly rather than honoring stale source offsets.
func formatFile(fileSet *token.FileSet, file *ast.File) ([]byte, error) {
	var buffer bytes.Buffer
	config := printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 8}
	if err := config.Fprint(&buffer, fileSet, file); err != nil {
		return nil, err
	}
	formatted, err := goformat.Source(buffer.Bytes())
	if err != nil {
		return nil, fmt.Errorf("format updated config: %w", err)
	}
	return formatted, nil
}
