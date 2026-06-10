package compiler

import (
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
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

// validateLayoutReferences checks a layout's own @layout parent declarations:
// a layout may not inherit from itself (layout_self_reference), may not form a
// cyclic inheritance chain (cyclic_layout_reference), and must reference
// layouts that exist (unknown_layout_id).
func validateLayoutReferences(layouts []gwdkir.Layout) []ValidationError {
	if len(layouts) == 0 {
		return nil
	}
	declared := map[string]gwdkir.Layout{}
	for _, layout := range layouts {
		if layout.ID != "" {
			declared[layoutIdentityKey(layout.Package, layout.ID)] = layout
		}
	}

	resolve := func(layout gwdkir.Layout, usesByAlias map[string]gwdkir.Use, ref string) (key string, known bool) {
		if alias, layoutID, ok := strings.Cut(ref, "."); ok {
			use, exists := usesByAlias[alias]
			if !exists {
				return ref, false
			}
			key = layoutIdentityKey(use.Package, layoutID)
			_, known = declared[key]
			return key, known
		}
		if layout.Package != "" {
			key = layoutIdentityKey(layout.Package, ref)
			if _, ok := declared[key]; ok {
				return key, true
			}
		}
		key = layoutIdentityKey("", ref)
		_, known = declared[key]
		return key, known
	}

	var diagnostics []ValidationError
	edges := map[string][]string{}
	for _, layout := range layouts {
		selfKey := layoutIdentityKey(layout.Package, layout.ID)
		usesByAlias := layoutUsesByAlias(layout)
		for index, ref := range layout.Layouts {
			key, known := resolve(layout, usesByAlias, ref)
			span := layoutRefSpan(layout, index)
			if key == selfKey {
				diagnostics = append(diagnostics, ValidationError{
					Code:   "layout_self_reference",
					Source: layout.Source,
					Span:   span,
					Message: fmt.Sprintf(
						"layout %s lists itself in @layout; a layout cannot inherit from itself (layout identity comes from the file name, not @layout)",
						layoutDisplayName(layout.Package, layout.ID),
					),
				})
				continue
			}
			if !known {
				diagnostics = append(diagnostics, ValidationError{
					Code:   "unknown_layout_id",
					Source: layout.Source,
					Span:   span,
					Message: fmt.Sprintf(
						"layout %s references parent layout %q, but no matching .layout.gwdk file declares it",
						layoutDisplayName(layout.Package, layout.ID),
						ref,
					),
				})
				continue
			}
			edges[selfKey] = append(edges[selfKey], key)
		}
	}
	return append(diagnostics, detectLayoutCycles(layouts, edges)...)
}

func detectLayoutCycles(layouts []gwdkir.Layout, edges map[string][]string) []ValidationError {
	bySelf := map[string]gwdkir.Layout{}
	for _, layout := range layouts {
		bySelf[layoutIdentityKey(layout.Package, layout.ID)] = layout
	}
	const (
		unvisited = 0
		active    = 1
		done      = 2
	)
	color := map[string]int{}
	reported := map[string]bool{}
	var diagnostics []ValidationError
	var dfs func(node string)
	dfs = func(node string) {
		color[node] = active
		for _, next := range edges[node] {
			switch color[next] {
			case active:
				if layout, ok := bySelf[next]; ok && !reported[next] {
					reported[next] = true
					diagnostics = append(diagnostics, ValidationError{
						Code:   "cyclic_layout_reference",
						Source: layout.Source,
						Span:   layout.Span,
						Message: fmt.Sprintf(
							"layout %s is part of a cyclic @layout inheritance chain",
							layoutDisplayName(layout.Package, layout.ID),
						),
					})
				}
			case unvisited:
				dfs(next)
			}
		}
		color[node] = done
	}
	for _, layout := range layouts {
		key := layoutIdentityKey(layout.Package, layout.ID)
		if color[key] == unvisited {
			dfs(key)
		}
	}
	return diagnostics
}

// validateLayoutSlots enforces that every layout contains exactly one
// `<slot />` placeholder, the spot where the page or inner layout is injected.
// This runs at validation time, so a slot-less layout is a hard error even if
// no page references it yet (composition would otherwise only catch it on use).
func validateLayoutSlots(layouts []gwdkir.Layout) []ValidationError {
	var diagnostics []ValidationError
	for _, layout := range layouts {
		count := countLayoutSlots(layout.Blocks.ViewBody)
		if count == 1 {
			continue
		}
		span := layout.Blocks.Spans.View
		if (span == source.SourceSpan{}) {
			span = layout.PackageSpan
		}
		diagnostics = append(diagnostics, ValidationError{
			Code:   "layout_slot_count",
			Source: layout.Source,
			Span:   span,
			Message: fmt.Sprintf(
				"layout %s must contain exactly one <slot /> placeholder for the page or inner layout, found %d",
				layoutDisplayName(layout.Package, layout.ID),
				count,
			),
		})
	}
	return diagnostics
}

// countLayoutSlots counts self-closing `<slot />` placeholders, mirroring the
// app-shell composition slot scan (whitespace tolerated, named slots ignored).
func countLayoutSlots(body string) int {
	isSpace := func(b byte) bool { return b == ' ' || b == '\t' || b == '\n' || b == '\r' }
	count := 0
	for index := 0; index < len(body); index++ {
		if body[index] != '<' || !strings.HasPrefix(body[index:], "<slot") {
			continue
		}
		cursor := index + len("<slot")
		for cursor < len(body) && isSpace(body[cursor]) {
			cursor++
		}
		if cursor >= len(body) || body[cursor] != '/' {
			continue
		}
		cursor++
		for cursor < len(body) && isSpace(body[cursor]) {
			cursor++
		}
		if cursor < len(body) && body[cursor] == '>' {
			count++
		}
	}
	return count
}

func layoutUsesByAlias(layout gwdkir.Layout) map[string]gwdkir.Use {
	usesByAlias := map[string]gwdkir.Use{}
	for _, use := range layout.Uses {
		if _, exists := usesByAlias[use.Alias]; !exists {
			usesByAlias[use.Alias] = use
		}
	}
	return usesByAlias
}

func layoutRefSpan(layout gwdkir.Layout, index int) source.SourceSpan {
	if index >= 0 && index < len(layout.LayoutSpans) {
		return layout.LayoutSpans[index].Span
	}
	return layout.Span
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
