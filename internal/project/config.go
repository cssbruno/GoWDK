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
	"time"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/addons/actions"
	"github.com/cssbruno/gowdk/addons/api"
	"github.com/cssbruno/gowdk/addons/auth"
	contractsaddon "github.com/cssbruno/gowdk/addons/contracts"
	"github.com/cssbruno/gowdk/addons/css"
	"github.com/cssbruno/gowdk/addons/db"
	"github.com/cssbruno/gowdk/addons/embed"
	"github.com/cssbruno/gowdk/addons/observability"
	"github.com/cssbruno/gowdk/addons/partial"
	"github.com/cssbruno/gowdk/addons/ratelimit"
	"github.com/cssbruno/gowdk/addons/realtime"
	"github.com/cssbruno/gowdk/addons/seo"
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
					if err := validateLoadedConfig(path, config); err != nil {
						return gowdk.Config{}, err
					}
					return config, nil
				}
				if err := validateLoadedConfig(path, config); err != nil {
					return gowdk.Config{}, err
				}
				return config, nil
			}
		}
	}
	return gowdk.Config{}, fmt.Errorf("%s missing Config variable", path)
}

func validateLoadedConfig(path string, config gowdk.Config) error {
	if err := config.Env.Validate(os.LookupEnv); err != nil {
		return fmt.Errorf("%s env contract: %w", path, err)
	}
	if err := config.Lifecycle.Validate(); err != nil {
		return fmt.Errorf("%s lifecycle contract: %w", path, err)
	}
	if err := config.I18N.Validate(); err != nil {
		return fmt.Errorf("%s i18n policy: %w", path, err)
	}
	if err := config.Build.CORS.Validate(); err != nil {
		return fmt.Errorf("%s CORS policy: %w", path, err)
	}
	return nil
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
	fields, ok := configLiteralFields(expression)
	if !ok {
		return gowdk.Config{}, false, false, nil
	}

	var config gowdk.Config
	var needsExecutableLoad bool
	for _, field := range fields {
		switch field.Name {
		case "AppName":
			if needsConfigExpressionEvaluation(field.Value) {
				needsExecutableLoad = true
				continue
			}
			config.AppName = parseString(field.Value)
		case "Source":
			if needsConfigExpressionEvaluation(field.Value) {
				needsExecutableLoad = true
				continue
			}
			config.Source = parseSourceConfig(field.Value)
		case "Modules":
			if needsConfigExpressionEvaluation(field.Value) {
				needsExecutableLoad = true
				continue
			}
			config.Modules = parseModuleConfigs(field.Value)
		case "Build":
			if needsConfigExpressionEvaluation(field.Value) {
				needsExecutableLoad = true
				continue
			}
			build, buildNeedsExecutableLoad := parseBuildConfig(field.Value)
			config.Build = build
			needsExecutableLoad = needsExecutableLoad || buildNeedsExecutableLoad
		case "CSS":
			if needsConfigExpressionEvaluation(field.Value) {
				needsExecutableLoad = true
				continue
			}
			config.CSS = parseCSSConfig(field.Value)
		case "Render":
			if needsConfigExpressionEvaluation(field.Value) {
				needsExecutableLoad = true
				continue
			}
			render, err := parseRenderConfig(field.Value)
			if err != nil {
				return gowdk.Config{}, false, false, err
			}
			config.Render = render
		case "I18N":
			if needsConfigExpressionEvaluation(field.Value) {
				needsExecutableLoad = true
				continue
			}
			config.I18N = parseI18NConfig(field.Value)
		case "Env":
			if needsConfigExpressionEvaluation(field.Value) {
				needsExecutableLoad = true
				continue
			}
			var err error
			config.Env, err = parseEnvConfig(field.Value)
			if err != nil {
				return gowdk.Config{}, false, false, err
			}
		case "Lifecycle":
			if needsConfigExpressionEvaluation(field.Value) {
				needsExecutableLoad = true
				continue
			}
			config.Lifecycle = parseLifecycleConfig(field.Value)
		case "Addons":
			addons, addonsNeedExecutableLoad := parseAddons(field.Value, imports)
			config.Addons = addons
			needsExecutableLoad = needsExecutableLoad || addonsNeedExecutableLoad
		default:
			return gowdk.Config{}, false, false, fmt.Errorf("unsupported Config field %q", field.Name)
		}
	}
	return config, needsExecutableLoad, true, nil
}

func supportedConfigLiteralFields() map[string]bool {
	return map[string]bool{
		"AppName":   true,
		"Source":    true,
		"Modules":   true,
		"Render":    true,
		"I18N":      true,
		"Env":       true,
		"Lifecycle": true,
		"Build":     true,
		"CSS":       true,
		"Addons":    true,
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

type configLiteralField struct {
	Name  string
	Value ast.Expr
}

func configLiteralFields(expression ast.Expr) ([]configLiteralField, bool) {
	return parseConfigLiteralFields(expression, false)
}

func strictConfigLiteralFields(expression ast.Expr) ([]configLiteralField, bool) {
	return parseConfigLiteralFields(expression, true)
}

func parseConfigLiteralFields(expression ast.Expr, strict bool) ([]configLiteralField, bool) {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok {
		return nil, false
	}
	fields := make([]configLiteralField, 0, len(literal.Elts))
	for _, element := range literal.Elts {
		keyValue, ok := element.(*ast.KeyValueExpr)
		if !ok {
			if strict {
				return nil, false
			}
			continue
		}
		key, ok := keyValue.Key.(*ast.Ident)
		if !ok {
			if strict {
				return nil, false
			}
			continue
		}
		fields = append(fields, configLiteralField{Name: key.Name, Value: keyValue.Value})
	}
	return fields, true
}

func parseSourceConfig(expression ast.Expr) gowdk.SourceConfig {
	fields, ok := configLiteralFields(expression)
	if !ok {
		return gowdk.SourceConfig{}
	}

	var source gowdk.SourceConfig
	for _, field := range fields {
		switch field.Name {
		case "Include":
			source.Include = parseStringList(field.Value)
		case "Exclude":
			source.Exclude = parseStringList(field.Value)
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
	fields, ok := configLiteralFields(expression)
	if !ok {
		return gowdk.ModuleConfig{}
	}

	var module gowdk.ModuleConfig
	for _, field := range fields {
		switch field.Name {
		case "Name":
			module.Name = parseString(field.Value)
		case "Type":
			module.Type = parseString(field.Value)
		case "Source":
			module.Source = parseSourceConfig(field.Value)
		}
	}
	return module
}

func parseRenderConfig(expression ast.Expr) (gowdk.RenderConfig, error) {
	fields, ok := configLiteralFields(expression)
	if !ok {
		return gowdk.RenderConfig{}, nil
	}

	var render gowdk.RenderConfig
	for _, field := range fields {
		if field.Name == "Default" {
			mode, err := parseRenderMode(field.Value)
			if err != nil {
				return gowdk.RenderConfig{}, err
			}
			render.Default = mode
		}
	}
	return render, nil
}

func parseI18NConfig(expression ast.Expr) gowdk.I18NConfig {
	fields, ok := configLiteralFields(expression)
	if !ok {
		return gowdk.I18NConfig{}
	}

	var config gowdk.I18NConfig
	for _, field := range fields {
		switch field.Name {
		case "Locales":
			config.Locales = parseLocaleConfigs(field.Value)
		case "DefaultLocale":
			config.DefaultLocale = parseString(field.Value)
		case "OmitDefaultPrefix":
			config.OmitDefaultPrefix = parseBool(field.Value)
		}
	}
	return config
}

func parseLocaleConfigs(expression ast.Expr) []gowdk.LocaleConfig {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok {
		return nil
	}
	locales := make([]gowdk.LocaleConfig, 0, len(literal.Elts))
	for _, element := range literal.Elts {
		locale, ok := parseLocaleConfig(element)
		if !ok {
			continue
		}
		if locale.Code == "" && locale.PathPrefix == "" && locale.Name == "" {
			continue
		}
		locales = append(locales, locale)
	}
	return locales
}

func parseLocaleConfig(expression ast.Expr) (gowdk.LocaleConfig, bool) {
	fields, ok := configLiteralFields(expression)
	if !ok {
		return gowdk.LocaleConfig{}, false
	}
	var locale gowdk.LocaleConfig
	for _, field := range fields {
		switch field.Name {
		case "Code":
			locale.Code = parseString(field.Value)
		case "PathPrefix":
			locale.PathPrefix = parseString(field.Value)
		case "Name":
			locale.Name = parseString(field.Value)
		}
	}
	return locale, true
}

func parseRenderMode(expression ast.Expr) (gowdk.RenderMode, error) {
	if value := parseString(expression); value != "" {
		mode, err := gowdk.ParseRenderMode(value)
		if err == nil {
			return mode, nil
		}
		return "", err
	}
	switch typed := expression.(type) {
	case *ast.SelectorExpr:
		return renderModeByName(typed.Sel.Name)
	case *ast.Ident:
		return renderModeByName(typed.Name)
	default:
		return "", fmt.Errorf("unsupported render mode expression")
	}
}

func renderModeByName(name string) (gowdk.RenderMode, error) {
	switch name {
	case "SPA":
		return gowdk.SPA, nil
	case "Hybrid":
		return gowdk.Hybrid, nil
	case "SSR":
		return gowdk.SSR, nil
	default:
		return "", fmt.Errorf("unknown render mode %q", name)
	}
}

func parseEnvConfig(expression ast.Expr) (gowdk.EnvConfig, error) {
	fields, ok := configLiteralFields(expression)
	if !ok {
		return gowdk.EnvConfig{}, nil
	}

	var env gowdk.EnvConfig
	for _, field := range fields {
		switch field.Name {
		case "Vars":
			env.Vars = parseEnvVars(field.Value)
		case "Secrets":
			secrets, err := parseSecretEnvs(field.Value)
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
	fields, ok := configLiteralFields(expression)
	if !ok {
		return gowdk.EnvVar{}, false
	}

	var variable gowdk.EnvVar
	for _, field := range fields {
		switch field.Name {
		case "Name":
			variable.Name = parseString(field.Value)
		case "Required":
			variable.Required = parseBool(field.Value)
		case "Default":
			variable.Default = parseString(field.Value)
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
	fields, ok := configLiteralFields(expression)
	if !ok {
		return gowdk.SecretEnv{}, false, nil
	}

	var secret gowdk.SecretEnv
	for _, field := range fields {
		switch field.Name {
		case "Name":
			secret.Name = parseString(field.Value)
		case "Required":
			secret.Required = parseBool(field.Value)
		case "MinBytes":
			secret.MinBytes = parseInt(field.Value)
		case "Default", "Value":
			return gowdk.SecretEnv{}, false, fmt.Errorf("Env.Secrets entries cannot declare %s; secret values must come from the runtime environment", field.Name)
		}
	}
	return secret, true, nil
}

func parseLifecycleConfig(expression ast.Expr) gowdk.LifecycleConfig {
	fields, ok := configLiteralFields(expression)
	if !ok {
		return gowdk.LifecycleConfig{}
	}

	var lifecycle gowdk.LifecycleConfig
	for _, field := range fields {
		if field.Name == "Services" {
			lifecycle.Services = parseServiceRefs(field.Value)
		}
	}
	return lifecycle
}

func parseServiceRefs(expression ast.Expr) []gowdk.ServiceRef {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok {
		return nil
	}

	services := make([]gowdk.ServiceRef, 0, len(literal.Elts))
	for _, element := range literal.Elts {
		service, ok := parseServiceRef(element)
		if !ok {
			continue
		}
		services = append(services, service)
	}
	return services
}

func parseServiceRef(expression ast.Expr) (gowdk.ServiceRef, bool) {
	fields, ok := configLiteralFields(expression)
	if !ok {
		return gowdk.ServiceRef{}, false
	}

	var service gowdk.ServiceRef
	for _, field := range fields {
		switch field.Name {
		case "ImportPath":
			service.ImportPath = parseString(field.Value)
		case "Function":
			service.Function = parseString(field.Value)
		}
	}
	return service, true
}

func parseBuildConfig(expression ast.Expr) (gowdk.BuildConfig, bool) {
	fields, ok := configLiteralFields(expression)
	if !ok {
		return gowdk.BuildConfig{}, false
	}

	var build gowdk.BuildConfig
	var needsExecutableLoad bool
	for _, field := range fields {
		switch field.Name {
		case "Output":
			build.Output = parseString(field.Value)
		case "Mode":
			build.Mode = parseBuildMode(field.Value)
		case "ObfuscateAssets":
			build.ObfuscateAssets = parseBool(field.Value)
		case "Head":
			build.Head = parseHeadConfig(field.Value)
		case "CSRF":
			build.CSRF = parseCSRFConfig(field.Value)
		case "CORS":
			var corsNeedsExecutableLoad bool
			build.CORS, corsNeedsExecutableLoad = parseCORSConfig(field.Value)
			needsExecutableLoad = needsExecutableLoad || corsNeedsExecutableLoad
		case "SecurityHeaders":
			build.SecurityHeaders = parseSecurityHeadersConfig(field.Value)
		case "BodyLimits":
			build.BodyLimits = parseBodyLimitsConfig(field.Value)
		case "AllowMissingBackend":
			build.AllowMissingBackend = parseBool(field.Value)
		case "Stylesheets":
			build.Stylesheets = parseStylesheets(field.Value)
		case "Scripts":
			build.Scripts = parseScripts(field.Value)
		case "Targets":
			build.Targets = parseBuildTargets(field.Value)
		}
	}
	return build, needsExecutableLoad
}

func parseCORSConfig(expression ast.Expr) (gowdk.CORSConfig, bool) {
	fields, ok := configLiteralFields(expression)
	if !ok {
		return gowdk.CORSConfig{}, false
	}

	var cors gowdk.CORSConfig
	var needsExecutableLoad bool
	for _, field := range fields {
		switch field.Name {
		case "Enabled":
			cors.Enabled = parseBool(field.Value)
		case "AllowedOrigins":
			var ok bool
			cors.AllowedOrigins, ok = parseStaticStringList(field.Value)
			needsExecutableLoad = needsExecutableLoad || !ok
		case "AllowedMethods":
			var ok bool
			cors.AllowedMethods, ok = parseStaticStringList(field.Value)
			needsExecutableLoad = needsExecutableLoad || !ok
		case "AllowedHeaders":
			var ok bool
			cors.AllowedHeaders, ok = parseStaticStringList(field.Value)
			needsExecutableLoad = needsExecutableLoad || !ok
		case "ExposedHeaders":
			var ok bool
			cors.ExposedHeaders, ok = parseStaticStringList(field.Value)
			needsExecutableLoad = needsExecutableLoad || !ok
		case "AllowCredentials":
			cors.AllowCredentials = parseBool(field.Value)
		case "MaxAgeSeconds":
			cors.MaxAgeSeconds = parseInt(field.Value)
		}
	}
	return cors, needsExecutableLoad
}

func parseSecurityHeadersConfig(expression ast.Expr) gowdk.SecurityHeadersConfig {
	fields, ok := configLiteralFields(expression)
	if !ok {
		return gowdk.SecurityHeadersConfig{}
	}

	var headers gowdk.SecurityHeadersConfig
	for _, field := range fields {
		switch field.Name {
		case "Enabled":
			headers.Enabled = parseBool(field.Value)
		case "Headers":
			headers.Headers = parseStringMap(field.Value)
		}
	}
	return headers
}

func parseBodyLimitsConfig(expression ast.Expr) gowdk.BodyLimitsConfig {
	fields, ok := configLiteralFields(expression)
	if !ok {
		return gowdk.BodyLimitsConfig{}
	}

	var limits gowdk.BodyLimitsConfig
	for _, field := range fields {
		switch field.Name {
		case "ActionBytes":
			limits.ActionBytes = parseInt64(field.Value)
		case "APIBytes":
			limits.APIBytes = parseInt64(field.Value)
		}
	}
	return limits
}

func parseHeadConfig(expression ast.Expr) gowdk.HeadConfig {
	fields, ok := configLiteralFields(expression)
	if !ok {
		return gowdk.HeadConfig{}
	}

	var head gowdk.HeadConfig
	for _, field := range fields {
		switch field.Name {
		case "SiteName":
			head.SiteName = parseString(field.Value)
		case "Favicon":
			head.Favicon = parseString(field.Value)
		case "Image":
			head.Image = parseString(field.Value)
		case "TwitterCard":
			head.TwitterCard = parseString(field.Value)
		}
	}
	return head
}

func parseCSRFConfig(expression ast.Expr) gowdk.CSRFConfig {
	fields, ok := configLiteralFields(expression)
	if !ok {
		return gowdk.CSRFConfig{}
	}

	var csrf gowdk.CSRFConfig
	for _, field := range fields {
		switch field.Name {
		case "Enabled":
			csrf.Enabled = parseBool(field.Value)
		case "Disabled":
			csrf.Disabled = parseBool(field.Value)
		case "SecretEnv":
			csrf.SecretEnv = parseString(field.Value)
		case "CookieName":
			csrf.CookieName = parseString(field.Value)
		case "FieldName":
			csrf.FieldName = parseString(field.Value)
		case "HeaderName":
			csrf.HeaderName = parseString(field.Value)
		case "Insecure":
			csrf.Insecure = parseBool(field.Value)
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
		if target.Name == "" && len(target.Modules) == 0 && target.Output == "" && target.App == "" && target.Binary == "" && target.WASM == "" && target.BackendApp == "" && target.BackendBinary == "" && len(target.DeployRecipes) == 0 {
			continue
		}
		targets = append(targets, target)
	}
	return targets
}

func parseBuildTarget(expression ast.Expr) gowdk.BuildTargetConfig {
	fields, ok := configLiteralFields(expression)
	if !ok {
		return gowdk.BuildTargetConfig{}
	}

	var target gowdk.BuildTargetConfig
	for _, field := range fields {
		switch field.Name {
		case "Name":
			target.Name = parseString(field.Value)
		case "Modules":
			target.Modules = parseStringList(field.Value)
		case "Output":
			target.Output = parseString(field.Value)
		case "App":
			target.App = parseString(field.Value)
		case "Binary":
			target.Binary = parseString(field.Value)
		case "WASM", "Wasm":
			target.WASM = parseString(field.Value)
		case "BackendApp":
			target.BackendApp = parseString(field.Value)
		case "BackendBinary":
			target.BackendBinary = parseString(field.Value)
		case "DeployRecipes":
			target.DeployRecipes = parseStringList(field.Value)
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
	fields, ok := configLiteralFields(expression)
	if !ok {
		return gowdk.Stylesheet{}
	}
	var stylesheet gowdk.Stylesheet
	for _, field := range fields {
		if field.Name == "Href" {
			stylesheet.Href = parseString(field.Value)
		}
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
	fields, ok := configLiteralFields(expression)
	if !ok {
		return gowdk.Script{}
	}
	var script gowdk.Script
	for _, field := range fields {
		switch field.Name {
		case "Src":
			script.Src = parseString(field.Value)
		case "Type":
			script.Type = parseString(field.Value)
		}
	}
	return script
}

func parseCSSConfig(expression ast.Expr) gowdk.CSSConfig {
	fields, ok := configLiteralFields(expression)
	if !ok {
		return gowdk.CSSConfig{}
	}

	var css gowdk.CSSConfig
	for _, field := range fields {
		switch field.Name {
		case "Include":
			css.Include = parseStringList(field.Value)
		case "Exclude":
			css.Exclude = parseStringList(field.Value)
		case "Default":
			css.Default = parseStringList(field.Value)
		case "Output":
			css.Output = parseCSSOutputConfig(field.Value)
		}
	}
	return css
}

func parseCSSOutputConfig(expression ast.Expr) gowdk.CSSOutputConfig {
	fields, ok := configLiteralFields(expression)
	if !ok {
		return gowdk.CSSOutputConfig{}
	}

	var output gowdk.CSSOutputConfig
	for _, field := range fields {
		switch field.Name {
		case "Dir":
			output.Dir = parseString(field.Value)
		case "HrefPrefix":
			output.HrefPrefix = parseString(field.Value)
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
		if addon, ok := parseAuthAddon(element, imports); ok {
			addons = append(addons, addon)
			continue
		}
		if addon, ok := parseSEOAddon(element, imports); ok {
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
	case observability.ImportPath:
		return observability.Addon(), true
	case partial.ImportPath:
		return partial.Addon(), true
	case ratelimit.ImportPath:
		return ratelimit.Addon(), true
	case realtime.ImportPath:
		return realtime.Addon(), true
	case seo.ImportPath:
		return seo.Addon(), true
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

func parseAuthAddon(expression ast.Expr, imports map[string]string) (gowdk.Addon, bool) {
	call, ok := expression.(*ast.CallExpr)
	if !ok {
		return nil, false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Addon" {
		return nil, false
	}
	packageName, ok := selector.X.(*ast.Ident)
	if !ok || imports[packageName.Name] != auth.ImportPath {
		return nil, false
	}
	if len(call.Args) > 1 {
		return nil, false
	}
	if len(call.Args) == 0 {
		return auth.Addon(), true
	}

	options, ok := parseAuthOptions(call.Args[0], imports)
	if !ok {
		return nil, false
	}
	return auth.Addon(options), true
}

func parseAuthOptions(expression ast.Expr, imports map[string]string) (auth.Options, bool) {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok || !isAuthOptionsType(literal.Type, imports) {
		return auth.Options{}, false
	}

	fields, ok := strictConfigLiteralFields(expression)
	if !ok {
		return auth.Options{}, false
	}
	var options auth.Options
	for _, field := range fields {
		switch field.Name {
		case "SecretEnv":
			value, ok := parseLiteralString(field.Value)
			if !ok {
				return auth.Options{}, false
			}
			options.SecretEnv = value
		case "CookieName":
			value, ok := parseLiteralString(field.Value)
			if !ok {
				return auth.Options{}, false
			}
			options.CookieName = value
		case "TTL":
			value, ok := parseDuration(field.Value, imports)
			if !ok {
				return auth.Options{}, false
			}
			options.TTL = value
		case "Insecure":
			value, ok := parseLiteralBool(field.Value)
			if !ok {
				return auth.Options{}, false
			}
			options.Insecure = value
		default:
			return auth.Options{}, false
		}
	}
	return options, true
}

func isAuthOptionsType(expression ast.Expr, imports map[string]string) bool {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Options" {
		return false
	}
	packageName, ok := selector.X.(*ast.Ident)
	return ok && imports[packageName.Name] == auth.ImportPath
}

func parseDuration(expression ast.Expr, imports map[string]string) (time.Duration, bool) {
	switch typed := expression.(type) {
	case *ast.BasicLit:
		if typed.Kind != token.INT {
			return 0, false
		}
		value, err := strconv.ParseInt(typed.Value, 10, 64)
		if err != nil {
			return 0, false
		}
		return time.Duration(value), true
	case *ast.SelectorExpr:
		return parseTimeDurationConstant(typed, imports)
	case *ast.BinaryExpr:
		if typed.Op != token.MUL {
			return 0, false
		}
		if multiplier, ok := parseDurationMultiplier(typed.X); ok {
			if unit, ok := parseDuration(typed.Y, imports); ok {
				return time.Duration(multiplier) * unit, true
			}
		}
		if multiplier, ok := parseDurationMultiplier(typed.Y); ok {
			if unit, ok := parseDuration(typed.X, imports); ok {
				return time.Duration(multiplier) * unit, true
			}
		}
	}
	return 0, false
}

func parseDurationMultiplier(expression ast.Expr) (int64, bool) {
	literal, ok := expression.(*ast.BasicLit)
	if !ok || literal.Kind != token.INT {
		return 0, false
	}
	value, err := strconv.ParseInt(literal.Value, 10, 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

func parseTimeDurationConstant(selector *ast.SelectorExpr, imports map[string]string) (time.Duration, bool) {
	packageName, ok := selector.X.(*ast.Ident)
	if !ok || imports[packageName.Name] != "time" {
		return 0, false
	}
	switch selector.Sel.Name {
	case "Nanosecond":
		return time.Nanosecond, true
	case "Microsecond":
		return time.Microsecond, true
	case "Millisecond":
		return time.Millisecond, true
	case "Second":
		return time.Second, true
	case "Minute":
		return time.Minute, true
	case "Hour":
		return time.Hour, true
	default:
		return 0, false
	}
}

func parseSEOAddon(expression ast.Expr, imports map[string]string) (gowdk.Addon, bool) {
	call, ok := expression.(*ast.CallExpr)
	if !ok {
		return nil, false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Addon" {
		return nil, false
	}
	packageName, ok := selector.X.(*ast.Ident)
	if !ok || imports[packageName.Name] != seo.ImportPath {
		return nil, false
	}
	if len(call.Args) > 1 {
		return nil, false
	}
	if len(call.Args) == 0 {
		return seo.Addon(), true
	}

	options, ok := parseSEOOptions(call.Args[0], imports)
	if !ok {
		return nil, false
	}
	return seo.Addon(options), true
}

func parseSEOOptions(expression ast.Expr, imports map[string]string) (seo.Options, bool) {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok || !isSEOOptionsType(literal.Type, imports) {
		return seo.Options{}, false
	}

	fields, ok := strictConfigLiteralFields(expression)
	if !ok {
		return seo.Options{}, false
	}
	var options seo.Options
	for _, field := range fields {
		switch field.Name {
		case "BaseURL":
			value, ok := parseLiteralString(field.Value)
			if !ok {
				return seo.Options{}, false
			}
			options.BaseURL = value
		case "Disallow":
			values, ok := parseLiteralStringList(field.Value)
			if !ok {
				return seo.Options{}, false
			}
			options.Disallow = values
		case "ExtraURLs":
			values, ok := parseSEOURLList(field.Value, imports)
			if !ok {
				return seo.Options{}, false
			}
			options.ExtraURLs = values
		default:
			return seo.Options{}, false
		}
	}
	return options, true
}

func isSEOOptionsType(expression ast.Expr, imports map[string]string) bool {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Options" {
		return false
	}
	packageName, ok := selector.X.(*ast.Ident)
	return ok && imports[packageName.Name] == seo.ImportPath
}

func parseSEOURLList(expression ast.Expr, imports map[string]string) ([]gowdk.SEOURL, bool) {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok {
		return nil, false
	}
	var urls []gowdk.SEOURL
	for _, element := range literal.Elts {
		url, ok := parseSEOURL(element, imports)
		if !ok {
			return nil, false
		}
		urls = append(urls, url)
	}
	return urls, true
}

func parseSEOURL(expression ast.Expr, imports map[string]string) (gowdk.SEOURL, bool) {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok || !isSEOURLType(literal.Type, imports) {
		return gowdk.SEOURL{}, false
	}

	fields, ok := strictConfigLiteralFields(expression)
	if !ok {
		return gowdk.SEOURL{}, false
	}
	var url gowdk.SEOURL
	for _, field := range fields {
		switch field.Name {
		case "Loc":
			value, ok := parseLiteralString(field.Value)
			if !ok {
				return gowdk.SEOURL{}, false
			}
			url.Loc = value
		case "LastMod":
			value, ok := parseLiteralString(field.Value)
			if !ok {
				return gowdk.SEOURL{}, false
			}
			url.LastMod = value
		case "ChangeFreq":
			value, ok := parseLiteralString(field.Value)
			if !ok {
				return gowdk.SEOURL{}, false
			}
			url.ChangeFreq = value
		case "Priority":
			value, ok := parseLiteralString(field.Value)
			if !ok {
				return gowdk.SEOURL{}, false
			}
			url.Priority = value
		default:
			return gowdk.SEOURL{}, false
		}
	}
	return url, true
}

func isSEOURLType(expression ast.Expr, imports map[string]string) bool {
	if expression == nil {
		return true
	}
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "URL" {
		return false
	}
	packageName, ok := selector.X.(*ast.Ident)
	return ok && imports[packageName.Name] == seo.ImportPath
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

	fields, ok := strictConfigLiteralFields(expression)
	if !ok {
		return tailwind.Options{}, false
	}
	var options tailwind.Options
	for _, field := range fields {
		switch field.Name {
		case "Input":
			options.Input = parseString(field.Value)
		case "OutputPath":
			options.OutputPath = parseString(field.Value)
		case "Href":
			options.Href = parseString(field.Value)
		case "Command":
			options.Command = parseString(field.Value)
		case "Minify":
			options.Minify = parseBool(field.Value)
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

func parseStaticStringList(expression ast.Expr) ([]string, bool) {
	if identifier, ok := expression.(*ast.Ident); ok && identifier.Name == "nil" {
		return nil, true
	}
	return parseLiteralStringList(expression)
}

func parseLiteralStringList(expression ast.Expr) ([]string, bool) {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok {
		return nil, false
	}

	var values []string
	for _, element := range literal.Elts {
		value, ok := parseLiteralString(element)
		if !ok {
			return nil, false
		}
		if value == "" {
			continue
		}
		values = append(values, value)
	}
	return values, true
}

func parseString(expression ast.Expr) string {
	value, ok := parseLiteralString(expression)
	if !ok {
		return ""
	}
	return value
}

func parseLiteralString(expression ast.Expr) (string, bool) {
	literal, ok := expression.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(literal.Value)
	if err != nil {
		return "", false
	}
	return value, true
}

func parseBool(expression ast.Expr) bool {
	value, ok := parseLiteralBool(expression)
	return ok && value
}

func parseLiteralBool(expression ast.Expr) (bool, bool) {
	identifier, ok := expression.(*ast.Ident)
	if !ok {
		return false, false
	}
	switch identifier.Name {
	case "true":
		return true, true
	case "false":
		return false, true
	default:
		return false, false
	}
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
