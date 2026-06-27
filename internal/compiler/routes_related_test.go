package compiler

import (
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

func span(line, startColumn, endColumn int) source.SourceSpan {
	return source.SourceSpan{
		Start: source.SourcePosition{Line: line, Column: startColumn},
		End:   source.SourcePosition{Line: line, Column: endColumn},
	}
}

func findByCode(diagnostics []ValidationError, code string) (ValidationError, bool) {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return diagnostic, true
		}
	}
	return ValidationError{}, false
}

func TestDuplicateRouteCarriesRelatedFirstDeclaration(t *testing.T) {
	pages := []gwdkir.Page{
		{ID: "home", Source: "home.page.gwdk", Route: "/", Spans: gwdkir.PageSpans{Route: span(2, 1, 9)}},
		{ID: "index", Source: "index.page.gwdk", Route: "/", Spans: gwdkir.PageSpans{Route: span(3, 1, 9)}},
	}

	diagnostic, ok := findByCode(validateUniquePageRoutes(gowdk.Config{}, pages), "duplicate_route")
	if !ok {
		t.Fatal("expected a duplicate_route diagnostic")
	}
	if diagnostic.Source != "index.page.gwdk" {
		t.Fatalf("primary diagnostic should point at the duplicate; got %q", diagnostic.Source)
	}
	if len(diagnostic.Related) != 1 {
		t.Fatalf("expected one related location, got %d", len(diagnostic.Related))
	}
	related := diagnostic.Related[0]
	if related.Source != "home.page.gwdk" {
		t.Fatalf("related location should point at the first declaration; got %q", related.Source)
	}
	if related.Span != span(2, 1, 9) {
		t.Fatalf("related span should be the first route span; got %+v", related.Span)
	}
	if related.Message == "" {
		t.Fatal("related location should carry a message")
	}
}

func TestDuplicatePageIDCarriesRelatedFirstDeclaration(t *testing.T) {
	pages := []gwdkir.Page{
		{ID: "home", Source: "a.page.gwdk", Spans: gwdkir.PageSpans{Page: span(1, 1, 9)}},
		{ID: "home", Source: "b.page.gwdk", Spans: gwdkir.PageSpans{Page: span(1, 1, 9)}},
	}
	diagnostic, ok := findByCode(validateUniquePages(pages), "duplicate_page_id")
	if !ok {
		t.Fatal("expected a duplicate_page_id diagnostic")
	}
	if len(diagnostic.Related) != 1 || diagnostic.Related[0].Source != "a.page.gwdk" {
		t.Fatalf("expected related location at first page; got %+v", diagnostic.Related)
	}
}

func TestDuplicatePageStoreCarriesRelatedFirstDeclaration(t *testing.T) {
	page := gwdkir.Page{
		ID:     "home",
		Source: "home.page.gwdk",
		Stores: []gwdkir.Store{
			{Name: "Counter", Span: span(3, 1, 8)},
			{Name: "Counter", Span: span(6, 1, 8)},
		},
	}
	diagnostic, ok := findByCode(validatePageStores(page), "duplicate_page_store")
	if !ok {
		t.Fatal("expected a duplicate_page_store diagnostic")
	}
	if len(diagnostic.Related) != 1 || diagnostic.Related[0].Span != span(3, 1, 8) {
		t.Fatalf("expected related location at first store; got %+v", diagnostic.Related)
	}
}

func TestContractRouteConflictCarriesRelatedFirstDeclaration(t *testing.T) {
	// Two differently-named query contracts on the same GET route conflict
	// through the shared route-registration path; the conflict must point back
	// at the first contract's declaration.
	refs := []gwdkir.ContractReference{
		{Kind: gwdkir.ContractQuery, Name: "Reports", Method: "GET", Path: "/reports", Source: "reports.gwdk", Span: span(4, 1, 12)},
		{Kind: gwdkir.ContractQuery, Name: "Summary", Method: "GET", Path: "/reports", Source: "summary.gwdk", Span: span(7, 1, 12)},
	}

	diagnostic, ok := findByCode(validateRouteMethodConflicts(gowdk.Config{}, nil, nil, gwdkir.SourceMap{}, refs), "route_method_conflict")
	if !ok {
		t.Fatal("expected a route_method_conflict diagnostic")
	}
	if len(diagnostic.Related) != 1 {
		t.Fatalf("expected one related location, got %d", len(diagnostic.Related))
	}
	related := diagnostic.Related[0]
	if related.Source != "reports.gwdk" {
		t.Fatalf("related location should point at the first contract; got %q", related.Source)
	}
	if related.Span != span(4, 1, 12) {
		t.Fatalf("related span should be the first contract span; got %+v", related.Span)
	}
}
