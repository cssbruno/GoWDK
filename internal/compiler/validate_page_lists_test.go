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

func TestValidateAcceptsGForOverServerData(t *testing.T) {
	// g:for over a server {} field renders server-side; it must not be rejected.
	report := validatePageListsFor(t, gwdkir.Page{
		Blocks: gwdkir.Blocks{
			Server:     true,
			ServerBody: `=> { columns }`,
			View:       true,
			ViewBody:   `<div g:for={col in columns}>{col.title}</div>`,
		},
	})
	for _, code := range []string{"server_for_invalid", "server_for_nested_scope"} {
		if _, ok := findCode(report, code); ok {
			t.Fatalf("g:for over a server field should be accepted, got %s: %v", code, report)
		}
	}
}

func TestValidateAcceptsGIfOverServerData(t *testing.T) {
	report := validatePageListsFor(t, gwdkir.Page{
		Blocks: gwdkir.Blocks{
			Server:     true,
			ServerBody: `=> { hasItems, count }`,
			View:       true,
			ViewBody:   `<section><p g:if={hasItems}>You have {count}</p><p g:if={!hasItems}>None</p></section>`,
		},
	})
	for _, code := range []string{"server_if_invalid", "server_if_nested_scope"} {
		if _, ok := findCode(report, code); ok {
			t.Fatalf("g:if over a server field should be accepted, got %s: %v", code, report)
		}
	}
}

func TestValidateAcceptsGForOverClientState(t *testing.T) {
	// No server {} block: g:for is the client lane and is validated by the island
	// validator, so the server-list validator must stay silent.
	report := validatePageListsFor(t, gwdkir.Page{
		Render: gowdk.SPA,
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<div g:for={item in items} g:key={item.id}>{item.title}</div>`,
		},
	})
	for _, code := range []string{"server_for_invalid", "server_for_nested_scope"} {
		if _, ok := findCode(report, code); ok {
			t.Fatalf("client-lane g:for must not be flagged by the server validator, got %s: %v", code, report)
		}
	}
}

func TestValidateAcceptsGIfInsideServerRow(t *testing.T) {
	report := validatePageListsFor(t, gwdkir.Page{
		Blocks: gwdkir.Blocks{
			Server:     true,
			ServerBody: `=> { issues }`,
			View:       true,
			ViewBody:   `<ul><li g:for={issue in issues}>{issue.id}<b g:if={issue.urgent}>!</b></li></ul>`,
		},
	})
	for _, code := range []string{"server_if_invalid", "server_if_nested_scope"} {
		if _, ok := findCode(report, code); ok {
			t.Fatalf("g:if referencing the row item should be accepted, got %s: %v", code, report)
		}
	}
}

func TestValidateAcceptsNestedGFor(t *testing.T) {
	report := validatePageListsFor(t, gwdkir.Page{
		Blocks: gwdkir.Blocks{
			Server:     true,
			ServerBody: `=> { columns }`,
			View:       true,
			ViewBody:   `<div g:for={col in columns}><span g:for={issue in col.issues}>{issue.id}</span></div>`,
		},
	})
	for _, code := range []string{"server_for_invalid", "server_for_nested_scope"} {
		if _, ok := findCode(report, code); ok {
			t.Fatalf("nested server g:for should be accepted, got %s: %v", code, report)
		}
	}
}

func TestValidateRejectsNestedGForWrongScope(t *testing.T) {
	report := validatePageListsFor(t, gwdkir.Page{
		Blocks: gwdkir.Blocks{
			Server:     true,
			ServerBody: `=> { columns }`,
			View:       true,
			ViewBody:   `<div g:for={col in columns}><span g:for={i in other}>{i.id}</span></div>`,
		},
	})
	if _, ok := findCode(report, "server_for_nested_scope"); !ok {
		t.Fatalf("expected server_for_nested_scope diagnostic, got %v", report)
	}
}

func TestValidateRejectsNestedGForOverParentItem(t *testing.T) {
	report := validatePageListsFor(t, gwdkir.Page{
		Blocks: gwdkir.Blocks{
			Server:     true,
			ServerBody: `=> { columns }`,
			View:       true,
			ViewBody:   `<div g:for={col in columns}><span g:for={issue in col}>{issue.id}</span></div>`,
		},
	})
	diag, ok := findCode(report, "server_for_nested_scope")
	if !ok {
		t.Fatalf("expected server_for_nested_scope when iterating the parent item itself, got %v", report)
	}
	if !strings.Contains(diag.Message, "itself") {
		t.Fatalf("diagnostic should explain it cannot be the parent item itself: %q", diag.Message)
	}
}

func TestValidateRejectsNestedGIfWrongScope(t *testing.T) {
	report := validatePageListsFor(t, gwdkir.Page{
		Blocks: gwdkir.Blocks{
			Server:     true,
			ServerBody: `=> { issues }`,
			View:       true,
			ViewBody:   `<ul><li g:for={issue in issues}><b g:if={other}>!</b></li></ul>`,
		},
	})
	if _, ok := findCode(report, "server_if_nested_scope"); !ok {
		t.Fatalf("expected server_if_nested_scope diagnostic, got %v", report)
	}
}

func TestValidateRejectsGHTMLOverServerData(t *testing.T) {
	report := validatePageListsFor(t, gwdkir.Page{
		Blocks: gwdkir.Blocks{
			Server:     true,
			ServerBody: `=> { bodyHTML }`,
			View:       true,
			ViewBody:   `<article g:unsafe-html={bodyHTML}></article>`,
		},
	})
	diag, ok := findCode(report, "ghtml_over_load_data")
	if !ok {
		t.Fatalf("expected ghtml_over_load_data diagnostic, got %v", report)
	}
	if strings.Contains(diag.Message, "route param") {
		t.Fatalf("diagnostic must not misname the cause as a route param: %q", diag.Message)
	}
	if !strings.Contains(diag.Message, "server {}") {
		t.Fatalf("diagnostic should name server {} as the source: %q", diag.Message)
	}
}

func TestValidateRejectsGHTMLInsideServerRow(t *testing.T) {
	report := validatePageListsFor(t, gwdkir.Page{
		Blocks: gwdkir.Blocks{
			Server:     true,
			ServerBody: `=> { issues }`,
			View:       true,
			ViewBody:   `<ul><li g:for={issue in issues}><span g:unsafe-html={issue.body}></span></li></ul>`,
		},
	})
	if _, ok := findCode(report, "ghtml_over_load_data"); !ok {
		t.Fatalf("expected ghtml_over_load_data for raw HTML inside a server row, got %v", report)
	}
}
