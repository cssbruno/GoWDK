package compiler

import (
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
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
	program := gwdkanalysis.BuildProgram(config, gwdkanalysis.Sources{Pages: []gwdkir.Page{page}})
	report := ValidateProgramReport(config, program)
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

func TestValidateRejectsServerLoadFieldConflictingWithRouteParam(t *testing.T) {
	report := validatePageListsFor(t, gwdkir.Page{
		Route: "/issue/{id}",
		Blocks: gwdkir.Blocks{
			Server:     true,
			ServerBody: `=> { id, found }`,
			View:       true,
			ViewBody:   `<main g:if={found}>{id}</main>`,
		},
	})
	diag, ok := findCode(report, "server_load_field_conflict")
	if !ok {
		t.Fatalf("expected server_load_field_conflict, got %v", report)
	}
	if !strings.Contains(diag.Message, "route params") {
		t.Fatalf("diagnostic should name route param conflict: %q", diag.Message)
	}
}

func TestValidateRejectsServerLoadFieldConflictingWithBuildData(t *testing.T) {
	report := validatePageListsFor(t, gwdkir.Page{
		Blocks: gwdkir.Blocks{
			Build: true,
			BuildRecords: []gwdkir.LiteralRecord{{
				Fields:     map[string]string{"title": "Build title"},
				FieldOrder: []string{"title"},
			}},
			Server:     true,
			ServerBody: `=> { title }`,
			View:       true,
			ViewBody:   `<main>{title}</main>`,
		},
	})
	diag, ok := findCode(report, "server_load_field_conflict")
	if !ok {
		t.Fatalf("expected server_load_field_conflict, got %v", report)
	}
	if !strings.Contains(diag.Message, "build data") {
		t.Fatalf("diagnostic should name build data conflict: %q", diag.Message)
	}
}

func TestValidateAcceptsTypedServerLoadResultFields(t *testing.T) {
	report := validatePageListsFor(t, gwdkir.Page{
		LoadBinding: gwdkir.Binding{
			Status:     source.BackendBindingBound,
			ResultType: "DashboardData",
			ResultFields: []source.BackendResultField{
				{Path: "user", Selector: "User"},
				{Path: "user.name", Selector: "User.Name"},
				{Path: "count", Selector: "Count"},
			},
		},
		Blocks: gwdkir.Blocks{
			Server:     true,
			ServerBody: `=> { user.name, count }`,
			View:       true,
			ViewBody:   `<main>{user.name} {count}</main>`,
		},
	})
	if _, ok := findCode(report, "server_load_field_unknown"); ok {
		t.Fatalf("typed load result fields should be accepted, got %v", report)
	}
}

func TestValidateRejectsUnknownTypedServerLoadResultField(t *testing.T) {
	report := validatePageListsFor(t, gwdkir.Page{
		LoadBinding: gwdkir.Binding{
			Status:     source.BackendBindingBound,
			ResultType: "DashboardData",
			ResultFields: []source.BackendResultField{
				{Path: "user", Selector: "User"},
				{Path: "user.name", Selector: "User.Name"},
			},
		},
		Blocks: gwdkir.Blocks{
			Server:     true,
			ServerBody: `=> { user.email }`,
			View:       true,
			ViewBody:   `<main>{user.email}</main>`,
		},
	})
	diag, ok := findCode(report, "server_load_field_unknown")
	if !ok {
		t.Fatalf("expected server_load_field_unknown, got %v", report)
	}
	if !strings.Contains(diag.Message, `"user.email"`) || !strings.Contains(diag.Message, "DashboardData") {
		t.Fatalf("diagnostic should name field and result type: %q", diag.Message)
	}
}

func TestValidateAcceptsRequestTimeURLTemplateInsideServerRow(t *testing.T) {
	report := validatePageListsFor(t, gwdkir.Page{
		Blocks: gwdkir.Blocks{
			Server:     true,
			ServerBody: `=> { issues }`,
			View:       true,
			ViewBody:   `<main><a g:for={issue in issues} href="/issue/{issue.id}">{issue.title}</a></main>`,
		},
	})
	if _, ok := findCode(report, "server_url_tainted"); ok {
		t.Fatalf("root-relative server row URL templates should be accepted, got %v", report)
	}
}

func TestValidateRejectsRequestTimeSrcsetCandidateWithoutStablePrefix(t *testing.T) {
	report := validatePageListsFor(t, gwdkir.Page{
		Blocks: gwdkir.Blocks{
			Server:     true,
			ServerBody: `=> { issues }`,
			View:       true,
			ViewBody:   `<main><img g:for={issue in issues} srcset="/safe.png 1x, {issue.image} 2x" /></main>`,
		},
	})
	diag, ok := findCode(report, "server_url_tainted")
	if !ok {
		t.Fatalf("expected server_url_tainted for tainted srcset candidate, got %v", report)
	}
	if !strings.Contains(diag.Message, `"srcset"`) {
		t.Fatalf("diagnostic should name srcset: %q", diag.Message)
	}
}

func TestValidateAcceptsRequestTimeSrcsetCandidatesWithStablePrefixes(t *testing.T) {
	report := validatePageListsFor(t, gwdkir.Page{
		Blocks: gwdkir.Blocks{
			Server:     true,
			ServerBody: `=> { issues }`,
			View:       true,
			ViewBody:   `<main><img g:for={issue in issues} srcset="/image/{issue.image} 1x, /image/{issue.image} 2x" /></main>`,
		},
	})
	if _, ok := findCode(report, "server_url_tainted"); ok {
		t.Fatalf("root-relative srcset URL templates should be accepted, got %v", report)
	}
}

func TestValidateRejectsRequestTimeURLAttributeControlledByServerRow(t *testing.T) {
	report := validatePageListsFor(t, gwdkir.Page{
		Blocks: gwdkir.Blocks{
			Server:     true,
			ServerBody: `=> { issues }`,
			View:       true,
			ViewBody:   `<main><a g:for={issue in issues} href={issue.id}>{issue.title}</a></main>`,
		},
	})
	diag, ok := findCode(report, "server_url_tainted")
	if !ok {
		t.Fatalf("expected server_url_tainted, got %v", report)
	}
	if !strings.Contains(diag.Message, `"href"`) || !strings.Contains(diag.Message, "request-time") {
		t.Fatalf("diagnostic should name href and request-time data: %q", diag.Message)
	}
}

func TestValidateRejectsRequestTimeURLTemplateWithDynamicRoot(t *testing.T) {
	report := validatePageListsFor(t, gwdkir.Page{
		Blocks: gwdkir.Blocks{
			Server:     true,
			ServerBody: `=> { issues }`,
			View:       true,
			ViewBody:   `<main><a g:for={issue in issues} href="/{issue.id}">{issue.title}</a></main>`,
		},
	})
	if _, ok := findCode(report, "server_url_tainted"); !ok {
		t.Fatalf("expected server_url_tainted for a URL whose first path segment is request-time data, got %v", report)
	}
}

func TestValidateRejectsRequestTimeLoadURLInterpolation(t *testing.T) {
	report := validatePageListsFor(t, gwdkir.Page{
		Blocks: gwdkir.Blocks{
			Server:     true,
			ServerBody: `=> { website }`,
			View:       true,
			ViewBody:   `<main><a href="{website}">Profile</a></main>`,
		},
	})
	if _, ok := findCode(report, "server_url_tainted"); !ok {
		t.Fatalf("expected server_url_tainted for load field href, got %v", report)
	}
}

func TestValidateRejectsDirectiveInsideServerRow(t *testing.T) {
	report := validatePageListsFor(t, gwdkir.Page{
		Blocks: gwdkir.Blocks{
			Server:     true,
			ServerBody: `=> { issues }`,
			Actions:    []gwdkir.Action{{Name: "Open", Method: "POST", Route: "/open"}},
			View:       true,
			ViewBody:   `<main><form g:for={issue in issues} g:post={Open}><input name="id" value={issue.id} /></form></main>`,
		},
	})
	diag, ok := findCode(report, "server_region_directive")
	if !ok {
		t.Fatalf("expected server_region_directive, got %v", report)
	}
	if !strings.Contains(diag.Message, `"g:post"`) {
		t.Fatalf("diagnostic should name the unsupported directive: %q", diag.Message)
	}
}

func TestValidateRejectsComponentInsideServerRow(t *testing.T) {
	report := validatePageListsFor(t, gwdkir.Page{
		Blocks: gwdkir.Blocks{
			Server:     true,
			ServerBody: `=> { issues }`,
			View:       true,
			ViewBody:   `<main><div g:for={issue in issues}><IssueCard /></div></main>`,
		},
	})
	if _, ok := findCode(report, "server_region_directive"); !ok {
		t.Fatalf("expected server_region_directive for component call, got %v", report)
	}
}

func TestValidateRejectsDirectiveInsideTopLevelServerIf(t *testing.T) {
	report := validatePageListsFor(t, gwdkir.Page{
		Blocks: gwdkir.Blocks{
			Server:     true,
			ServerBody: `=> { found }`,
			Actions:    []gwdkir.Action{{Name: "Open", Method: "POST", Route: "/open"}},
			View:       true,
			ViewBody:   `<main><section g:if={found}><form g:post={Open}></form></section></main>`,
		},
	})
	diag, ok := findCode(report, "server_region_directive")
	if !ok {
		t.Fatalf("expected server_region_directive inside server g:if, got %v", report)
	}
	if !strings.Contains(diag.Message, `"g:post"`) {
		t.Fatalf("diagnostic should name the unsupported directive: %q", diag.Message)
	}
}

func TestValidateRejectsComponentInsideTopLevelServerIf(t *testing.T) {
	report := validatePageListsFor(t, gwdkir.Page{
		Blocks: gwdkir.Blocks{
			Server:     true,
			ServerBody: `=> { found }`,
			View:       true,
			ViewBody:   `<main><section g:if={found}><IssueCard /></section></main>`,
		},
	})
	if _, ok := findCode(report, "server_region_directive"); !ok {
		t.Fatalf("expected server_region_directive for component call inside server g:if, got %v", report)
	}
}

func TestValidateRejectsRequestTimeURLInsideTopLevelServerIf(t *testing.T) {
	report := validatePageListsFor(t, gwdkir.Page{
		Blocks: gwdkir.Blocks{
			Server:     true,
			ServerBody: `=> { found, website }`,
			View:       true,
			ViewBody:   `<main><section g:if={found}><a href="{website}">Profile</a></section></main>`,
		},
	})
	if _, ok := findCode(report, "server_url_tainted"); !ok {
		t.Fatalf("expected server_url_tainted inside server g:if, got %v", report)
	}
}
