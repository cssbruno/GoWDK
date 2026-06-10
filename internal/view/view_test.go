package view

import (
	"strings"
	"testing"

	"github.com/cssbruno/gowdk/internal/clientlang"
)

func TestRenderSPAEscapesTextAndAttributes(t *testing.T) {
	got, err := RenderSPA(`<main class="hero & lead"><h1>GOWDK & friends</h1><input disabled /></main>`)
	if err != nil {
		t.Fatal(err)
	}
	want := `<main class="hero &amp; lead"><h1>GOWDK &amp; friends</h1><input disabled></main>`
	if got != want {
		t.Fatalf("unexpected HTML:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenderSPADecodesSourceTextEntitiesBeforeEscaping(t *testing.T) {
	got, err := RenderSPA(`<pre><code>expected=&#34;$(awk &#39;$2 == &#34;gowdk-darwin-amd64&#34; &#123; print $1 &#125;&#39;)&#34;
Project &lt;file&gt; stays literal.</code></pre>`)
	if err != nil {
		t.Fatal(err)
	}
	want := `<pre><code>expected=&#34;$(awk &#39;$2 == &#34;gowdk-darwin-amd64&#34; { print $1 }&#39;)&#34;
Project &lt;file&gt; stays literal.</code></pre>`
	if got != want {
		t.Fatalf("unexpected HTML:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenderSPAExpandsClassAndIDShorthand(t *testing.T) {
	got, err := RenderSPA(`<main #hero .text-4xl .font-bold class="lead">Title</main>`)
	if err != nil {
		t.Fatal(err)
	}
	want := `<main class="text-4xl font-bold lead" id="hero">Title</main>`
	if got != want {
		t.Fatalf("unexpected HTML:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenderSPARejectsDuplicateIDShorthand(t *testing.T) {
	_, err := RenderSPA(`<main #hero id="other">Title</main>`)
	if err == nil {
		t.Fatal("expected duplicate id error")
	}
	if !strings.Contains(err.Error(), "multiple id attributes") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderSPARejectsMissingComponent(t *testing.T) {
	_, err := RenderSPA(`<Page />`)
	if err == nil {
		t.Fatal("expected missing component error")
	}
	if !strings.Contains(err.Error(), `missing component "Page"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseRejectsUnsupportedTemplateSyntaxWithGOWDKAlternatives(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		message string
	}{
		{
			name:    "conditional",
			source:  `<main>{#if open}<p>Open</p>{/if}</main>`,
			message: "use g:if, g:else-if, and g:else",
		},
		{
			name:    "loop",
			source:  `<ul>{#each items as item}<li>{item.Name}</li>{/each}</ul>`,
			message: "use g:for with g:key",
		},
		{
			name:    "raw html",
			source:  `<main>{@html body}</main>`,
			message: "GOWDK escapes rendered text by default",
		},
		{
			name:    "snippet",
			source:  `<main>{#snippet row(item)}<p>{item}</p>{/snippet}</main>`,
			message: "use GOWDK component slots",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Parse(test.source)
			if err == nil {
				t.Fatal("expected unsupported template syntax error")
			}
			if !strings.Contains(err.Error(), test.message) {
				t.Fatalf("expected error containing %q, got %v", test.message, err)
			}
		})
	}
}

func TestRenderWithComponentsExpandsSPAStringProps(t *testing.T) {
	got, err := RenderWithComponents(`<main><Hero title="GOWDK & compiler" /></main>`, map[string]Component{
		"Hero": {
			Name:  "Hero",
			Props: []string{"title"},
			Body:  `<section><h1>{title}</h1></section>`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `<main><section><h1>GOWDK &amp; compiler</h1></section></main>`
	if got != want {
		t.Fatalf("unexpected HTML:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenderWithComponentsExpandsQualifiedComponentCall(t *testing.T) {
	got, err := RenderWithOptions(`<main><ui.Hero title="GOWDK" /></main>`, map[string]Component{
		"ui.Hero": {
			Name:    "Hero",
			Package: "components",
			Props:   []string{"title"},
			Body:    `<section><h1>{title}</h1></section>`,
		},
	}, nil, Options{Package: "pages"})
	if err != nil {
		t.Fatal(err)
	}
	want := `<main><section><h1>GOWDK</h1></section></main>`
	if got != want {
		t.Fatalf("unexpected HTML:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenderWithComponentsResolvesImportedComponentChildrenInTheirOwnPackage(t *testing.T) {
	got, err := RenderWithOptions(`<main><ui.Hero title="GOWDK" /></main>`, map[string]Component{
		"ui.Hero": {
			Name:    "Hero",
			Package: "components",
			Props:   []string{"title"},
			Body:    `<section><Badge label={title} /></section>`,
		},
		"Badge": {
			Name:    "Badge",
			Package: "components",
			Props:   []string{"label"},
			Body:    `<strong>{label}</strong>`,
		},
	}, nil, Options{Package: "pages"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<main><section><gowdk-island data-gowdk-component="Badge"`,
		`<strong><span data-gowdk-bind="label" data-gowdk-binding-text="b1">GOWDK</span></strong>`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in component output:\n%s", want, got)
		}
	}
	if !strings.HasSuffix(got, `</gowdk-island></section></main>`) {
		t.Fatalf("unexpected HTML suffix:\n%s", got)
	}
}

func TestRenderWithComponentsResolvesComponentScopedUse(t *testing.T) {
	got, err := RenderWithOptions(`<main><Marketing title="GOWDK" /></main>`, map[string]Component{
		"Marketing": {
			Name:    "Marketing",
			Package: "pages",
			Uses:    map[string]string{"icons": "icons"},
			Props:   []string{"title"},
			Body:    `<section><icons.Badge label={title} /></section>`,
		},
		"Badge": {
			Name:    "Badge",
			Package: "icons",
			Props:   []string{"label"},
			Body:    `<strong>{label}</strong>`,
		},
	}, nil, Options{Package: "pages"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<section><gowdk-island data-gowdk-component="Badge"`,
		`<strong><span data-gowdk-bind="label" data-gowdk-binding-text="b1">GOWDK</span></strong>`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in component output:\n%s", want, got)
		}
	}
}

func TestRenderWithComponentsDoesNotResolveImportedComponentByUnqualifiedNameFromPage(t *testing.T) {
	_, err := RenderWithOptions(`<main><Badge label="GOWDK" /></main>`, map[string]Component{
		"Badge": {
			Name:    "Badge",
			Package: "components",
			Props:   []string{"label"},
			Body:    `<strong>{label}</strong>`,
		},
	}, nil, Options{Package: "pages"})
	if err == nil {
		t.Fatal("expected missing component error")
	}
	if !strings.Contains(err.Error(), `missing component "Badge"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestComponentReferencesIncludesQualifiedNames(t *testing.T) {
	refs, err := ComponentReferences(`<main><ui.Hero /><Card /></main>`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(refs, ",") != "Card,ui.Hero" {
		t.Fatalf("unexpected component references: %#v", refs)
	}
}

func TestRenderWithComponentsExpandsSlotChildrenInCallerScope(t *testing.T) {
	got, err := RenderWithData(`<Panel title="Welcome"><p>{message}</p></Panel>`, map[string]Component{
		"Panel": {
			Name:  "Panel",
			Props: []string{"title"},
			Body:  `<section><h2>{title}</h2><slot /></section>`,
		},
	}, map[string]string{"message": "Hello <GOWDK>"})
	if err != nil {
		t.Fatal(err)
	}
	want := `<section><h2>Welcome</h2><p>Hello &lt;GOWDK&gt;</p></section>`
	if got != want {
		t.Fatalf("unexpected HTML:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenderWithComponentsUsesSlotFallbackWithoutChildren(t *testing.T) {
	got, err := RenderWithComponents(`<Panel />`, map[string]Component{
		"Panel": {
			Name: "Panel",
			Body: `<section><slot><p>Empty</p></slot></section>`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `<section><p>Empty</p></section>`
	if got != want {
		t.Fatalf("unexpected HTML:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenderWithComponentsExpandsNamedSlots(t *testing.T) {
	got, err := RenderWithData(`<Panel title="Welcome"><template g:slot="actions"><button>{label}</button></template><p>Body</p></Panel>`, map[string]Component{
		"Panel": {
			Name:  "Panel",
			Props: []string{"title"},
			Body:  `<section><h2>{title}</h2><div><slot name="actions"><span>None</span></slot></div><slot /></section>`,
		},
	}, map[string]string{"label": "Save"})
	if err != nil {
		t.Fatal(err)
	}
	want := `<section><h2>Welcome</h2><div><button>Save</button></div><p>Body</p></section>`
	if got != want {
		t.Fatalf("unexpected HTML:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenderWithComponentsExpandsScopedSlots(t *testing.T) {
	got, err := RenderWithComponents(`<Panel><template g:slot="item" let:item><strong>{item}</strong></template></Panel>`, map[string]Component{
		"Panel": {
			Name: "Panel",
			Body: `<section><slot name="item" item="Ada" /></section>`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `<section><strong>Ada</strong></section>`
	if got != want {
		t.Fatalf("unexpected HTML:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenderWithComponentsEmitsDefaultJSIslandForState(t *testing.T) {
	got, err := RenderWithComponents(`<Counter />`, map[string]Component{
		"Counter": {
			Name:      "Counter",
			State:     map[string]string{"Count": "1"},
			StateJSON: `{"Count":1}`,
			Body:      `<button g:on:click={Count++}>{Count}</button>`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<gowdk-island data-gowdk-component="Counter" data-gowdk-island="i1" data-gowdk-runtime="js" data-gowdk-state="{&#34;Count&#34;:1}">`,
		`<button data-gowdk-on-click="Count++" data-gowdk-binding-on-click="b1"><span data-gowdk-bind="Count" data-gowdk-binding-text="b2">1</span></button>`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in island output:\n%s", want, got)
		}
	}
}

func TestRenderWithComponentsEmitsClientHandlers(t *testing.T) {
	got, err := RenderWithComponents(`<Counter />`, map[string]Component{
		"Counter": {
			Name:         "Counter",
			State:        map[string]string{"Count": "1"},
			StateJSON:    `{"Count":1}`,
			Handlers:     map[string]clientlang.Handler{"Add": {Params: []string{"step"}, Statements: []string{"Count = step"}}},
			HandlersJSON: `{"Add":{"params":["step"],"statements":["Count = step"]}}`,
			Body:         `<button g:on:click={Add(2)}>{Count}</button>`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`data-gowdk-client="{&#34;Add&#34;:{&#34;params&#34;:[&#34;step&#34;],&#34;statements&#34;:[&#34;Count = step&#34;]}}"`,
		`<button data-gowdk-on-click="Add(2)" data-gowdk-binding-on-click="b1"><span data-gowdk-bind="Count" data-gowdk-binding-text="b2">1</span></button>`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in island output:\n%s", want, got)
		}
	}
}

func TestRenderWithComponentsWiresParentComponentEventListener(t *testing.T) {
	got, err := RenderWithComponents(`<Parent />`, map[string]Component{
		"Parent": {
			Name:       "Parent",
			State:      map[string]string{"SelectedID": ""},
			StateJSON:  `{"SelectedID":""}`,
			StateTypes: map[string]clientlang.ValueType{"SelectedID": clientlang.TypeString},
			Body:       `<Child g:on:select={SelectedID = event.id} />`,
		},
		"Child": {
			Name:       "Child",
			State:      map[string]string{"ID": "first"},
			StateJSON:  `{"ID":"first"}`,
			StateTypes: map[string]clientlang.ValueType{"ID": clientlang.TypeString},
			Emits: map[string]clientlang.Emit{
				"select": {Name: "select", Params: []string{"id"}, ParamTypes: []clientlang.ValueType{clientlang.TypeString}},
			},
			HandlersJSON: `{"emits":{"select":{"params":["id"]}}}`,
			Body:         `<button g:on:click={emit select(ID)}>{ID}</button>`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`data-gowdk-parent-on-select="SelectedID = event.id"`,
		`data-gowdk-client="{&#34;emits&#34;:{&#34;select&#34;:{&#34;params&#34;:[&#34;id&#34;]}}}"`,
		`<button data-gowdk-on-click="emit select(ID)" data-gowdk-binding-on-click="b1"><span data-gowdk-bind="ID" data-gowdk-binding-text="b2">first</span></button>`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in component event output:\n%s", want, got)
		}
	}
}

func TestRenderWithComponentsMarksReactivePropExpressions(t *testing.T) {
	got, err := RenderWithComponents(`<Parent />`, map[string]Component{
		"Parent": {
			Name:      "Parent",
			State:     map[string]string{"SelectedName": "Ada"},
			StateJSON: `{"SelectedName":"Ada"}`,
			Body:      `<Child label={SelectedName} />`,
		},
		"Child": {
			Name:  "Child",
			Props: []string{"label"},
			Body:  `<p>{label}</p>`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`data-gowdk-props="{&#34;label&#34;:&#34;SelectedName&#34;}"`,
		`data-gowdk-state="{&#34;label&#34;:&#34;Ada&#34;}"`,
		`<p><span data-gowdk-bind="label" data-gowdk-binding-text="b1">Ada</span></p>`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in reactive prop output:\n%s", want, got)
		}
	}
}

func TestRenderWithComponentsLowersEventModifiers(t *testing.T) {
	got, err := RenderWithComponents(`<Counter />`, map[string]Component{
		"Counter": {
			Name:      "Counter",
			State:     map[string]string{"Count": "1"},
			StateJSON: `{"Count":1}`,
			Body:      `<button g:on:click.prevent.stop.once.capture.debounce(250ms)={Count++}>{Count}</button><button g:on:click.throttle(1s)={Count++}>Throttle</button>`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`data-gowdk-on-click="Count++"`,
		`data-gowdk-event-click="prevent stop once capture debounce:250"`,
		`data-gowdk-event-click="throttle:1000"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in event modifier output:\n%s", want, got)
		}
	}
}

func TestRenderWithComponentsAllowsDOMEventObjectAccess(t *testing.T) {
	got, err := RenderWithComponents(`<Search />`, map[string]Component{
		"Search": {
			Name:       "Search",
			State:      map[string]string{"Query": ""},
			StateJSON:  `{"Query":""}`,
			StateTypes: map[string]clientlang.ValueType{"Query": clientlang.TypeString},
			Body:       `<input g:on:input={Query = event.value} value="" />`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `data-gowdk-on-input="Query = event.value"`) {
		t.Fatalf("expected DOM event binding in output:\n%s", got)
	}
}

func TestRenderWithComponentsLowersDOMRefs(t *testing.T) {
	got, err := RenderWithComponents(`<Search />`, map[string]Component{
		"Search": {
			Name:      "Search",
			State:     map[string]string{"Query": "initial"},
			StateJSON: `{"Query":"initial"}`,
			Refs:      map[string]clientlang.Ref{"searchInput": {Name: "searchInput", Kind: "HTMLInputElement"}},
			Body:      `<input g:ref={searchInput} />`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `data-gowdk-ref="searchInput"`) {
		t.Fatalf("expected ref marker in output:\n%s", got)
	}
}

func TestRenderWithComponentsRejectsUnknownDOMRef(t *testing.T) {
	_, err := RenderWithComponents(`<Search />`, map[string]Component{
		"Search": {
			Name:      "Search",
			State:     map[string]string{"Query": "initial"},
			StateJSON: `{"Query":"initial"}`,
			Refs:      map[string]clientlang.Ref{"searchInput": {Name: "searchInput", Kind: "HTMLInputElement"}},
			Body:      `<input g:ref={missingInput} />`,
		},
	})
	if err == nil {
		t.Fatal("expected unknown ref error")
	}
	if !strings.Contains(err.Error(), `unknown DOM ref "missingInput"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderWithComponentsLowersGIfForStatefulIsland(t *testing.T) {
	got, err := RenderWithComponents(`<Disclosure />`, map[string]Component{
		"Disclosure": {
			Name:      "Disclosure",
			State:     map[string]string{"Open": "false"},
			StateJSON: `{"Open":false}`,
			Body:      `<section g:if={Open}>Open</section>`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<!--gowdk-if:c1-0:start-->`,
		`data-gowdk-if="Open" hidden`,
		`<!--gowdk-if:c1-0:end-->`,
		`data-gowdk-runtime="js"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in conditional island output:\n%s", want, got)
		}
	}
}

func TestRenderWithComponentsLeavesGIfVisibleWhenInitiallyTrue(t *testing.T) {
	got, err := RenderWithComponents(`<Disclosure />`, map[string]Component{
		"Disclosure": {
			Name:      "Disclosure",
			State:     map[string]string{"Open": "true"},
			StateJSON: `{"Open":true}`,
			Body:      `<section g:if={Open}>Open</section>`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `data-gowdk-if="Open"`) {
		t.Fatalf("expected g:if marker in output:\n%s", got)
	}
	if strings.Contains(got, `data-gowdk-if="Open" hidden`) {
		t.Fatalf("did not expect hidden on initially true g:if:\n%s", got)
	}
}

func TestRenderWithComponentsLowersGElseChain(t *testing.T) {
	got, err := RenderWithComponents(`<Disclosure />`, map[string]Component{
		"Disclosure": {
			Name:      "Disclosure",
			State:     map[string]string{"Open": "false", "Loading": "true"},
			StateJSON: `{"Open":false,"Loading":true}`,
			Body:      `<section g:if={Open}>Open</section><section g:else-if={Loading}>Loading</section><section g:else>Closed</section>`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<!--gowdk-if:c1-0:start-->`,
		`data-gowdk-if-group="c1" data-gowdk-if-index="0" data-gowdk-binding-if="b1" data-gowdk-if="Open" hidden`,
		`<!--gowdk-if:c1-0:end-->`,
		`<!--gowdk-if:c1-1:start-->`,
		`data-gowdk-if-group="c1" data-gowdk-if-index="1" data-gowdk-binding-if="b2" data-gowdk-if="Loading"`,
		`<!--gowdk-if:c1-1:end-->`,
		`<!--gowdk-if:c1-2:start-->`,
		`data-gowdk-if-group="c1" data-gowdk-if-index="2" data-gowdk-binding-if="b3" data-gowdk-else hidden`,
		`<!--gowdk-if:c1-2:end-->`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in else chain output:\n%s", want, got)
		}
	}
}

func TestRenderWithComponentsRejectsMisplacedGElse(t *testing.T) {
	_, err := RenderWithComponents(`<Disclosure />`, map[string]Component{
		"Disclosure": {
			Name:      "Disclosure",
			State:     map[string]string{"Open": "false"},
			StateJSON: `{"Open":false}`,
			Body:      `<section g:else>Closed</section>`,
		},
	})
	if err == nil {
		t.Fatal("expected misplaced g:else error")
	}
	if !strings.Contains(err.Error(), "g:else must follow") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderWithComponentsLowersGForList(t *testing.T) {
	got, err := RenderWithComponents(`<Nested />`, map[string]Component{
		"Nested": {
			Name:      "Nested",
			State:     map[string]string{"Items": `[{"ID":"first","Name":"first","Done":false},{"ID":"second","Name":"second","Done":true}]`},
			StateJSON: `{"Items":[{"ID":"first","Name":"first","Done":false},{"ID":"second","Name":"second","Done":true}]}`,
			StateTypes: map[string]clientlang.ValueType{
				"Items":        clientlang.TypeArray,
				"Items[]":      clientlang.TypeObject,
				"Items[].ID":   clientlang.TypeString,
				"Items[].Name": clientlang.TypeString,
				"Items[].Done": clientlang.TypeBool,
			},
			Body: `<ul><li g:for={item in Items} g:key={item.ID}>{item.Name}</li></ul>`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<template data-gowdk-for="l1" data-gowdk-binding-list="b1" data-gowdk-for-var="item" data-gowdk-for-source="Items" data-gowdk-for-key="item.ID"`,
		`data-gowdk-for-template="&lt;li data-gowdk-for-item=&#34;l1&#34; data-gowdk-key-value=&#34;{{item.ID}}&#34;&gt;{{item.Name}}&lt;/li&gt;"`,
		`<li data-gowdk-for-item="l1" data-gowdk-key-value="first">first</li>`,
		`<li data-gowdk-for-item="l1" data-gowdk-key-value="second">second</li>`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in g:for output:\n%s", want, got)
		}
	}
}

func TestRenderWithComponentsLowersGForIndexVariable(t *testing.T) {
	got, err := RenderWithComponents(`<Nested />`, map[string]Component{
		"Nested": {
			Name:      "Nested",
			State:     map[string]string{"Items": `[{"ID":"first","Name":"first","Done":false},{"ID":"second","Name":"second","Done":true}]`},
			StateJSON: `{"Items":[{"ID":"first","Name":"first","Done":false},{"ID":"second","Name":"second","Done":true}]}`,
			StateTypes: map[string]clientlang.ValueType{
				"Items":        clientlang.TypeArray,
				"Items[]":      clientlang.TypeObject,
				"Items[].ID":   clientlang.TypeString,
				"Items[].Name": clientlang.TypeString,
			},
			Body: `<ol><li g:for={item, i in Items} g:key={item.ID}>{i}: {item.Name}</li></ol>`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`data-gowdk-for-index-var="i"`,
		`{{i}}: {{item.Name}}`,
		`<li data-gowdk-for-item="l1" data-gowdk-key-value="first">0: first</li>`,
		`<li data-gowdk-for-item="l1" data-gowdk-key-value="second">1: second</li>`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in g:for index output:\n%s", want, got)
		}
	}
}

func TestRenderWithComponentsRejectsGForWithoutKey(t *testing.T) {
	_, err := RenderWithComponents(`<Nested />`, map[string]Component{
		"Nested": {
			Name:      "Nested",
			State:     map[string]string{"Items": `[]`},
			StateJSON: `{"Items":[]}`,
			StateTypes: map[string]clientlang.ValueType{
				"Items": clientlang.TypeArray,
			},
			Body: `<ul><li g:for={item in Items}>{item.Name}</li></ul>`,
		},
	})
	if err == nil {
		t.Fatal("expected missing g:key error")
	}
	if !strings.Contains(err.Error(), "g:for requires g:key") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderWithComponentsLowersValueBinding(t *testing.T) {
	got, err := RenderWithComponents(`<Search />`, map[string]Component{
		"Search": {
			Name:      "Search",
			State:     map[string]string{"Query": "initial"},
			StateJSON: `{"Query":"initial"}`,
			Body:      `<input g:bind:value={Query} />`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`data-gowdk-bind-value="Query"`,
		`value="initial"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in value binding output:\n%s", want, got)
		}
	}
}

func TestRenderWithComponentsLowersNumericValueBinding(t *testing.T) {
	got, err := RenderWithComponents(`<Counter />`, map[string]Component{
		"Counter": {
			Name:       "Counter",
			State:      map[string]string{"Count": "7"},
			StateJSON:  `{"Count":7}`,
			StateTypes: map[string]clientlang.ValueType{"Count": clientlang.TypeInt},
			Body:       `<input type="number" g:bind:value={Count} />`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`data-gowdk-bind-value="Count"`,
		`data-gowdk-bind-type="int"`,
		`value="7"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in numeric value binding output:\n%s", want, got)
		}
	}
}

func TestRenderWithComponentsLowersTextareaValueBinding(t *testing.T) {
	got, err := RenderWithComponents(`<Search />`, map[string]Component{
		"Search": {
			Name:      "Search",
			State:     map[string]string{"Query": "initial"},
			StateJSON: `{"Query":"initial"}`,
			Body:      `<textarea g:bind:value={Query}></textarea>`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`data-gowdk-bind-value="Query"`,
		`>initial</textarea>`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in textarea value binding output:\n%s", want, got)
		}
	}
}

func TestRenderWithComponentsLowersSelectValueBinding(t *testing.T) {
	got, err := RenderWithComponents(`<Search />`, map[string]Component{
		"Search": {
			Name:      "Search",
			State:     map[string]string{"Query": "b"},
			StateJSON: `{"Query":"b"}`,
			Body:      `<select g:bind:value={Query}><option value="a">A</option><option value="b">B</option></select>`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`data-gowdk-bind-value="Query"`,
		`<option value="b" selected>B</option>`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in select value binding output:\n%s", want, got)
		}
	}
}

func TestRenderWithComponentsLowersRadioValueBinding(t *testing.T) {
	got, err := RenderWithComponents(`<Search />`, map[string]Component{
		"Search": {
			Name:      "Search",
			State:     map[string]string{"Query": "b"},
			StateJSON: `{"Query":"b"}`,
			Body:      `<input type="radio" name="choice" value="a" g:bind:value={Query} /><input type="radio" name="choice" value="b" g:bind:value={Query} />`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`type="radio"`,
		`data-gowdk-bind-value="Query"`,
		`value="b" checked`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in radio value binding output:\n%s", want, got)
		}
	}
}

func TestRenderWithComponentsRejectsRadioValueBindingWithoutValue(t *testing.T) {
	_, err := RenderWithComponents(`<Search />`, map[string]Component{
		"Search": {
			Name:      "Search",
			State:     map[string]string{"Query": "b"},
			StateJSON: `{"Query":"b"}`,
			Body:      `<input type="radio" g:bind:value={Query} />`,
		},
	})
	if err == nil {
		t.Fatal("expected radio value binding error")
	}
	if !strings.Contains(err.Error(), `radio <input> requires a literal value attribute`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderWithComponentsRejectsNumericValueBindingOutsideNumberInput(t *testing.T) {
	_, err := RenderWithComponents(`<Counter />`, map[string]Component{
		"Counter": {
			Name:       "Counter",
			State:      map[string]string{"Count": "7"},
			StateJSON:  `{"Count":7}`,
			StateTypes: map[string]clientlang.ValueType{"Count": clientlang.TypeInt},
			Body:       `<textarea g:bind:value={Count}></textarea>`,
		},
	})
	if err == nil {
		t.Fatal("expected numeric value binding target error")
	}
	if !strings.Contains(err.Error(), `numeric target "Count" requires <input type="number">`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderWithComponentsRejectsValueBindingOutsideSupportedControls(t *testing.T) {
	_, err := RenderWithComponents(`<Search />`, map[string]Component{
		"Search": {
			Name:      "Search",
			State:     map[string]string{"Query": "initial"},
			StateJSON: `{"Query":"initial"}`,
			Body:      `<div g:bind:value={Query}></div>`,
		},
	})
	if err == nil {
		t.Fatal("expected unsupported g:bind:value target error")
	}
	if !strings.Contains(err.Error(), `g:bind:value is only supported on <input>, <textarea>, and <select>`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderWithComponentsLowersCheckedBinding(t *testing.T) {
	got, err := RenderWithComponents(`<Toggle />`, map[string]Component{
		"Toggle": {
			Name:      "Toggle",
			State:     map[string]string{"Open": "true"},
			StateJSON: `{"Open":true}`,
			Body:      `<input type="checkbox" g:bind:checked={Open} />`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`type="checkbox"`,
		`data-gowdk-bind-checked="Open"`,
		`checked`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in checked binding output:\n%s", want, got)
		}
	}
}

func TestRenderWithComponentsRejectsCheckedBindingOutsideCheckbox(t *testing.T) {
	_, err := RenderWithComponents(`<Toggle />`, map[string]Component{
		"Toggle": {
			Name:      "Toggle",
			State:     map[string]string{"Open": "true"},
			StateJSON: `{"Open":true}`,
			Body:      `<input g:bind:checked={Open} />`,
		},
	})
	if err == nil {
		t.Fatal("expected unsupported g:bind:checked target error")
	}
	if !strings.Contains(err.Error(), `g:bind:checked is only supported on checkbox <input>`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderWithComponentsLowersReactiveAttributes(t *testing.T) {
	got, err := RenderWithComponents(`<Toggle />`, map[string]Component{
		"Toggle": {
			Name:      "Toggle",
			State:     map[string]string{"Open": "true"},
			StateJSON: `{"Open":true}`,
			Body:      `<button disabled={Open} aria-expanded={Open}>Toggle</button>`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`data-gowdk-attr-disabled="Open"`,
		`data-gowdk-binding-attr-disabled="b1"`,
		` disabled`,
		`data-gowdk-attr-aria-expanded="Open"`,
		`data-gowdk-binding-attr-aria-expanded="b2"`,
		`aria-expanded="true"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in reactive attr output:\n%s", want, got)
		}
	}
}

func TestRenderWithComponentsLowersClassToggles(t *testing.T) {
	got, err := RenderWithComponents(`<Toggle />`, map[string]Component{
		"Toggle": {
			Name:      "Toggle",
			State:     map[string]string{"Open": "true"},
			StateJSON: `{"Open":true}`,
			Body:      `<button class="base active" class:active={Open} class:closed={!Open}>Toggle</button>`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`data-gowdk-class-active="Open"`,
		`data-gowdk-binding-class-active="b1"`,
		`data-gowdk-class-closed="!Open"`,
		`data-gowdk-binding-class-closed="b2"`,
		`class="base active"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in class toggle output:\n%s", want, got)
		}
	}
	if strings.Contains(got, `class:active`) || strings.Contains(got, `class:closed`) {
		t.Fatalf("did not expect source class toggle attrs in output:\n%s", got)
	}
}

func TestRenderWithComponentsLowersStyleBindings(t *testing.T) {
	got, err := RenderWithComponents(`<Meter />`, map[string]Component{
		"Meter": {
			Name:      "Meter",
			State:     map[string]string{"Height": "12", "Percent": "50"},
			StateJSON: `{"Height":12,"Percent":50}`,
			Body:      `<div style="color: red" style:height.px={Height} style:width.%={Percent}>Meter</div>`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`data-gowdk-style-height="Height"`,
		`data-gowdk-binding-style-height="b1"`,
		`data-gowdk-style-unit-height="px"`,
		`data-gowdk-style-width="Percent"`,
		`data-gowdk-binding-style-width="b2"`,
		`data-gowdk-style-unit-width="%"`,
		`style="color: red; height: 12px; width: 50%"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in style binding output:\n%s", want, got)
		}
	}
	if strings.Contains(got, `style:height`) || strings.Contains(got, `style:width`) {
		t.Fatalf("did not expect source style binding attrs in output:\n%s", got)
	}
}

func TestRenderWithComponentsRejectsUnknownIslandMode(t *testing.T) {
	_, err := RenderWithComponents(`<Counter g:island="js" />`, map[string]Component{
		"Counter": {
			Name:      "Counter",
			State:     map[string]string{"Count": "1"},
			StateJSON: `{"Count":1}`,
			Body:      `<button>{Count}</button>`,
		},
	})
	if err == nil {
		t.Fatal("expected unknown island mode error")
	}
	if !strings.Contains(err.Error(), `unsupported g:island value "js"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderWithComponentsUsesComponentDefaultWASMIsland(t *testing.T) {
	got, err := RenderWithComponents(`<Counter />`, map[string]Component{
		"Counter": {
			Name:          "Counter",
			DefaultIsland: "wasm",
			Body:          `<button>Count</button>`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		`<gowdk-island data-gowdk-component="Counter"`,
		`data-gowdk-runtime="wasm"`,
		`<button>Count</button>`,
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("expected %q in default wasm island output:\n%s", expected, got)
		}
	}
	if strings.Contains(got, `g:island`) {
		t.Fatalf("did not expect source g:island in output:\n%s", got)
	}
}

func TestCanonicalNormalizesDirectiveExpressions(t *testing.T) {
	left, err := Canonical(`<button class:active={Open&&Count>0} g:on:click={Count=Count+1}>{Count}</button>`)
	if err != nil {
		t.Fatal(err)
	}
	right, err := Canonical(`<button g:on:click={Count = Count + 1} class:active={Open && Count > 0}>{Count}</button>`)
	if err != nil {
		t.Fatal(err)
	}
	if left != right {
		t.Fatalf("expected equivalent canonical view output:\nleft  %s\nright %s", left, right)
	}
}

func TestActionFormSchemaIncludesSlottedControls(t *testing.T) {
	schema, err := ActionFormSchema(`<form g:post={save}><Panel><input name="email" required /></Panel></form>`)
	if err != nil {
		t.Fatal(err)
	}
	fields := schema["save"]
	if len(fields) != 1 || fields[0].Name != "email" || !fields[0].Required {
		t.Fatalf("unexpected slotted action fields: %#v", fields)
	}
}

func TestRenderWithDataInterpolatesTextAndAttributes(t *testing.T) {
	got, err := RenderWithData(`<main data-slug="{slug}"><h1>{title}</h1></main>`, nil, map[string]string{
		"slug":  `hello&gowdk`,
		"title": `GOWDK <SPA>`,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `<main data-slug="hello&amp;gowdk"><h1>GOWDK &lt;SPA&gt;</h1></main>`
	if got != want {
		t.Fatalf("unexpected HTML:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenderWithDataInterpolatesExpressionAttributes(t *testing.T) {
	got, err := RenderWithData(`<main data-title={post.Title}></main>`, nil, map[string]string{
		"post.Title": `GOWDK <SPA>`,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `<main data-title="GOWDK &lt;SPA&gt;"></main>`
	if got != want {
		t.Fatalf("unexpected HTML:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenderWithDataInterpolatesDottedTextExpressionNames(t *testing.T) {
	got, err := RenderWithData(`<main>{post.Title}</main>`, nil, map[string]string{
		"post.Title": `GOWDK <SPA>`,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `<main>GOWDK &lt;SPA&gt;</main>`
	if got != want {
		t.Fatalf("unexpected HTML:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenderWithDataInterpolatesRouteParams(t *testing.T) {
	got, err := RenderWithData(`<main data-slug="{param(\"slug\")}"><h1>{param("slug")}</h1></main>`, nil, map[string]string{
		"slug": `hello&gowdk`,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `<main data-slug="hello&amp;gowdk"><h1>hello&amp;gowdk</h1></main>`
	if got != want {
		t.Fatalf("unexpected HTML:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenderWithDataRejectsRouteParamsInDangerousAttributes(t *testing.T) {
	for _, source := range []string{
		`<img src="x" onerror="{param(\"slug\")}" />`,
		`<a href="/blog/{param(\"slug\")}">Post</a>`,
		`<iframe srcdoc="{param(\"slug\")}"></iframe>`,
		`<main style="color: {param(\"slug\")}">Post</main>`,
	} {
		t.Run(source, func(t *testing.T) {
			_, err := RenderWithData(source, nil, map[string]string{"slug": `alert(1)`})
			if err == nil {
				t.Fatal("expected dangerous route param attribute error")
			}
			if !strings.Contains(err.Error(), "route param interpolation is not allowed") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestRenderWithDataRejectsRouteParamPassedToDangerousComponentAttribute(t *testing.T) {
	_, err := RenderWithData(`<Avatar handler="{param(\"slug\")}" />`, map[string]Component{
		"Avatar": {
			Name:  "Avatar",
			Props: []string{"handler"},
			Body:  `<img src="x" onerror="{handler}" />`,
		},
	}, map[string]string{"slug": `alert(1)`})
	if err == nil {
		t.Fatal("expected dangerous route param component prop error")
	}
	if !strings.Contains(err.Error(), `route param interpolation is not allowed in "onerror" attributes`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderWithDataRejectsUnknownRouteParam(t *testing.T) {
	_, err := RenderWithData(`<main>{param("slug")}</main>`, nil, nil)
	if err == nil {
		t.Fatal("expected unknown route param error")
	}
	if !strings.Contains(err.Error(), `unknown route param "slug"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderWithDataInterpolatesComponentPropValues(t *testing.T) {
	got, err := RenderWithData(`<main><Hero title="{slug}" /></main>`, map[string]Component{
		"Hero": {
			Name:  "Hero",
			Props: []string{"title"},
			Body:  `<section><h1>{title}</h1></section>`,
		},
	}, map[string]string{
		"slug": "hello-gowdk",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `<main><section><h1>hello-gowdk</h1></section></main>`
	if got != want {
		t.Fatalf("unexpected HTML:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenderWithOptionsLowersGPostDirective(t *testing.T) {
	got, err := RenderWithOptions(`<form class="signup" g:post={submit}><input name="email" /></form>`, nil, nil, Options{
		Actions: map[string]string{"submit": "/signup"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `<form class="signup" method="post" action="/signup"><input name="email"></form>`
	if got != want {
		t.Fatalf("unexpected HTML:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenderWithOptionsMarksGCommandForm(t *testing.T) {
	got, err := RenderWithOptions(`<form method="post" action="/patients" g:command="patients.CreatePatient"><input name="email" /></form>`, nil, nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := `<form method="post" action="/patients" data-gowdk-command="patients.CreatePatient"><input name="email"></form>`
	if got != want {
		t.Fatalf("unexpected HTML:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenderWithOptionsMarksGQueryElement(t *testing.T) {
	got, err := RenderWithOptions(`<section g:query="patients.GetPatientPage"><h1>Patients</h1></section>`, nil, nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := `<section data-gowdk-query="patients.GetPatientPage"><h1>Patients</h1></section>`
	if got != want {
		t.Fatalf("unexpected HTML:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestCommandReferencesFindsGCommandForms(t *testing.T) {
	refs, err := CommandReferences(`<main><form method="patch" action="/patients" g:command="patients.CreatePatient"></form><form g:command={billing.PayInvoice}></form></main>`)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 2 || refs[0].Command != "patients.CreatePatient" || refs[1].Command != "billing.PayInvoice" {
		t.Fatalf("unexpected command refs: %#v", refs)
	}
	if refs[0].Method != "PATCH" || refs[0].Path != "/patients" {
		t.Fatalf("unexpected command method/path: %#v", refs[0])
	}
	if refs[1].Method != "POST" || refs[1].Path != "" {
		t.Fatalf("unexpected default command method/path: %#v", refs[1])
	}
}

func TestContractReferencesFindsCommandsAndQueries(t *testing.T) {
	refs, err := ContractReferences(`<main><section g:query="patients.GetPatientPage"></section><form method="post" action="/patients" g:command={patients.CreatePatient}></form></main>`)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 2 {
		t.Fatalf("expected two contract refs, got %#v", refs)
	}
	if refs[0].Kind != ContractReferenceQuery || refs[0].Name != "patients.GetPatientPage" {
		t.Fatalf("unexpected query ref: %#v", refs[0])
	}
	if refs[1].Kind != ContractReferenceCommand || refs[1].Name != "patients.CreatePatient" || refs[1].Method != "POST" || refs[1].Path != "/patients" {
		t.Fatalf("unexpected command ref: %#v", refs[1])
	}
}

func TestRenderWithOptionsRejectsFrontendDomainEventDirective(t *testing.T) {
	_, err := RenderWithOptions(`<form g:event="PatientCreated"></form>`, nil, nil, Options{})
	if err == nil {
		t.Fatal("expected g:event rejection")
	}
	if !strings.Contains(err.Error(), `frontend templates must not declare g:event`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderWithOptionsRejectsInvalidGCommand(t *testing.T) {
	_, err := RenderWithOptions(`<form g:command="CreatePatient"></form>`, nil, nil, Options{})
	if err == nil {
		t.Fatal("expected invalid g:command error")
	}
	if !strings.Contains(err.Error(), `must be a package-qualified Go contract reference`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderWithOptionsRejectsInvalidGQuery(t *testing.T) {
	_, err := RenderWithOptions(`<section g:query="GetPatientPage"></section>`, nil, nil, Options{})
	if err == nil {
		t.Fatal("expected invalid g:query error")
	}
	if !strings.Contains(err.Error(), `must be a package-qualified Go contract reference`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderWithOptionsRejectsGCommandAndGQueryOnSameForm(t *testing.T) {
	_, err := RenderWithOptions(`<form g:command="patients.CreatePatient" g:query="patients.GetPatientPage"></form>`, nil, nil, Options{})
	if err == nil {
		t.Fatal("expected mixed g:command/g:query error")
	}
	if !strings.Contains(err.Error(), `must not declare both g:command and g:query`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderWithOptionsAllowsGPostWithLocalValueBinding(t *testing.T) {
	got, err := RenderWithOptions(`<Search />`, map[string]Component{
		"Search": {
			Name:      "Search",
			State:     map[string]string{"Query": "initial"},
			StateJSON: `{"Query":"initial"}`,
			Body:      `<form g:post={submit}><input name="query" g:bind:value={Query} /></form>`,
		},
	}, nil, Options{
		Actions: map[string]string{"submit": "/search"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<form method="post" action="/search">`,
		`name="query"`,
		`data-gowdk-bind-value="Query"`,
		`value="initial"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in g:post binding output:\n%s", want, got)
		}
	}
}

func TestRenderWithOptionsLowersPartialDirectives(t *testing.T) {
	got, err := RenderWithOptions(`<form g:post={refresh} g:target="#patients" g:swap="outerHTML"><button>Refresh</button></form><section id="patients"></section>`, nil, nil, Options{
		Actions: map[string]string{"refresh": "/patients"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `<form method="post" action="/patients" data-gowdk-target="#patients" data-gowdk-swap="outerHTML"><button>Refresh</button></form><section id="patients"></section>`
	if got != want {
		t.Fatalf("unexpected HTML:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenderWithOptionsRejectsMissingPartialTarget(t *testing.T) {
	_, err := RenderWithOptions(`<form g:post={refresh} g:target="#patients"><button>Refresh</button></form>`, nil, nil, Options{
		Actions: map[string]string{"refresh": "/patients"},
	})
	if err == nil {
		t.Fatal("expected missing target error")
	}
	if !strings.Contains(err.Error(), `g:target "#patients" does not reference a literal id in this view`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderWithOptionsRejectsUnknownGPostAction(t *testing.T) {
	_, err := RenderWithOptions(`<form g:post={missing}></form>`, nil, nil, Options{})
	if err == nil {
		t.Fatal("expected unknown action error")
	}
	if !strings.Contains(err.Error(), `unknown action "missing" for g:post`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderWithOptionsRejectsGPostOutsideForm(t *testing.T) {
	_, err := RenderWithOptions(`<button g:post={submit}>Save</button>`, nil, nil, Options{
		Actions: map[string]string{"submit": "/signup"},
	})
	if err == nil {
		t.Fatal("expected invalid g:post target error")
	}
	if !strings.Contains(err.Error(), `g:post is only supported on <form>`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderWithOptionsRejectsGPostWithManualMethod(t *testing.T) {
	_, err := RenderWithOptions(`<form method="post" g:post={submit}></form>`, nil, nil, Options{
		Actions: map[string]string{"submit": "/signup"},
	})
	if err == nil {
		t.Fatal("expected duplicate method error")
	}
	if !strings.Contains(err.Error(), `form with g:post must not declare "method"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderWithOptionsRejectsPartialDirectivesWithoutGPost(t *testing.T) {
	_, err := RenderWithOptions(`<form g:target="#patients"></form>`, nil, nil, Options{})
	if err == nil {
		t.Fatal("expected g:target without g:post error")
	}
	if !strings.Contains(err.Error(), `g:target and g:swap require g:post`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderWithOptionsRejectsInvalidPartialDirectives(t *testing.T) {
	cases := []struct {
		source string
		want   string
	}{
		{`<form g:post={refresh} g:target="patients"></form><section id="patients"></section>`, `must be a literal id selector`},
		{`<form g:post={refresh} g:swap="append"></form>`, `unsupported g:swap mode "append"`},
		{`<form g:post={refresh} g:swap="innerHTML"></form>`, `g:swap requires g:target`},
	}
	for _, tc := range cases {
		_, err := RenderWithOptions(tc.source, nil, nil, Options{
			Actions: map[string]string{"refresh": "/patients"},
		})
		if err == nil {
			t.Fatalf("expected error for %s", tc.source)
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("expected %q in %v", tc.want, err)
		}
	}
}

func TestComponentReferencesReturnsSortedUniqueComponentNames(t *testing.T) {
	names, err := ComponentReferences(`
		<main>
			<Hero title="GOWDK" />
			<section><Card title="One" /><Hero title="Again" /></section>
		</main>
	`)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(names, ",")
	if got != "Card,Hero" {
		t.Fatalf("unexpected component references: %#v", names)
	}
}

func TestComponentCallUsagesMarksReactiveProps(t *testing.T) {
	usages, err := ComponentCallUsages(`<Parent><Child label={SelectedName} SPA="ok" /></Parent>`)
	if err != nil {
		t.Fatal(err)
	}
	if len(usages) != 2 {
		t.Fatalf("unexpected component usages: %#v", usages)
	}
	if usages[0].Component != "Parent" || usages[0].ReactiveProps {
		t.Fatalf("unexpected parent usage: %#v", usages[0])
	}
	if usages[1].Component != "Child" || !usages[1].ReactiveProps {
		t.Fatalf("expected child reactive prop usage, got %#v", usages[1])
	}
}

func TestViewDependenciesIncludesClassShorthand(t *testing.T) {
	dependencies, err := ViewDependencies(`<main .text-4xl .font-bold class="lead"></main>`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(dependencies.CSSClasses, ",") != "font-bold,lead,text-4xl" {
		t.Fatalf("unexpected classes: %#v", dependencies.CSSClasses)
	}
}

func TestParamReferencesReturnsSortedUniqueRouteParamNames(t *testing.T) {
	names, err := ParamReferences(`
		<main data-slug="{param(\"slug\")}">
			<h1>{param("title")}</h1>
			<Hero title="{param(\"slug\")}" />
		</main>
	`)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(names, ",")
	if got != "slug,title" {
		t.Fatalf("unexpected param references: %#v", names)
	}
}

func TestActionFormFieldsFindsDirectSPAControls(t *testing.T) {
	fields, err := ActionFormFields(`
		<form g:post={submit}>
			<input name="email" />
			<textarea name="bio"></textarea>
			<select name="topic"></select>
			<input name="email" />
			<Button />
		</form>
	`)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(fields["submit"], ",")
	if got != "bio,email,topic" {
		t.Fatalf("unexpected fields: %#v", fields)
	}
}

func TestViewDependenciesCollectsAssetsClassesAndStyles(t *testing.T) {
	deps, err := ViewDependencies(`
		<main class="hero lead hero" style="color: red;">
			<img src="/assets/hero.png" />
			<a href="docs/start.html">Start</a>
			<video poster="{poster}"></video>
			<a href="https://example.com">External</a>
			<Hero class="component-prop" />
		</main>
	`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(deps.Assets, ",") != "/assets/hero.png,docs/start.html" {
		t.Fatalf("unexpected assets: %#v", deps.Assets)
	}
	if strings.Join(deps.CSSClasses, ",") != "hero,lead" {
		t.Fatalf("unexpected classes: %#v", deps.CSSClasses)
	}
	if strings.Join(deps.StyleAttributes, ",") != "color: red;" {
		t.Fatalf("unexpected style attributes: %#v", deps.StyleAttributes)
	}
}

func TestActionFormFieldsUnionsMultipleFormsForAction(t *testing.T) {
	fields, err := ActionFormFields(`
		<form g:post={submit}><input name="email" /></form>
		<form g:post={submit}><input name="name" /></form>
	`)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(fields["submit"], ",")
	if got != "email,name" {
		t.Fatalf("unexpected fields: %#v", fields)
	}
}

func TestActionFormFieldsFindsSubmitIntentControls(t *testing.T) {
	fields, err := ActionFormFields(`
		<form g:post={submit}>
			<input name="email" />
			<input type="submit" name="intent" value="save" />
			<button name="intent" value="publish">Publish</button>
			<button type="submit" name="confirm" value="yes">Confirm</button>
			<button type="button" name="local">Local</button>
			<input type="reset" name="resetIntent" />
		</form>
	`)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(fields["submit"], ",")
	if got != "confirm,email,intent" {
		t.Fatalf("unexpected fields: %#v", fields)
	}
}

func TestActionFormSchemaInfersRequiredFields(t *testing.T) {
	schema, err := ActionFormSchema(`
		<form g:post={submit}>
			<input name="email" required minlength="3" maxlength="120" pattern="[a-z]+@[a-z]+[.][a-z]{2,4}" g:message:required="Email required" g:message:pattern="Use a real email" />
			<textarea name="note" maxlength="500"></textarea>
		</form>
		<form g:post={submit}>
			<input name="note" required />
		</form>
	`)
	if err != nil {
		t.Fatal(err)
	}
	fields := schema["submit"]
	if len(fields) != 2 {
		t.Fatalf("expected two fields, got %#v", fields)
	}
	if fields[0].Name != "email" || !fields[0].Required {
		t.Fatalf("expected required email first, got %#v", fields)
	}
	if fields[0].MinLength != 3 || fields[0].MaxLength != 120 || fields[0].Pattern != `[a-z]+@[a-z]+[.][a-z]{2,4}` {
		t.Fatalf("expected email constraints, got %#v", fields[0])
	}
	if fields[0].RequiredMessage != "Email required" || fields[0].PatternMessage != "Use a real email" {
		t.Fatalf("expected email validation messages, got %#v", fields[0])
	}
	if fields[1].Name != "note" || !fields[1].Required {
		t.Fatalf("expected required note second, got %#v", fields)
	}
	if fields[1].MaxLength != 500 {
		t.Fatalf("expected merged note maxlength, got %#v", fields[1])
	}
}

func TestActionFormSchemaRejectsMessageWithoutConstraint(t *testing.T) {
	_, err := ActionFormSchema(`<form g:post={submit}><input name="email" g:message:pattern="Use a real email" /></form>`)
	if err == nil {
		t.Fatal("expected validation message without constraint error")
	}
	if !strings.Contains(err.Error(), `declares g:message:pattern without pattern`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestActionFormFieldsRejectsDynamicControlName(t *testing.T) {
	_, err := ActionFormFields(`<form g:post={submit}><input name="{field}" /></form>`)
	if err == nil {
		t.Fatal("expected dynamic field name error")
	}
	if !strings.Contains(err.Error(), `action form field name "{field}" must be literal`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestActionFormSchemaRejectsDynamicConstraint(t *testing.T) {
	_, err := ActionFormSchema(`<form g:post={submit}><input name="email" pattern={EmailPattern} /></form>`)
	if err == nil {
		t.Fatal("expected dynamic constraint error")
	}
	if !strings.Contains(err.Error(), `action form input pattern "{EmailPattern}" must be literal`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestActionFormSchemaRejectsFileInputs(t *testing.T) {
	_, err := ActionFormSchema(`<form g:post={submit}><input name="avatar" type="FILE" /></form>`)
	if err == nil {
		t.Fatal("expected file input error")
	}
	if !strings.Contains(err.Error(), `file input "avatar" is not supported`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestActionFormSchemaRejectsDynamicInputType(t *testing.T) {
	_, err := ActionFormSchema(`<form g:post={submit}><input name="avatar" type="{kind}" /></form>`)
	if err == nil {
		t.Fatal("expected dynamic input type error")
	}
	if !strings.Contains(err.Error(), `action form input "avatar" type "{kind}" must be literal`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestActionFormSchemaRejectsMultipartForms(t *testing.T) {
	_, err := ActionFormSchema(`<form g:post={submit} enctype="multipart/form-data"><input name="email" /></form>`)
	if err == nil {
		t.Fatal("expected multipart form error")
	}
	if !strings.Contains(err.Error(), `multipart action forms are not supported`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderWithDataRejectsUnknownInterpolation(t *testing.T) {
	_, err := RenderWithData(`<main>{missing}</main>`, nil, nil)
	if err == nil {
		t.Fatal("expected unknown interpolation error")
	}
	if !strings.Contains(err.Error(), `unknown interpolation "missing"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderWithComponentsRejectsMissingRequiredProp(t *testing.T) {
	_, err := RenderWithComponents(`<Hero />`, map[string]Component{
		"Hero": {
			Name:  "Hero",
			Props: []string{"title"},
			Body:  `<section><h1>{title}</h1></section>`,
		},
	})
	if err == nil {
		t.Fatal("expected missing prop error")
	}
	if !strings.Contains(err.Error(), `missing required prop "title"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderSPARejectsMismatchedTags(t *testing.T) {
	_, err := RenderSPA(`<main><h1>Home</main>`)
	if err == nil {
		t.Fatal("expected mismatched tag error")
	}
	if !strings.Contains(err.Error(), "expected closing tag") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderSPAOmitsVoidElementEndTags(t *testing.T) {
	got, err := RenderSPA(`<div>line one<br />line two<hr /><img src="/logo.png" alt="logo" /></div>`)
	if err != nil {
		t.Fatal(err)
	}
	want := `<div>line one<br>line two<hr><img src="/logo.png" alt="logo"></div>`
	if got != want {
		t.Fatalf("unexpected HTML:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}
