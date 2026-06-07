package buildgen

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/manifest"
)

func TestBuildEmitsJSIslandAssetsForStatefulComponent(t *testing.T) {
	outputDir := t.TempDir()
	component := counterComponent()
	component.Span = manifest.SourceSpan{Start: manifest.SourcePosition{Line: 1, Column: 1}, End: manifest.SourcePosition{Line: 1, Column: 19}}
	component.Blocks.Spans.View = manifest.SourceSpan{Start: manifest.SourcePosition{Line: 3, Column: 1}, End: manifest.SourcePosition{Line: 3, Column: 7}}
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "counter",
			Route: "/counter",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Counter /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	result, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	jsPath := filepath.Join(outputDir, "assets", "gowdk", "islands", "Counter.js")
	jsMapPath := filepath.Join(outputDir, "assets", "gowdk", "islands", "Counter.js.map")
	if !hasAssetArtifact(result.AssetArtifacts, jsPath) {
		t.Fatalf("expected Counter.js asset, got %#v", result.AssetArtifacts)
	}
	if !hasAssetArtifact(result.AssetArtifacts, jsMapPath) {
		t.Fatalf("expected Counter.js.map asset, got %#v", result.AssetArtifacts)
	}
	html := readFile(t, filepath.Join(outputDir, "counter", "index.html"))
	for _, expected := range []string{
		`<script src="/assets/gowdk/islands/Counter.js" defer></script>`,
		`<gowdk-island data-gowdk-component="Counter" data-gowdk-island="i1" data-gowdk-runtime="js"`,
		`data-gowdk-on-click="Count++"`,
		`data-gowdk-bind="Count" data-gowdk-binding-text="b2">1</span>`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in island page:\n%s", expected, html)
		}
	}
	js := readFile(t, jsPath)
	if !strings.Contains(js, `data-gowdk-runtime=\"js\"`) ||
		!strings.Contains(js, `const selector = "gowdk-island[data-gowdk-component=\"" + component + "\"][data-gowdk-runtime=\"js\"]";`) ||
		!strings.Contains(js, "\n  function parseExpression(source)") ||
		!strings.Contains(js, `applyExpression`) ||
		!strings.Contains(js, `window.__gowdkMountIslands`) ||
		!strings.Contains(js, `window.__gowdkDestroyIslands`) ||
		!strings.Contains(js, `async function mountCounterIsland(scope)`) ||
		!strings.Contains(js, `async function destroyCounterIsland()`) ||
		!strings.Contains(js, `registry.components[component] = mountCounterIsland`) ||
		!strings.Contains(js, `registry.roots`) ||
		!strings.Contains(js, `data-gowdk-mounted`) ||
		!strings.Contains(js, `//# sourceMappingURL=Counter.js.map`) {
		t.Fatalf("expected generated JS island runtime, got:\n%s", js)
	}
	for _, unexpected := range []string{
		`document.body.innerHTML`,
		`document.documentElement.innerHTML`,
	} {
		if strings.Contains(js, unexpected) {
			t.Fatalf("expected island-scoped runtime without full-page hydration, found %q in:\n%s", unexpected, js)
		}
	}
	assetManifestPayload := readFile(t, filepath.Join(outputDir, assetManifestFile))
	if !strings.Contains(assetManifestPayload, `"assets/gowdk/islands/Counter.js": "assets/gowdk/islands/Counter.js"`) {
		t.Fatalf("expected island JS in asset manifest:\n%s", assetManifestPayload)
	}
	if !strings.Contains(assetManifestPayload, `"assets/gowdk/islands/Counter.js.map": "assets/gowdk/islands/Counter.js.map"`) {
		t.Fatalf("expected island JS source map in asset manifest:\n%s", assetManifestPayload)
	}
	sourceMapPayload := readFile(t, jsMapPath)
	var sourceMap struct {
		Version        int      `json:"version"`
		File           string   `json:"file"`
		Sources        []string `json:"sources"`
		SourcesContent []string `json:"sourcesContent"`
		Names          []string `json:"names"`
		Mappings       string   `json:"mappings"`
	}
	if err := json.Unmarshal([]byte(sourceMapPayload), &sourceMap); err != nil {
		t.Fatalf("expected valid source map JSON: %v\n%s", err, sourceMapPayload)
	}
	if sourceMap.Version != 3 || sourceMap.File != "Counter.js" || len(sourceMap.Sources) != 1 || sourceMap.Sources[0] != "components/counter.cmp.gwdk" {
		t.Fatalf("unexpected source map metadata: %#v", sourceMap)
	}
	if sourceMap.Mappings == "" {
		t.Fatalf("expected generated JS source map mappings: %#v", sourceMap)
	}
	if len(sourceMap.SourcesContent) != 1 || !strings.Contains(sourceMap.SourcesContent[0], `view {`) || !strings.Contains(sourceMap.SourcesContent[0], `{Count}`) {
		t.Fatalf("expected component source content in source map: %#v", sourceMap.SourcesContent)
	}
}

func TestBuildEmitsStoreUsesInIslandBootstrap(t *testing.T) {
	outputDir := t.TempDir()
	component := counterComponent()
	component.Blocks.Client = true
	component.Blocks.ClientBody = `
use cart

fn Add() {
  Count++
}
`
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:      "counter",
			Route:   "/counter",
			Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
			Stores: []manifest.Store{{
				Name: "cart",
				Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
				Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
			}},
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Counter /><Counter /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	result, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	storePath := filepath.Join(outputDir, "assets", "gowdk", "islands", "stores.js")
	if !hasAssetArtifact(result.AssetArtifacts, storePath) {
		t.Fatalf("expected stores.js asset, got %#v", result.AssetArtifacts)
	}
	html := readFile(t, filepath.Join(outputDir, "counter", "index.html"))
	for _, expected := range []string{
		`<script type="application/json" data-gowdk-store="cart">{"Count":1,"Open":false}</script>`,
		`<script src="/assets/gowdk/islands/stores.js" defer></script>`,
		`<script src="/assets/gowdk/islands/Counter.js" defer></script>`,
		`data-gowdk-client="{&#34;handlers&#34;:{&#34;Add&#34;:{&#34;statements&#34;:[&#34;Count++&#34;]}},&#34;stores&#34;:[&#34;cart&#34;]}"`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in island page:\n%s", expected, html)
		}
	}
	if count := strings.Count(html, `&#34;stores&#34;:[&#34;cart&#34;]`); count != 2 {
		t.Fatalf("expected two island roots to use cart store, got %d in:\n%s", count, html)
	}
	if storesIndex := strings.Index(html, `/assets/gowdk/islands/stores.js`); storesIndex < 0 {
		t.Fatalf("expected store runtime script in island page:\n%s", html)
	} else if islandIndex := strings.Index(html, `/assets/gowdk/islands/Counter.js`); islandIndex < 0 || islandIndex < storesIndex {
		t.Fatalf("expected store runtime before component runtime:\n%s", html)
	}
	storeJS := readFile(t, storePath)
	for _, expected := range []string{
		`window.__gowdkStores`,
		`registry.set = (name, next)`,
		`registry.subscribe = (name, listener)`,
		`data-gowdk-store`,
	} {
		if !strings.Contains(storeJS, expected) {
			t.Fatalf("expected %q in store runtime:\n%s", expected, storeJS)
		}
	}
	islandJS := readFile(t, filepath.Join(outputDir, "assets", "gowdk", "islands", "Counter.js"))
	for _, expected := range []string{
		`const storeNames = Array.isArray(client.stores) ? client.stores : [];`,
		`Object.assign(state, storeRegistry.get(name));`,
		`storeNames.forEach((name) => storeRegistry.set(name, state));`,
		`storeRegistry.subscribe(name, async (next) =>`,
		`storeUnsubscribers.forEach((unsubscribe) => unsubscribe());`,
	} {
		if !strings.Contains(islandJS, expected) {
			t.Fatalf("expected %q in island runtime:\n%s", expected, islandJS)
		}
	}
}

func TestJSIslandsSharePageStoreInBrowser(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}
	chromium, err := lookupChromium()
	if err != nil {
		t.Skip(err)
	}
	requireNodePlaywright(t, node)

	outputDir := t.TempDir()
	component := counterComponent()
	component.Blocks.Client = true
	component.Blocks.ClientBody = `use cart`
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:      "counter",
			Route:   "/counter",
			Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
			Stores: []manifest.Store{{
				Name: "cart",
				Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
				Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
			}},
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Counter /><Counter /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}
	if _, err := Build(gowdk.Config{}, app, outputDir); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.FileServer(http.Dir(outputDir)))
	defer server.Close()

	script := filepath.Join(t.TempDir(), "gowdk-js-store-browser-test.cjs")
	if err := os.WriteFile(script, []byte(jsIslandStoreBrowserHarness()), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, node, script, server.URL, chromium)
	command.Dir = mustWorkingDir(t)
	output, err := command.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("browser JS store test timed out:\n%s", output)
	}
	if err != nil {
		t.Fatalf("browser JS store test failed: %v\n%s", err, output)
	}
}

func TestJSIslandEffectsCleanupAndBatchingInBrowser(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}
	chromium, err := lookupChromium()
	if err != nil {
		t.Skip(err)
	}
	requireNodePlaywright(t, node)

	outputDir := t.TempDir()
	component := counterComponent()
	component.Blocks.Client = true
	component.Blocks.ClientBody = `effect when Open {
  Count = Count + 1
  return {
    Count = if Open { 10 } else { 20 }
  }
}

fn Burst() {
  Count++
  Count++
}

fn Flip() {
  Open = !Open
}`
	component.Blocks.ViewBody = `<section><button id="burst" g:on:click={Burst()}><span id="count">{Count}</span></button><button id="flip" g:on:click={Flip()}><span id="open">{Open}</span></button></section>`
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "counter",
			Route: "/counter",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Counter /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}
	if _, err := Build(gowdk.Config{}, app, outputDir); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.FileServer(http.Dir(outputDir)))
	defer server.Close()

	script := filepath.Join(t.TempDir(), "gowdk-js-effects-browser-test.cjs")
	if err := os.WriteFile(script, []byte(jsIslandEffectsBrowserHarness()), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, node, script, server.URL, chromium)
	command.Dir = mustWorkingDir(t)
	output, err := command.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("browser JS effects test timed out:\n%s", output)
	}
	if err != nil {
		t.Fatalf("browser JS effects test failed: %v\n%s", err, output)
	}
}

func TestBuildRejectsComputedDependencyCycle(t *testing.T) {
	outputDir := t.TempDir()
	component := counterComponent()
	component.Blocks.Client = true
	component.Blocks.ClientBody = `computed A string {
  return B
}

computed B string {
  return A
}`
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "counter",
			Route: "/counter",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Counter /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}
	_, err := Build(gowdk.Config{}, app, outputDir)
	if err == nil {
		t.Fatal("expected computed dependency cycle error")
	}
	if !strings.Contains(err.Error(), "computed dependency cycle") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIslandJSSourceMapMappingsUseComponentSpans(t *testing.T) {
	component := counterComponent()
	component.Span = manifest.SourceSpan{Start: manifest.SourcePosition{Line: 2, Column: 1}, End: manifest.SourcePosition{Line: 2, Column: 19}}
	component.Blocks.Client = true
	component.Blocks.ClientBody = `fn Add() {
  Count++
}`
	component.Blocks.Spans.Client = manifest.SourceSpan{Start: manifest.SourcePosition{Line: 7, Column: 1}, End: manifest.SourcePosition{Line: 7, Column: 9}}
	component.Blocks.Spans.View = manifest.SourceSpan{Start: manifest.SourcePosition{Line: 13, Column: 1}, End: manifest.SourcePosition{Line: 13, Column: 7}}
	source := islandJSSource(component.Name, true)

	var sourceMap struct {
		Mappings string `json:"mappings"`
	}
	if err := json.Unmarshal(islandJSSourceMap(component, source), &sourceMap); err != nil {
		t.Fatal(err)
	}
	if sourceMap.Mappings == "" {
		t.Fatal("expected non-empty source map mappings")
	}

	mappings := decodeSourceMapMappings(t, sourceMap.Mappings)
	mountLine := generatedLineContaining(t, source, `async function mountCounterIsland(scope)`)
	applyStatementsLine := generatedLineContaining(t, source, `async function applyStatements(`)
	bindingTableLine := generatedLineContaining(t, source, `const bindingTable = Object.freeze(`)
	updateBindingsLine := generatedLineContaining(t, source, `function updateBindings(root, state, helpers, bindings)`)
	renderLine := generatedLineContaining(t, source, `function render(root, state, helpers, bindings)`)
	if mappings[mountLine].SourceLine != 7 || mappings[mountLine].SourceColumn != 1 {
		t.Fatalf("expected mount function to map to client span, got %#v", mappings[mountLine])
	}
	if mappings[applyStatementsLine].SourceLine != 7 || mappings[applyStatementsLine].SourceColumn != 1 {
		t.Fatalf("expected statement runtime to map to client span, got %#v", mappings[applyStatementsLine])
	}
	if mappings[bindingTableLine].SourceLine != 13 || mappings[bindingTableLine].SourceColumn != 1 {
		t.Fatalf("expected binding table to map to view span, got %#v", mappings[bindingTableLine])
	}
	if mappings[updateBindingsLine].SourceLine != 13 || mappings[updateBindingsLine].SourceColumn != 1 {
		t.Fatalf("expected binding updater to map to view span, got %#v", mappings[updateBindingsLine])
	}
	if mappings[renderLine].SourceLine != 13 || mappings[renderLine].SourceColumn != 1 {
		t.Fatalf("expected render function to map to view span, got %#v", mappings[renderLine])
	}
}

type decodedSourceMapMapping struct {
	SourceLine   int
	SourceColumn int
}

func decodeSourceMapMappings(t *testing.T, mappings string) map[int]decodedSourceMapMapping {
	t.Helper()
	out := map[int]decodedSourceMapMapping{}
	previousSourceLine := 0
	previousSourceColumn := 0
	for lineIndex, line := range strings.Split(mappings, ";") {
		if line == "" {
			continue
		}
		segments := strings.Split(line, ",")
		if len(segments) == 0 || segments[0] == "" {
			continue
		}
		values := decodeSourceMapSegment(t, segments[0])
		if len(values) < 4 {
			t.Fatalf("expected source-map segment with four fields, got %#v", values)
		}
		previousSourceLine += values[2]
		previousSourceColumn += values[3]
		out[lineIndex+1] = decodedSourceMapMapping{
			SourceLine:   previousSourceLine + 1,
			SourceColumn: previousSourceColumn + 1,
		}
	}
	return out
}

func decodeSourceMapSegment(t *testing.T, segment string) []int {
	t.Helper()
	var values []int
	for index := 0; index < len(segment); {
		value, next := decodeSourceMapVLQ(t, segment, index)
		values = append(values, value)
		index = next
	}
	return values
}

func decodeSourceMapVLQ(t *testing.T, source string, index int) (int, int) {
	t.Helper()
	shift := 0
	value := 0
	for index < len(source) {
		digit := strings.IndexByte(sourceMapBase64, source[index])
		if digit < 0 {
			t.Fatalf("invalid source-map base64 digit %q in %q", source[index], source)
		}
		index++
		continuation := digit&32 != 0
		digit &= 31
		value += digit << shift
		shift += 5
		if continuation {
			continue
		}
		negative := value&1 == 1
		value >>= 1
		if negative {
			value = -value
		}
		return value, index
	}
	t.Fatalf("unterminated source-map VLQ in %q", source)
	return 0, index
}

func generatedLineContaining(t *testing.T, source, needle string) int {
	t.Helper()
	for index, line := range strings.Split(source, "\n") {
		if strings.Contains(line, needle) {
			return index + 1
		}
	}
	t.Fatalf("missing generated line containing %q", needle)
	return 0
}

func TestBuildProductionModeOmitsJSIslandSourceMaps(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "counter",
			Route: "/counter",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Counter /></main>`,
			},
		}},
		Components: []manifest.Component{counterComponent()},
	}

	result, err := Build(gowdk.Config{Build: gowdk.BuildConfig{Mode: gowdk.Production}}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	jsPath := filepath.Join(outputDir, "assets", "gowdk", "islands", "Counter.js")
	jsMapPath := filepath.Join(outputDir, "assets", "gowdk", "islands", "Counter.js.map")
	if !hasAssetArtifact(result.AssetArtifacts, jsPath) {
		t.Fatalf("expected Counter.js asset, got %#v", result.AssetArtifacts)
	}
	if hasAssetArtifact(result.AssetArtifacts, jsMapPath) {
		t.Fatalf("did not expect Counter.js.map asset in production mode: %#v", result.AssetArtifacts)
	}
	js := readFile(t, jsPath)
	if strings.Contains(js, `sourceMappingURL`) {
		t.Fatalf("did not expect sourceMappingURL in production JS:\n%s", js)
	}
	if strings.Contains(js, "\n  function parseExpression(source)") {
		t.Fatalf("did not expect development indentation in production JS:\n%s", js)
	}
	if !strings.Contains(js, "\nfunction parseExpression(source)") {
		t.Fatalf("expected compact function line in production JS:\n%s", js)
	}
	assetManifestPayload := readFile(t, filepath.Join(outputDir, assetManifestFile))
	if strings.Contains(assetManifestPayload, `"assets/gowdk/islands/Counter.js.map"`) {
		t.Fatalf("did not expect island source map in production asset manifest:\n%s", assetManifestPayload)
	}
	if _, err := os.Stat(jsMapPath); err == nil {
		t.Fatalf("did not expect source map file at %s", jsMapPath)
	}
}

func TestBuildEmitsClientFunctionHandlersForJSIsland(t *testing.T) {
	outputDir := t.TempDir()
	component := counterComponent()
	component.Blocks.Client = true
	component.Blocks.ClientBody = `fn Add(step int) {
  let next int = Count + step
  Count = next
}`
	component.Blocks.ViewBody = `<button g:on:click={Add(Count + 1)}>{Count}</button>`
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "counter",
			Route: "/counter",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Counter /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	result, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	jsPath := filepath.Join(outputDir, "assets", "gowdk", "islands", "Counter.js")
	if !hasAssetArtifact(result.AssetArtifacts, jsPath) {
		t.Fatalf("expected Counter.js asset, got %#v", result.AssetArtifacts)
	}
	html := readFile(t, filepath.Join(outputDir, "counter", "index.html"))
	for _, expected := range []string{
		`data-gowdk-client="{&#34;Add&#34;:{&#34;params&#34;:[&#34;step&#34;],&#34;statements&#34;:[&#34;let next int = Count + step&#34;,&#34;Count = next&#34;]}}"`,
		`data-gowdk-on-click="Add(Count + 1)"`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in island page:\n%s", expected, html)
		}
	}
	js := readFile(t, jsPath)
	for _, expected := range []string{
		`data-gowdk-client`,
		`nextScope[param] = valueOf(args[index] || "", state, scope, helpers);`,
		`let local = expr.match(/^let\s+([A-Za-z_][A-Za-z0-9_]*)\s+[A-Za-z_][A-Za-z0-9_]*\s*=\s*(.+)$/);`,
		`scope[local[1]] = valueOf(local[2], state, scope, helpers);`,
		`function evalExpression(expr, state, scope, helpers, stack)`,
		`applyExpression(statement, state, handlers, helpers, nextScope, refs, computeds, asyncTokens, root, emitEvents)`,
	} {
		if !strings.Contains(js, expected) {
			t.Fatalf("expected %q in generated JS island runtime:\n%s", expected, js)
		}
	}
}

func TestBuildEmitsComponentEventRuntimeForJSIsland(t *testing.T) {
	outputDir := t.TempDir()
	parent := textComponent()
	parent.Name = "Parent"
	parent.Source = "components/parent.cmp.gwdk"
	parent.Blocks.ViewBody = `<Option g:on:select={Query = event.id} />`
	option := textComponent()
	option.Name = "Option"
	option.Source = "components/option.cmp.gwdk"
	option.Emits = []manifest.Emit{{
		Name:   "select",
		Params: []manifest.EmitParam{{Name: "id", Type: "string"}},
	}}
	option.Blocks.ViewBody = `<button g:on:click={emit select(Query)}>{Query}</button>`
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "picker",
			Route: "/picker",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Parent /></main>`,
			},
		}},
		Components: []manifest.Component{parent, option},
	}

	result, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	optionJS := filepath.Join(outputDir, "assets", "gowdk", "islands", "Option.js")
	if !hasAssetArtifact(result.AssetArtifacts, optionJS) {
		t.Fatalf("expected Option.js asset, got %#v", result.AssetArtifacts)
	}
	html := readFile(t, filepath.Join(outputDir, "picker", "index.html"))
	for _, expected := range []string{
		`data-gowdk-parent-on-select="Query = event.id"`,
		`data-gowdk-client="{&#34;emits&#34;:{&#34;select&#34;:{&#34;params&#34;:[&#34;id&#34;]}}}"`,
		`data-gowdk-on-click="emit select(Query)"`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in component event page:\n%s", expected, html)
		}
	}
	js := readFile(t, optionJS)
	for _, expected := range []string{
		`new CustomEvent(name, { detail: payload, bubbles: true })`,
		`data-gowdk-parent-on-`,
		`eventScope.event = customEvent.detail || {};`,
		`const emitEvents = client.emits || {};`,
	} {
		if !strings.Contains(js, expected) {
			t.Fatalf("expected %q in generated component event runtime:\n%s", expected, js)
		}
	}
}

func TestBuildEmitsReactiveComponentPropRuntimeForJSIsland(t *testing.T) {
	outputDir := t.TempDir()
	parent := textComponent()
	parent.Name = "Parent"
	parent.Source = "components/parent.cmp.gwdk"
	parent.Blocks.ViewBody = `<Preview label={Query} />`
	preview := manifest.Component{
		Name:   "Preview",
		Source: "components/preview.cmp.gwdk",
		Props:  []manifest.Prop{{Name: "label", Type: "string"}},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<p>{label}</p>`,
		},
	}
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "preview",
			Route: "/preview",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Parent /></main>`,
			},
		}},
		Components: []manifest.Component{parent, preview},
	}

	result, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	previewJS := filepath.Join(outputDir, "assets", "gowdk", "islands", "Preview.js")
	if !hasAssetArtifact(result.AssetArtifacts, previewJS) {
		t.Fatalf("expected Preview.js asset, got %#v", result.AssetArtifacts)
	}
	html := readFile(t, filepath.Join(outputDir, "preview", "index.html"))
	for _, expected := range []string{
		`<script src="/assets/gowdk/islands/Preview.js" defer></script>`,
		`data-gowdk-props="{&#34;label&#34;:&#34;Query&#34;}"`,
		`<p><span data-gowdk-bind="label" data-gowdk-binding-text="b1">initial</span></p>`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in reactive prop page:\n%s", expected, html)
		}
	}
	js := readFile(t, previewJS)
	for _, expected := range []string{
		`syncChildProps(root, state, helpers)`,
		`root.addEventListener("gowdk:props"`,
		`Object.assign(state, event.detail || {});`,
		`ownsNode(root, node)`,
	} {
		if !strings.Contains(js, expected) {
			t.Fatalf("expected %q in generated reactive prop runtime:\n%s", expected, js)
		}
	}
}

func TestBuildEmitsClientHelperFunctionsForJSIsland(t *testing.T) {
	outputDir := t.TempDir()
	component := counterComponent()
	component.Blocks.Client = true
	component.Blocks.ClientBody = `fn Next(value int) int {
  return value + 1
}

fn Add() {
  Count = Next(Count)
}`
	component.Blocks.ViewBody = `<button g:on:click={Add()}>{Count}</button>`
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "counter",
			Route: "/counter",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Counter /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	result, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	jsPath := filepath.Join(outputDir, "assets", "gowdk", "islands", "Counter.js")
	if !hasAssetArtifact(result.AssetArtifacts, jsPath) {
		t.Fatalf("expected Counter.js asset, got %#v", result.AssetArtifacts)
	}
	html := readFile(t, filepath.Join(outputDir, "counter", "index.html"))
	for _, expected := range []string{
		`&#34;handlers&#34;:{&#34;Add&#34;:{&#34;statements&#34;:[&#34;Count = Next(Count)&#34;]}}`,
		`&#34;helpers&#34;:{&#34;Next&#34;:{&#34;params&#34;:[&#34;value&#34;],&#34;return&#34;:&#34;value + 1&#34;}}`,
		`data-gowdk-on-click="Add()"`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in island page:\n%s", expected, html)
		}
	}
	js := readFile(t, jsPath)
	for _, expected := range []string{
		`function callHelper(name, args, state, helpers, stack)`,
		`return callHelper(expr.name, args, state, helpers, stack);`,
		`const helpers = client.helpers || {};`,
	} {
		if !strings.Contains(js, expected) {
			t.Fatalf("expected %q in generated JS island runtime:\n%s", expected, js)
		}
	}
}

func TestBuildEmitsEventModifierRuntimeForJSIsland(t *testing.T) {
	outputDir := t.TempDir()
	component := counterComponent()
	component.Blocks.ViewBody = `<button g:on:click.prevent.stop.once.capture.debounce(250ms)={Count++}>{Count}</button><button g:on:input.throttle(1s)={Count++}>Throttle</button>`
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "counter",
			Route: "/counter",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Counter /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	html := readFile(t, filepath.Join(outputDir, "counter", "index.html"))
	for _, expected := range []string{
		`data-gowdk-on-click="Count++"`,
		`data-gowdk-event-click="prevent stop once capture debounce:250"`,
		`data-gowdk-event-input="throttle:1000"`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in island page:\n%s", expected, html)
		}
	}
	js := readFile(t, filepath.Join(outputDir, "assets", "gowdk", "islands", "Counter.js"))
	for _, expected := range []string{
		`function eventModifiers(source)`,
		`if (modifiers.prevent) domEvent.preventDefault();`,
		`if (modifiers.stop) domEvent.stopPropagation();`,
		`debounceTimer = setTimeout(() => invoke(domEvent), modifiers.debounce);`,
		`if (now < throttleUntil) return;`,
		`node.addEventListener(event, listener, { once: modifiers.once, capture: modifiers.capture });`,
	} {
		if !strings.Contains(js, expected) {
			t.Fatalf("expected %q in generated JS island runtime:\n%s", expected, js)
		}
	}
}

func TestBuildEmitsLifecycleRuntimeForJSIsland(t *testing.T) {
	outputDir := t.TempDir()
	component := counterComponent()
	component.Blocks.Client = true
	component.Blocks.ClientBody = `on mount {
  Open = true
}

effect when Count {
  Open = false
  return {
    Open = true
  }
}

on destroy {
  Open = false
}`
	component.Blocks.ViewBody = `<button g:on:click={Count++}>{Count}</button>`
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "counter",
			Route: "/counter",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Counter /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	html := readFile(t, filepath.Join(outputDir, "counter", "index.html"))
	for _, expected := range []string{
		`&#34;mount&#34;:[&#34;Open = true&#34;]`,
		`&#34;destroy&#34;:[&#34;Open = false&#34;]`,
		`&#34;effects&#34;:[{&#34;field&#34;:&#34;Count&#34;,&#34;statements&#34;:[&#34;Open = false&#34;],&#34;cleanup&#34;:[&#34;Open = true&#34;]}]`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in island page:\n%s", expected, html)
		}
	}
	js := readFile(t, filepath.Join(outputDir, "assets", "gowdk", "islands", "Counter.js"))
	for _, expected := range []string{
		`const mountStatements = client.mount || [];`,
		`const destroyStatements = client.destroy || [];`,
		`const effects = client.effects || [];`,
		`const effectCleanups = Object.create(null);`,
		`const runEffectCleanup = async (effect) => {`,
		`for (let pass = 0; pass < 10; pass++)`,
		`await runEffectCleanup(effect);`,
		`effectCleanups[effect.field] = effect.cleanup || null;`,
		`await applyStatements(mountStatements, state, handlers, helpers, null, refs, computeds, asyncTokens, root, emitEvents);`,
		`const destroyIsland = async function destroyCounterIsland() {`,
		`registry.roots.delete(root);`,
		`registry.roots.set(root, destroyIsland);`,
		`await runAllEffectCleanups();`,
		`window.addEventListener("pagehide"`,
	} {
		if !strings.Contains(js, expected) {
			t.Fatalf("expected %q in generated JS island runtime:\n%s", expected, js)
		}
	}
}

func TestBuildEmitsDOMRefRuntimeForJSIsland(t *testing.T) {
	outputDir := t.TempDir()
	component := textComponent()
	component.Blocks.Client = true
	component.Blocks.ClientBody = `ref searchInput HTMLInputElement

fn FocusSearch() {
  searchInput.Focus()
}`
	component.Blocks.ViewBody = `<input g:ref={searchInput} /><button g:on:click={FocusSearch()}>Focus</button>`
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "search",
			Route: "/search",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Search /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	html := readFile(t, filepath.Join(outputDir, "search", "index.html"))
	for _, expected := range []string{
		`data-gowdk-ref="searchInput"`,
		`data-gowdk-on-click="FocusSearch()"`,
		`&#34;FocusSearch&#34;:{&#34;statements&#34;:[&#34;searchInput.Focus()&#34;]}`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in island page:\n%s", expected, html)
		}
	}
	js := readFile(t, filepath.Join(outputDir, "assets", "gowdk", "islands", "Search.js"))
	for _, expected := range []string{
		`root.querySelectorAll("[data-gowdk-ref]")`,
		`refs[node.getAttribute("data-gowdk-ref")] = node;`,
		`let refCall = expr.match`,
		`node.focus();`,
	} {
		if !strings.Contains(js, expected) {
			t.Fatalf("expected %q in generated JS island runtime:\n%s", expected, js)
		}
	}
}

func TestBuildEmitsGIfRuntimeUpdatesForJSIsland(t *testing.T) {
	outputDir := t.TempDir()
	component := counterComponent()
	component.Blocks.ViewBody = `<section g:if={Open}><button g:on:click={Open = !Open}>{Count}</button></section><section g:else>Closed</section>`
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "counter",
			Route: "/counter",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Counter /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	html := readFile(t, filepath.Join(outputDir, "counter", "index.html"))
	for _, expected := range []string{
		`<!--gowdk-if:c1-0:start-->`,
		`data-gowdk-if-group="c1" data-gowdk-if-index="0" data-gowdk-binding-if="b1" data-gowdk-if="Open" hidden`,
		`<!--gowdk-if:c1-0:end-->`,
		`<!--gowdk-if:c1-1:start-->`,
		`data-gowdk-if-group="c1" data-gowdk-if-index="1" data-gowdk-binding-if="b4" data-gowdk-else`,
		`<!--gowdk-if:c1-1:end-->`,
		`data-gowdk-on-click="Open = !Open"`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in island page:\n%s", expected, html)
		}
	}
	js := readFile(t, filepath.Join(outputDir, "assets", "gowdk", "islands", "Counter.js"))
	for _, expected := range []string{
		`function conditionalRecords(root, options)`,
		`document.createComment("gowdk-if:" + id)`,
		`function mountConditional(record)`,
		`function unmountConditional(record)`,
		`{ kind: "conditional", selector: "[data-gowdk-binding-if]", id: "data-gowdk-binding-if" },`,
		`else if (spec.kind === "conditional") bindings.conditionals.push({ id, node });`,
		`renderConditionals(root, state, null, helpers, { owner: root, skipLoopItems: true });`,
	} {
		if !strings.Contains(js, expected) {
			t.Fatalf("expected %q in generated JS:\n%s", expected, js)
		}
	}
}

func TestBuildEmitsNestedAndIndexExpressionsForJSIsland(t *testing.T) {
	outputDir := t.TempDir()
	component := nestedComponent()
	component.Blocks.ViewBody = `<section g:if={User.Open && Items[0].Name == "first" && Flags[Count]}>{Count}</section>`
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "nested",
			Route: "/nested",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Nested /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	html := readFile(t, filepath.Join(outputDir, "nested", "index.html"))
	for _, expected := range []string{
		`data-gowdk-if="User.Open &amp;&amp; Items[0].Name == &#34;first&#34; &amp;&amp; Flags[Count]"`,
		`&#34;Items&#34;:[{&#34;Done&#34;:false,&#34;ID&#34;:&#34;first&#34;,&#34;Name&#34;:&#34;first&#34;},{&#34;Done&#34;:true,&#34;ID&#34;:&#34;second&#34;,&#34;Name&#34;:&#34;second&#34;}]`,
		`&#34;User&#34;:{&#34;Name&#34;:&#34;Ada&#34;,&#34;Open&#34;:true}`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in nested island page:\n%s", expected, html)
		}
	}
	if strings.Contains(html, `data-gowdk-if="User.Open`) && strings.Contains(html, ` hidden`) {
		t.Fatalf("expected initial nested condition to render visible:\n%s", html)
	}
	js := readFile(t, filepath.Join(outputDir, "assets", "gowdk", "islands", "Nested.js"))
	for _, forbidden := range []string{`with (env)`, `Function("env"`} {
		if strings.Contains(js, forbidden) {
			t.Fatalf("did not expect dynamic expression evaluation %q in generated JS:\n%s", forbidden, js)
		}
	}
	for _, expected := range []string{
		`function parseExpression(source)`,
		`function evalExpression(expr, state, scope, helpers, stack)`,
		`function evalBinaryExpression(expr, state, scope, helpers, stack)`,
	} {
		if !strings.Contains(js, expected) {
			t.Fatalf("expected compiler-owned expression interpreter %q in generated JS:\n%s", expected, js)
		}
	}
}

func TestBuildEmitsGForListRenderingForJSIsland(t *testing.T) {
	outputDir := t.TempDir()
	component := nestedComponent()
	component.Blocks.Client = true
	component.Blocks.ClientBody = `fn AddItem() {
  append(Items, { ID: "third", Name: "third", Done: false })
}

fn RemoveFirst() {
  remove(Items, 0)
}

fn SwapFirstTwo() {
  move(Items, 1, 0)
}`
	component.Blocks.ViewBody = `<ul><li g:for={item, i in Items} g:key={item.ID}><button g:on:click={remove(Items, i)}>{i}: {item.Name}</button></li></ul><button g:on:click={AddItem()}>Add</button><button g:on:click={RemoveFirst()}>Remove</button><button g:on:click={SwapFirstTwo()}>Swap</button>`
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "list",
			Route: "/list",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Nested /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	html := readFile(t, filepath.Join(outputDir, "list", "index.html"))
	for _, expected := range []string{
		`<template data-gowdk-for="l1" data-gowdk-binding-list="b2" data-gowdk-for-var="item" data-gowdk-for-source="Items" data-gowdk-for-key="item.ID" data-gowdk-for-index-var="i"`,
		`data-gowdk-for-template="&lt;li data-gowdk-for-item=&#34;l1&#34; data-gowdk-key-value=&#34;{{item.ID}}&#34;&gt;&lt;button data-gowdk-on-click=&#34;remove(Items, i)&#34; data-gowdk-binding-on-click=&#34;b1&#34;&gt;{{i}}: {{item.Name}}&lt;/button&gt;&lt;/li&gt;"`,
		`<li data-gowdk-for-item="l1" data-gowdk-key-value="first"><button data-gowdk-on-click="remove(Items, i)" data-gowdk-binding-on-click="b3">0: first</button></li>`,
		`<li data-gowdk-for-item="l1" data-gowdk-key-value="second"><button data-gowdk-on-click="remove(Items, i)" data-gowdk-binding-on-click="b4">1: second</button></li>`,
		`data-gowdk-on-click="AddItem()"`,
		`append(Items, { ID: \&#34;third\&#34;, Name: \&#34;third\&#34;, Done: false })`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in g:for page:\n%s", expected, html)
		}
	}
	js := readFile(t, filepath.Join(outputDir, "assets", "gowdk", "islands", "Nested.js"))
	for _, expected := range []string{
		`function renderListLoops(root, state, helpers, bindings)`,
		`function updateBindings(root, state, helpers, bindings)`,
		`updateTextBindings(bindings, state);`,
		`updateAttrBindings(bindings, state, helpers);`,
		`template[data-gowdk-for]`,
		`call[1] === "append"`,
		`state[field] = state[field].concat([valueOf(args[1], state, scope, helpers)]);`,
		`state[field] = state[field].slice(0, index).concat(state[field].slice(index + 1));`,
		`next.splice(to, 0, item);`,
		`const existing = new Map();`,
		`const key = String(valueOf(keyExpr, state, scope, helpers) ?? "");`,
		`if (reused && !used.has(key)) {`,
		`syncElement(reused, fresh);`,
		`if (!used.has(key) && node.parentNode) node.parentNode.removeChild(node);`,
		`if (indexName) scope[indexName] = index;`,
		`const rerender = () => {`,
		`const scheduleRender = () => {`,
		`data-gowdk-bound-on-`,
		`interpolateTemplate(template, state, scope, helpers)`,
		`renderListLoops(root, state, helpers, bindings);`,
		`scheduleRender();`,
	} {
		if !strings.Contains(js, expected) {
			t.Fatalf("expected %q in generated JS:\n%s", expected, js)
		}
	}
}

func TestBuildEmitsFilterListBindingForJSIsland(t *testing.T) {
	outputDir := t.TempDir()
	component := filterComponent()
	component.Blocks.ViewBody = `<label>Search <input g:bind:value={Query} /></label><ul><li g:for={item in Items} g:key={item.ID} g:if={Query == "" || contains(lower(item.Name), lower(Query))}>{item.Name}</li></ul>`
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "filter",
			Route: "/filter",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Filter /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	html := readFile(t, filepath.Join(outputDir, "filter", "index.html"))
	for _, expected := range []string{
		`data-gowdk-bind-value="Query"`,
		`value="fir"`,
		`data-gowdk-for-source="Items"`,
		`data-gowdk-if="Query == &#34;&#34; || contains(lower(item.Name), lower(Query))"`,
		`data-gowdk-key-value="first"`,
		`>First result</li>`,
		`data-gowdk-key-value="second"`,
		`hidden>Second result</li>`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in filter page:\n%s", expected, html)
		}
	}
	js := readFile(t, filepath.Join(outputDir, "assets", "gowdk", "islands", "Filter.js"))
	for _, expected := range []string{
		`lower(value)`,
		`contains(value, query)`,
		`renderConditionals(fresh, state, scope, helpers);`,
		`matchingNodes(root, "[data-gowdk-binding-if]")`,
		`if (options.skipLoopItems && node.closest("[data-gowdk-for-item]")) return true;`,
	} {
		if !strings.Contains(js, expected) {
			t.Fatalf("expected %q in generated filter JS:\n%s", expected, js)
		}
	}
}

func TestBuildEmitsGoishConditionalExpressionsForJSIsland(t *testing.T) {
	outputDir := t.TempDir()
	component := counterComponent()
	component.Blocks.Client = true
	component.Blocks.ClientBody = `fn ToggleCount() {
  Count = if Open { Count + 1 } else { 0 }
}`
	component.Blocks.ViewBody = `<section g:if={if Open { Count > 0 } else { false }}><button g:on:click={ToggleCount()}>{Count}</button></section>`
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "counter",
			Route: "/counter",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Counter /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	html := readFile(t, filepath.Join(outputDir, "counter", "index.html"))
	for _, expected := range []string{
		`data-gowdk-if="if Open { Count &gt; 0 } else { false }" hidden`,
		`&#34;ToggleCount&#34;:{&#34;statements&#34;:[&#34;Count = if Open { Count + 1 } else { 0 }&#34;]}`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in conditional island page:\n%s", expected, html)
		}
	}
	js := readFile(t, filepath.Join(outputDir, "assets", "gowdk", "islands", "Counter.js"))
	for _, expected := range []string{
		`parseConditional()`,
		`if (this.match("ident", "if"))`,
		`return Boolean(evalExpression(expr.cond, state, scope, helpers, stack))`,
	} {
		if !strings.Contains(js, expected) {
			t.Fatalf("expected %q in generated JS island runtime:\n%s", expected, js)
		}
	}
}

func TestBuildEmitsDOMEventScopeForJSIsland(t *testing.T) {
	outputDir := t.TempDir()
	component := textComponent()
	component.Blocks.ViewBody = `<input g:on:input={Query = event.value} value="" /><p>{Query}</p>`
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "event",
			Route: "/event",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Search /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	js := readFile(t, filepath.Join(outputDir, "assets", "gowdk", "islands", "Search.js"))
	for _, expected := range []string{
		`function domEventScope(domEvent)`,
		`value: target.value == null ? "" : String(target.value)`,
		`await applyExpression(attr.value, state, handlers, helpers, domEventScope(domEvent), refs, computeds, asyncTokens, root, emitEvents);`,
	} {
		if !strings.Contains(js, expected) {
			t.Fatalf("expected %q in generated JS island runtime:\n%s", expected, js)
		}
	}
}

func TestBuildEmitsComputedStateForJSIsland(t *testing.T) {
	outputDir := t.TempDir()
	component := counterComponent()
	component.Blocks.Client = true
	component.Blocks.ClientBody = `computed Label string {
  return if Open { "open" } else { "closed" }
}

computed Visible bool {
  return Label == "open"
}

fn Toggle() {
  Open = !Open
}`
	component.Blocks.ViewBody = `<section g:if={Visible}>{Label}<button g:on:click={Toggle()}>{Count}</button></section>`
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "counter",
			Route: "/counter",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Counter /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	html := readFile(t, filepath.Join(outputDir, "counter", "index.html"))
	for _, expected := range []string{
		`data-gowdk-if="Visible" hidden`,
		`data-gowdk-bind="Label" data-gowdk-binding-text="b2">closed</span>`,
		`&#34;computed&#34;:[{&#34;name&#34;:&#34;Label&#34;,&#34;expr&#34;:&#34;if Open { \&#34;open\&#34; } else { \&#34;closed\&#34; }&#34;},{&#34;name&#34;:&#34;Visible&#34;,&#34;expr&#34;:&#34;Label == \&#34;open\&#34;&#34;}]`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in computed island page:\n%s", expected, html)
		}
	}
	js := readFile(t, filepath.Join(outputDir, "assets", "gowdk", "islands", "Counter.js"))
	for _, expected := range []string{
		`function recomputeComputed(state, computeds, helpers)`,
		`state[computed.name] = valueOf(computed.expr, state, null, helpers);`,
		`const computeds = client.computed || [];`,
		`recomputeComputed(state, computeds, helpers);`,
	} {
		if !strings.Contains(js, expected) {
			t.Fatalf("expected %q in generated JS island runtime:\n%s", expected, js)
		}
	}
}

func TestBuildOrdersComputedDependenciesForJSIsland(t *testing.T) {
	outputDir := t.TempDir()
	component := counterComponent()
	component.Blocks.Client = true
	component.Blocks.ClientBody = `computed Visible bool {
  return Label == "open"
}

computed Label string {
  return if Open { "open" } else { "closed" }
}`
	component.Blocks.ViewBody = `<section g:if={Visible}>{Label}</section>`
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "counter",
			Route: "/counter",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Counter /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	html := readFile(t, filepath.Join(outputDir, "counter", "index.html"))
	expected := `&#34;computed&#34;:[{&#34;name&#34;:&#34;Label&#34;,&#34;expr&#34;:&#34;if Open { \&#34;open\&#34; } else { \&#34;closed\&#34; }&#34;},{&#34;name&#34;:&#34;Visible&#34;,&#34;expr&#34;:&#34;Label == \&#34;open\&#34;&#34;}]`
	if !strings.Contains(html, expected) {
		t.Fatalf("expected dependency-ordered computed bootstrap in page:\n%s", html)
	}
}

func TestBuildEmitsClientBuiltinsForJSIsland(t *testing.T) {
	outputDir := t.TempDir()
	component := manifest.Component{
		Name:    "Nested",
		Source:  "components/nested.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "NestedState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewNestedState"},
		},
		Blocks: manifest.Blocks{
			Client: true,
			ClientBody: `computed ItemCount string {
  return string(len(Items))
}

fn SetCount() {
  Count = len(Items) + int("1")
}`,
			View:     true,
			ViewBody: `<button g:on:click={SetCount()}>{ItemCount}:{Count}</button>`,
		},
	}
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "nested",
			Route: "/nested",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Nested /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	html := readFile(t, filepath.Join(outputDir, "nested", "index.html"))
	for _, expected := range []string{
		`>2:0</button>`,
		`data-gowdk-on-click="SetCount()"`,
		`Count = len(Items) + int(\&#34;1\&#34;)`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in built-in island page:\n%s", expected, html)
		}
	}
	js := readFile(t, filepath.Join(outputDir, "assets", "gowdk", "islands", "Nested.js"))
	for _, expected := range []string{
		`const builtins = Object.freeze({`,
		`len(value) {`,
		`string(value) {`,
		`int(value) {`,
		`float(value) {`,
		`if (Object.prototype.hasOwnProperty.call(builtins, expr.name)) return builtins[expr.name].apply(null, args);`,
	} {
		if !strings.Contains(js, expected) {
			t.Fatalf("expected %q in generated JS island runtime:\n%s", expected, js)
		}
	}
}

func TestBuildEmitsAsyncFetchJSONRuntimeForJSIsland(t *testing.T) {
	outputDir := t.TempDir()
	component := manifest.Component{
		Name:    "Nested",
		Source:  "components/nested.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "NestedState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewNestedState"},
		},
		Blocks: manifest.Blocks{
			Client: true,
			ClientBody: `async fn Refresh() {
  Items = await fetchJSON[[]ui.Item]("/api/items")
}`,
			View:     true,
			ViewBody: `<button g:on:click={Refresh()}>Refresh</button>`,
		},
	}
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "nested",
			Route: "/nested",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Nested /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	html := readFile(t, filepath.Join(outputDir, "nested", "index.html"))
	for _, expected := range []string{
		`data-gowdk-on-click="Refresh()"`,
		`&#34;Refresh&#34;:{&#34;async&#34;:true,&#34;statements&#34;:[&#34;Items = await fetchJSON[[]ui.Item](\&#34;/api/items\&#34;)&#34;]}`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in async island page:\n%s", expected, html)
		}
	}
	js := readFile(t, filepath.Join(outputDir, "assets", "gowdk", "islands", "Nested.js"))
	for _, expected := range []string{
		`const staleAsyncResult = Symbol("gowdk stale async result");`,
		`async function fetchJSON(url, signal)`,
		`function recordAsyncError(state, error)`,
		`async function applyExpression(expr, state, handlers, helpers, scope, refs, computeds, asyncTokens, root, emitEvents)`,
		`const asyncTokens = Object.create(null);`,
		`clearAsyncError(state);`,
		`const token = (asyncTokens[target] || 0) + 1;`,
		`const controllers = asyncTokens.__controllers || (asyncTokens.__controllers = Object.create(null));`,
		`controllers[target].abort();`,
		`const controller = typeof AbortController === "undefined" ? null : new AbortController();`,
		`fetchJSON(valueOf(awaitedFetch[3], state, scope, helpers), controller ? controller.signal : undefined);`,
		`if (asyncTokens[target] !== token) throw staleAsyncResult;`,
		`state[target] = next;`,
		`GOWDK fetchJSON expected JSON response`,
		`GOWDK fetchJSON received invalid JSON`,
		`if (error !== staleAsyncResult) recordAsyncError(state, error);`,
		`await applyExpression(attr.value, state, handlers, helpers, domEventScope(domEvent), refs, computeds, asyncTokens, root, emitEvents);`,
	} {
		if !strings.Contains(js, expected) {
			t.Fatalf("expected %q in generated JS island runtime:\n%s", expected, js)
		}
	}
}

func TestBuildEmitsValueBindingRuntimeForJSIsland(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "search",
			Route: "/search",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Search /></main>`,
			},
		}},
		Components: []manifest.Component{textComponent()},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	html := readFile(t, filepath.Join(outputDir, "search", "index.html"))
	for _, expected := range []string{
		`data-gowdk-bind-value="Query"`,
		`value="initial"`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in binding page:\n%s", expected, html)
		}
	}
	js := readFile(t, filepath.Join(outputDir, "assets", "gowdk", "islands", "Search.js"))
	for _, expected := range []string{
		`const bindingTable = Object.freeze([`,
		`{ kind: "value", selector: "[data-gowdk-binding-value]", id: "data-gowdk-binding-value", field: "data-gowdk-bind-value" },`,
		`else if (spec.kind === "value") bindings.value.push({ id, node, field: node.getAttribute(spec.field) });`,
		`function updateValueBindings(bindings, state)`,
		`state[field] = node.value;`,
		`const event = node.tagName === "SELECT" || node.type === "radio" ? "change" : "input";`,
		`scheduleRender();`,
		`node.addEventListener(event`,
	} {
		if !strings.Contains(js, expected) {
			t.Fatalf("expected %q in generated JS:\n%s", expected, js)
		}
	}
}

func TestBuildEmitsTextareaAndSelectValueBindings(t *testing.T) {
	outputDir := t.TempDir()
	component := textComponent()
	component.Blocks.ViewBody = `<textarea g:bind:value={Query}></textarea><select g:bind:value={Query}><option value="other">Other</option><option value="initial">Initial</option></select>`
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "controls",
			Route: "/controls",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Search /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	html := readFile(t, filepath.Join(outputDir, "controls", "index.html"))
	for _, expected := range []string{
		`<textarea data-gowdk-bind-value="Query" data-gowdk-binding-value="b1">initial</textarea>`,
		`<option value="initial" selected>Initial</option>`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in controls page:\n%s", expected, html)
		}
	}
}

func TestBuildEmitsNumericValueBindingRuntimeForJSIsland(t *testing.T) {
	outputDir := t.TempDir()
	component := counterComponent()
	component.Blocks.ViewBody = `<input type="number" g:bind:value={Count} />`
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "number",
			Route: "/number",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Counter /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	html := readFile(t, filepath.Join(outputDir, "number", "index.html"))
	for _, expected := range []string{
		`type="number"`,
		`data-gowdk-bind-value="Count"`,
		`data-gowdk-bind-type="int"`,
		`value="1"`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in numeric binding page:\n%s", expected, html)
		}
	}
	js := readFile(t, filepath.Join(outputDir, "assets", "gowdk", "islands", "Counter.js"))
	for _, expected := range []string{
		`const type = node.getAttribute("data-gowdk-bind-type") || "string";`,
		`parseInt(node.value, 10)`,
		`parseFloat(node.value)`,
	} {
		if !strings.Contains(js, expected) {
			t.Fatalf("expected %q in generated JS:\n%s", expected, js)
		}
	}
}

func TestBuildEmitsRadioValueBindingRuntimeForJSIsland(t *testing.T) {
	outputDir := t.TempDir()
	component := textComponent()
	component.Blocks.ViewBody = `<input type="radio" name="choice" value="other" g:bind:value={Query} /><input type="radio" name="choice" value="initial" g:bind:value={Query} />`
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "radios",
			Route: "/radios",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Search /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	html := readFile(t, filepath.Join(outputDir, "radios", "index.html"))
	for _, expected := range []string{
		`type="radio"`,
		`data-gowdk-bind-value="Query"`,
		`value="initial" checked`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in radio binding page:\n%s", expected, html)
		}
	}
	js := readFile(t, filepath.Join(outputDir, "assets", "gowdk", "islands", "Search.js"))
	for _, expected := range []string{
		`node.checked = String(state[field] == null ? "" : state[field]) === node.value;`,
		`node.tagName === "SELECT" || node.type === "radio" ? "change" : "input";`,
		`if (!node.checked) return;`,
		`state[field] = node.value;`,
	} {
		if !strings.Contains(js, expected) {
			t.Fatalf("expected %q in generated JS:\n%s", expected, js)
		}
	}
}

func TestBuildEmitsCheckedBindingRuntimeForJSIsland(t *testing.T) {
	outputDir := t.TempDir()
	component := counterComponent()
	component.Blocks.ViewBody = `<input type="checkbox" g:bind:checked={Open} />`
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "toggle",
			Route: "/toggle",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Counter /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	html := readFile(t, filepath.Join(outputDir, "toggle", "index.html"))
	for _, expected := range []string{
		`type="checkbox"`,
		`data-gowdk-bind-checked="Open"`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in checked binding page:\n%s", expected, html)
		}
	}
	js := readFile(t, filepath.Join(outputDir, "assets", "gowdk", "islands", "Counter.js"))
	for _, expected := range []string{
		`{ kind: "checked", selector: "[data-gowdk-binding-checked]", id: "data-gowdk-binding-checked", field: "data-gowdk-bind-checked" },`,
		`else if (spec.kind === "checked") bindings.checked.push({ id, node, field: node.getAttribute(spec.field) });`,
		`state[field] = node.checked;`,
		`node.addEventListener("change"`,
	} {
		if !strings.Contains(js, expected) {
			t.Fatalf("expected %q in generated JS:\n%s", expected, js)
		}
	}
}

func TestBuildEmitsReactiveAttributeRuntimeForJSIsland(t *testing.T) {
	outputDir := t.TempDir()
	component := counterComponent()
	component.Blocks.ViewBody = `<button disabled={Open} aria-expanded={Open} g:on:click={Open = !Open}>{Count}</button>`
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "attrs",
			Route: "/attrs",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Counter /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	html := readFile(t, filepath.Join(outputDir, "attrs", "index.html"))
	for _, expected := range []string{
		`data-gowdk-attr-disabled="Open"`,
		`data-gowdk-attr-aria-expanded="Open"`,
		`aria-expanded="false"`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in reactive attr page:\n%s", expected, html)
		}
	}
	js := readFile(t, filepath.Join(outputDir, "assets", "gowdk", "islands", "Counter.js"))
	for _, expected := range []string{
		`data-gowdk-attr-`,
		`booleanAttrs.has(name)`,
		`node.setAttribute(name, String(value));`,
	} {
		if !strings.Contains(js, expected) {
			t.Fatalf("expected %q in generated JS:\n%s", expected, js)
		}
	}
}

func TestBuildEmitsClassToggleRuntimeForJSIsland(t *testing.T) {
	outputDir := t.TempDir()
	component := counterComponent()
	component.Blocks.ViewBody = `<button class="base" class:active={Open} g:on:click={Open = !Open}>{Count}</button>`
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "classes",
			Route: "/classes",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Counter /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	html := readFile(t, filepath.Join(outputDir, "classes", "index.html"))
	for _, expected := range []string{
		`data-gowdk-class-active="Open"`,
		`class="base"`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in class toggle page:\n%s", expected, html)
		}
	}
	js := readFile(t, filepath.Join(outputDir, "assets", "gowdk", "islands", "Counter.js"))
	for _, expected := range []string{
		`data-gowdk-class-`,
		`{ kind: "class", attrPrefix: "data-gowdk-binding-class-", valuePrefix: "data-gowdk-class-" },`,
		`function collectPrefixBinding(bindings, spec, node, attr)`,
		`function updateClassBindings(bindings, state, helpers)`,
		`node.classList.toggle(name, Boolean(valueOf(expression, state, null, helpers)));`,
		`const scheduleRender = () => {`,
	} {
		if !strings.Contains(js, expected) {
			t.Fatalf("expected %q in generated JS:\n%s", expected, js)
		}
	}
}

func TestBuildEmitsStyleBindingRuntimeForJSIsland(t *testing.T) {
	outputDir := t.TempDir()
	component := counterComponent()
	component.Blocks.ViewBody = `<div style="color: red" style:height.px={Count} g:on:click={Count++}>{Count}</div>`
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "styles",
			Route: "/styles",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Counter /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	html := readFile(t, filepath.Join(outputDir, "styles", "index.html"))
	for _, expected := range []string{
		`data-gowdk-style-height="Count"`,
		`data-gowdk-style-unit-height="px"`,
		`style="color: red; height: 1px"`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in style binding page:\n%s", expected, html)
		}
	}
	js := readFile(t, filepath.Join(outputDir, "assets", "gowdk", "islands", "Counter.js"))
	for _, expected := range []string{
		`data-gowdk-style-`,
		`{ kind: "style", attrPrefix: "data-gowdk-binding-style-", valuePrefix: "data-gowdk-style-", unitPrefix: "data-gowdk-style-unit-" },`,
		`node.style.setProperty(name, String(value) + unit);`,
		`node.style.removeProperty(name);`,
	} {
		if !strings.Contains(js, expected) {
			t.Fatalf("expected %q in generated JS:\n%s", expected, js)
		}
	}
}

func TestBuildSerializesStateInitByGoFieldName(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "tagged",
			Route: "/tagged",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><TaggedCounter /></main>`,
			},
		}},
		Components: []manifest.Component{taggedCounterComponent()},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	html := readFile(t, filepath.Join(outputDir, "tagged", "index.html"))
	for _, expected := range []string{
		`data-gowdk-state="{&#34;Count&#34;:0}"`,
		`data-gowdk-bind="Count" data-gowdk-binding-text="b1">0</span>`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in tagged state page:\n%s", expected, html)
		}
	}
}

func TestBuildEmitsWASMIslandAssetsOnlyWhenExplicit(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "counter",
			Route: "/counter",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Counter g:island="wasm" /></main>`,
			},
		}},
		Components: []manifest.Component{counterComponent()},
	}

	result, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	jsPath := filepath.Join(outputDir, "assets", "gowdk", "islands", "Counter.js")
	wasmPath := filepath.Join(outputDir, "assets", "gowdk", "islands", "Counter.wasm")
	loaderPath := filepath.Join(outputDir, "assets", "gowdk", "islands", "Counter.wasm.js")
	if hasAssetArtifact(result.AssetArtifacts, jsPath) {
		t.Fatalf("did not expect default JS asset for explicit wasm usage: %#v", result.AssetArtifacts)
	}
	if !hasAssetArtifact(result.AssetArtifacts, wasmPath) || !hasAssetArtifact(result.AssetArtifacts, loaderPath) {
		t.Fatalf("expected wasm and loader assets, got %#v", result.AssetArtifacts)
	}
	html := readFile(t, filepath.Join(outputDir, "counter", "index.html"))
	for _, expected := range []string{
		`<script src="/assets/gowdk/islands/Counter.wasm.js" defer></script>`,
		`data-gowdk-island="i1"`,
		`data-gowdk-runtime="wasm"`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in wasm island page:\n%s", expected, html)
		}
	}
	wasm := readBytes(t, wasmPath)
	if !bytes.Equal(wasm, []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}) {
		t.Fatalf("expected minimal valid wasm module, got %#v", wasm)
	}
	loader := readFile(t, loaderPath)
	for _, expected := range []string{
		`const wasmExecPath = "/assets/gowdk/islands/wasm_exec.js";`,
		`const mountExport = "GOWDKMount" + component;`,
		`const handleExport = "GOWDKHandle" + component;`,
		`const destroyExport = "GOWDKDestroy" + component;`,
		`state: parseJSON(root.getAttribute("data-gowdk-state"), {}),`,
		`props: parseJSON(root.getAttribute("data-gowdk-props"), {}),`,
		`bindings: collectBindings(root)`,
		`applyPatches(root, callExport(exports, mountExport, mountPayload));`,
		`node.addEventListener(event`,
		`binding: attr.value,`,
		`applyPatches(root, callExport(exports, handleExport`,
		`applyPatches(root, callExport(exports, destroyExport`,
		`if (patch.type === "setText" && node) node.textContent`,
		`else if (patch.type === "setHidden" && node) node.hidden`,
		`else if (patch.type === "replaceList" && node) node.innerHTML`,
		`else if (patch.type === "emit" && patch.name) root.dispatchEvent`,
		`console.error("GOWDK WASM island rejected patch"`,
		`console.error("GOWDK WASM island missing exports"`,
		`const go = await loadGoRuntime();`,
		`go.run(result.instance)`,
	} {
		if !strings.Contains(loader, expected) {
			t.Fatalf("expected %q in wasm loader:\n%s", expected, loader)
		}
	}
	assetManifestPayload := readFile(t, filepath.Join(outputDir, assetManifestFile))
	for _, expected := range []string{
		`"assets/gowdk/islands/Counter.wasm": "assets/gowdk/islands/Counter.wasm"`,
		`"assets/gowdk/islands/Counter.wasm.js": "assets/gowdk/islands/Counter.wasm.js"`,
	} {
		if !strings.Contains(assetManifestPayload, expected) {
			t.Fatalf("expected %q in asset manifest:\n%s", expected, assetManifestPayload)
		}
	}
	if strings.Contains(assetManifestPayload, `"assets/gowdk/islands/wasm_exec.js"`) {
		t.Fatalf("did not expect Go wasm runtime for placeholder wasm island:\n%s", assetManifestPayload)
	}
}

func TestWASMIslandLoaderRunsInBrowser(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}
	chromium, err := lookupChromium()
	if err != nil {
		t.Skip(err)
	}
	requireNodePlaywright(t, node)

	outputDir := t.TempDir()
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "counter",
			Route: "/counter",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Counter g:island="wasm" /></main>`,
			},
		}},
		Components: []manifest.Component{counterComponent()},
	}
	if _, err := Build(gowdk.Config{}, app, outputDir); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.FileServer(http.Dir(outputDir)))
	defer server.Close()

	script := filepath.Join(t.TempDir(), "gowdk-wasm-island-browser-test.cjs")
	if err := os.WriteFile(script, []byte(wasmIslandBrowserHarness()), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, node, script, server.URL, chromium)
	command.Dir = mustWorkingDir(t)
	output, err := command.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("browser wasm island test timed out:\n%s", output)
	}
	if err != nil {
		t.Fatalf("browser wasm island test failed: %v\n%s", err, output)
	}
}

func TestBuildCompilesDeclaredWASMIslandPackage(t *testing.T) {
	packageDir := writeWASMIslandPackage(t, "main", requiredWASMExportsSource("Counter"))
	outputDir := t.TempDir()
	component := counterComponent()
	component.WASM = manifest.WASMContract{Package: packageDir}
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "counter",
			Route: "/counter",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Counter /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	result, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	wasmPath := filepath.Join(outputDir, "assets", "gowdk", "islands", "Counter.wasm")
	wasmExecPath := filepath.Join(outputDir, "assets", "gowdk", "islands", "wasm_exec.js")
	if !hasAssetArtifact(result.AssetArtifacts, wasmPath) {
		t.Fatalf("expected compiled wasm asset, got %#v", result.AssetArtifacts)
	}
	if !hasAssetArtifact(result.AssetArtifacts, wasmExecPath) {
		t.Fatalf("expected Go wasm_exec.js runtime asset, got %#v", result.AssetArtifacts)
	}
	html := readFile(t, filepath.Join(outputDir, "counter", "index.html"))
	for _, expected := range []string{
		`<script src="/assets/gowdk/islands/Counter.wasm.js" defer></script>`,
		`data-gowdk-runtime="wasm"`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in declared wasm component page:\n%s", expected, html)
		}
	}
	wasm := readBytes(t, wasmPath)
	if !bytes.HasPrefix(wasm, wasmMagic) {
		t.Fatalf("expected browser wasm module, got %#v", wasm[:min(len(wasm), 8)])
	}
	if bytes.Equal(wasm, []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}) {
		t.Fatal("expected compiled Go wasm, got placeholder module")
	}
	wasmExec := readFile(t, wasmExecPath)
	if !strings.Contains(wasmExec, "globalThis.Go = class") {
		t.Fatalf("expected Go wasm runtime asset, got:\n%s", wasmExec[:min(len(wasmExec), 256)])
	}
}

func TestBuildCompilesSPAGoBlockBrowserMount(t *testing.T) {
	sourceDir := t.TempDir()
	outputDir := t.TempDir()
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:     "home",
			Route:  "/",
			Source: filepath.Join(sourceDir, "home.gwdk"),
			Blocks: manifest.Blocks{
				GoBlocks: []manifest.GoBlock{{
					Target: "spa",
					Body: `//go:wasmexport GOWDKMountHome
func GOWDKMountHome() uint32 { return 0 }
`,
				}},
				View:     true,
				ViewBody: `<main><h1>Home</h1></main>`,
			},
		}},
	}

	result, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	wasmPath := filepath.Join(outputDir, "assets", "gowdk", "islands", "pages", "Home.wasm")
	loaderPath := filepath.Join(outputDir, "assets", "gowdk", "islands", "pages", "Home.wasm.js")
	wasmExecPath := filepath.Join(outputDir, "assets", "gowdk", "islands", "wasm_exec.js")
	for _, path := range []string{wasmPath, loaderPath, wasmExecPath} {
		if !hasAssetArtifact(result.AssetArtifacts, path) {
			t.Fatalf("expected SPA browser go block asset %s, got %#v", path, result.AssetArtifacts)
		}
	}
	html := readFile(t, filepath.Join(outputDir, "index.html"))
	if !strings.Contains(html, `<script src="/assets/gowdk/islands/pages/Home.wasm.js" defer></script>`) {
		t.Fatalf("expected SPA browser go block loader in page:\n%s", html)
	}
	wasm := readBytes(t, wasmPath)
	if !bytes.HasPrefix(wasm, wasmMagic) {
		t.Fatalf("expected browser wasm module, got %#v", wasm[:min(len(wasm), 8)])
	}
	loader := readFile(t, loaderPath)
	for _, expected := range []string{
		`const mountExport = "GOWDKMountHome";`,
		`window.__gowdkMountSPAGoBlocks`,
		`exports[mountExport]();`,
		`go.run(result.instance)`,
		`GOWDK SPA go block mount failed`,
	} {
		if !strings.Contains(loader, expected) {
			t.Fatalf("expected %q in SPA browser go block loader:\n%s", expected, loader)
		}
	}
	assetManifestPayload := readFile(t, filepath.Join(outputDir, assetManifestFile))
	for _, expected := range []string{
		`"assets/gowdk/islands/pages/Home.wasm": "assets/gowdk/islands/pages/Home.wasm"`,
		`"assets/gowdk/islands/pages/Home.wasm.js": "assets/gowdk/islands/pages/Home.wasm.js"`,
		`"assets/gowdk/islands/wasm_exec.js": "assets/gowdk/islands/wasm_exec.js"`,
	} {
		if !strings.Contains(assetManifestPayload, expected) {
			t.Fatalf("expected %q in asset manifest:\n%s", expected, assetManifestPayload)
		}
	}
}

func TestBuildKeepsStaticSPAGoBlockOutOfBrowser(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "home",
			Route: "/",
			Blocks: manifest.Blocks{
				GoBlocks: []manifest.GoBlock{{
					Target: "spa",
					Body: `func HomePageForBuild() map[string]string {
	return map[string]string{"title": "GOWDK ships apps"}
}
`,
				}},
				View:     true,
				ViewBody: `<main><h1>Home</h1></main>`,
			},
		}},
	}

	result, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		filepath.Join(outputDir, "assets", "gowdk", "islands", "pages", "Home.wasm"),
		filepath.Join(outputDir, "assets", "gowdk", "islands", "pages", "Home.wasm.js"),
		filepath.Join(outputDir, "assets", "gowdk", "islands", "wasm_exec.js"),
	} {
		if hasAssetArtifact(result.AssetArtifacts, path) {
			t.Fatalf("did not expect static SPA go block to emit browser asset %s: %#v", path, result.AssetArtifacts)
		}
	}
	html := readFile(t, filepath.Join(outputDir, "index.html"))
	if strings.Contains(html, `Home.wasm.js`) {
		t.Fatalf("did not expect SPA browser go block loader in static script page:\n%s", html)
	}
}

func TestBuildRejectsWASMIslandPackageMissingABIExports(t *testing.T) {
	packageDir := writeWASMIslandPackage(t, "main", `func main() {}`)
	outputDir := t.TempDir()
	component := counterComponent()
	component.WASM = manifest.WASMContract{Package: packageDir}
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "counter",
			Route: "/counter",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Counter g:island="wasm" /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err == nil {
		t.Fatal("expected missing wasm export error")
	}
	for _, expected := range []string{
		`missing required WASM exports`,
		`GOWDKMountCounter`,
		`GOWDKHandleCounter`,
		`GOWDKDestroyCounter`,
	} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("expected %q in error: %v", expected, err)
		}
	}
	requireBuildDiagnostic(t, err, "wasm_package_export_error", "Counter")
}

func TestBuildRejectsWASMIslandPackageWithoutMainEntrypoint(t *testing.T) {
	packageDir := writeWASMIslandPackage(t, "counter", `func NotMain() {}`)
	outputDir := t.TempDir()
	component := counterComponent()
	component.WASM = manifest.WASMContract{Package: packageDir}
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "counter",
			Route: "/counter",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Counter g:island="wasm" /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err == nil {
		t.Fatal("expected non-main wasm package to fail")
	}
	if !strings.Contains(err.Error(), "did not produce a browser WASM module") ||
		!strings.Contains(err.Error(), "declare a package main with a main function") {
		t.Fatalf("unexpected error: %v", err)
	}
	requireBuildDiagnostic(t, err, "wasm_package_entrypoint_error", "Counter")
}

func TestBuildSurfacesWASMIslandPackageImportErrors(t *testing.T) {
	packageDir := writeWASMIslandPackage(t, "main", `import _ "example.com/gowdkwasmtest/missing"

func main() {}`)
	outputDir := t.TempDir()
	component := counterComponent()
	component.WASM = manifest.WASMContract{Package: packageDir}
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "counter",
			Route: "/counter",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Counter g:island="wasm" /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err == nil {
		t.Fatal("expected wasm package import error")
	}
	for _, expected := range []string{
		`component Counter wasm package`,
		`GOOS=js GOARCH=wasm`,
		`example.com/gowdkwasmtest/missing`,
	} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("expected %q in error: %v", expected, err)
		}
	}
	requireBuildDiagnostic(t, err, "wasm_package_build_error", "Counter")
}

func TestBuildRejectsUnsupportedWASMIslandPackageImports(t *testing.T) {
	packageDir := writeWASMIslandPackage(t, "main", `import _ "os/exec"

func main() {}`)
	outputDir := t.TempDir()
	component := counterComponent()
	component.WASM = manifest.WASMContract{Package: packageDir}
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "counter",
			Route: "/counter",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Counter g:island="wasm" /></main>`,
			},
		}},
		Components: []manifest.Component{component},
	}

	_, err := Build(gowdk.Config{}, app, outputDir)
	if err == nil {
		t.Fatal("expected unsupported wasm import error")
	}
	for _, expected := range []string{
		`component Counter wasm package`,
		`imports unsupported browser package "os/exec"`,
		`process execution is not available in browser WASM islands`,
	} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("expected %q in error: %v", expected, err)
		}
	}
	requireBuildDiagnostic(t, err, "unsupported_wasm_import", "Counter")
}

func requireBuildDiagnostic(t *testing.T, err error, code string, componentName string) {
	t.Helper()
	var buildErr *BuildError
	if !errors.As(err, &buildErr) {
		t.Fatalf("expected BuildError, got %T: %v", err, err)
	}
	if len(buildErr.Diagnostics) != 1 {
		t.Fatalf("expected one build diagnostic, got %#v", buildErr.Diagnostics)
	}
	diagnostic := buildErr.Diagnostics[0]
	if diagnostic.Code != code || diagnostic.ComponentName != componentName {
		t.Fatalf("unexpected build diagnostic: %#v", diagnostic)
	}
}

func writeWASMIslandPackage(t *testing.T, packageName, body string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/gowdkwasmtest\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	packageDir := filepath.Join(root, "browser", "counter")
	if err := os.MkdirAll(packageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	source := "package " + packageName + "\n\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(packageDir, "counter.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	return packageDir
}

func requiredWASMExportsSource(component string) string {
	return `//go:wasmexport GOWDKMount` + component + `
func GOWDKMount` + component + `() uint32 { return 0 }

//go:wasmexport GOWDKHandle` + component + `
func GOWDKHandle` + component + `() uint32 { return 0 }

//go:wasmexport GOWDKDestroy` + component + `
func GOWDKDestroy` + component + `() uint32 { return 0 }

func main() {}`
}

func lookupChromium() (string, error) {
	for _, name := range []string{"chromium", "chromium-browser", "google-chrome"} {
		path, err := exec.LookPath(name)
		if err == nil {
			return path, nil
		}
	}
	return "", errors.New("chromium is not installed")
}

func requireNodePlaywright(t *testing.T, node string) {
	t.Helper()
	command := exec.Command(node, "-e", `const module = require("node:module"); module.createRequire(process.cwd() + "/gowdk-test.js").resolve("playwright");`)
	command.Dir = mustWorkingDir(t)
	if output, err := command.CombinedOutput(); err != nil {
		t.Skipf("playwright is not installed: %v\n%s", err, output)
	}
}

func mustWorkingDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func jsIslandStoreBrowserHarness() string {
	return `
"use strict";

const assert = require("node:assert/strict");
const nodeModule = require("node:module");

const baseURL = process.argv[2];
const executablePath = process.argv[3];
const { chromium } = nodeModule.createRequire(process.cwd() + "/gowdk-test.js")("playwright");

async function waitForButtonTexts(page, expected) {
  await page.waitForFunction((expected) => {
    const values = Array.from(document.querySelectorAll("gowdk-island button")).map((node) => node.textContent);
    return values.length === expected.length && values.every((value, index) => value === expected[index]);
  }, expected);
}

(async () => {
  const browser = await chromium.launch({
    executablePath,
    headless: true,
    args: ["--no-sandbox"]
  });
  const page = await browser.newPage();
  const consoleErrors = [];
  page.on("console", (message) => {
    if (message.type() === "error" && message.text().includes("GOWDK")) consoleErrors.push(message.text());
  });

  await page.goto(baseURL + "/counter/", { waitUntil: "networkidle" });
  await waitForButtonTexts(page, ["1", "1"]);
  assert.equal(await page.evaluate(() => window.__gowdkStores.get("cart").Count), 1);

  await page.evaluate(() => document.querySelectorAll("gowdk-island button")[0].click());
  await waitForButtonTexts(page, ["2", "2"]);
  assert.equal(await page.evaluate(() => window.__gowdkStores.get("cart").Count), 2);

  await page.evaluate(() => document.querySelectorAll("gowdk-island button")[1].click());
  await waitForButtonTexts(page, ["3", "3"]);
  assert.equal(await page.evaluate(() => window.__gowdkStores.get("cart").Count), 3);
  assert.deepEqual(consoleErrors, []);
  await browser.close();
})().catch(async (error) => {
  console.error(error && error.stack || error);
  process.exit(1);
});
`
}

func jsIslandEffectsBrowserHarness() string {
	return `
"use strict";

const assert = require("node:assert/strict");
const nodeModule = require("node:module");

const baseURL = process.argv[2];
const executablePath = process.argv[3];
const { chromium } = nodeModule.createRequire(process.cwd() + "/gowdk-test.js")("playwright");

async function waitForText(page, selector, expected) {
  await page.waitForFunction(({ selector, expected }) => {
    return document.querySelector(selector)?.textContent === expected;
  }, { selector, expected });
}

(async () => {
  const browser = await chromium.launch({
    executablePath,
    headless: true,
    args: ["--no-sandbox"]
  });
  const page = await browser.newPage();
  const consoleErrors = [];
  page.on("console", (message) => {
    if (message.type() === "error" && message.text().includes("GOWDK")) consoleErrors.push(message.text());
  });

  await page.goto(baseURL + "/counter/", { waitUntil: "networkidle" });
  await waitForText(page, "#count", "1");
  await waitForText(page, "#open", "false");
  await page.evaluate(() => {
    window.__gowdkCountTextChanges = [];
    const target = document.querySelector("#count");
    new MutationObserver(() => {
      window.__gowdkCountTextChanges.push(target.textContent);
    }).observe(target, { childList: true, subtree: true, characterData: true });
  });

  await page.click("#burst");
  await waitForText(page, "#count", "3");
  assert.deepEqual(await page.evaluate(() => window.__gowdkCountTextChanges), ["3"]);

  await page.click("#flip");
  await waitForText(page, "#open", "true");
  await waitForText(page, "#count", "4");

  await page.click("#flip");
  await waitForText(page, "#open", "false");
  await waitForText(page, "#count", "21");
  assert.deepEqual(consoleErrors, []);
  await browser.close();
})().catch(async (error) => {
  console.error(error && error.stack || error);
  process.exit(1);
});
`
}

func wasmIslandBrowserHarness() string {
	return `
"use strict";

const assert = require("node:assert/strict");
const nodeModule = require("node:module");

const baseURL = process.argv[2];
const executablePath = process.argv[3];
const { chromium } = nodeModule.createRequire(process.cwd() + "/gowdk-test.js")("playwright");

async function waitForCall(calls, kind) {
  const started = Date.now();
  while (Date.now() - started < 5000) {
    const call = calls.find((item) => item.kind === kind);
    if (call) return call;
    await new Promise((resolve) => setTimeout(resolve, 25));
  }
  throw new Error("timed out waiting for " + kind + " call; got " + JSON.stringify(calls));
}

(async () => {
  const calls = [];
  const browser = await chromium.launch({
    executablePath,
    headless: true,
    args: ["--no-sandbox"]
  });
  const page = await browser.newPage();
  const consoleErrors = [];
  page.on("console", (message) => {
    if (message.type() === "error" && message.text().includes("GOWDK")) consoleErrors.push(message.text());
  });
  await page.exposeFunction("gowdkRecordWASMCall", (call) => {
    calls.push(call);
  });
  await page.addInitScript(() => {
    const exports = {
      GOWDKMountCounter(payload) {
        window.gowdkRecordWASMCall({ kind: "mount", payload });
        return [{ type: "setText", target: "b2", value: "mounted" }];
      },
      GOWDKHandleCounter(payload) {
        window.gowdkRecordWASMCall({ kind: "handle", payload });
        return [
          { type: "setText", target: "b2", value: "clicked" },
          { type: "emit", name: "counter-ready", detail: { count: 2 } }
        ];
      },
      GOWDKDestroyCounter(payload) {
        window.gowdkRecordWASMCall({ kind: "destroy", payload });
        return [];
      }
    };
    WebAssembly.instantiateStreaming = async () => ({ instance: { exports } });
    WebAssembly.instantiate = async () => ({ instance: { exports } });
  });

  await page.goto(baseURL + "/counter/", { waitUntil: "networkidle" });
  await page.waitForFunction(() => document.querySelector("[data-gowdk-binding-text='b2']")?.textContent === "mounted");
  const mount = await waitForCall(calls, "mount");
  assert.equal(mount.payload.component, "Counter");
  assert.equal(mount.payload.state.Count, 1);
  assert.equal(mount.payload.bindings.text[0].field, "Count");
  assert.equal(mount.payload.bindings.events[0].event, "click");

  await page.evaluate(() => {
    window.__gowdkWASMEmit = null;
    document.querySelector("gowdk-island").addEventListener("counter-ready", (event) => {
      window.__gowdkWASMEmit = event.detail;
    });
  });
  await page.click("button");
  await page.waitForFunction(() => document.querySelector("[data-gowdk-binding-text='b2']")?.textContent === "clicked");
  const handle = await waitForCall(calls, "handle");
  assert.equal(handle.payload.event, "click");
  assert.ok(handle.payload.binding);
  assert.deepEqual(await page.evaluate(() => window.__gowdkWASMEmit), { count: 2 });

  await page.goto("about:blank");
  const destroy = await waitForCall(calls, "destroy");
  assert.equal(destroy.payload.component, "Counter");
  assert.deepEqual(consoleErrors, []);
  await browser.close();
})().catch(async (error) => {
  console.error(error && error.stack || error);
  process.exit(1);
});
`
}

func TestBuildAllowsJSAndWASMIslandsOnSamePage(t *testing.T) {
	outputDir := t.TempDir()
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "mixed",
			Route: "/mixed",
			Blocks: manifest.Blocks{
				View:     true,
				ViewBody: `<main><Counter /><TaggedCounter g:island="wasm" /></main>`,
			},
		}},
		Components: []manifest.Component{counterComponent(), taggedCounterComponent()},
	}

	result, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	counterJS := filepath.Join(outputDir, "assets", "gowdk", "islands", "Counter.js")
	counterMap := filepath.Join(outputDir, "assets", "gowdk", "islands", "Counter.js.map")
	taggedWASM := filepath.Join(outputDir, "assets", "gowdk", "islands", "TaggedCounter.wasm")
	taggedLoader := filepath.Join(outputDir, "assets", "gowdk", "islands", "TaggedCounter.wasm.js")
	for _, path := range []string{counterJS, counterMap, taggedWASM, taggedLoader} {
		if !hasAssetArtifact(result.AssetArtifacts, path) {
			t.Fatalf("expected mixed island asset %s, got %#v", path, result.AssetArtifacts)
		}
	}
	html := readFile(t, filepath.Join(outputDir, "mixed", "index.html"))
	for _, expected := range []string{
		`<script src="/assets/gowdk/islands/Counter.js" defer></script>`,
		`<script src="/assets/gowdk/islands/TaggedCounter.wasm.js" defer></script>`,
		`data-gowdk-component="Counter" data-gowdk-island="i1" data-gowdk-runtime="js"`,
		`data-gowdk-component="TaggedCounter" data-gowdk-island="i2" data-gowdk-runtime="wasm"`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in mixed island page:\n%s", expected, html)
		}
	}
}
