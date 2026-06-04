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
