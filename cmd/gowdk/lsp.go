package main

import (
	"errors"
	"os"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/addons/ssr"
	"github.com/cssbruno/gowdk/internal/lsp"
	"github.com/cssbruno/gowdk/internal/project"
)

const lspUsage = "usage: gowdk lsp [--config <file>] [--ssr]"

func languageServer(args []string) error {
	config, err := languageServerConfig(args)
	if err != nil {
		return err
	}
	return lsp.NewServer(config).Serve(os.Stdin, os.Stdout)
}

// languageServerConfig loads the project config for the language server the
// same way check does, so config-declared addons (for example SSR) are honored
// by editor diagnostics. When no config file exists and none was requested it
// falls back to the flag-only config instead of failing.
func languageServerConfig(args []string) (gowdk.Config, error) {
	var options cliOptions
	var configPath string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if value, next, ok, missing := consumeValueFlag(args, i, "--config", true); ok {
			if missing {
				return gowdk.Config{}, errors.New(lspUsage)
			}
			configPath = value
			i = next
			continue
		}
		switch {
		case arg == "--ssr":
			options.Config.Addons = append(options.Config.Addons, ssr.Addon())
		default:
			return gowdk.Config{}, errors.New(lspUsage)
		}
	}
	if strings.TrimSpace(configPath) == "" {
		if _, err := os.Stat(project.DefaultConfigFile); err != nil {
			if os.IsNotExist(err) {
				return options.Config, nil
			}
			return gowdk.Config{}, err
		}
	}
	if err := loadProjectConfig(&options, configPath); err != nil {
		return gowdk.Config{}, err
	}
	return options.Config, nil
}
