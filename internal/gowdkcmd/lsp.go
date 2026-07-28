package gowdkcmd

import (
	"errors"
	"os"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/addons/ssr"
	"github.com/cssbruno/gowdk/internal/lsp"
)

const lspUsage = "usage: gowdk lsp [--config <file>] [--project-root <dir>] [--module <name>] [--ssr]"

func languageServer(args []string) error {
	settings, err := loadLanguageServerSettings(args)
	if err != nil {
		return err
	}
	return lsp.NewProjectServer(settings.Config, lsp.ProjectOptions{
		Root:    settings.ProjectRoot,
		Modules: settings.Modules,
	}).Serve(os.Stdin, os.Stdout)
}

type languageServerSettings struct {
	Config      gowdk.Config
	ProjectRoot string
	Modules     []string
}

// languageServerConfig loads the project config for the language server the
// same way check does, so config-declared addons (for example SSR) are honored
// by editor diagnostics. When no config file exists and none was requested it
// falls back to the flag-only config instead of failing.
func languageServerConfig(args []string) (gowdk.Config, error) {
	settings, err := loadLanguageServerSettings(args)
	return settings.Config, err
}

func loadLanguageServerSettings(args []string) (languageServerSettings, error) {
	var options cliOptions
	var configPath string
	var moduleNames []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if value, next, ok, missing := consumeValueFlag(args, i, "--config", true); ok {
			if missing {
				return languageServerSettings{}, errors.New(lspUsage)
			}
			configPath = value
			i = next
			continue
		}
		if value, next, ok, missing := consumeValueFlag(args, i, "--project-root", true); ok {
			if missing {
				return languageServerSettings{}, errors.New(lspUsage)
			}
			options.ProjectRoot = value
			i = next
			continue
		}
		if value, next, ok, missing := consumeValueFlag(args, i, "--module", true); ok {
			if missing {
				return languageServerSettings{}, errors.New(lspUsage)
			}
			moduleNames = appendModuleNames(moduleNames, value)
			i = next
			continue
		}
		switch arg {
		case "--ssr":
			options.Config.Addons = append(options.Config.Addons, ssr.Addon())
		default:
			return languageServerSettings{}, errors.New(lspUsage)
		}
	}
	if strings.TrimSpace(configPath) == "" {
		if _, _, err := resolveProjectRoot(configPath, options.ProjectRoot, nil); err != nil {
			if strings.TrimSpace(options.ProjectRoot) == "" && strings.Contains(err.Error(), "gowdk.config.go is required") {
				if _, moduleErr := buildModules(options.Config.Modules, moduleNames); moduleErr != nil {
					return languageServerSettings{}, moduleErr
				}
				root, cwdErr := os.Getwd()
				if cwdErr != nil {
					return languageServerSettings{}, cwdErr
				}
				return languageServerSettings{
					Config:      options.Config,
					ProjectRoot: root,
					Modules:     moduleNames,
				}, nil
			}
			return languageServerSettings{}, err
		}
	}
	if err := loadProjectConfig(&options, configPath); err != nil {
		return languageServerSettings{}, err
	}
	if _, err := buildModules(options.Config.Modules, moduleNames); err != nil {
		return languageServerSettings{}, err
	}
	return languageServerSettings{
		Config:      options.Config,
		ProjectRoot: options.ProjectRoot,
		Modules:     moduleNames,
	}, nil
}
