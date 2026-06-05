package compiler

import (
	"fmt"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gotypes"
	"github.com/cssbruno/gowdk/internal/manifest"
	"regexp"
	"strings"
)

var cssReferencePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]*$`)

func ValidatePage(config gowdk.Config, page manifest.Page) []ValidationError {
	mode := page.RenderMode(config.Render.DefaultMode())
	var diagnostics []ValidationError
	pageRoute, pageRouteIssues := parseRoute(page.Route)
	diagnostics = append(diagnostics, routeDiagnostics(page, "page route", pageRouteIssues, page.Spans.Route, page.Spans.RouteParams)...)
	diagnostics = append(diagnostics, validatePageStores(page)...)
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
			Code:   "spa_dynamic_route_missing_paths",
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

func validatePageStores(page manifest.Page) []ValidationError {
	seen := map[string]manifest.Store{}
	var diagnostics []ValidationError
	for _, store := range page.Stores {
		if first, exists := seen[store.Name]; exists {
			diagnostics = append(diagnostics, ValidationError{
				Code:   "duplicate_page_store",
				PageID: page.ID,
				Source: page.Source,
				Span:   store.Span,
				Message: fmt.Sprintf(
					"%s declares duplicate store %q; first declared at line %d and duplicated at line %d",
					page.ID,
					store.Name,
					first.Span.Start.Line,
					store.Span.Start.Line,
				),
			})
			continue
		}
		seen[store.Name] = store
		if _, err := gotypes.ResolveStruct(page.Imports, store.Type); err != nil {
			diagnostics = append(diagnostics, ValidationError{
				Code:    "page_store_error",
				PageID:  page.ID,
				Source:  page.Source,
				Span:    firstSpan(store.Type.Span, store.Span, page.Spans.Page),
				Message: fmt.Sprintf("page %s store %q type is invalid: %v", page.ID, store.Name, err),
			})
			continue
		}
		if err := gotypes.ValidateStateInit(page.Imports, manifest.StateContract{Type: store.Type, Init: store.Init, Span: store.Span}); err != nil {
			diagnostics = append(diagnostics, ValidationError{
				Code:    "page_store_error",
				PageID:  page.ID,
				Source:  page.Source,
				Span:    firstSpan(store.Init.Span, store.Span, page.Spans.Page),
				Message: fmt.Sprintf("page %s store %q init is invalid: %v", page.ID, store.Name, err),
			})
		}
	}
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
