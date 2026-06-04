// Package project loads project-level compiler configuration.
package project

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"

	"github.com/gowdk/gowdk"
)

// DefaultConfigFile is the config file discovered from a project root.
const DefaultConfigFile = "gowdk.config.go"

// LoadConfigFile reads the supported static subset of gowdk.config.go.
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
				config, ok := parseConfigLiteral(valueSpec.Values[index])
				if !ok {
					return gowdk.Config{}, fmt.Errorf("%s must assign Config to a gowdk.Config literal", path)
				}
				return config, nil
			}
		}
	}
	return gowdk.Config{}, fmt.Errorf("%s missing Config variable", path)
}

// LoadOptionalConfig loads an explicitly requested config file, or the default
// config file when it exists. It reports whether a file was loaded.
func LoadOptionalConfig(path string) (gowdk.Config, bool, error) {
	if path != "" {
		config, err := LoadConfigFile(path)
		return config, true, err
	}
	if _, err := os.Stat(DefaultConfigFile); err != nil {
		if os.IsNotExist(err) {
			return gowdk.Config{}, false, nil
		}
		return gowdk.Config{}, false, err
	}
	config, err := LoadConfigFile(DefaultConfigFile)
	return config, true, err
}

func parseConfigLiteral(expression ast.Expr) (gowdk.Config, bool) {
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
		case "Source":
			config.Source = parseSourceConfig(keyValue.Value)
		case "Modules":
			config.Modules = parseModuleConfigs(keyValue.Value)
		case "Build":
			config.Build = parseBuildConfig(keyValue.Value)
		}
	}
	return config, true
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
		case "Stylesheets":
			build.Stylesheets = parseStylesheets(keyValue.Value)
		}
	}
	return build
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
