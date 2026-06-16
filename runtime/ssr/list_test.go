package ssr

import "testing"

func TestRenderRegionsTopLevel(t *testing.T) {
	specs := []ListSpec{{
		Placeholder: "@LIST@",
		SourcePath:  "items",
		RowTemplate: "<li>@NAME@</li>",
		Fields:      []ListField{{Placeholder: "@NAME@", Path: "name"}},
	}}
	data := map[string]any{"items": []any{
		map[string]any{"name": "alpha"},
		map[string]any{"name": "beta"},
	}}
	got := RenderRegions("<ul>@LIST@</ul>", specs, nil, data)
	want := "<ul><li>alpha</li><li>beta</li></ul>"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRenderRegionsEscapesFields(t *testing.T) {
	specs := []ListSpec{{
		Placeholder: "@LIST@",
		SourcePath:  "items",
		RowTemplate: "<li>@NAME@</li>",
		Fields:      []ListField{{Placeholder: "@NAME@", Path: "name"}},
	}}
	data := map[string]any{"items": []any{map[string]any{"name": "<script>x</script>"}}}
	got := RenderRegions("@LIST@", specs, nil, data)
	want := "<li>&lt;script&gt;x&lt;/script&gt;</li>"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRenderRegionsIndexField(t *testing.T) {
	specs := []ListSpec{{
		Placeholder: "@LIST@",
		SourcePath:  "items",
		RowTemplate: "<li>@I@:@NAME@</li>",
		Fields: []ListField{
			{Placeholder: "@I@", Index: true},
			{Placeholder: "@NAME@", Path: "name"},
		},
	}}
	data := map[string]any{"items": []any{
		map[string]any{"name": "a"},
		map[string]any{"name": "b"},
	}}
	got := RenderRegions("@LIST@", specs, nil, data)
	want := "<li>0:a</li><li>1:b</li>"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRenderRegionsNestedList(t *testing.T) {
	specs := []ListSpec{{
		Placeholder: "@COLS@",
		SourcePath:  "columns",
		RowTemplate: "<div>@TITLE@<ul>@ISSUES@</ul></div>",
		Fields:      []ListField{{Placeholder: "@TITLE@", Path: "title"}},
		Lists: []ListSpec{{
			Placeholder: "@ISSUES@",
			SourcePath:  "issues",
			RowTemplate: "<li>@ID@</li>",
			Fields:      []ListField{{Placeholder: "@ID@", Path: "id"}},
		}},
	}}
	data := map[string]any{"columns": []any{
		map[string]any{"title": "Todo", "issues": []any{
			map[string]any{"id": "T-1"},
			map[string]any{"id": "T-2"},
		}},
		map[string]any{"title": "Done", "issues": []any{
			map[string]any{"id": "D-1"},
		}},
	}}
	got := RenderRegions("@COLS@", specs, nil, data)
	want := "<div>Todo<ul><li>T-1</li><li>T-2</li></ul></div><div>Done<ul><li>D-1</li></ul></div>"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRenderRegionsConditionalTopLevel(t *testing.T) {
	conds := []CondSpec{
		{Placeholder: "@YES@", SourcePath: "hasItems", Template: "<p>You have @COUNT@ items</p>", Fields: []ListField{{Placeholder: "@COUNT@", Path: "count"}}},
		{Placeholder: "@NO@", SourcePath: "hasItems", Negate: true, Template: "<p>No issues yet</p>"},
	}
	yes := RenderRegions("<div>@YES@@NO@</div>", nil, conds, map[string]any{"hasItems": true, "count": 3})
	if yes != "<div><p>You have 3 items</p></div>" {
		t.Fatalf("truthy branch wrong: %q", yes)
	}
	no := RenderRegions("<div>@YES@@NO@</div>", nil, conds, map[string]any{"hasItems": false, "count": 0})
	if no != "<div><p>No issues yet</p></div>" {
		t.Fatalf("falsy branch wrong: %q", no)
	}
}

func TestRenderRegionsConditionalExpression(t *testing.T) {
	conds := []CondSpec{
		{Placeholder: "@SHOW@", Expr: "count > 0 && status == \"open\"", Template: "<p>open</p>"},
	}
	on := RenderRegions("<div>@SHOW@</div>", nil, conds, map[string]any{"count": 3, "status": "open"})
	if on != "<div><p>open</p></div>" {
		t.Fatalf("expression branch should render when true: %q", on)
	}
	off := RenderRegions("<div>@SHOW@</div>", nil, conds, map[string]any{"count": 0, "status": "open"})
	if off != "<div></div>" {
		t.Fatalf("expression branch should be hidden when false: %q", off)
	}
}

func TestRenderRegionsConditionalExpressionFailsClosed(t *testing.T) {
	// A condition that cannot evaluate (missing field) must hide the branch
	// rather than render attacker-influenceable markup.
	conds := []CondSpec{{Placeholder: "@SHOW@", Expr: "missing > 0", Template: "<p>x</p>"}}
	got := RenderRegions("<div>@SHOW@</div>", nil, conds, map[string]any{"count": 3})
	if got != "<div></div>" {
		t.Fatalf("unevaluable condition should fail closed: %q", got)
	}
}

func TestRenderRegionsConditionalInsideRow(t *testing.T) {
	specs := []ListSpec{{
		Placeholder: "@LIST@",
		SourcePath:  "issues",
		RowTemplate: "<li>@ID@@BADGE@</li>",
		Fields:      []ListField{{Placeholder: "@ID@", Path: "id"}},
		Conds: []CondSpec{{
			Placeholder: "@BADGE@",
			SourcePath:  "urgent",
			Template:    " <b>!</b>",
		}},
	}}
	data := map[string]any{"issues": []any{
		map[string]any{"id": "A", "urgent": true},
		map[string]any{"id": "B", "urgent": false},
	}}
	got := RenderRegions("@LIST@", specs, nil, data)
	want := "<li>A <b>!</b></li><li>B</li>"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRenderRegionsListInsideConditional(t *testing.T) {
	conds := []CondSpec{{
		Placeholder: "@WHEN@",
		SourcePath:  "hasItems",
		Template:    "<ul>@LIST@</ul>",
		Lists: []ListSpec{{
			Placeholder: "@LIST@",
			SourcePath:  "items",
			RowTemplate: "<li>@NAME@</li>",
			Fields:      []ListField{{Placeholder: "@NAME@", Path: "name"}},
		}},
	}}
	data := map[string]any{"hasItems": true, "items": []any{
		map[string]any{"name": "a"}, map[string]any{"name": "b"},
	}}
	got := RenderRegions("@WHEN@", nil, conds, data)
	if got != "<ul><li>a</li><li>b</li></ul>" {
		t.Fatalf("got %q", got)
	}
}

func TestTruthy(t *testing.T) {
	cases := []struct {
		value any
		want  bool
	}{
		{true, true}, {false, false},
		{"x", true}, {"", false},
		{1, true}, {0, false},
		{[]any{1}, true}, {[]any{}, false},
		{nil, false},
	}
	for _, tc := range cases {
		if got := truthy(tc.value); got != tc.want {
			t.Fatalf("truthy(%#v) = %v want %v", tc.value, got, tc.want)
		}
	}
}

func TestRenderRegionsStructSliceAndDottedPath(t *testing.T) {
	type issue struct {
		ID     string
		Author struct{ Name string }
	}
	a := issue{ID: "X-1"}
	a.Author.Name = "Ada"
	specs := []ListSpec{{
		Placeholder: "@LIST@",
		SourcePath:  "issues",
		RowTemplate: "<li>@ID@ by @AUTHOR@</li>",
		Fields: []ListField{
			{Placeholder: "@ID@", Path: "ID"},
			{Placeholder: "@AUTHOR@", Path: "Author.Name"},
		},
	}}
	data := map[string]any{"issues": []issue{a}}
	got := RenderRegions("@LIST@", specs, nil, data)
	want := "<li>X-1 by Ada</li>"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRenderRegionsMissingSliceRendersEmpty(t *testing.T) {
	specs := []ListSpec{{
		Placeholder: "@LIST@",
		SourcePath:  "items",
		RowTemplate: "<li>@NAME@</li>",
		Fields:      []ListField{{Placeholder: "@NAME@", Path: "name"}},
	}}
	got := RenderRegions("[@LIST@]", specs, nil, map[string]any{})
	if got != "[]" {
		t.Fatalf("got %q want %q", got, "[]")
	}
}
