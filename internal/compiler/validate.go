package compiler

import (
	"fmt"
	"strings"

	"github.com/gowdk/gowdk"
	"github.com/gowdk/gowdk/internal/manifest"
)

// ValidationError is a compiler diagnostic that can be shown to users.
type ValidationError struct {
	Code          string
	PageID        string
	ComponentName string
	Source        string
	Message       string
}

func (err ValidationError) Error() string {
	if err.PageID == "" {
		if err.ComponentName != "" {
			return fmt.Sprintf("%s: %s", err.ComponentName, err.Message)
		}
		return err.Message
	}
	return fmt.Sprintf("%s: %s", err.PageID, err.Message)
}

// ValidateManifest checks render-mode invariants that must hold before codegen.
func ValidateManifest(config gowdk.Config, app manifest.Manifest) error {
	var diagnostics []ValidationError
	diagnostics = append(diagnostics, validateUniquePages(app.Pages)...)
	diagnostics = append(diagnostics, validateUniqueComponents(app.Components)...)
	for _, page := range app.Pages {
		diagnostics = append(diagnostics, ValidatePage(config, page)...)
	}
	if len(diagnostics) == 0 {
		return nil
	}
	return ValidationErrors(diagnostics)
}

func validateUniquePages(pages []manifest.Page) []ValidationError {
	seen := map[string]manifest.Page{}
	var diagnostics []ValidationError
	for _, page := range pages {
		if page.ID == "" {
			continue
		}
		first, exists := seen[page.ID]
		if !exists {
			seen[page.ID] = page
			continue
		}
		diagnostics = append(diagnostics, ValidationError{
			Code:   "duplicate_page_id",
			PageID: page.ID,
			Source: page.Source,
			Message: duplicateIdentityMessage(
				"page ID",
				page.ID,
				first.Source,
				page.Source,
			),
		})
	}
	return diagnostics
}

func validateUniqueComponents(components []manifest.Component) []ValidationError {
	seen := map[string]manifest.Component{}
	var diagnostics []ValidationError
	for _, component := range components {
		if component.Name == "" {
			continue
		}
		first, exists := seen[component.Name]
		if !exists {
			seen[component.Name] = component
			continue
		}
		diagnostics = append(diagnostics, ValidationError{
			Code:          "duplicate_component_name",
			ComponentName: component.Name,
			Source:        component.Source,
			Message: duplicateIdentityMessage(
				"component name",
				component.Name,
				first.Source,
				component.Source,
			),
		})
	}
	return diagnostics
}

func duplicateIdentityMessage(kind, value, firstSource, duplicateSource string) string {
	message := fmt.Sprintf("duplicate %s %q", kind, value)
	if firstSource != "" && duplicateSource != "" {
		return fmt.Sprintf("%s; first declared in %s and duplicated in %s", message, firstSource, duplicateSource)
	}
	return message
}

// ValidatePage checks one page for compile-first render mode rules.
func ValidatePage(config gowdk.Config, page manifest.Page) []ValidationError {
	mode := page.RenderMode(config.Render.DefaultMode())
	var diagnostics []ValidationError

	if mode.RequiresSSR() && !config.HasFeature(gowdk.FeatureSSR) {
		diagnostics = append(diagnostics, ValidationError{
			Code:   "missing_ssr_addon",
			PageID: page.ID,
			Message: fmt.Sprintf(
				"%s.page.gwdk uses @render %s, but the SSR addon is not enabled. Fix: enable ssr.Addon() in gowdk.config.go",
				page.ID,
				mode,
			),
		})
	}

	params := page.DynamicParams()
	if mode.IsBuildTime() && len(params) > 0 && !page.Paths {
		diagnostics = append(diagnostics, ValidationError{
			Code:   "static_dynamic_route_missing_paths",
			PageID: page.ID,
			Message: fmt.Sprintf(
				"%s has dynamic route params: {%s}, but render mode is %s and no paths block exists. Fix: add paths { ... } or use @render ssr",
				page.ID,
				strings.Join(params, ", "),
				mode,
			),
		})
	}

	if page.Blocks.Load && mode != gowdk.SSR && mode != gowdk.Hybrid {
		diagnostics = append(diagnostics, ValidationError{
			Code:   "load_requires_request_render",
			PageID: page.ID,
			Message: fmt.Sprintf(
				"%s declares load {}, but load runs at request time and requires @render ssr or @render hybrid",
				page.ID,
			),
		})
	}

	return diagnostics
}

// ValidationErrors is a set of compiler diagnostics.
type ValidationErrors []ValidationError

func (errs ValidationErrors) Error() string {
	lines := make([]string, 0, len(errs))
	for _, err := range errs {
		lines = append(lines, err.Error())
	}
	return strings.Join(lines, "\n")
}
