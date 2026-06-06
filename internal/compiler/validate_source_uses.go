package compiler

import (
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/internal/manifest"
	"github.com/cssbruno/gowdk/internal/view"
)

func validateGOWDKUses(app manifest.Manifest) []ValidationError {
	componentPackages := map[string]bool{}
	componentByPackageName := map[string]bool{}
	sourcePackages := map[string]bool{}
	for _, component := range app.Components {
		if component.Package == "" || component.Name == "" {
			continue
		}
		componentPackages[component.Package] = true
		sourcePackages[component.Package] = true
		componentByPackageName[component.Package+"."+component.Name] = true
	}
	for _, layout := range app.Layouts {
		if layout.Package == "" {
			continue
		}
		sourcePackages[layout.Package] = true
	}
	for _, page := range app.Pages {
		if page.Package == "" {
			continue
		}
		sourcePackages[page.Package] = true
	}

	var diagnostics []ValidationError
	for _, component := range app.Components {
		usesByAlias := map[string]manifest.Use{}
		diagnostics = append(diagnostics, validateComponentUses(component, usesByAlias, sourcePackages)...)
		diagnostics = append(diagnostics, validateComponentQualifiedComponentRefs(component, usesByAlias, componentPackages, componentByPackageName, sourcePackages)...)
	}
	for _, layout := range app.Layouts {
		for _, use := range layout.Uses {
			diagnostics = append(diagnostics, ValidationError{
				Code:   "unsupported_gowdk_use_scope",
				Source: layout.Source,
				Span:   use.Span,
				Message: fmt.Sprintf(
					"layout %s declares GOWDK use alias %q, but layouts do not support GOWDK use yet; pages and components support qualified component calls",
					layout.ID,
					use.Alias,
				),
			})
		}
	}
	for _, page := range app.Pages {
		usesByAlias := map[string]manifest.Use{}
		for _, use := range page.Uses {
			if first, exists := usesByAlias[use.Alias]; exists {
				diagnostics = append(diagnostics, ValidationError{
					Code:   "duplicate_gowdk_use_alias",
					PageID: page.ID,
					Source: page.Source,
					Span:   use.Span,
					Message: fmt.Sprintf(
						"%s declares duplicate GOWDK use alias %q; first declared at line %d",
						page.ID,
						use.Alias,
						first.Span.Start.Line,
					),
				})
				continue
			}
			usesByAlias[use.Alias] = use
			if !sourcePackages[use.Package] {
				diagnostics = append(diagnostics, ValidationError{
					Code:   "unknown_gowdk_use_package",
					PageID: page.ID,
					Source: page.Source,
					Span:   use.Span,
					Message: fmt.Sprintf(
						"%s uses GOWDK package %q as %s, but no discovered .gwdk component or layout file declares package %s",
						page.ID,
						use.Package,
						use.Alias,
						use.Package,
					),
				})
			}
		}
		diagnostics = append(diagnostics, validatePageQualifiedComponentRefs(page, usesByAlias, componentPackages, componentByPackageName, sourcePackages)...)
	}
	return diagnostics
}

func validateComponentUses(component manifest.Component, usesByAlias map[string]manifest.Use, sourcePackages map[string]bool) []ValidationError {
	var diagnostics []ValidationError
	for _, use := range component.Uses {
		if first, exists := usesByAlias[use.Alias]; exists {
			diagnostics = append(diagnostics, ValidationError{
				Code:          "duplicate_gowdk_use_alias",
				ComponentName: component.Name,
				Source:        component.Source,
				Span:          use.Span,
				Message: fmt.Sprintf(
					"component %s declares duplicate GOWDK use alias %q; first declared at line %d",
					component.Name,
					use.Alias,
					first.Span.Start.Line,
				),
			})
			continue
		}
		usesByAlias[use.Alias] = use
		if !sourcePackages[use.Package] {
			diagnostics = append(diagnostics, ValidationError{
				Code:          "unknown_gowdk_use_package",
				ComponentName: component.Name,
				Source:        component.Source,
				Span:          use.Span,
				Message: fmt.Sprintf(
					"component %s uses GOWDK package %q as %s, but no discovered .gwdk file declares package %s",
					component.Name,
					use.Package,
					use.Alias,
					use.Package,
				),
			})
		}
	}
	return diagnostics
}

func validatePageQualifiedComponentRefs(page manifest.Page, usesByAlias map[string]manifest.Use, componentPackages map[string]bool, componentByPackageName map[string]bool, sourcePackages map[string]bool) []ValidationError {
	if !page.Blocks.View || strings.TrimSpace(page.Blocks.ViewBody) == "" {
		return nil
	}
	refs, err := view.ComponentReferences(page.Blocks.ViewBody)
	if err != nil {
		return []ValidationError{{
			Code:    "view_parse_error",
			PageID:  page.ID,
			Source:  page.Source,
			Span:    firstSpan(page.Blocks.Spans.View, page.Spans.Page),
			Message: fmt.Sprintf("%s view cannot be parsed: %v", page.ID, err),
		}}
	}
	var diagnostics []ValidationError
	for _, ref := range refs {
		alias, name, ok := strings.Cut(ref, ".")
		if !ok {
			continue
		}
		use, exists := usesByAlias[alias]
		if !exists {
			diagnostics = append(diagnostics, ValidationError{
				Code:   "unknown_gowdk_use_alias",
				PageID: page.ID,
				Source: page.Source,
				Span:   firstSpan(page.Blocks.Spans.View, page.Spans.Page),
				Message: fmt.Sprintf(
					"%s references component <%s />, but alias %q is not declared. Add `use %s \"<package>\"` before the view block",
					page.ID,
					ref,
					alias,
					alias,
				),
			})
			continue
		}
		if !componentPackages[use.Package] {
			if sourcePackages[use.Package] {
				diagnostics = append(diagnostics, ValidationError{
					Code:   "unknown_gowdk_component",
					PageID: page.ID,
					Source: page.Source,
					Span:   firstSpan(page.Blocks.Spans.View, page.Spans.Page),
					Message: fmt.Sprintf(
						"%s references component <%s />, but package %s does not declare components",
						page.ID,
						ref,
						use.Package,
					),
				})
			}
			continue
		}
		if componentByPackageName[use.Package+"."+name] {
			continue
		}
		diagnostics = append(diagnostics, ValidationError{
			Code:   "unknown_gowdk_component",
			PageID: page.ID,
			Source: page.Source,
			Span:   firstSpan(page.Blocks.Spans.View, page.Spans.Page),
			Message: fmt.Sprintf(
				"%s references component <%s />, but package %s does not declare @component %s",
				page.ID,
				ref,
				use.Package,
				name,
			),
		})
	}
	return diagnostics
}

func validateComponentQualifiedComponentRefs(component manifest.Component, usesByAlias map[string]manifest.Use, componentPackages map[string]bool, componentByPackageName map[string]bool, sourcePackages map[string]bool) []ValidationError {
	if !component.Blocks.View || strings.TrimSpace(component.Blocks.ViewBody) == "" {
		return nil
	}
	refs, err := view.ComponentReferences(component.Blocks.ViewBody)
	if err != nil {
		return []ValidationError{{
			Code:          "view_parse_error",
			ComponentName: component.Name,
			Source:        component.Source,
			Span:          firstSpan(component.Blocks.Spans.View, component.Span),
			Message:       fmt.Sprintf("component %s view cannot be parsed: %v", component.Name, err),
		}}
	}
	var diagnostics []ValidationError
	for _, ref := range refs {
		alias, name, ok := strings.Cut(ref, ".")
		if !ok {
			continue
		}
		use, exists := usesByAlias[alias]
		if !exists {
			diagnostics = append(diagnostics, ValidationError{
				Code:          "unknown_gowdk_use_alias",
				ComponentName: component.Name,
				Source:        component.Source,
				Span:          firstSpan(component.Blocks.Spans.View, component.Span),
				Message: fmt.Sprintf(
					"component %s references component <%s />, but alias %q is not declared. Add `use %s \"<package>\"` before the view block",
					component.Name,
					ref,
					alias,
					alias,
				),
			})
			continue
		}
		if !componentPackages[use.Package] {
			if sourcePackages[use.Package] {
				diagnostics = append(diagnostics, ValidationError{
					Code:          "unknown_gowdk_component",
					ComponentName: component.Name,
					Source:        component.Source,
					Span:          firstSpan(component.Blocks.Spans.View, component.Span),
					Message: fmt.Sprintf(
						"component %s references component <%s />, but package %s does not declare components",
						component.Name,
						ref,
						use.Package,
					),
				})
			}
			continue
		}
		if componentByPackageName[use.Package+"."+name] {
			continue
		}
		diagnostics = append(diagnostics, ValidationError{
			Code:          "unknown_gowdk_component",
			ComponentName: component.Name,
			Source:        component.Source,
			Span:          firstSpan(component.Blocks.Spans.View, component.Span),
			Message: fmt.Sprintf(
				"component %s references component <%s />, but package %s does not declare @component %s",
				component.Name,
				ref,
				use.Package,
				name,
			),
		})
	}
	return diagnostics
}
