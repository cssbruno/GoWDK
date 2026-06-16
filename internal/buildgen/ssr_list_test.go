package buildgen

import (
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	gowdkssr "github.com/cssbruno/gowdk/addons/ssr"
	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

func toRuntimeListSpecs(specs []source.SSRListSpec) []gowdkssr.ListSpec {
	out := make([]gowdkssr.ListSpec, 0, len(specs))
	for _, spec := range specs {
		runtime := gowdkssr.ListSpec{
			Placeholder: spec.Placeholder,
			SourcePath:  spec.SourcePath,
			RowTemplate: spec.RowTemplate,
			Fields:      toRuntimeListFields(spec.Fields),
			Lists:       toRuntimeListSpecs(spec.Lists),
			Conds:       toRuntimeCondSpecs(spec.Conds),
		}
		out = append(out, runtime)
	}
	return out
}

func toRuntimeListFields(fields []source.SSRListField) []gowdkssr.ListField {
	out := make([]gowdkssr.ListField, 0, len(fields))
	for _, field := range fields {
		out = append(out, gowdkssr.ListField{Placeholder: field.Placeholder, Path: field.Path, Index: field.Index})
	}
	return out
}

func toRuntimeCondSpecs(specs []source.SSRCondSpec) []gowdkssr.CondSpec {
	out := make([]gowdkssr.CondSpec, 0, len(specs))
	for _, spec := range specs {
		out = append(out, gowdkssr.CondSpec{
			Placeholder: spec.Placeholder,
			SourcePath:  spec.SourcePath,
			Negate:      spec.Negate,
			Template:    spec.Template,
			Fields:      toRuntimeListFields(spec.Fields),
			Lists:       toRuntimeListSpecs(spec.Lists),
			Conds:       toRuntimeCondSpecs(spec.Conds),
		})
	}
	return out
}

func buildSSRListArtifact(t *testing.T, view string) SSRArtifact {
	return buildSSRRegionArtifact(t, `=> { columns }`, view)
}

func buildSSRRegionArtifact(t *testing.T, loadBody, view string) SSRArtifact {
	t.Helper()
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:     "board",
		Route:  "/board",
		Render: gowdk.SSR,
		Guards: []string{"public"},
		Blocks: gwdkir.Blocks{
			Load:     true,
			LoadBody: loadBody,
			View:     view != "",
			ViewBody: view,
		},
	}}}
	artifacts, err := SSRArtifacts(gowdk.Config{Addons: []gowdk.Addon{gowdk.NewAddon("ssr", gowdk.FeatureSSR)}}, app, t.TempDir())
	if err != nil {
		t.Fatalf("build SSR artifacts: %v", err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("expected one artifact, got %d", len(artifacts))
	}
	return artifacts[0]
}

// TestSSRArtifactServerListEndToEnd builds a request-time page with a nested
// g:each and exercises the full pipeline: the build-time row templates plus the
// runtime list renderer must produce the expected escaped HTML.
func TestSSRArtifactServerListEndToEnd(t *testing.T) {
	view := `<section class="board">` +
		`<div class="col" g:each={col in columns}>` +
		`<h2>{col.title}</h2>` +
		`<article class="card" g:each={issue in col.issues}><span>{issue.id}</span> {issue.title}</article>` +
		`</div>` +
		`</section>`
	artifact := buildSSRListArtifact(t, view)

	if len(artifact.ListSpecs) != 1 {
		t.Fatalf("expected one top-level list spec, got %#v", artifact.ListSpecs)
	}
	if !strings.Contains(artifact.HTML, artifact.ListSpecs[0].Placeholder) {
		t.Fatalf("artifact HTML missing list placeholder")
	}

	data := map[string]any{"columns": []any{
		map[string]any{"title": "Todo", "issues": []any{
			map[string]any{"id": "T-1", "title": "Wire <auth>"},
			map[string]any{"id": "T-2", "title": "Ship board"},
		}},
		map[string]any{"title": "Done", "issues": []any{
			map[string]any{"id": "D-1", "title": "Spec"},
		}},
	}}

	html := gowdkssr.RenderRegions(artifact.HTML, toRuntimeListSpecs(artifact.ListSpecs), toRuntimeCondSpecs(artifact.CondSpecs), data)

	for _, want := range []string{
		`<h2>Todo</h2>`,
		`<h2>Done</h2>`,
		`<span>T-1</span> Wire &lt;auth&gt;`,
		`<span>T-2</span> Ship board`,
		`<span>D-1</span> Spec`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("rendered HTML missing %q\n%s", want, html)
		}
	}
	// The build-time placeholders must all be consumed.
	if strings.Contains(html, "__GOWDK_SSR_LIST_") || strings.Contains(html, "__GOWDK_SSR_FIELD_") {
		t.Fatalf("unconsumed placeholder remains:\n%s", html)
	}
	// Escape-by-default: the raw angle brackets must not survive.
	if strings.Contains(html, "<auth>") {
		t.Fatalf("server data was not escaped:\n%s", html)
	}
}

// TestSSRArtifactServerConditionalEndToEnd builds a request-time page with an
// empty-state g:when pair and a per-row conditional, then renders through the
// runtime region renderer to verify the active branches.
func TestSSRArtifactServerConditionalEndToEnd(t *testing.T) {
	view := `<section>` +
		`<p g:when={hasItems}>You have {count} items</p>` +
		`<p g:when={!hasItems}>No issues yet</p>` +
		`<ul><li g:each={issue in issues}>{issue.id}<b g:when={issue.urgent}> URGENT</b></li></ul>` +
		`</section>`
	artifact := buildSSRRegionArtifact(t, `=> { hasItems, count, issues }`, view)
	if len(artifact.CondSpecs) != 2 {
		t.Fatalf("expected 2 top-level conditionals, got %#v", artifact.CondSpecs)
	}

	render := func(data map[string]any) string {
		return gowdkssr.RenderRegions(artifact.HTML, toRuntimeListSpecs(artifact.ListSpecs), toRuntimeCondSpecs(artifact.CondSpecs), data)
	}

	populated := render(map[string]any{
		"hasItems": true, "count": 2,
		"issues": []any{
			map[string]any{"id": "A", "urgent": true},
			map[string]any{"id": "B", "urgent": false},
		},
	})
	for _, want := range []string{"You have 2 items", "<li>A<b> URGENT</b></li>", "<li>B</li>"} {
		if !strings.Contains(populated, want) {
			t.Fatalf("populated render missing %q\n%s", want, populated)
		}
	}
	if strings.Contains(populated, "No issues yet") {
		t.Fatalf("empty branch should not render when populated:\n%s", populated)
	}

	empty := render(map[string]any{"hasItems": false, "count": 0, "issues": []any{}})
	if !strings.Contains(empty, "No issues yet") || strings.Contains(empty, "You have") {
		t.Fatalf("empty-state branch wrong:\n%s", empty)
	}
	if strings.Contains(empty, "__GOWDK_SSR_") {
		t.Fatalf("unconsumed placeholder remains:\n%s", empty)
	}
}
