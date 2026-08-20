package compiler

import (
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/viewparse"
)

func validateDirectiveLanes(program gwdkir.Program) []ValidationError {
	var diagnostics []ValidationError
	for _, page := range program.Pages {
		serverRoots := map[string]bool{}
		for _, field := range page.Blocks.ServerFields {
			serverRoots[exprRoot(field)] = true
		}
		serverPaths := map[string]bool{}
		for _, directive := range page.Blocks.DirectiveLanes {
			if directive.Lane == "" {
				continue
			}
			resolved := "client"
			if parentDirectiveLaneIsServer(directive.Path, serverPaths) || serverRoots[directiveExpressionRoot(directive)] {
				resolved = "server"
				serverPaths[directive.Path] = true
			}
			if directive.Lane != resolved {
				diagnostics = append(diagnostics, ValidationError{
					Code:    "directive_lane_mismatch",
					PageID:  page.ID,
					Source:  page.Source,
					Span:    page.Blocks.Spans.View,
					Message: fmt.Sprintf("%s declares %s g:lane=%q, but expression %q resolves to the %s lane", page.ID, directive.Directive, directive.Lane, directive.Expression, resolved),
				})
			}
		}
	}
	for _, component := range program.Components {
		for _, directive := range component.Blocks.DirectiveLanes {
			if directive.Lane == "" {
				continue
			}
			if directive.Lane != "client" {
				diagnostics = append(diagnostics, ValidationError{Code: "directive_lane_mismatch", ComponentName: component.Name, Source: component.Source, Span: component.Blocks.Spans.View, Message: fmt.Sprintf("component %s %s must use g:lane=\"client\"; components do not own request-time server data", component.Name, directive.Directive)})
			}
		}
	}
	for _, layout := range program.Layouts {
		for _, directive := range layout.Blocks.DirectiveLanes {
			if directive.Lane == "" {
				continue
			}
			if directive.Lane != "client" {
				diagnostics = append(diagnostics, ValidationError{Code: "directive_lane_mismatch", Source: layout.Source, Span: layout.Blocks.Spans.View, Message: fmt.Sprintf("layout %s %s must use g:lane=\"client\"; layouts do not own request-time server data", layout.ID, directive.Directive)})
			}
		}
	}
	return diagnostics
}

func directiveExpressionRoot(directive gwdkir.DirectiveLane) string {
	expression := strings.TrimSpace(directive.Expression)
	if directive.Directive == "g:for" {
		if parsed, err := viewparse.ParseForDirective(expression); err == nil {
			expression = parsed.Collection
		}
	}
	expression = strings.TrimSpace(strings.TrimPrefix(expression, "!"))
	return exprRoot(expression)
}

func parentDirectiveLaneIsServer(path string, serverPaths map[string]bool) bool {
	for parent := path; strings.Contains(parent, "."); {
		parent = parent[:strings.LastIndex(parent, ".")]
		if serverPaths[parent] {
			return true
		}
	}
	return false
}
