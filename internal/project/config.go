// Package project loads project-level compiler configuration.
package project

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"strconv"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/addons/ssr"
	"github.com/cssbruno/gowdk/addons/tailwind"
)

// DefaultConfigFile is the config file discovered from a project root.
const DefaultConfigFile = "gowdk.config.go"

// LoadConfigFile reads the supported SPA subset of gowdk.config.go.
func LoadConfigFile(path string) (gowdk.Config, error) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, path, nil, 0)
	if err != nil {
		return gowdk.Config{}, err
	}

	for _, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok || general.Tok != token.VAR {
			continue
		}
		for _, spec := range general.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for index, name := range valueSpec.Names {
				if name.Name != "Config" || index >= len(valueSpec.Values) {
					continue
				}
				config, ok := parseConfigLiteral(valueSpec.Values[index], importNames(file))
				if !ok {
					return gowdk.Config{}, fmt.Errorf("%s must assign Config to a gowdk.Config literal", path)
				}
				return config, nil
			}
		}
	}
	return gowdk.Config{}, fmt.Errorf("%s missing Config variable", path)
}

// LoadConfig loads an explicitly requested config file, or the required default
// config file when no path is provided.
func LoadConfig(path string) (gowdk.Config, error) {
	if path != "" {
		return LoadConfigFile(path)
	}
	if _, err := os.Stat(DefaultConfigFile); err != nil {
		if os.IsNotExist(err) {
			return gowdk.Config{}, fmt.Errorf("%s is required; run \"gowdk init\" or pass --config <file>", DefaultConfigFile)
		}
		return gowdk.Config{}, err
	}
	return LoadConfigFile(DefaultConfigFile)
}

func parseConfigLiteral(expression ast.Expr, imports map[string]string) (gowdk.Config, bool) {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok || !isConfigType(literal.Type) {
		return gowdk.Config{}, false
	}

	var config gowdk.Config
	for _, element := range literal.Elts {
		keyValue, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := keyValue.Key.(*ast.Ident)
		if !ok {
			continue
		}
		switch key.Name {
		case "AppName":
			config.AppName = parseString(keyValue.Value)
		case "Source":
			config.Source = parseSourceConfig(keyValue.Value)
		case "Modules":
			config.Modules = parseModuleConfigs(keyValue.Value)
		case "Build":
			config.Build = parseBuildConfig(keyValue.Value)
		case "CSS":
			config.CSS = parseCSSConfig(keyValue.Value)
		case "Render":
			config.Render = parseRenderConfig(keyValue.Value)
		case "Addons":
			config.Addons = parseAddons(keyValue.Value, imports)
		}
	}
	return config, true
}

func importNames(file *ast.File) map[string]string {
	imports := map[string]string{}
	for _, spec := range file.Imports {
		importPath := parseString(spec.Path)
		if importPath == "" {
			continue
		}
		name := path.Base(importPath)
		if spec.Name != nil && spec.Name.Name != "" && spec.Name.Name != "." && spec.Name.Name != "_" {
			name = spec.Name.Name
		}
		imports[name] = importPath
	}
	return imports
}

func parseSourceConfig(expression ast.Expr) gowdk.SourceConfig {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok {
		return gowdk.SourceConfig{}
	}

	var source gowdk.SourceConfig
	for _, element := range literal.Elts {
		keyValue, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := keyValue.Key.(*ast.Ident)
		if !ok {
			continue
		}
		switch key.Name {
		case "Include":
			source.Include = parseStringList(keyValue.Value)
		case "Exclude":
			source.Exclude = parseStringList(keyValue.Value)
		}
	}
	return source
}

func parseModuleConfigs(expression ast.Expr) []gowdk.ModuleConfig {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok {
		return nil
	}

	var modules []gowdk.ModuleConfig
	for _, element := range literal.Elts {
		module := parseModuleConfig(element)
		if module.Name == "" && module.Type == "" && len(module.Source.Include) == 0 && len(module.Source.Exclude) == 0 {
			continue
		}
		modules = append(modules, module)
	}
	return modules
}

func parseModuleConfig(expression ast.Expr) gowdk.ModuleConfig {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok {
		return gowdk.ModuleConfig{}
	}

	var module gowdk.ModuleConfig
	for _, element := range literal.Elts {
		keyValue, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := keyValue.Key.(*ast.Ident)
		if !ok {
			continue
		}
		switch key.Name {
		case "Name":
			module.Name = parseString(keyValue.Value)
		case "Type":
			module.Type = parseString(keyValue.Value)
		case "Source":
			module.Source = parseSourceConfig(keyValue.Value)
		}
	}
	return module
}

func parseRenderConfig(expression ast.Expr) gowdk.RenderConfig {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok {
		return gowdk.RenderConfig{}
	}

	var render gowdk.RenderConfig
	for _, element := range literal.Elts {
		keyValue, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := keyValue.Key.(*ast.Ident)
		if !ok || key.Name != "Default" {
			continue
		}
		render.Default = parseRenderMode(keyValue.Value)
	}
	return render
}

func parseRenderMode(expression ast.Expr) gowdk.RenderMode {
	if value := parseString(expression); value != "" {
		mode, err := gowdk.ParseRenderMode(value)
		if err == nil {
			return mode
		}
		return ""
	}
	switch typed := expression.(type) {
	case *ast.SelectorExpr:
		return renderModeByName(typed.Sel.Name)
	case *ast.Ident:
		return renderModeByName(typed.Name)
	default:
		return ""
	}
}

func renderModeByName(name string) gowdk.RenderMode {
	switch name {
	case "SPA":
		return gowdk.SPA
	case "Action":
		return gowdk.Action
	case "Hybrid":
		return gowdk.Hybrid
	case "SSR":
		return gowdk.SSR
	default:
		return ""
	}
}

func parseBuildConfig(expression ast.Expr) gowdk.BuildConfig {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok {
		return gowdk.BuildConfig{}
	}

	var build gowdk.BuildConfig
	for _, element := range literal.Elts {
		keyValue, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := keyValue.Key.(*ast.Ident)
		if !ok {
			continue
		}
		switch key.Name {
		case "Output":
			build.Output = parseString(keyValue.Value)
		case "Mode":
			build.Mode = parseBuildMode(keyValue.Value)
		case "Head":
			build.Head = parseHeadConfig(keyValue.Value)
		case "CSRF":
			build.CSRF = parseCSRFConfig(keyValue.Value)
		case "AllowMissingBackend":
			build.AllowMissingBackend = parseBool(keyValue.Value)
		case "Stylesheets":
			build.Stylesheets = parseStylesheets(keyValue.Value)
		case "Targets":
			build.Targets = parseBuildTargets(keyValue.Value)
		}
	}
	return build
}

func parseHeadConfig(expression ast.Expr) gowdk.HeadConfig {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok {
		return gowdk.HeadConfig{}
	}

	var head gowdk.HeadConfig
	for _, element := range literal.Elts {
		keyValue, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := keyValue.Key.(*ast.Ident)
		if !ok {
			continue
		}
		switch key.Name {
		case "SiteName":
			head.SiteName = parseString(keyValue.Value)
		case "Favicon":
			head.Favicon = parseString(keyValue.Value)
		case "Image":
			head.Image = parseString(keyValue.Value)
		case "TwitterCard":
			head.TwitterCard = parseString(keyValue.Value)
		}
	}
	return head
}

func parseCSRFConfig(expression ast.Expr) gowdk.CSRFConfig {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok {
		return gowdk.CSRFConfig{}
	}

	var csrf gowdk.CSRFConfig
	for _, element := range literal.Elts {
		keyValue, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := keyValue.Key.(*ast.Ident)
		if !ok {
			continue
		}
		switch key.Name {
		case "Enabled":
			csrf.Enabled = parseBool(keyValue.Value)
		case "SecretEnv":
			csrf.SecretEnv = parseString(keyValue.Value)
		case "CookieName":
			csrf.CookieName = parseString(keyValue.Value)
		case "FieldName":
			csrf.FieldName = parseString(keyValue.Value)
		case "HeaderName":
			csrf.HeaderName = parseString(keyValue.Value)
		case "Insecure":
			csrf.Insecure = parseBool(keyValue.Value)
		}
	}
	return csrf
}

func parseBuildMode(expression ast.Expr) gowdk.BuildMode {
	if value := parseString(expression); value != "" {
		switch gowdk.BuildMode(value) {
		case gowdk.Development, gowdk.Production:
			return gowdk.BuildMode(value)
		default:
			return ""
		}
	}
	switch typed := expression.(type) {
	case *ast.SelectorExpr:
		return buildModeByName(typed.Sel.Name)
	case *ast.Ident:
		return buildModeByName(typed.Name)
	default:
		return ""
	}
}

func buildModeByName(name string) gowdk.BuildMode {
	switch name {
	case "Development":
		return gowdk.Development
	case "Production":
		return gowdk.Production
	default:
		return ""
	}
}

func parseBuildTargets(expression ast.Expr) []gowdk.BuildTargetConfig {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok {
		return nil
	}

	var targets []gowdk.BuildTargetConfig
	for _, element := range literal.Elts {
		target := parseBuildTarget(element)
		if target.Name == "" && len(target.Modules) == 0 && target.Output == "" && target.App == "" && target.Binary == "" && target.WASM == "" && target.BackendApp == "" && target.BackendBinary == "" {
			continue
		}
		targets = append(targets, target)
	}
	return targets
}

func parseBuildTarget(expression ast.Expr) gowdk.BuildTargetConfig {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok {
		return gowdk.BuildTargetConfig{}
	}

	var target gowdk.BuildTargetConfig
	for _, element := range literal.Elts {
		keyValue, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := keyValue.Key.(*ast.Ident)
		if !ok {
			continue
		}
		switch key.Name {
		case "Name":
			target.Name = parseString(keyValue.Value)
		case "Modules":
			target.Modules = parseStringList(keyValue.Value)
		case "Output":
			target.Output = parseString(keyValue.Value)
		case "App":
			target.App = parseString(keyValue.Value)
		case "Binary":
			target.Binary = parseString(keyValue.Value)
		case "WASM", "Wasm":
			target.WASM = parseString(keyValue.Value)
		case "BackendApp":
			target.BackendApp = parseString(keyValue.Value)
		case "BackendBinary":
			target.BackendBinary = parseString(keyValue.Value)
		}
	}
	return target
}

func parseStylesheets(expression ast.Expr) []gowdk.Stylesheet {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok {
		return nil
	}

	var stylesheets []gowdk.Stylesheet
	for _, element := range literal.Elts {
		stylesheet := parseStylesheet(element)
		if stylesheet.Href == "" {
			continue
		}
		stylesheets = append(stylesheets, stylesheet)
	}
	return stylesheets
}

func parseStylesheet(expression ast.Expr) gowdk.Stylesheet {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok {
		return gowdk.Stylesheet{}
	}
	var stylesheet gowdk.Stylesheet
	for _, element := range literal.Elts {
		keyValue, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := keyValue.Key.(*ast.Ident)
		if !ok || key.Name != "Href" {
			continue
		}
		stylesheet.Href = parseString(keyValue.Value)
	}
	return stylesheet
}

func parseCSSConfig(expression ast.Expr) gowdk.CSSConfig {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok {
		return gowdk.CSSConfig{}
	}

	var css gowdk.CSSConfig
	for _, element := range literal.Elts {
		keyValue, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := keyValue.Key.(*ast.Ident)
		if !ok {
			continue
		}
		switch key.Name {
		case "Include":
			css.Include = parseStringList(keyValue.Value)
		case "Exclude":
			css.Exclude = parseStringList(keyValue.Value)
		case "Default":
			css.Default = parseStringList(keyValue.Value)
		case "Output":
			css.Output = parseCSSOutputConfig(keyValue.Value)
		}
	}
	return css
}

func parseCSSOutputConfig(expression ast.Expr) gowdk.CSSOutputConfig {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok {
		return gowdk.CSSOutputConfig{}
	}

	var output gowdk.CSSOutputConfig
	for _, element := range literal.Elts {
		keyValue, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := keyValue.Key.(*ast.Ident)
		if !ok {
			continue
		}
		switch key.Name {
		case "Dir":
			output.Dir = parseString(keyValue.Value)
		case "HrefPrefix":
			output.HrefPrefix = parseString(keyValue.Value)
		}
	}
	return output
}

func parseAddons(expression ast.Expr, imports map[string]string) []gowdk.Addon {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok {
		return nil
	}

	var addons []gowdk.Addon
	for _, element := range literal.Elts {
		if addon, ok := parseSSRAddon(element, imports); ok {
			addons = append(addons, addon)
			continue
		}
		if addon, ok := parseTailwindAddon(element, imports); ok {
			addons = append(addons, addon)
		}
	}
	return addons
}

func parseSSRAddon(expression ast.Expr, imports map[string]string) (gowdk.Addon, bool) {
	call, ok := expression.(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return nil, false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Addon" {
		return nil, false
	}
	packageName, ok := selector.X.(*ast.Ident)
	if !ok || imports[packageName.Name] != ssr.ImportPath {
		return nil, false
	}
	return ssr.Addon(), true
}

func parseTailwindAddon(expression ast.Expr, imports map[string]string) (gowdk.Addon, bool) {
	call, ok := expression.(*ast.CallExpr)
	if !ok {
		return nil, false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Addon" {
		return nil, false
	}
	packageName, ok := selector.X.(*ast.Ident)
	if !ok || imports[packageName.Name] != tailwind.ImportPath {
		return nil, false
	}

	var options tailwind.Options
	if len(call.Args) > 0 {
		options = parseTailwindOptions(call.Args[0], imports)
	}
	return tailwind.Addon(options), true
}

func parseTailwindOptions(expression ast.Expr, imports map[string]string) tailwind.Options {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok || !isTailwindOptionsType(literal.Type, imports) {
		return tailwind.Options{}
	}

	var options tailwind.Options
	for _, element := range literal.Elts {
		keyValue, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := keyValue.Key.(*ast.Ident)
		if !ok {
			continue
		}
		switch key.Name {
		case "Input":
			options.Input = parseString(keyValue.Value)
		case "OutputPath":
			options.OutputPath = parseString(keyValue.Value)
		case "Href":
			options.Href = parseString(keyValue.Value)
		case "Command":
			options.Command = parseString(keyValue.Value)
		case "Minify":
			options.Minify = parseBool(keyValue.Value)
		}
	}
	return options
}

func isTailwindOptionsType(expression ast.Expr, imports map[string]string) bool {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Options" {
		return false
	}
	packageName, ok := selector.X.(*ast.Ident)
	return ok && imports[packageName.Name] == tailwind.ImportPath
}

func parseStringList(expression ast.Expr) []string {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok {
		return nil
	}

	var values []string
	for _, element := range literal.Elts {
		value := parseString(element)
		if value == "" {
			continue
		}
		values = append(values, value)
	}
	return values
}

func parseString(expression ast.Expr) string {
	literal, ok := expression.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return ""
	}
	value, err := strconv.Unquote(literal.Value)
	if err != nil {
		return ""
	}
	return value
}

func parseBool(expression ast.Expr) bool {
	identifier, ok := expression.(*ast.Ident)
	return ok && identifier.Name == "true"
}

func isConfigType(expression ast.Expr) bool {
	switch typed := expression.(type) {
	case *ast.SelectorExpr:
		return typed.Sel.Name == "Config"
	case *ast.Ident:
		return typed.Name == "Config"
	default:
		return false
	}
}
