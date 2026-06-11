package compiler

import (
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/view"
)

func validateGOWDKUses(app gwdkir.Program, crossFile bool) []ValidationError {
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
		usesByAlias := map[string]gwdkir.Use{}
		diagnostics = append(diagnostics, validateComponentUses(component, usesByAlias, sourcePackages, crossFile)...)
		diagnostics = append(diagnostics, validateComponentQualifiedComponentRefs(component, usesByAlias, componentPackages, componentByPackageName, sourcePackages, crossFile)...)
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
		usesByAlias := map[string]gwdkir.Use{}
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
			if crossFile && !sourcePackages[use.Package] {
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
		diagnostics = append(diagnostics, validatePageQualifiedComponentRefs(page, usesByAlias, componentPackages, componentByPackageName, sourcePackages, crossFile)...)
	}
	return diagnostics
}

func validateComponentUses(component gwdkir.Component, usesByAlias map[string]gwdkir.Use, sourcePackages map[string]bool, crossFile bool) []ValidationError {
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
		if crossFile && !sourcePackages[use.Package] {
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

func validatePageQualifiedComponentRefs(page gwdkir.Page, usesByAlias map[string]gwdkir.Use, componentPackages map[string]bool, componentByPackageName map[string]bool, sourcePackages map[string]bool, crossFile bool) []ValidationError {
	if !page.Blocks.View || strings.TrimSpace(page.Blocks.ViewBody) == "" {
		return nil
	}
	refs, err := view.ComponentReferenceSpans(page.Blocks.ViewBody)
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
		alias, name, ok := strings.Cut(ref.Name, ".")
		if !ok {
			continue
		}
		span := pageViewBodyOffsetSpan(page, ref.Start, ref.End)
		use, exists := usesByAlias[alias]
		if !exists {
			diagnostics = append(diagnostics, ValidationError{
				Code:   "unknown_gowdk_use_alias",
				PageID: page.ID,
				Source: page.Source,
				Span:   span,
				Message: fmt.Sprintf(
					"%s references component <%s />, but alias %q is not declared. Add `use %s \"<package>\"` before the view block",
					page.ID,
					ref.Name,
					alias,
					alias,
				),
			})
			continue
		}
		if !crossFile {
			continue
		}
		if !componentPackages[use.Package] {
			if sourcePackages[use.Package] {
				diagnostics = append(diagnostics, ValidationError{
					Code:   "unknown_gowdk_component",
					PageID: page.ID,
					Source: page.Source,
					Span:   span,
					Message: fmt.Sprintf(
						"%s references component <%s />, but package %s does not declare components",
						page.ID,
						ref.Name,
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
			Span:   span,
			Message: fmt.Sprintf(
				"%s references component <%s />, but package %s does not declare component %s",
				page.ID,
				ref.Name,
				use.Package,
				name,
			),
		})
	}
	return diagnostics
}

func validateComponentQualifiedComponentRefs(component gwdkir.Component, usesByAlias map[string]gwdkir.Use, componentPackages map[string]bool, componentByPackageName map[string]bool, sourcePackages map[string]bool, crossFile bool) []ValidationError {
	if !component.Blocks.View || strings.TrimSpace(component.Blocks.ViewBody) == "" {
		return nil
	}
	refs, err := view.ComponentReferenceSpans(component.Blocks.ViewBody)
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
		alias, name, ok := strings.Cut(ref.Name, ".")
		if !ok {
			continue
		}
		span := componentViewBodyOffsetSpan(component, ref.Start, ref.End)
		use, exists := usesByAlias[alias]
		if !exists {
			diagnostics = append(diagnostics, ValidationError{
				Code:          "unknown_gowdk_use_alias",
				ComponentName: component.Name,
				Source:        component.Source,
				Span:          span,
				Message: fmt.Sprintf(
					"component %s references component <%s />, but alias %q is not declared. Add `use %s \"<package>\"` before the view block",
					component.Name,
					ref.Name,
					alias,
					alias,
				),
			})
			continue
		}
		if !crossFile {
			continue
		}
		if !componentPackages[use.Package] {
			if sourcePackages[use.Package] {
				diagnostics = append(diagnostics, ValidationError{
					Code:          "unknown_gowdk_component",
					ComponentName: component.Name,
					Source:        component.Source,
					Span:          span,
					Message: fmt.Sprintf(
						"component %s references component <%s />, but package %s does not declare components",
						component.Name,
						ref.Name,
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
			Span:          span,
			Message: fmt.Sprintf(
				"component %s references component <%s />, but package %s does not declare component %s",
				component.Name,
				ref.Name,
				use.Package,
				name,
			),
		})
	}
	return diagnostics
}
