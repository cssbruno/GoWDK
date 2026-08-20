package gowdk

import (
	"fmt"
	"strings"
)

// ValidateStructural rejects invalid configuration states before compiler,
// generator, or runtime planning begins. It never reads process environment.
func (config Config) ValidateStructural() error {
	if err := config.Source.Validate("Source"); err != nil {
		return err
	}
	seenModules := map[string]bool{}
	for index, module := range config.Modules {
		name := strings.TrimSpace(module.Name)
		if name == "" {
			return fmt.Errorf("Modules[%d].Name is required", index)
		}
		if seenModules[name] {
			return fmt.Errorf("Modules[%d].Name %q is declared more than once", index, name)
		}
		seenModules[name] = true
		if err := module.Source.Validate(fmt.Sprintf("Modules[%d].Source", index)); err != nil {
			return err
		}
	}
	if err := config.Render.Validate(); err != nil {
		return err
	}
	if err := config.Env.Validate(nil); err != nil {
		return fmt.Errorf("env: %w", err)
	}
	if err := config.Lifecycle.Validate(); err != nil {
		return err
	}
	if err := config.Interop.Validate(); err != nil {
		return err
	}
	if err := config.I18N.Validate(); err != nil {
		return err
	}
	if err := config.Build.Validate(); err != nil {
		return err
	}
	if err := ValidateAddons(config.Addons); err != nil {
		return fmt.Errorf("addons: %w", err)
	}
	if err := ValidateExtensions(config.Extensions); err != nil {
		return fmt.Errorf("extensions: %w", err)
	}
	return nil
}

// Validate checks source include/exclude entries for silent empty values.
func (config SourceConfig) Validate(path string) error {
	for index, value := range config.Include {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s.Include[%d] must not be empty", path, index)
		}
	}
	for index, value := range config.Exclude {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s.Exclude[%d] must not be empty", path, index)
		}
	}
	return nil
}

// Validate checks the configured default rendering lane.
func (config RenderConfig) Validate() error {
	switch config.Default {
	case "", SPA, Hybrid, SSR:
		return nil
	default:
		return fmt.Errorf("Render.Default has unknown mode %q", config.Default)
	}
}

// Validate rejects build fields that would otherwise be ignored or normalized
// differently by separate command paths.
func (config BuildConfig) Validate() error {
	switch config.Mode {
	case "", Development, Production:
	default:
		return fmt.Errorf("Build.Mode has unknown value %q", config.Mode)
	}
	switch config.Assets {
	case "", AssetExternal, Embed:
	default:
		return fmt.Errorf("Build.Assets has unknown value %q", config.Assets)
	}
	if config.ObfuscateAssets && config.Mode != Production {
		return fmt.Errorf("Build.ObfuscateAssets requires Build.Mode = gowdk.Production")
	}
	if err := config.CORS.Validate(); err != nil {
		return err
	}
	if err := config.CSRF.Validate(); err != nil {
		return err
	}
	if !config.SecurityHeaders.Enabled && len(config.SecurityHeaders.Headers) > 0 {
		return fmt.Errorf("Build.SecurityHeaders.Headers requires Enabled = true")
	}
	if config.BodyLimits.ActionBytes < 0 {
		return fmt.Errorf("Build.BodyLimits.ActionBytes must not be negative")
	}
	if config.BodyLimits.APIBytes < 0 {
		return fmt.Errorf("Build.BodyLimits.APIBytes must not be negative")
	}
	seenTargets := map[string]bool{}
	for index, target := range config.Targets {
		if err := target.Validate(index, seenTargets); err != nil {
			return err
		}
	}
	return nil
}

// Validate checks one configured artifact target before command selection.
func (target BuildTargetConfig) Validate(index int, seen map[string]bool) error {
	name := strings.TrimSpace(target.Name)
	if name == "" {
		return fmt.Errorf("Build.Targets[%d].Name is required", index)
	}
	if seen[name] {
		return fmt.Errorf("Build.Targets[%d].Name %q is declared more than once", index, name)
	}
	seen[name] = true
	dependencies := []struct {
		artifact string
		owner    string
		label    string
	}{
		{target.Binary, target.App, "Binary requires App"},
		{target.WASM, target.App, "WASM requires App"},
		{target.BackendBinary, target.BackendApp, "BackendBinary requires BackendApp"},
		{target.WorkerBinary, target.WorkerApp, "WorkerBinary requires WorkerApp"},
		{target.CronBinary, target.CronApp, "CronBinary requires CronApp"},
	}
	for _, dependency := range dependencies {
		if strings.TrimSpace(dependency.artifact) != "" && strings.TrimSpace(dependency.owner) == "" {
			return fmt.Errorf("Build.Targets[%d].%s", index, dependency.label)
		}
	}
	for recipeIndex, recipe := range target.DeployRecipes {
		if strings.TrimSpace(recipe) == "" {
			return fmt.Errorf("Build.Targets[%d].DeployRecipes[%d] must not be empty", index, recipeIndex)
		}
	}
	return nil
}
