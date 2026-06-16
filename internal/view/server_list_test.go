package view

import (
	"strings"
	"testing"
)

func renderServerList(t *testing.T, source string, tainted map[string]bool) (string, []SSRListReplacement) {
	t.Helper()
	html, lists, _ := renderServerRegions(t, source, tainted)
	return html, lists
}

func renderServerRegions(t *testing.T, source string, tainted map[string]bool) (string, []SSRListReplacement, []SSRCondReplacement) {
	t.Helper()
	var lists []SSRListReplacement
	var conds []SSRCondReplacement
	html, err := RenderWithOptions(source, nil, nil, Options{
		Tainted:        tainted,
		ServerListSink: &lists,
		ServerCondSink: &conds,
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	return html, lists, conds
}

func TestServerConditionalTopLevel(t *testing.T) {
	source := `<section><p g:if={hasItems}>You have {count} items</p><p g:if={!hasItems}>No issues yet</p></section>`
	html, _, conds := renderServerRegions(t, source, map[string]bool{"hasItems": true, "count": true})
	if len(conds) != 2 {
		t.Fatalf("want 2 conditionals, got %d", len(conds))
	}
	if conds[0].SourcePath != "hasItems" || conds[0].Negate {
		t.Fatalf("first cond wrong: %+v", conds[0])
	}
	if conds[1].SourcePath != "hasItems" || !conds[1].Negate {
		t.Fatalf("second cond should be negated: %+v", conds[1])
	}
	if !strings.Contains(html, conds[0].Placeholder) || !strings.Contains(html, conds[1].Placeholder) {
		t.Fatalf("html missing cond placeholders: %q", html)
	}
	if len(conds[0].Fields) != 1 || conds[0].Fields[0].Path != "count" {
		t.Fatalf("branch should interpolate load field count: %+v", conds[0].Fields)
	}
}

func TestConditionalUnknownSourceRejected(t *testing.T) {
	// g:if over a name that is neither server {} request-time data nor client
	// state/store resolves to the client lane and is rejected as an undeclared
	// value. (The precise lane-aware "unknown source" diagnostic is reported at
	// check time.)
	_, err := RenderWithOptions(`<p g:if={ready}>x</p>`, nil, nil, Options{
		Tainted:        map[string]bool{},
		ServerListSink: &[]SSRListReplacement{},
		ServerCondSink: &[]SSRCondReplacement{},
	})
	if err == nil {
		t.Fatalf("want an error for an undeclared g:if source")
	}
}

func TestServerConditionalCompoundExpression(t *testing.T) {
	// A top-level server g:if accepts a full bool expression over server {} fields;
	// it is stored as Expr and evaluated at request time, not as a field path.
	_, _, conds := renderServerRegions(t, `<p g:if={count > 0}>has</p>`, map[string]bool{"count": true})
	if len(conds) != 1 {
		t.Fatalf("want 1 conditional, got %d", len(conds))
	}
	if conds[0].Expr != "count > 0" {
		t.Fatalf("compound g:if should store the expression, got %+v", conds[0])
	}
	if conds[0].SourcePath != "" {
		t.Fatalf("compound g:if should not set SourcePath, got %q", conds[0].SourcePath)
	}
}

func TestServerConditionalCompoundRejectsUnknownField(t *testing.T) {
	_, err := RenderWithOptions(`<p g:if={count > limit}>x</p>`, nil, nil, Options{
		Tainted:        map[string]bool{"count": true},
		ServerCondSink: &[]SSRCondReplacement{},
	})
	if err == nil || !strings.Contains(err.Error(), "not a declared server {} field") {
		t.Fatalf("want unknown-field error for compound g:if, got %v", err)
	}
}

func TestServerConditionalCompoundRejectedInRow(t *testing.T) {
	_, err := RenderWithOptions(`<ul><li g:for={item in items}><b g:if={item.count > 0}>!</b></li></ul>`, nil, nil, Options{
		Tainted:        map[string]bool{"items": true},
		ServerListSink: &[]SSRListReplacement{},
		ServerCondSink: &[]SSRCondReplacement{},
	})
	if err == nil || !strings.Contains(err.Error(), "single row field") {
		t.Fatalf("want nested-compound rejection, got %v", err)
	}
}

func TestServerListRejectsDirectiveOnRowElement(t *testing.T) {
	// A disallowed directive on the repeated row element itself (not a child)
	// must be rejected, matching the documented server-row contract.
	_, err := RenderWithOptions(`<li g:for={item in items} g:if={item.show} g:key={item.id}>{item.name}</li>`, nil, nil, Options{
		Tainted:        map[string]bool{"items": true},
		ServerListSink: &[]SSRListReplacement{},
		ServerCondSink: &[]SSRCondReplacement{},
	})
	if err == nil || !strings.Contains(err.Error(), "g:if") {
		t.Fatalf("want row-element g:if rejection, got %v", err)
	}
}

func TestServerListRejectedOutsidePageContext(t *testing.T) {
	// A server-lane g:for (its collection is a server {} field) without a page
	// region sink (e.g. a component/fragment render) must be rejected rather than
	// silently dropping the spec and emitting an unconsumed placeholder.
	_, err := RenderWithOptions(`<li g:for={item in items}>{item.name}</li>`, nil, nil, Options{
		Tainted: map[string]bool{"items": true},
	})
	if err == nil || !strings.Contains(err.Error(), "only supported in a request-time page view") {
		t.Fatalf("want page-only rejection, got %v", err)
	}
}

func TestServerConditionalInsideEachRow(t *testing.T) {
	source := `<ul><li g:for={issue in issues}>{issue.id}<b g:if={issue.urgent}>!</b></li></ul>`
	_, lists, conds := renderServerRegions(t, source, map[string]bool{"issues": true})
	if len(conds) != 0 {
		t.Fatalf("conditional should be nested in the row, not top-level: %d", len(conds))
	}
	if len(lists) != 1 || len(lists[0].Conds) != 1 {
		t.Fatalf("want 1 nested conditional in the row, got %+v", lists)
	}
	if lists[0].Conds[0].SourcePath != "urgent" {
		t.Fatalf("nested cond should be item-relative: %q", lists[0].Conds[0].SourcePath)
	}
}

func TestServerEachInsideConditionalBranch(t *testing.T) {
	source := `<div g:if={hasItems}><article g:for={item in items} g:key={item.id}>{item.name}</article></div>`
	_, _, conds := renderServerRegions(t, source, map[string]bool{"hasItems": true, "items": true})
	if len(conds) != 1 || len(conds[0].Lists) != 1 {
		t.Fatalf("want a list nested in the conditional branch, got %+v", conds)
	}
	if conds[0].Lists[0].SourcePath != "items" {
		t.Fatalf("nested list source wrong: %q", conds[0].Lists[0].SourcePath)
	}
}

func TestServerListTopLevel(t *testing.T) {
	source := `<ul><li g:for={item in items}>{item.name}</li></ul>`
	html, lists := renderServerList(t, source, map[string]bool{"items": true})
	if len(lists) != 1 {
		t.Fatalf("want 1 list, got %d", len(lists))
	}
	spec := lists[0]
	if spec.SourcePath != "items" {
		t.Fatalf("source path = %q", spec.SourcePath)
	}
	if !strings.Contains(html, spec.Placeholder) {
		t.Fatalf("html %q missing placeholder %q", html, spec.Placeholder)
	}
	if len(spec.Fields) != 1 || spec.Fields[0].Path != "name" {
		t.Fatalf("fields = %+v", spec.Fields)
	}
	if !strings.Contains(spec.RowTemplate, spec.Fields[0].Placeholder) {
		t.Fatalf("row template %q missing field placeholder", spec.RowTemplate)
	}
	if !strings.HasPrefix(html, "<ul>") || !strings.HasSuffix(html, "</ul>") {
		t.Fatalf("static markup not preserved: %q", html)
	}
}

func TestForUnknownSourceRejected(t *testing.T) {
	// g:for over a collection that is neither a server {} field nor client state
	// resolves to the client lane and is rejected as an undeclared collection.
	source := `<li g:for={item in items}>{item.name}</li>`
	_, err := RenderWithOptions(source, nil, nil, Options{ServerListSink: &[]SSRListReplacement{}})
	if err == nil {
		t.Fatalf("want an error for an undeclared g:for source")
	}
}

func TestServerListRejectsNonItemInterpolation(t *testing.T) {
	source := `<li g:for={item in items}>{other}</li>`
	_, err := RenderWithOptions(source, nil, nil, Options{
		Tainted:        map[string]bool{"items": true},
		ServerListSink: &[]SSRListReplacement{},
	})
	if err == nil || !strings.Contains(err.Error(), "may only interpolate the row item") {
		t.Fatalf("want item-scope error, got %v", err)
	}
}

func TestServerListNested(t *testing.T) {
	source := `<div g:for={col in columns}>{col.title}<ul><li g:for={issue in col.issues}>{issue.id}</li></ul></div>`
	_, lists := renderServerList(t, source, map[string]bool{"columns": true})
	if len(lists) != 1 {
		t.Fatalf("want 1 top list, got %d", len(lists))
	}
	outer := lists[0]
	if outer.SourcePath != "columns" {
		t.Fatalf("outer source = %q", outer.SourcePath)
	}
	if len(outer.Lists) != 1 {
		t.Fatalf("want 1 nested list, got %d", len(outer.Lists))
	}
	inner := outer.Lists[0]
	if inner.SourcePath != "issues" {
		t.Fatalf("inner source = %q (want relative path)", inner.SourcePath)
	}
	if !strings.Contains(outer.RowTemplate, inner.Placeholder) {
		t.Fatalf("outer row %q missing nested placeholder %q", outer.RowTemplate, inner.Placeholder)
	}
}

func TestServerListIndexVar(t *testing.T) {
	source := `<li g:for={item, i in items}>{i}:{item.name}</li>`
	_, lists := renderServerList(t, source, map[string]bool{"items": true})
	spec := lists[0]
	var hasIndex, hasName bool
	for _, f := range spec.Fields {
		if f.Index {
			hasIndex = true
		}
		if f.Path == "name" {
			hasName = true
		}
	}
	if !hasIndex || !hasName {
		t.Fatalf("fields = %+v", spec.Fields)
	}
}

func TestServerListRejectsNestedNonItem(t *testing.T) {
	source := `<div g:for={col in columns}><span g:for={issue in other}>{issue.id}</span></div>`
	_, err := RenderWithOptions(source, nil, nil, Options{
		Tainted:        map[string]bool{"columns": true},
		ServerListSink: &[]SSRListReplacement{},
	})
	if err == nil || !strings.Contains(err.Error(), "must reference the parent item") {
		t.Fatalf("want nested-scope error, got %v", err)
	}
}
