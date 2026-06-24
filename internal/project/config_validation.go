package project

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"

	"github.com/cssbruno/gowdk"
)

// LoadConfigFileStructural loads a project configuration while validating only
// schema and compiler-facing policy. Required runtime values are deliberately
// checked later by generated-app startup or ValidateRuntimeEnvironment.
func LoadConfigFileStructural(path string) (gowdk.Config, error) {
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
					config, err = loadExecutableConfig(path)
					if err != nil {
						return gowdk.Config{}, fmt.Errorf("%s contains config expressions outside the AST-only subset: %w", path, err)
					}
				}
				if err := validateLoadedConfigStructure(path, config); err != nil {
					return gowdk.Config{}, err
				}
				return config, nil
			}
		}
	}
	return gowdk.Config{}, fmt.Errorf("%s missing Config variable", path)
}

// LoadConfigStructural loads an explicit config or the required default config
// without requiring deployment environment values to exist in the CLI process.
func LoadConfigStructural(path string) (gowdk.Config, error) {
	if path != "" {
		return LoadConfigFileStructural(path)
	}
	if _, err := os.Stat(DefaultConfigFile); err != nil {
		if os.IsNotExist(err) {
			return gowdk.Config{}, fmt.Errorf("%s is required; run \"gowdk init\" or pass --config <file>", DefaultConfigFile)
		}
		return gowdk.Config{}, err
	}
	return LoadConfigFileStructural(DefaultConfigFile)
}

// ValidateRuntimeEnvironment checks required runtime variables and secret
// minimums against the provided value lookup. Passing nil performs only the
// structural checks used during config loading.
func ValidateRuntimeEnvironment(config gowdk.Config, lookup func(string) (string, bool)) error {
	return config.Env.Validate(lookup)
}

func validateLoadedConfigStructure(path string, config gowdk.Config) error {
	if err := config.Env.Validate(nil); err != nil {
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
