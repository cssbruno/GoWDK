package compiler

import (
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/viewmodel"
	"github.com/cssbruno/gowdk/internal/viewparse"
)

// validatePageServerLists checks request-time list rendering at check time so
// the diagnostics match build behavior. It rejects g:for over request-time
// load {} data (pointing at g:each), and validates that g:each targets server
// load data or, when nested, the enclosing row item.
func validatePageServerLists(page gwdkir.Page) []ValidationError {
	nodes := pageViewNodes(page)
	if len(nodes) == 0 {
		return nil
	}
	loadRoots := pageLoadRoots(page)
	var diagnostics []ValidationError
	walkPageListNodes(page, nodes, loadRoots, nil, &diagnostics)
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

// pageLoadRoots returns the set of top-level load {} field roots, e.g. "columns"
// from `=> { columns }` and "user" from `=> { user.name }`.
func pageLoadRoots(page gwdkir.Page) map[string]bool {
	roots := map[string]bool{}
	if !page.Blocks.Load {
		return roots
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
			roots[exprRoot(field)] = true
		}
	}
	return roots
}

func walkPageListNodes(page gwdkir.Page, nodes []viewmodel.Node, loadRoots map[string]bool, eachVars []string, diagnostics *[]ValidationError) {
	for _, node := range nodes {
		element, ok := node.(viewmodel.Element)
		if !ok {
			if call, isCall := node.(viewmodel.ComponentCall); isCall {
				walkPageListNodes(page, call.Children, loadRoots, eachVars, diagnostics)
			}
			continue
		}
		childVars := eachVars
		if attr, has := elementAttr(element, "g:for"); has {
			validatePageForDirective(page, attr, loadRoots, eachVars, diagnostics)
		}
		if attr, has := elementAttr(element, "g:each"); has {
			pushed, ok := validatePageEachDirective(page, attr, loadRoots, eachVars, diagnostics)
			if ok {
				childVars = append(append([]string(nil), eachVars...), pushed)
			}
		}
		walkPageListNodes(page, element.Children, loadRoots, childVars, diagnostics)
	}
}

func validatePageForDirective(page gwdkir.Page, attr viewmodel.Attr, loadRoots map[string]bool, eachVars []string, diagnostics *[]ValidationError) {
	loop, err := viewparse.ParseForDirective(attr.Value)
	if err != nil {
		return
	}
	root := exprRoot(loop.Collection)
	if containsString(eachVars, root) {
		return
	}
	if loadRoots[root] {
		*diagnostics = append(*diagnostics, ValidationError{
			Code:    "gfor_over_load_data",
			PageID:  page.ID,
			Source:  page.Source,
			Span:    firstSpan(page.Blocks.Spans.View, page.Spans.Page),
			Message: fmt.Sprintf("%s: g:for cannot iterate request-time load {} data %q; g:for binds client/island state. Render the server-side list with g:each={%s in %s}", page.ID, loop.Collection, loop.Var, loop.Collection),
		})
	}
}

func validatePageEachDirective(page gwdkir.Page, attr viewmodel.Attr, loadRoots map[string]bool, eachVars []string, diagnostics *[]ValidationError) (string, bool) {
	each, err := viewparse.ParseEachDirective(attr.Value)
	if err != nil {
		*diagnostics = append(*diagnostics, ValidationError{
			Code:    "geach_invalid",
			PageID:  page.ID,
			Source:  page.Source,
			Span:    firstSpan(page.Blocks.Spans.View, page.Spans.Page),
			Message: fmt.Sprintf("%s: %v", page.ID, err),
		})
		return "", false
	}
	root := exprRoot(each.Collection)
	if len(eachVars) == 0 {
		if !loadRoots[root] {
			*diagnostics = append(*diagnostics, ValidationError{
				Code:    "geach_requires_load",
				PageID:  page.ID,
				Source:  page.Source,
				Span:    firstSpan(page.Blocks.Spans.View, page.Spans.Page),
				Message: fmt.Sprintf("%s: g:each collection %q must be an SSR load {} field; g:each renders request-time server data — use g:for for client/island state", page.ID, each.Collection),
			})
			return each.Var, true
		}
		return each.Var, true
	}
	parent := eachVars[len(eachVars)-1]
	if root != parent {
		*diagnostics = append(*diagnostics, ValidationError{
			Code:    "geach_nested_scope",
			PageID:  page.ID,
			Source:  page.Source,
			Span:    firstSpan(page.Blocks.Spans.View, page.Spans.Page),
			Message: fmt.Sprintf("%s: nested g:each collection %q must reference the parent item %q (for example %s.field)", page.ID, each.Collection, parent, parent),
		})
	}
	return each.Var, true
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

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
