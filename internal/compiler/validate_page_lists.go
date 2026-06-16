package compiler

import (
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/viewmodel"
	"github.com/cssbruno/gowdk/internal/viewparse"
)

// validatePageServerLists checks request-time region rendering at check time so
// the diagnostics match build behavior. It rejects g:for/g:if/g:unsafe-html over
// request-time load {} data (pointing at g:each/g:when), and validates that
// g:each/g:when target server load data or, when nested, the enclosing row item.
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

// pageLoads describes a page's declared load {} fields. fields holds the exact
// declared paths (e.g. "columns", "user.name") used to match a server region's
// collection/condition the same way the renderer's taint set does. roots holds
// the leading identifiers (e.g. "user") used for the advisory "you used a client
// directive on load data" diagnostics, which only need to recognize the source.
type pageLoads struct {
	fields map[string]bool
	roots  map[string]bool
}

func collectPageLoads(page gwdkir.Page) pageLoads {
	loads := pageLoads{fields: map[string]bool{}, roots: map[string]bool{}}
	if !page.Blocks.Load {
		return loads
	}
	for _, line := range strings.Split(page.Blocks.LoadBody, "\n") {
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
			validatePageHTMLDirective(page, attr, loads, inRow, diagnostics)
		}
		if attr, has := elementAttr(element, "g:for"); has {
			validatePageForDirective(page, attr, loads, inRow, diagnostics)
		}
		if attr, has := elementAttr(element, "g:if"); has {
			validatePageIfDirective(page, attr, loads, inRow, diagnostics)
		}
		if attr, has := elementAttr(element, "g:else-if"); has {
			validatePageIfDirective(page, attr, loads, inRow, diagnostics)
		}
		if attr, has := elementAttr(element, "g:when"); has {
			validatePageWhenDirective(page, attr, loads, eachVars, diagnostics)
		}
		if attr, has := elementAttr(element, "g:each"); has {
			pushed, ok := validatePageEachDirective(page, attr, loads, eachVars, diagnostics)
			if ok {
				childVars = append(append([]string(nil), eachVars...), pushed)
			}
		}
		walkPageListNodes(page, element.Children, loads, childVars, diagnostics)
	}
}

func validatePageIfDirective(page gwdkir.Page, attr viewmodel.Attr, loads pageLoads, inRow bool, diagnostics *[]ValidationError) {
	expr := strings.TrimSpace(attr.Value)
	if expr == "" {
		return
	}
	if inRow {
		*diagnostics = append(*diagnostics, pageListDiagnostic(page, "gif_over_load_data",
			fmt.Sprintf("%s: %s is not supported inside a g:each row; %s binds client/island state. Branch on the row item with a nested g:when={item.field}", page.ID, attr.Name, attr.Name)))
		return
	}
	if loads.roots[exprRoot(expr)] {
		*diagnostics = append(*diagnostics, pageListDiagnostic(page, "gif_over_load_data",
			fmt.Sprintf("%s: %s cannot branch on request-time load {} data %q; %s binds client/island state. Render the server-side conditional with g:when={%s} (or g:when={!%s} for the empty branch)", page.ID, attr.Name, expr, attr.Name, expr, expr)))
	}
}

func validatePageWhenDirective(page gwdkir.Page, attr viewmodel.Attr, loads pageLoads, eachVars []string, diagnostics *[]ValidationError) {
	expr := strings.TrimSpace(attr.Value)
	expr = strings.TrimSpace(strings.TrimPrefix(expr, "!"))
	if expr == "" {
		return
	}
	if len(eachVars) == 0 {
		if !loads.fields[expr] {
			*diagnostics = append(*diagnostics, pageListDiagnostic(page, "gwhen_requires_load",
				fmt.Sprintf("%s: g:when condition %q must be a declared SSR load {} field; g:when renders request-time server data — use g:if for client/island state", page.ID, expr)))
		}
		return
	}
	parent := eachVars[len(eachVars)-1]
	if expr == parent {
		*diagnostics = append(*diagnostics, pageListDiagnostic(page, "gwhen_nested_scope",
			fmt.Sprintf("%s: nested g:when condition cannot be the row item %q itself; reference a field such as %s.field", page.ID, parent, parent)))
		return
	}
	if exprRoot(expr) != parent {
		*diagnostics = append(*diagnostics, pageListDiagnostic(page, "gwhen_nested_scope",
			fmt.Sprintf("%s: nested g:when condition %q must reference the enclosing row item %q (for example %s.field)", page.ID, expr, parent, parent)))
	}
}

func validatePageHTMLDirective(page gwdkir.Page, attr viewmodel.Attr, loads pageLoads, inRow bool, diagnostics *[]ValidationError) {
	expr := strings.TrimSpace(attr.Value)
	if expr == "" {
		return
	}
	if inRow {
		*diagnostics = append(*diagnostics, pageListDiagnostic(page, "ghtml_over_load_data",
			fmt.Sprintf("%s: g:unsafe-html is not supported inside a g:each row; row data is attacker-influenceable and bypasses escape-by-default. Render row text with escape-by-default interpolation instead of raw HTML", page.ID)))
		return
	}
	if loads.roots[exprRoot(expr)] {
		*diagnostics = append(*diagnostics, pageListDiagnostic(page, "ghtml_over_load_data",
			fmt.Sprintf("%s: g:unsafe-html cannot render request-time load {} data %q; load fields are attacker-influenceable and bypass escape-by-default. Render request-time text with escape-by-default interpolation (e.g. inside g:each) instead of raw HTML", page.ID, expr)))
	}
}

func validatePageForDirective(page gwdkir.Page, attr viewmodel.Attr, loads pageLoads, inRow bool, diagnostics *[]ValidationError) {
	loop, err := viewparse.ParseForDirective(attr.Value)
	if err != nil {
		return
	}
	if inRow {
		*diagnostics = append(*diagnostics, pageListDiagnostic(page, "gfor_over_load_data",
			fmt.Sprintf("%s: g:for is not supported inside a g:each row; g:for binds client/island state. Iterate row data with a nested g:each={%s in %s}", page.ID, loop.Var, loop.Collection)))
		return
	}
	if loads.roots[exprRoot(loop.Collection)] {
		*diagnostics = append(*diagnostics, pageListDiagnostic(page, "gfor_over_load_data",
			fmt.Sprintf("%s: g:for cannot iterate request-time load {} data %q; g:for binds client/island state. Render the server-side list with g:each={%s in %s}", page.ID, loop.Collection, loop.Var, loop.Collection)))
	}
}

func validatePageEachDirective(page gwdkir.Page, attr viewmodel.Attr, loads pageLoads, eachVars []string, diagnostics *[]ValidationError) (string, bool) {
	each, err := viewparse.ParseEachDirective(attr.Value)
	if err != nil {
		*diagnostics = append(*diagnostics, pageListDiagnostic(page, "geach_invalid", fmt.Sprintf("%s: %v", page.ID, err)))
		return "", false
	}
	if len(eachVars) == 0 {
		if !loads.fields[each.Collection] {
			*diagnostics = append(*diagnostics, pageListDiagnostic(page, "geach_requires_load",
				fmt.Sprintf("%s: g:each collection %q must be a declared SSR load {} field; g:each renders request-time server data — use g:for for client/island state", page.ID, each.Collection)))
		}
		return each.Var, true
	}
	parent := eachVars[len(eachVars)-1]
	if each.Collection == parent {
		*diagnostics = append(*diagnostics, pageListDiagnostic(page, "geach_nested_scope",
			fmt.Sprintf("%s: nested g:each collection cannot be the parent item %q itself; reference a slice field such as %s.items", page.ID, parent, parent)))
		return each.Var, true
	}
	if exprRoot(each.Collection) != parent {
		*diagnostics = append(*diagnostics, pageListDiagnostic(page, "geach_nested_scope",
			fmt.Sprintf("%s: nested g:each collection %q must reference the parent item %q (for example %s.field)", page.ID, each.Collection, parent, parent)))
	}
	return each.Var, true
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
