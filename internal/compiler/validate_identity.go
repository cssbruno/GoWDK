package compiler

import (
	"fmt"
	"github.com/cssbruno/gowdk/internal/manifest"
)

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

func validateUniqueLayouts(layouts []manifest.Layout) []ValidationError {
	seen := map[string]manifest.Layout{}
	var diagnostics []ValidationError
	for _, layout := range layouts {
		if layout.ID == "" {
			continue
		}
		first, exists := seen[layout.ID]
		if !exists {
			seen[layout.ID] = layout
			continue
		}
		diagnostics = append(diagnostics, ValidationError{
			Code:   "duplicate_layout_id",
			Source: layout.Source,
			Span:   layout.Span,
			Message: duplicateIdentityMessage(
				"layout ID",
				layout.ID,
				first.Source,
				layout.Source,
			),
		})
	}
	return diagnostics
}

func validatePageLayoutReferences(pages []manifest.Page, layouts []manifest.Layout) []ValidationError {
	if len(layouts) == 0 {
		return nil
	}
	declared := map[string]bool{}
	for _, layout := range layouts {
		if layout.ID != "" {
			declared[layout.ID] = true
		}
	}
	var diagnostics []ValidationError
	for _, page := range pages {
		for _, layoutID := range page.Layouts {
			if declared[layoutID] {
				continue
			}
			diagnostics = append(diagnostics, ValidationError{
				Code:   "unknown_layout_id",
				PageID: page.ID,
				Source: page.Source,
				Span:   spanForName(page.Spans.Layouts, layoutID, page.Spans.Page),
				Message: fmt.Sprintf(
					"%s references layout %q, but no .layout.gwdk file declares @layout %s",
					page.ID,
					layoutID,
					layoutID,
				),
			})
		}
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

func validateComponentEmits(components []manifest.Component) []ValidationError {
	var diagnostics []ValidationError
	for _, component := range components {
		seen := map[string]manifest.Emit{}
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
