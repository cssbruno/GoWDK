package view

import (
	"strings"
	"testing"
)

func TestRenderStaticEscapesTextAndAttributes(t *testing.T) {
	got, err := RenderStatic(`<main class="hero & lead"><h1>GOWDK & friends</h1><input disabled /></main>`)
	if err != nil {
		t.Fatal(err)
	}
	want := `<main class="hero &amp; lead"><h1>GOWDK &amp; friends</h1><input disabled></input></main>`
	if got != want {
		t.Fatalf("unexpected HTML:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenderStaticExpandsClassAndIDShorthand(t *testing.T) {
	got, err := RenderStatic(`<main #hero .text-4xl .font-bold class="lead">Title</main>`)
	if err != nil {
		t.Fatal(err)
	}
	want := `<main class="text-4xl font-bold lead" id="hero">Title</main>`
	if got != want {
		t.Fatalf("unexpected HTML:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenderStaticRejectsDuplicateIDShorthand(t *testing.T) {
	_, err := RenderStatic(`<main #hero id="other">Title</main>`)
	if err == nil {
		t.Fatal("expected duplicate id error")
	}
	if !strings.Contains(err.Error(), "multiple id attributes") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderStaticRejectsMissingComponent(t *testing.T) {
	_, err := RenderStatic(`<Page />`)
	if err == nil {
		t.Fatal("expected missing component error")
	}
	if !strings.Contains(err.Error(), `missing component "Page"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderWithComponentsExpandsStaticStringProps(t *testing.T) {
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
		"title": `GOWDK <static>`,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `<main data-slug="hello&amp;gowdk"><h1>GOWDK &lt;static&gt;</h1></main>`
	if got != want {
		t.Fatalf("unexpected HTML:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenderWithDataInterpolatesExpressionAttributes(t *testing.T) {
	got, err := RenderWithData(`<main data-title={post.Title}></main>`, nil, map[string]string{
		"post.Title": `GOWDK <static>`,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `<main data-title="GOWDK &lt;static&gt;"></main>`
	if got != want {
		t.Fatalf("unexpected HTML:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenderWithDataInterpolatesDottedTextExpressionNames(t *testing.T) {
	got, err := RenderWithData(`<main>{post.Title}</main>`, nil, map[string]string{
		"post.Title": `GOWDK <static>`,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `<main>GOWDK &lt;static&gt;</main>`
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
	want := `<form class="signup" method="post" action="/signup"><input name="email"></input></form>`
	if got != want {
		t.Fatalf("unexpected HTML:\n--- got ---\n%s\n--- want ---\n%s", got, want)
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
	if !strings.Contains(err.Error(), `g:target "#patients" does not reference a static id in this view`) {
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
		{`<form g:post={refresh} g:target="patients"></form><section id="patients"></section>`, `must be a static id selector`},
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

func TestActionFormFieldsFindsDirectStaticControls(t *testing.T) {
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

func TestViewDependenciesCollectsStaticAssetsClassesAndStyles(t *testing.T) {
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
	if strings.Join(deps.StaticAssets, ",") != "/assets/hero.png,docs/start.html" {
		t.Fatalf("unexpected assets: %#v", deps.StaticAssets)
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

func TestActionFormSchemaInfersRequiredFields(t *testing.T) {
	schema, err := ActionFormSchema(`
		<form g:post={submit}>
			<input name="email" required />
			<textarea name="note"></textarea>
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
	if fields[1].Name != "note" || !fields[1].Required {
		t.Fatalf("expected required note second, got %#v", fields)
	}
}

func TestActionFormFieldsRejectsDynamicControlName(t *testing.T) {
	_, err := ActionFormFields(`<form g:post={submit}><input name="{field}" /></form>`)
	if err == nil {
		t.Fatal("expected dynamic field name error")
	}
	if !strings.Contains(err.Error(), `action form field name "{field}" must be static`) {
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
	if !strings.Contains(err.Error(), `action form input "avatar" type "{kind}" must be static`) {
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

func TestRenderStaticRejectsMismatchedTags(t *testing.T) {
	_, err := RenderStatic(`<main><h1>Home</main>`)
	if err == nil {
		t.Fatal("expected mismatched tag error")
	}
	if !strings.Contains(err.Error(), "expected closing tag") {
		t.Fatalf("unexpected error: %v", err)
	}
}
