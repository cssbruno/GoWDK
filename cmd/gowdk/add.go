package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	goformat "go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/cssbruno/gowdk/internal/addonregistry"
	"github.com/cssbruno/gowdk/internal/project"
)

const addUsage = "usage: gowdk add <addon> [--config <file>] [--base-url <url>] | gowdk add --list [--registry] [--json]"

// addonSpec describes a built-in addon that `gowdk add` can wire into a
// project's gowdk.config.go. The selector package name is the import alias used
// in generated source; it matches the final path segment of ImportPath.
type addonSpec struct {
	Name       string
	ImportPath string
	Package    string
	Summary    string
	Options    string
}

type addCommandOptions struct {
	SEOBaseURL string
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
	"observability": {
		Name:       "observability",
		ImportPath: "github.com/cssbruno/gowdk/addons/observability",
		Package:    "observability",
		Summary:    "generated app tracing, local collector, and trace viewer",
	},
	"ratelimit": {
		Name:       "ratelimit",
		ImportPath: "github.com/cssbruno/gowdk/addons/ratelimit",
		Package:    "ratelimit",
		Summary:    "request-time rate limiting",
	},
	"seo": {
		Name:       "seo",
		ImportPath: "github.com/cssbruno/gowdk/addons/seo",
		Package:    "seo",
		Summary:    "build-time sitemap.xml and robots.txt output",
		Options:    "Options",
	},
	"realtime": {
		Name:       "realtime",
		ImportPath: "github.com/cssbruno/gowdk/addons/realtime",
		Package:    "realtime",
		Summary:    "browser presentation-event fanout over SSE or WebSocket",
	},
	"ssr": {
		Name:       "ssr",
		ImportPath: "github.com/cssbruno/gowdk/addons/ssr",
		Package:    "ssr",
		Summary:    "server-side rendering",
	},
	"static": {
		Name:       "static",
		ImportPath: "github.com/cssbruno/gowdk/addons/static",
		Package:    "static",
		Summary:    "build-time static page output",
	},
}

func addAddon(args []string) error {
	configPath := project.DefaultConfigFile
	var options addCommandOptions
	var names []string
	list := false
	registryList := false
	jsonOutput := false
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if value, next, ok, missing := consumeValueFlag(args, index, "--config", true); ok {
			if missing {
				return fmt.Errorf("%s\n--config requires a value", addUsage)
			}
			configPath = value
			index = next
			continue
		}
		if value, next, ok, missing := consumeValueFlag(args, index, "--base-url", true); ok {
			if missing || value == "" {
				return fmt.Errorf("%s\n--base-url requires a value", addUsage)
			}
			options.SEOBaseURL = value
			index = next
			continue
		}
		switch {
		case arg == "--list":
			list = true
		case arg == "--registry":
			registryList = true
		case arg == "--json":
			jsonOutput = true
		case strings.HasPrefix(arg, "-"):
			return fmt.Errorf("unknown add flag %q", arg)
		default:
			names = append(names, arg)
		}
	}

	if list {
		if len(names) > 0 {
			return fmt.Errorf("%s\n--list cannot be combined with addon names", addUsage)
		}
		return listAddons(registryList, jsonOutput)
	}
	if registryList {
		return fmt.Errorf("%s\n--registry is only supported with --list", addUsage)
	}
	if jsonOutput {
		return fmt.Errorf("%s\n--json is only supported with --list", addUsage)
	}
	if len(names) == 0 {
		return fmt.Errorf("%s\nrun `gowdk add --list` to see available addons", addUsage)
	}

	specs := make([]addonSpec, 0, len(names))
	addsSEO := false
	for _, name := range names {
		spec, ok := addonRegistry[name]
		if !ok {
			return fmt.Errorf("unknown addon %q\nrun `gowdk add --list` to see available addons", name)
		}
		if spec.Name == "seo" {
			addsSEO = true
		}
		specs = append(specs, spec)
	}
	if options.SEOBaseURL != "" && !addsSEO {
		return fmt.Errorf("--base-url is only supported with addon %q", "seo")
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
		if spec.Name == "seo" {
			if err := validateSEOBaseURLForAdd(options.SEOBaseURL); err != nil {
				return err
			}
		}
		ensureImport(file, spec.ImportPath)
		if err := appendAddon(configLit, spec, options); err != nil {
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

func listAddons(registryList bool, jsonOutput bool) error {
	registry, err := addonregistry.Bundled()
	if err != nil {
		return err
	}
	if registryList {
		if jsonOutput {
			payload, err := json.MarshalIndent(registry, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(payload))
			return nil
		}
		writeAddonRegistryList(os.Stdout, registry.Addons)
		return nil
	}
	if jsonOutput {
		var addable []addonregistry.Entry
		for _, entry := range registry.Addons {
			if entry.Constructor.Addable {
				addable = append(addable, entry)
			}
		}
		payload, err := json.MarshalIndent(addable, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(payload))
		return nil
	}
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

func writeAddonRegistryList(writer io.Writer, entries []addonregistry.Entry) {
	fmt.Fprintln(writer, "Addon registry (metadata only; external installation stays explicit):")
	fmt.Fprintf(writer, "  %-14s %-19s %-12s %-13s %-5s %s\n", "NAME", "KIND", "LIFECYCLE", "COMPAT", "ADD", "SUMMARY")
	for _, entry := range entries {
		addable := "no"
		if entry.Constructor.Addable {
			addable = "yes"
		}
		fmt.Fprintf(writer, "  %-14s %-19s %-12s %-13s %-5s %s\n", entry.Name, entry.Kind, entry.Lifecycle, entry.Compatibility, addable, entry.Summary)
	}
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
func appendAddon(configLit *ast.CompositeLit, spec addonSpec, options addCommandOptions) error {
	call := &ast.CallExpr{Fun: &ast.SelectorExpr{
		X:   ast.NewIdent(spec.Package),
		Sel: ast.NewIdent("Addon"),
	}}
	if spec.Options != "" {
		call.Args = []ast.Expr{&ast.CompositeLit{Type: &ast.SelectorExpr{
			X:   ast.NewIdent(spec.Package),
			Sel: ast.NewIdent(spec.Options),
		}}}
		if spec.Name == "seo" {
			call.Args[0].(*ast.CompositeLit).Elts = []ast.Expr{
				&ast.KeyValueExpr{
					Key: ast.NewIdent("BaseURL"),
					Value: &ast.BasicLit{
						Kind:  token.STRING,
						Value: strconv.Quote(strings.TrimSpace(options.SEOBaseURL)),
					},
				},
			}
		}
	}

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

func validateSEOBaseURLForAdd(raw string) error {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fmt.Errorf("gowdk add seo requires --base-url <url>")
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("invalid --base-url %q: %w", raw, err)
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("--base-url must be an absolute http or https URL")
	}
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
