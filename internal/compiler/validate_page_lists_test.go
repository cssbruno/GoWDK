package compiler

import (
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkir"
)

func validatePageListsFor(t *testing.T, page gwdkir.Page) []ValidationError {
	t.Helper()
	page.ID = "board"
	if page.Route == "" {
		page.Route = "/board"
	}
	if len(page.Guards) == 0 {
		page.Guards = []string{"public"}
	}
	if page.Render == "" {
		page.Render = gowdk.SSR
	}
	config := gowdk.Config{Addons: []gowdk.Addon{gowdk.NewAddon("ssr", gowdk.FeatureSSR)}}
	report := ValidateProgramReport(config, gwdkir.Program{Pages: []gwdkir.Page{page}})
	return report
}

func findCode(report []ValidationError, code string) (ValidationError, bool) {
	for _, item := range report {
		if item.Code == code {
			return item, true
		}
	}
	return ValidationError{}, false
}

func TestValidateRejectsGForOverLoadData(t *testing.T) {
	report := validatePageListsFor(t, gwdkir.Page{
		Blocks: gwdkir.Blocks{
			Load:     true,
			LoadBody: `=> { columns }`,
			View:     true,
			ViewBody: `<div g:for={col in columns} g:key={col.id}>{col.title}</div>`,
		},
	})
	diag, ok := findCode(report, "gfor_over_load_data")
	if !ok {
		t.Fatalf("expected gfor_over_load_data diagnostic, got %v", report)
	}
	if !strings.Contains(diag.Message, "g:each={col in columns}") {
		t.Fatalf("diagnostic should suggest g:each: %q", diag.Message)
	}
}

func TestValidateAcceptsGEachOverLoadData(t *testing.T) {
	report := validatePageListsFor(t, gwdkir.Page{
		Blocks: gwdkir.Blocks{
			Load:     true,
			LoadBody: `=> { columns }`,
			View:     true,
			ViewBody: `<div g:each={col in columns}>{col.title}</div>`,
		},
	})
	if _, ok := findCode(report, "geach_requires_load"); ok {
		t.Fatalf("g:each over a load field should be accepted: %v", report)
	}
	if _, ok := findCode(report, "gfor_over_load_data"); ok {
		t.Fatalf("unexpected g:for diagnostic: %v", report)
	}
}

func TestValidateRejectsGEachOverNonLoadField(t *testing.T) {
	report := validatePageListsFor(t, gwdkir.Page{
		Blocks: gwdkir.Blocks{
			Load:     true,
			LoadBody: `=> { columns }`,
			View:     true,
			ViewBody: `<div g:each={x in other}>{x.name}</div>`,
		},
	})
	if _, ok := findCode(report, "geach_requires_load"); !ok {
		t.Fatalf("expected geach_requires_load diagnostic, got %v", report)
	}
}

func TestValidateRejectsNestedGEachWrongScope(t *testing.T) {
	report := validatePageListsFor(t, gwdkir.Page{
		Blocks: gwdkir.Blocks{
			Load:     true,
			LoadBody: `=> { columns }`,
			View:     true,
			ViewBody: `<div g:each={col in columns}><span g:each={i in other}>{i.id}</span></div>`,
		},
	})
	if _, ok := findCode(report, "geach_nested_scope"); !ok {
		t.Fatalf("expected geach_nested_scope diagnostic, got %v", report)
	}
}

func TestValidateRejectsGIfOverLoadData(t *testing.T) {
	report := validatePageListsFor(t, gwdkir.Page{
		Blocks: gwdkir.Blocks{
			Load:     true,
			LoadBody: `=> { hasItems, count }`,
			View:     true,
			ViewBody: `<p g:if={hasItems}>You have {count}</p>`,
		},
	})
	diag, ok := findCode(report, "gif_over_load_data")
	if !ok {
		t.Fatalf("expected gif_over_load_data diagnostic, got %v", report)
	}
	if !strings.Contains(diag.Message, "g:when={hasItems}") {
		t.Fatalf("diagnostic should suggest g:when: %q", diag.Message)
	}
}

func TestValidateAcceptsGWhenOverLoadData(t *testing.T) {
	report := validatePageListsFor(t, gwdkir.Page{
		Blocks: gwdkir.Blocks{
			Load:     true,
			LoadBody: `=> { hasItems, count }`,
			View:     true,
			ViewBody: `<section><p g:when={hasItems}>You have {count}</p><p g:when={!hasItems}>None</p></section>`,
		},
	})
	for _, code := range []string{"gwhen_requires_load", "gwhen_nested_scope", "gif_over_load_data"} {
		if _, ok := findCode(report, code); ok {
			t.Fatalf("g:when over a load field should be accepted, got %s: %v", code, report)
		}
	}
}

func TestValidateRejectsGWhenOverNonLoadField(t *testing.T) {
	report := validatePageListsFor(t, gwdkir.Page{
		Blocks: gwdkir.Blocks{
			Load:     true,
			LoadBody: `=> { hasItems }`,
			View:     true,
			ViewBody: `<p g:when={ready}>x</p>`,
		},
	})
	if _, ok := findCode(report, "gwhen_requires_load"); !ok {
		t.Fatalf("expected gwhen_requires_load diagnostic, got %v", report)
	}
}

func TestValidateAcceptsGWhenInsideEachRow(t *testing.T) {
	report := validatePageListsFor(t, gwdkir.Page{
		Blocks: gwdkir.Blocks{
			Load:     true,
			LoadBody: `=> { issues }`,
			View:     true,
			ViewBody: `<ul><li g:each={issue in issues}>{issue.id}<b g:when={issue.urgent}>!</b></li></ul>`,
		},
	})
	for _, code := range []string{"gwhen_requires_load", "gwhen_nested_scope"} {
		if _, ok := findCode(report, code); ok {
			t.Fatalf("g:when referencing the row item should be accepted, got %s: %v", code, report)
		}
	}
}

func TestValidateRejectsGHTMLOverLoadData(t *testing.T) {
	report := validatePageListsFor(t, gwdkir.Page{
		Blocks: gwdkir.Blocks{
			Load:     true,
			LoadBody: `=> { bodyHTML }`,
			View:     true,
			ViewBody: `<article g:html={bodyHTML}></article>`,
		},
	})
	diag, ok := findCode(report, "ghtml_over_load_data")
	if !ok {
		t.Fatalf("expected ghtml_over_load_data diagnostic, got %v", report)
	}
	if strings.Contains(diag.Message, "route param") {
		t.Fatalf("diagnostic must not misname the cause as a route param: %q", diag.Message)
	}
	if !strings.Contains(diag.Message, "load {}") {
		t.Fatalf("diagnostic should name load {} as the source: %q", diag.Message)
	}
}

func TestValidateAcceptsNestedGEach(t *testing.T) {
	report := validatePageListsFor(t, gwdkir.Page{
		Blocks: gwdkir.Blocks{
			Load:     true,
			LoadBody: `=> { columns }`,
			View:     true,
			ViewBody: `<div g:each={col in columns}><span g:each={issue in col.issues}>{issue.id}</span></div>`,
		},
	})
	for _, code := range []string{"geach_requires_load", "geach_nested_scope", "geach_invalid"} {
		if _, ok := findCode(report, code); ok {
			t.Fatalf("nested g:each should be accepted, got %s: %v", code, report)
		}
	}
}
