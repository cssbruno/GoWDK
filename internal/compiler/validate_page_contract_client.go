package compiler

import (
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/viewanalysis"
)

// validatePageContractClient warns when a request-time page declares a
// g:command write form. Generated SPA/static page output ships the small client
// runtime that turns a g:command submit into a fetch-and-apply, but request-time
// (server {} / SSR / hybrid) pages render live server data and ship no such
// client for the command write path. The generated command adapter answers with
// application/json, so a plain browser submit navigates to the adapter route and
// replaces the page with raw JSON. Surface this at build time instead of letting
// it break silently in the browser.
func validatePageContractClient(page gwdkir.Page, mode gowdk.RenderMode) []ValidationError {
	if isBuildTimeRoute(mode, page) || !page.Blocks.View {
		return nil
	}
	refs, err := pageCommandReferences(page)
	if err != nil || len(refs) == 0 {
		return nil
	}
	diagnostics := make([]ValidationError, 0, len(refs))
	for _, ref := range refs {
		diagnostics = append(diagnostics, ValidationError{
			Code:     "ssr_command_no_client",
			PageID:   page.ID,
			Source:   page.Source,
			Span:     pageViewBodyOffsetSpan(page, ref.Start, ref.End),
			Severity: SeverityWarning,
			Message: fmt.Sprintf(
				"%s renders request-time HTML and declares g:command %q, but the generated contract write adapter responds with application/json and the page ships no client to apply it. A plain form submit navigates to the adapter route and replaces the page with raw JSON. Use a g:post action handler that returns a response.Response (for example response.RedirectTo) so the write path works without client JavaScript",
				page.ID,
				ref.Command,
			),
		})
	}
	return diagnostics
}

func pageCommandReferences(page gwdkir.Page) ([]viewanalysis.CommandReference, error) {
	if len(page.Blocks.ViewNodes) > 0 {
		return viewanalysis.CommandReferencesFromNodes(page.Blocks.ViewNodes)
	}
	if strings.TrimSpace(page.Blocks.ViewBody) == "" {
		return nil, nil
	}
	return viewanalysis.CommandReferences(page.Blocks.ViewBody)
}
