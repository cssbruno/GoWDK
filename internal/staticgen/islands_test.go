package staticgen

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/manifest"
)

func TestBuildEmitsJSIslandAssetsForStatefulComponent(t *testing.T) {
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
		!strings.Contains(js, `applyExpression`) ||
		!strings.Contains(js, `window.__gowdkMountIslands`) ||
		!strings.Contains(js, `window.__gowdkDestroyIslands`) ||
		!strings.Contains(js, `registry.components[component] = mountComponent`) ||
		!strings.Contains(js, `registry.roots`) ||
		!strings.Contains(js, `data-gowdk-mounted`) {
		t.Fatalf("expected generated JS island runtime, got:\n%s", js)
	}
	assetManifestPayload := readFile(t, filepath.Join(outputDir, assetManifestFile))
	if !strings.Contains(assetManifestPayload, `"assets/gowdk/islands/Counter.js": "assets/gowdk/islands/Counter.js"`) {
		t.Fatalf("expected island JS in asset manifest:\n%s", assetManifestPayload)
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
		`with (env) { return (`,
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
		`env[name] = (...args) => callHelper(name, args, state, helpers, stack || []);`,
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
		`debounceTimer = setTimeout(invoke, modifiers.debounce);`,
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
		`const destroyIsland = async () => {`,
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
		`const conditionalGroups = new Map();`,
		`bindings.conditionals.push({ id: node.getAttribute("data-gowdk-binding-if"), node });`,
		`renderConditionals(root, state, null, helpers, { owner: root, skipLoopItems: true, bindings });`,
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
	if !strings.Contains(js, `with (env) { return (`) {
		t.Fatalf("expected expression lowering path in generated JS:\n%s", js)
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
		`syncElement(reused, fresh);`,
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
		`matchingNodes(container, "[data-gowdk-if]:not([data-gowdk-if-group]), [data-gowdk-if-group]")`,
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
		`function expressionSource(source)`,
		`return "(" + expressionSource(cond) + " ? " + expressionSource(thenExpr) + " : " + expressionSource(elseExpr) + ")"`,
		`return Function("env", "with (env) { return (" + expressionSource(token) + "); }")(env);`,
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
		`Object.assign(Object.create(null), builtins, state, scope || {})`,
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
		`await applyExpression(attr.value, state, handlers, helpers, null, refs, computeds, asyncTokens, root, emitEvents);`,
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
		`bindings.value.push({ id: node.getAttribute("data-gowdk-binding-value"), node, field: node.getAttribute("data-gowdk-bind-value") });`,
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
		`bindings.checked.push({ id: node.getAttribute("data-gowdk-binding-checked"), node, field: node.getAttribute("data-gowdk-bind-checked") });`,
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
	assetManifestPayload := readFile(t, filepath.Join(outputDir, assetManifestFile))
	for _, expected := range []string{
		`"assets/gowdk/islands/Counter.wasm": "assets/gowdk/islands/Counter.wasm"`,
		`"assets/gowdk/islands/Counter.wasm.js": "assets/gowdk/islands/Counter.wasm.js"`,
	} {
		if !strings.Contains(assetManifestPayload, expected) {
			t.Fatalf("expected %q in asset manifest:\n%s", expected, assetManifestPayload)
		}
	}
}
