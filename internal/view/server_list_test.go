package view

import (
	"strings"
	"testing"
)

func renderServerList(t *testing.T, source string, tainted map[string]bool) (string, []SSRListReplacement) {
	t.Helper()
	var lists []SSRListReplacement
	html, err := RenderWithOptions(source, nil, nil, Options{
		Tainted:        tainted,
		ServerListSink: &lists,
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	return html, lists
}

func TestServerListTopLevel(t *testing.T) {
	source := `<ul><li g:each={item in items}>{item.name}</li></ul>`
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

func TestServerListRejectsNonLoadField(t *testing.T) {
	source := `<li g:each={item in items}>{item.name}</li>`
	_, err := RenderWithOptions(source, nil, nil, Options{ServerListSink: &[]SSRListReplacement{}})
	if err == nil || !strings.Contains(err.Error(), "must be an SSR load") {
		t.Fatalf("want load-field error, got %v", err)
	}
}

func TestServerListRejectsNonItemInterpolation(t *testing.T) {
	source := `<li g:each={item in items}>{other}</li>`
	_, err := RenderWithOptions(source, nil, nil, Options{
		Tainted:        map[string]bool{"items": true},
		ServerListSink: &[]SSRListReplacement{},
	})
	if err == nil || !strings.Contains(err.Error(), "may only interpolate its item") {
		t.Fatalf("want item-scope error, got %v", err)
	}
}

func TestServerListNested(t *testing.T) {
	source := `<div g:each={col in columns}>{col.title}<ul><li g:each={issue in col.issues}>{issue.id}</li></ul></div>`
	_, lists := renderServerList(t, source, map[string]bool{"columns": true})
	if len(lists) != 1 {
		t.Fatalf("want 1 top list, got %d", len(lists))
	}
	outer := lists[0]
	if outer.SourcePath != "columns" {
		t.Fatalf("outer source = %q", outer.SourcePath)
	}
	if len(outer.Children) != 1 {
		t.Fatalf("want 1 nested list, got %d", len(outer.Children))
	}
	inner := outer.Children[0]
	if inner.SourcePath != "issues" {
		t.Fatalf("inner source = %q (want relative path)", inner.SourcePath)
	}
	if !strings.Contains(outer.RowTemplate, inner.Placeholder) {
		t.Fatalf("outer row %q missing nested placeholder %q", outer.RowTemplate, inner.Placeholder)
	}
}

func TestServerListIndexVar(t *testing.T) {
	source := `<li g:each={item, i in items}>{i}:{item.name}</li>`
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
	source := `<div g:each={col in columns}><span g:each={issue in other}>{issue.id}</span></div>`
	_, err := RenderWithOptions(source, nil, nil, Options{
		Tainted:        map[string]bool{"columns": true},
		ServerListSink: &[]SSRListReplacement{},
	})
	if err == nil || !strings.Contains(err.Error(), "must reference the parent item") {
		t.Fatalf("want nested-scope error, got %v", err)
	}
}
