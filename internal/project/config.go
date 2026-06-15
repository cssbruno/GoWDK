// Package project loads project-level compiler configuration.
package project

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"os"
	"path"
	"strconv"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/addons/actions"
	"github.com/cssbruno/gowdk/addons/api"
	"github.com/cssbruno/gowdk/addons/auth"
	contractsaddon "github.com/cssbruno/gowdk/addons/contracts"
	"github.com/cssbruno/gowdk/addons/css"
	"github.com/cssbruno/gowdk/addons/db"
	"github.com/cssbruno/gowdk/addons/embed"
	"github.com/cssbruno/gowdk/addons/partial"
	"github.com/cssbruno/gowdk/addons/ratelimit"
	"github.com/cssbruno/gowdk/addons/realtime"
	"github.com/cssbruno/gowdk/addons/spa"
	"github.com/cssbruno/gowdk/addons/ssr"
	"github.com/cssbruno/gowdk/addons/static"
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
				config, needsExecutableLoad, ok, err := parseConfigLiteral(valueSpec.Values[index], importNames(file))
				if err != nil {
					return gowdk.Config{}, err
				}
				if !ok {
					return gowdk.Config{}, fmt.Errorf("%s must assign Config to a gowdk.Config literal", path)
				}
				if needsExecutableLoad {
					config, err := loadExecutableConfig(path)
					if err != nil {
						return gowdk.Config{}, fmt.Errorf("%s contains config expressions outside the AST-only subset: %w", path, err)
					}
					if err := config.Env.Validate(os.LookupEnv); err != nil {
						return gowdk.Config{}, fmt.Errorf("%s env contract: %w", path, err)
					}
					return config, nil
				}
				if err := config.Env.Validate(os.LookupEnv); err != nil {
					return gowdk.Config{}, fmt.Errorf("%s env contract: %w", path, err)
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

func parseConfigLiteral(expression ast.Expr, imports map[string]string) (gowdk.Config, bool, bool, error) {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok || !isConfigType(literal.Type) {
		return gowdk.Config{}, false, false, nil
	}

	var config gowdk.Config
	var needsExecutableLoad bool
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
			if needsConfigExpressionEvaluation(keyValue.Value) {
				needsExecutableLoad = true
				continue
			}
			config.AppName = parseString(keyValue.Value)
		case "Source":
			if needsConfigExpressionEvaluation(keyValue.Value) {
				needsExecutableLoad = true
				continue
			}
			config.Source = parseSourceConfig(keyValue.Value)
		case "Modules":
			if needsConfigExpressionEvaluation(keyValue.Value) {
				needsExecutableLoad = true
				continue
			}
			config.Modules = parseModuleConfigs(keyValue.Value)
		case "Build":
			if needsConfigExpressionEvaluation(keyValue.Value) {
				needsExecutableLoad = true
				continue
			}
			config.Build = parseBuildConfig(keyValue.Value)
		case "CSS":
			if needsConfigExpressionEvaluation(keyValue.Value) {
				needsExecutableLoad = true
				continue
			}
			config.CSS = parseCSSConfig(keyValue.Value)
		case "Render":
			if needsConfigExpressionEvaluation(keyValue.Value) {
				needsExecutableLoad = true
				continue
			}
			config.Render = parseRenderConfig(keyValue.Value)
		case "Env":
			if needsConfigExpressionEvaluation(keyValue.Value) {
				needsExecutableLoad = true
				continue
			}
			var err error
			config.Env, err = parseEnvConfig(keyValue.Value)
			if err != nil {
				return gowdk.Config{}, false, false, err
			}
		case "Addons":
			addons, addonsNeedExecutableLoad := parseAddons(keyValue.Value, imports)
			config.Addons = addons
			needsExecutableLoad = needsExecutableLoad || addonsNeedExecutableLoad
		default:
			return gowdk.Config{}, false, false, fmt.Errorf("unsupported Config field %q", key.Name)
		}
	}
	return config, needsExecutableLoad, true, nil
}

func supportedConfigLiteralFields() map[string]bool {
	return map[string]bool{
		"AppName": true,
		"Source":  true,
		"Modules": true,
		"Render":  true,
		"Env":     true,
		"Build":   true,
		"CSS":     true,
		"Addons":  true,
	}
}

func needsConfigExpressionEvaluation(expression ast.Expr) bool {
	switch typed := expression.(type) {
	case *ast.BasicLit:
		return false
	case *ast.Ident:
		return typed.Name != "true" && typed.Name != "false" && typed.Name != "nil"
	case *ast.SelectorExpr:
		return false
	case *ast.UnaryExpr:
		return needsConfigExpressionEvaluation(typed.X)
	case *ast.CompositeLit:
		for _, element := range typed.Elts {
			if keyValue, ok := element.(*ast.KeyValueExpr); ok {
				if needsConfigExpressionEvaluation(keyValue.Value) {
					return true
				}
				continue
			}
			if needsConfigExpressionEvaluation(element) {
				return true
			}
		}
		return false
	default:
		return true
	}
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

func parseEnvConfig(expression ast.Expr) (gowdk.EnvConfig, error) {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok {
		return gowdk.EnvConfig{}, nil
	}

	var env gowdk.EnvConfig
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
		case "Vars":
			env.Vars = parseEnvVars(keyValue.Value)
		case "Secrets":
			secrets, err := parseSecretEnvs(keyValue.Value)
			if err != nil {
				return gowdk.EnvConfig{}, err
			}
			env.Secrets = secrets
		}
	}
	return env, nil
}

func parseEnvVars(expression ast.Expr) []gowdk.EnvVar {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok {
		return nil
	}

	var variables []gowdk.EnvVar
	for _, element := range literal.Elts {
		variable, ok := parseEnvVar(element)
		if !ok {
			continue
		}
		variables = append(variables, variable)
	}
	return variables
}

func parseEnvVar(expression ast.Expr) (gowdk.EnvVar, bool) {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok {
		return gowdk.EnvVar{}, false
	}

	var variable gowdk.EnvVar
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
			variable.Name = parseString(keyValue.Value)
		case "Required":
			variable.Required = parseBool(keyValue.Value)
		case "Default":
			variable.Default = parseString(keyValue.Value)
		}
	}
	return variable, true
}

func parseSecretEnvs(expression ast.Expr) ([]gowdk.SecretEnv, error) {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok {
		return nil, nil
	}

	var secrets []gowdk.SecretEnv
	for _, element := range literal.Elts {
		secret, ok, err := parseSecretEnv(element)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		secrets = append(secrets, secret)
	}
	return secrets, nil
}

func parseSecretEnv(expression ast.Expr) (gowdk.SecretEnv, bool, error) {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok {
		return gowdk.SecretEnv{}, false, nil
	}

	var secret gowdk.SecretEnv
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
			secret.Name = parseString(keyValue.Value)
		case "Required":
			secret.Required = parseBool(keyValue.Value)
		case "MinBytes":
			secret.MinBytes = parseInt(keyValue.Value)
		case "Default", "Value":
			return gowdk.SecretEnv{}, false, fmt.Errorf("Env.Secrets entries cannot declare %s; secret values must come from the runtime environment", key.Name)
		}
	}
	return secret, true, nil
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
		case "SecurityHeaders":
			build.SecurityHeaders = parseSecurityHeadersConfig(keyValue.Value)
		case "BodyLimits":
			build.BodyLimits = parseBodyLimitsConfig(keyValue.Value)
		case "AllowMissingBackend":
			build.AllowMissingBackend = parseBool(keyValue.Value)
		case "Stylesheets":
			build.Stylesheets = parseStylesheets(keyValue.Value)
		case "Scripts":
			build.Scripts = parseScripts(keyValue.Value)
		case "Targets":
			build.Targets = parseBuildTargets(keyValue.Value)
		}
	}
	return build
}

func parseSecurityHeadersConfig(expression ast.Expr) gowdk.SecurityHeadersConfig {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok {
		return gowdk.SecurityHeadersConfig{}
	}

	var headers gowdk.SecurityHeadersConfig
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
			headers.Enabled = parseBool(keyValue.Value)
		case "Headers":
			headers.Headers = parseStringMap(keyValue.Value)
		}
	}
	return headers
}

func parseBodyLimitsConfig(expression ast.Expr) gowdk.BodyLimitsConfig {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok {
		return gowdk.BodyLimitsConfig{}
	}

	var limits gowdk.BodyLimitsConfig
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
		case "ActionBytes":
			limits.ActionBytes = parseInt64(keyValue.Value)
		case "APIBytes":
			limits.APIBytes = parseInt64(keyValue.Value)
		}
	}
	return limits
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
		case "Disabled":
			csrf.Disabled = parseBool(keyValue.Value)
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

func parseStringMap(expression ast.Expr) map[string]string {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok {
		return nil
	}
	values := map[string]string{}
	for _, element := range literal.Elts {
		keyValue, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key := parseString(keyValue.Key)
		value := parseString(keyValue.Value)
		if key == "" {
			continue
		}
		values[key] = value
	}
	if len(values) == 0 {
		return nil
	}
	return values
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

func parseScripts(expression ast.Expr) []gowdk.Script {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok {
		return nil
	}

	var scripts []gowdk.Script
	for _, element := range literal.Elts {
		script := parseScript(element)
		if script.Src == "" {
			continue
		}
		scripts = append(scripts, script)
	}
	return scripts
}

func parseScript(expression ast.Expr) gowdk.Script {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok {
		return gowdk.Script{}
	}
	var script gowdk.Script
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
		case "Src":
			script.Src = parseString(keyValue.Value)
		case "Type":
			script.Type = parseString(keyValue.Value)
		}
	}
	return script
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

func parseAddons(expression ast.Expr, imports map[string]string) ([]gowdk.Addon, bool) {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok {
		return nil, false
	}

	var addons []gowdk.Addon
	var needsExecutableLoad bool
	for _, element := range literal.Elts {
		if addon, ok := parseBuiltInAddon(element, imports); ok {
			addons = append(addons, addon)
			continue
		}
		if addon, ok := parseTailwindAddon(element, imports); ok {
			addons = append(addons, addon)
			continue
		}
		needsExecutableLoad = true
	}
	return addons, needsExecutableLoad
}

func parseBuiltInAddon(expression ast.Expr, imports map[string]string) (gowdk.Addon, bool) {
	call, ok := expression.(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return nil, false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Addon" {
		return nil, false
	}
	packageName, ok := selector.X.(*ast.Ident)
	if !ok {
		return nil, false
	}
	switch imports[packageName.Name] {
	case actions.ImportPath:
		return actions.Addon(), true
	case api.ImportPath:
		return api.Addon(), true
	case auth.ImportPath:
		return auth.Addon(), true
	case contractsaddon.ImportPath:
		return contractsaddon.Addon(), true
	case css.ImportPath:
		return css.Addon(), true
	case db.ImportPath:
		return db.Addon(), true
	case embed.ImportPath:
		return embed.Addon(), true
	case partial.ImportPath:
		return partial.Addon(), true
	case ratelimit.ImportPath:
		return ratelimit.Addon(), true
	case realtime.ImportPath:
		return realtime.Addon(), true
	case spa.ImportPath:
		return spa.Addon(), true
	case ssr.ImportPath:
		return ssr.Addon(), true
	case static.ImportPath:
		return static.Addon(), true
	default:
		return nil, false
	}
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
	if len(call.Args) > 1 {
		return nil, false
	}

	var options tailwind.Options
	if len(call.Args) > 0 {
		var ok bool
		options, ok = parseTailwindOptions(call.Args[0], imports)
		if !ok {
			return nil, false
		}
	}
	return tailwind.Addon(options), true
}

func parseTailwindOptions(expression ast.Expr, imports map[string]string) (tailwind.Options, bool) {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok || !isTailwindOptionsType(literal.Type, imports) {
		return tailwind.Options{}, false
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
		default:
			return tailwind.Options{}, false
		}
	}
	return options, true
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

// parseInt narrows a parsed integer literal to int, clamping values that fall
// outside the platform int range so a malformed config value cannot wrap into a
// small or negative length on 32-bit platforms.
func parseInt(expression ast.Expr) int {
	value := parseInt64(expression)
	switch {
	case value > math.MaxInt:
		return math.MaxInt
	case value < math.MinInt:
		return math.MinInt
	default:
		return int(value)
	}
}

func parseInt64(expression ast.Expr) int64 {
	switch typed := expression.(type) {
	case *ast.BasicLit:
		if typed.Kind != token.INT {
			return 0
		}
		value, err := strconv.ParseInt(typed.Value, 0, 64)
		if err != nil {
			return 0
		}
		return value
	case *ast.UnaryExpr:
		value := parseInt64(typed.X)
		switch typed.Op {
		case token.ADD:
			return value
		case token.SUB:
			return -value
		default:
			return 0
		}
	default:
		return 0
	}
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
