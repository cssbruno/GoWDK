package compiler

import (
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/viewmodel"
	"github.com/cssbruno/gowdk/internal/viewparse"
)

// validatePageServerLists mirrors, at check time, the server-lane rules the
// renderer enforces at build time so the diagnostics match. In the lane model
// g:for/g:if render server-side when their operand is a server {} request-time
// field (or, when nested, the enclosing row item); this validates those
// server-lane uses. A g:for/g:if over client state/store is the client lane and
// is validated by the island validator, not here.
func validatePageServerLists(page gwdkir.Page) []ValidationError {
	nodes := pageViewNodes(page)
	if len(nodes) == 0 {
		return nil
	}
	loads := collectPageLoads(page)
	var diagnostics []ValidationError
	walkPageListNodes(page, nodes, loads, nil, &diagnostics)
	return diagnostics
}

func pageViewNodes(page gwdkir.Page) []viewmodel.Node {
	if len(page.Blocks.ViewNodes) > 0 {
		return page.Blocks.ViewNodes
	}
	if strings.TrimSpace(page.Blocks.ViewBody) == "" {
		return nil
	}
	nodes, err := viewparse.Parse(page.Blocks.ViewBody)
	if err != nil {
		return nil
	}
	return nodes
}

// pageLoads describes a page's declared server {} fields. fields holds the exact
// declared paths (e.g. "columns", "user.name"); roots holds the leading
// identifiers (e.g. "user"). The renderer resolves a top-level region's lane by
// the operand's root, so root membership decides whether a g:for/g:if is the
// server lane.
type pageLoads struct {
	fields map[string]bool
	roots  map[string]bool
}

func collectPageLoads(page gwdkir.Page) pageLoads {
	loads := pageLoads{fields: map[string]bool{}, roots: map[string]bool{}}
	if !page.Blocks.Server {
		return loads
	}
	for _, line := range strings.Split(page.Blocks.ServerBody, "\n") {
		_, body, ok := strings.Cut(line, "=>")
		if !ok {
			continue
		}
		body = strings.TrimSpace(body)
		body = strings.TrimPrefix(body, "{")
		body = strings.TrimSuffix(body, "}")
		for _, field := range strings.Split(body, ",") {
			field = strings.TrimSpace(field)
			if field == "" {
				continue
			}
			loads.fields[field] = true
			loads.roots[exprRoot(field)] = true
		}
	}
	return loads
}

func walkPageListNodes(page gwdkir.Page, nodes []viewmodel.Node, loads pageLoads, eachVars []string, diagnostics *[]ValidationError) {
	for _, node := range nodes {
		element, ok := node.(viewmodel.Element)
		if !ok {
			if call, isCall := node.(viewmodel.ComponentCall); isCall {
				walkPageListNodes(page, call.Children, loads, eachVars, diagnostics)
			}
			continue
		}
		inRow := len(eachVars) > 0
		childVars := eachVars
		if attr, has := elementAttr(element, "g:unsafe-html"); has {
			validatePageRawHTMLDirective(page, attr, loads, inRow, diagnostics)
		}
		if attr, has := elementAttr(element, "g:if"); has {
			validatePageServerIfDirective(page, attr, loads, eachVars, diagnostics)
		}
		if attr, has := elementAttr(element, "g:for"); has {
			pushed, ok := validatePageServerForDirective(page, attr, loads, eachVars, diagnostics)
			if ok {
				childVars = append(append([]string(nil), eachVars...), pushed)
			}
		}
		walkPageListNodes(page, element.Children, loads, childVars, diagnostics)
	}
}

// serverLaneForCollection reports whether a g:for collection is the server lane:
// nested inside a server row (inherits), or a top-level collection whose root is
// a declared server {} field.
func serverLaneForCollection(collection string, loads pageLoads, eachVars []string) bool {
	if len(eachVars) > 0 {
		return true
	}
	return loads.roots[exprRoot(collection)]
}

func serverLaneForCondition(expr string, loads pageLoads, eachVars []string) bool {
	if len(eachVars) > 0 {
		return true
	}
	return loads.roots[exprRoot(expr)]
}

func validatePageServerForDirective(page gwdkir.Page, attr viewmodel.Attr, loads pageLoads, eachVars []string, diagnostics *[]ValidationError) (string, bool) {
	loop, err := viewparse.ParseForDirective(attr.Value)
	if err != nil {
		if serverLaneForCollection(strings.TrimSpace(attr.Value), loads, eachVars) {
			*diagnostics = append(*diagnostics, pageListDiagnostic(page, "server_for_invalid", fmt.Sprintf("%s: %v", page.ID, err)))
		}
		return "", false
	}
	if !serverLaneForCollection(loop.Collection, loads, eachVars) {
		// Client lane: a g:for over client state/store is validated by the island
		// validator. Do not push a client loop variable into the server row scope.
		return "", false
	}
	if len(eachVars) == 0 {
		return loop.Var, true
	}
	parent := eachVars[len(eachVars)-1]
	if loop.Collection == parent {
		*diagnostics = append(*diagnostics, pageListDiagnostic(page, "server_for_nested_scope",
			fmt.Sprintf("%s: nested g:for collection cannot be the parent row item %q itself; reference a slice field such as %s.items", page.ID, parent, parent)))
		return loop.Var, true
	}
	if exprRoot(loop.Collection) != parent {
		*diagnostics = append(*diagnostics, pageListDiagnostic(page, "server_for_nested_scope",
			fmt.Sprintf("%s: nested g:for collection %q must reference the parent row item %q (for example %s.field)", page.ID, loop.Collection, parent, parent)))
	}
	return loop.Var, true
}

func validatePageServerIfDirective(page gwdkir.Page, attr viewmodel.Attr, loads pageLoads, eachVars []string, diagnostics *[]ValidationError) {
	raw := strings.TrimSpace(attr.Value)
	condition := strings.TrimSpace(strings.TrimPrefix(raw, "!"))
	if condition == "" {
		return
	}
	if !serverLaneForCondition(condition, loads, eachVars) {
		// Client lane: validated by the island validator.
		return
	}
	if strings.ContainsAny(condition, "!&|=<>(){}") {
		*diagnostics = append(*diagnostics, pageListDiagnostic(page, "server_if_invalid",
			fmt.Sprintf("%s: server-lane g:if condition %q must be a single server {} field, optionally negated with a leading !; compute compound conditions in Go and expose a bool server {} field", page.ID, condition)))
		return
	}
	if len(eachVars) == 0 {
		return
	}
	parent := eachVars[len(eachVars)-1]
	if condition == parent {
		*diagnostics = append(*diagnostics, pageListDiagnostic(page, "server_if_nested_scope",
			fmt.Sprintf("%s: nested g:if condition cannot be the row item %q itself; reference a field such as %s.field", page.ID, parent, parent)))
		return
	}
	if exprRoot(condition) != parent {
		*diagnostics = append(*diagnostics, pageListDiagnostic(page, "server_if_nested_scope",
			fmt.Sprintf("%s: nested g:if condition %q must reference the enclosing row item %q (for example %s.field)", page.ID, condition, parent, parent)))
	}
}

func validatePageRawHTMLDirective(page gwdkir.Page, attr viewmodel.Attr, loads pageLoads, inRow bool, diagnostics *[]ValidationError) {
	expr := strings.TrimSpace(attr.Value)
	if expr == "" {
		return
	}
	if inRow {
		*diagnostics = append(*diagnostics, pageListDiagnostic(page, "ghtml_over_load_data",
			fmt.Sprintf("%s: g:unsafe-html is not supported inside a server-lane g:for row; row data is attacker-influenceable and bypasses escape-by-default. Render row text with escape-by-default interpolation instead of raw HTML", page.ID)))
		return
	}
	if loads.roots[exprRoot(expr)] {
		*diagnostics = append(*diagnostics, pageListDiagnostic(page, "ghtml_over_load_data",
			fmt.Sprintf("%s: g:unsafe-html cannot render request-time server {} data %q; server {} fields are attacker-influenceable and bypass escape-by-default. Render request-time text with escape-by-default interpolation (e.g. inside g:for) instead of raw HTML", page.ID, expr)))
	}
}

func pageListDiagnostic(page gwdkir.Page, code, message string) ValidationError {
	return ValidationError{
		Code:    code,
		PageID:  page.ID,
		Source:  page.Source,
		Span:    firstSpan(page.Blocks.Spans.View, page.Spans.Page),
		Message: message,
	}
}

func elementAttr(element viewmodel.Element, name string) (viewmodel.Attr, bool) {
	for _, attr := range element.Attrs {
		if attr.Name == name {
			return attr, true
		}
	}
	return viewmodel.Attr{}, false
}

// exprRoot returns the leading identifier of a dotted/indexed expression, e.g.
// "columns" from "columns" and "col" from "col.issues".
func exprRoot(expr string) string {
	expr = strings.TrimSpace(expr)
	if cut := strings.IndexAny(expr, ".[ "); cut >= 0 {
		return expr[:cut]
	}
	return expr
}
