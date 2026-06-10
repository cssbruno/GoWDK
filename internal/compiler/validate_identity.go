package compiler

import (
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/internal/gwdkir"
)

func validateUniquePages(pages []gwdkir.Page) []ValidationError {
	seen := map[string]gwdkir.Page{}
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
			Span:   page.Spans.Page,
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

func validateUniqueLayouts(layouts []gwdkir.Layout) []ValidationError {
	seen := map[string]gwdkir.Layout{}
	var diagnostics []ValidationError
	for _, layout := range layouts {
		if layout.ID == "" {
			continue
		}
		key := layoutIdentityKey(layout.Package, layout.ID)
		first, exists := seen[key]
		if !exists {
			seen[key] = layout
			continue
		}
		diagnostics = append(diagnostics, ValidationError{
			Code:   "duplicate_layout_id",
			Source: layout.Source,
			Span:   layout.Span,
			Message: duplicateIdentityMessage(
				"layout ID",
				layoutDisplayName(layout.Package, layout.ID),
				first.Source,
				layout.Source,
			),
		})
	}
	return diagnostics
}

func validatePageLayoutReferences(pages []gwdkir.Page, layouts []gwdkir.Layout) []ValidationError {
	if len(layouts) == 0 {
		return nil
	}
	declared := map[string]gwdkir.Layout{}
	for _, layout := range layouts {
		if layout.ID != "" {
			declared[layoutIdentityKey(layout.Package, layout.ID)] = layout
		}
	}
	var diagnostics []ValidationError
	for _, page := range pages {
		usesByAlias := pageUsesByAlias(page)
		for _, layoutRef := range page.Layouts {
			if alias, layoutID, ok := strings.Cut(layoutRef, "."); ok {
				use, exists := usesByAlias[alias]
				if !exists {
					diagnostics = append(diagnostics, ValidationError{
						Code:   "unknown_gowdk_use_alias",
						PageID: page.ID,
						Source: page.Source,
						Span:   spanForName(page.Spans.Layouts, layoutRef, page.Spans.Page),
						Message: fmt.Sprintf(
							"%s references layout %q, but alias %q is not declared. Add `use %s \"<package>\"` before @layout",
							page.ID,
							layoutRef,
							alias,
							alias,
						),
					})
					continue
				}
				if _, ok := declared[layoutIdentityKey(use.Package, layoutID)]; ok {
					continue
				}
				diagnostics = append(diagnostics, ValidationError{
					Code:   "unknown_layout_id",
					PageID: page.ID,
					Source: page.Source,
					Span:   spanForName(page.Spans.Layouts, layoutRef, page.Spans.Page),
					Message: fmt.Sprintf(
						"%s references layout %q through alias %q, but GOWDK package %s does not declare @layout %s",
						page.ID,
						layoutRef,
						alias,
						use.Package,
						layoutID,
					),
				})
				continue
			}
			if page.Package != "" {
				if _, ok := declared[layoutIdentityKey(page.Package, layoutRef)]; ok {
					continue
				}
			}
			if _, ok := declared[layoutIdentityKey("", layoutRef)]; ok {
				continue
			}
			diagnostics = append(diagnostics, ValidationError{
				Code:   "unknown_layout_id",
				PageID: page.ID,
				Source: page.Source,
				Span:   spanForName(page.Spans.Layouts, layoutRef, page.Spans.Page),
				Message: fmt.Sprintf(
					"%s references layout %q, but no same-package .layout.gwdk file declares @layout %s. For cross-package layouts, add `use alias \"package\"` and write `@layout alias.%s`",
					page.ID,
					layoutRef,
					layoutRef,
					layoutRef,
				),
			})
		}
	}
	return diagnostics
}

func pageUsesByAlias(page gwdkir.Page) map[string]gwdkir.Use {
	usesByAlias := map[string]gwdkir.Use{}
	for _, use := range page.Uses {
		if _, exists := usesByAlias[use.Alias]; !exists {
			usesByAlias[use.Alias] = use
		}
	}
	return usesByAlias
}

func layoutIdentityKey(packageName, layoutID string) string {
	if packageName == "" {
		return layoutID
	}
	return packageName + "." + layoutID
}

func layoutDisplayName(packageName, layoutID string) string {
	if packageName == "" {
		return layoutID
	}
	return packageName + "." + layoutID
}

func validateUniqueComponents(components []gwdkir.Component) []ValidationError {
	seen := map[string]gwdkir.Component{}
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
			Span:          component.Span,
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

func validateComponentEmits(components []gwdkir.Component) []ValidationError {
	var diagnostics []ValidationError
	for _, component := range components {
		seen := map[string]gwdkir.Emit{}
		for _, event := range component.Emits {
			if event.Name == "" {
				continue
			}
			first, exists := seen[event.Name]
			if !exists {
				seen[event.Name] = event
				continue
			}
			diagnostics = append(diagnostics, ValidationError{
				Code:          "duplicate_component_emit",
				ComponentName: component.Name,
				Source:        component.Source,
				Span:          event.Span,
				Message: fmt.Sprintf(
					"component %s declares duplicate emit %q; first declared at line %d and duplicated at line %d",
					component.Name,
					event.Name,
					first.Span.Start.Line,
					event.Span.Start.Line,
				),
			})
		}
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
