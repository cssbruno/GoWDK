package ssr

import "testing"

func TestRenderListsTopLevel(t *testing.T) {
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
	got := RenderLists("<ul>@LIST@</ul>", specs, data)
	want := "<ul><li>alpha</li><li>beta</li></ul>"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRenderListsEscapesFields(t *testing.T) {
	specs := []ListSpec{{
		Placeholder: "@LIST@",
		SourcePath:  "items",
		RowTemplate: "<li>@NAME@</li>",
		Fields:      []ListField{{Placeholder: "@NAME@", Path: "name"}},
	}}
	data := map[string]any{"items": []any{map[string]any{"name": "<script>x</script>"}}}
	got := RenderLists("@LIST@", specs, data)
	want := "<li>&lt;script&gt;x&lt;/script&gt;</li>"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRenderListsIndexField(t *testing.T) {
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
	got := RenderLists("@LIST@", specs, data)
	want := "<li>0:a</li><li>1:b</li>"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRenderListsNested(t *testing.T) {
	specs := []ListSpec{{
		Placeholder: "@COLS@",
		SourcePath:  "columns",
		RowTemplate: "<div>@TITLE@<ul>@ISSUES@</ul></div>",
		Fields:      []ListField{{Placeholder: "@TITLE@", Path: "title"}},
		Children: []ListSpec{{
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
	got := RenderLists("@COLS@", specs, data)
	want := "<div>Todo<ul><li>T-1</li><li>T-2</li></ul></div><div>Done<ul><li>D-1</li></ul></div>"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRenderListsStructSliceAndDottedPath(t *testing.T) {
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
	got := RenderLists("@LIST@", specs, data)
	want := "<li>X-1 by Ada</li>"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRenderListsMissingSliceRendersEmpty(t *testing.T) {
	specs := []ListSpec{{
		Placeholder: "@LIST@",
		SourcePath:  "items",
		RowTemplate: "<li>@NAME@</li>",
		Fields:      []ListField{{Placeholder: "@NAME@", Path: "name"}},
	}}
	got := RenderLists("[@LIST@]", specs, map[string]any{})
	if got != "[]" {
		t.Fatalf("got %q want %q", got, "[]")
	}
}
