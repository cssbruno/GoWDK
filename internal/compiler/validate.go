package compiler

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/manifest"
)

var cssReferencePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]*$`)

// ValidationError is a compiler diagnostic that can be shown to users.
type ValidationError struct {
	Code          string
	PageID        string
	ComponentName string
	Source        string
	Span          manifest.SourceSpan
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
	diagnostics = append(diagnostics, validateUniqueLayouts(app.Layouts)...)
	diagnostics = append(diagnostics, validatePageLayoutReferences(app.Pages, app.Layouts)...)
	diagnostics = append(diagnostics, validateUniquePageRoutes(app.Pages)...)
	diagnostics = append(diagnostics, validateAmbiguousDynamicPageRoutes(app.Pages)...)
	diagnostics = append(diagnostics, validateRouteMethodConflicts(app.Pages)...)
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
	pageRoute, pageRouteIssues := parseRoute(page.Route)
	diagnostics = append(diagnostics, routeDiagnostics(page, "page route", pageRouteIssues, page.Spans.Route, page.Spans.RouteParams)...)
	for _, api := range page.Blocks.APIs {
		if api.Route == "" {
			continue
		}
		label := "api route"
		if api.Name != "" {
			label = fmt.Sprintf("api %s route", api.Name)
		}
		_, issues := parseRoute(api.Route)
		diagnostics = append(diagnostics, routeDiagnostics(page, label, issues, api.RouteSpan, api.RouteParams)...)
	}

	if mode.RequiresSSR() && !config.HasFeature(gowdk.FeatureSSR) {
		diagnostics = append(diagnostics, ValidationError{
			Code:   "missing_ssr_addon",
			PageID: page.ID,
			Source: page.Source,
			Span:   firstSpan(page.Spans.Render, page.Spans.Page),
			Message: fmt.Sprintf(
				"%s.page.gwdk uses @render %s, but the SSR addon is not enabled. Fix: enable ssr.Addon() in gowdk.config.go",
				page.ID,
				mode,
			),
		})
	}
	if mode == gowdk.Hybrid && !page.Blocks.Load {
		diagnostics = append(diagnostics, ValidationError{
			Code:   "hybrid_requires_explicit_request_policy",
			PageID: page.ID,
			Source: page.Source,
			Span:   firstSpan(page.Spans.Render, page.Spans.Page),
			Message: fmt.Sprintf(
				"%s uses @render hybrid, but no accepted request-time full-page policy is declared. Current hybrid pages must declare load {} so they do not become implicit SSR",
				page.ID,
			),
		})
	}

	if !page.Blocks.View {
		diagnostics = append(diagnostics, ValidationError{
			Code:   "missing_view_block",
			PageID: page.ID,
			Source: page.Source,
			Span:   firstSpan(page.Spans.Page, page.Spans.Route),
			Message: fmt.Sprintf(
				"%s declares a page route but is missing view {}. Current pages must render HTML for their GET route",
				page.ID,
			),
		})
	}

	var params []string
	if len(pageRouteIssues) == 0 {
		params = pageRoute.Params
	}
	if mode.IsBuildTime() && len(params) > 0 && !page.Paths {
		diagnostics = append(diagnostics, ValidationError{
			Code:   "static_dynamic_route_missing_paths",
			PageID: page.ID,
			Source: page.Source,
			Span:   firstNamedSpan(page.Spans.RouteParams, page.Spans.Route),
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
			Source: page.Source,
			Span:   firstSpan(page.Blocks.Spans.Load, page.Spans.Render, page.Spans.Page),
			Message: fmt.Sprintf(
				"%s declares load {}, but load runs at request time and requires @render ssr or @render hybrid",
				page.ID,
			),
		})
	}
	diagnostics = append(diagnostics, validatePageCSS(page)...)

	return diagnostics
}

func validatePageCSS(page manifest.Page) []ValidationError {
	if len(page.CSS) == 0 {
		return nil
	}
	if len(page.CSS) > 1 && containsCSSReference(page.CSS, "none") {
		return []ValidationError{{
			Code:   "invalid_css_selection",
			PageID: page.ID,
			Source: page.Source,
			Span:   spanForName(page.Spans.CSS, "none", page.Spans.Page),
			Message: fmt.Sprintf(
				"%s uses @css none with other CSS inputs. Fix: use @css none by itself or remove none",
				page.ID,
			),
		}}
	}

	seen := map[string]bool{}
	var diagnostics []ValidationError
	for _, name := range page.CSS {
		if !cssReferencePattern.MatchString(name) {
			diagnostics = append(diagnostics, ValidationError{
				Code:   "invalid_css_selection",
				PageID: page.ID,
				Source: page.Source,
				Span:   spanForName(page.Spans.CSS, name, page.Spans.Page),
				Message: fmt.Sprintf(
					"%s uses invalid @css input %q. CSS inputs must be identifiers such as default, page, forms, or blog.post",
					page.ID,
					name,
				),
			})
			continue
		}
		if seen[name] {
			diagnostics = append(diagnostics, ValidationError{
				Code:    "duplicate_css_selection",
				PageID:  page.ID,
				Source:  page.Source,
				Span:    spanForName(page.Spans.CSS, name, page.Spans.Page),
				Message: fmt.Sprintf("%s repeats @css input %q", page.ID, name),
			})
			continue
		}
		seen[name] = true
	}
	return diagnostics
}

func containsCSSReference(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func firstSpan(spans ...manifest.SourceSpan) manifest.SourceSpan {
	for _, span := range spans {
		if hasSpan(span) {
			return span
		}
	}
	return manifest.SourceSpan{}
}

func firstNamedSpan(spans []manifest.NamedSpan, fallback manifest.SourceSpan) manifest.SourceSpan {
	for _, item := range spans {
		if hasSpan(item.Span) {
			return item.Span
		}
	}
	return fallback
}

func spanForName(spans []manifest.NamedSpan, name string, fallback manifest.SourceSpan) manifest.SourceSpan {
	for _, item := range spans {
		if item.Name == name && hasSpan(item.Span) {
			return item.Span
		}
	}
	return fallback
}

func hasSpan(span manifest.SourceSpan) bool {
	return span.Start.Line > 0 && span.Start.Column > 0 && span.End.Line > 0 && span.End.Column > 0
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
