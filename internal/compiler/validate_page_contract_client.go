package compiler

import (
	"fmt"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/viewanalysis"
)

// validatePageContractClient warns when a request-time page declares a
// g:command write form that has no g:query region to refresh. Request-time
// (server {} / SSR / hybrid) pages now ship the small client runtime, so a
// command submit posts in the background and applies the single-flight region
// refresh the adapter names in X-GOWDK-Queries. But when the page renders no
// g:query region the command can refresh, the write is non-reactive: with
// JavaScript it only fires gowdk:command-success, and with JavaScript disabled a
// plain submit still navigates to the adapter's raw JSON. Surface that dead-end
// at build time so the author either adds a reactive read region or moves to a
// g:post action handler that returns a response.Response for a no-JS write path.
//
// The check is conservative: any g:query region on the page suppresses the
// warning, even though strict reactivity requires the command's domain events to
// invalidate that specific query. The edge-aware check needs the build's query
// invalidation graph, which ValidatePage does not receive; it is tracked as a
// follow-up.
func validatePageContractClient(page gwdkir.Page, mode gowdk.RenderMode) []ValidationError {
	if isBuildTimeRoute(mode, page) || !page.Blocks.View {
		return nil
	}
	refs, err := pageCommandReferences(page)
	if err != nil || len(refs) == 0 {
		return nil
	}
	if queries, err := pageQueryReferences(page); err == nil && len(queries) > 0 {
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
				"%s renders request-time HTML and declares g:command %q with no g:query region for it to refresh. The page ships the client runtime that posts the command in the background, but with no reactive read region the write only fires gowdk:command-success, and with client JavaScript disabled a plain submit navigates to the adapter's raw JSON. Bind a g:query region the command invalidates for reactivity, or use a g:post action handler that returns a response.Response (for example response.RedirectTo) for a no-JavaScript write path",
				page.ID,
				ref.Command,
			),
		})
	}
	return diagnostics
}

func pageQueryReferences(page gwdkir.Page) ([]viewanalysis.QueryReference, error) {
	return viewanalysis.QueryReferencesFromNodes(page.Blocks.ViewNodes)
}

func pageCommandReferences(page gwdkir.Page) ([]viewanalysis.CommandReference, error) {
	return viewanalysis.CommandReferencesFromNodes(page.Blocks.ViewNodes)
}
