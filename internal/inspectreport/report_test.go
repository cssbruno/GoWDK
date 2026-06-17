package inspectreport

import (
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkir"
)

func TestBuildTreeLinksComponentCallsAcrossUsesAndReportsCycles(t *testing.T) {
	ir := gwdkir.Program{
		Packages: []gwdkir.Package{{Name: "pages"}, {Name: "ui"}},
		Pages: []gwdkir.Page{{
			Package: "pages",
			ID:      "home",
			Source:  "pages/home.page.gwdk",
			Uses:    []gwdkir.Use{{Alias: "ui", Package: "ui"}},
		}},
		Components: []gwdkir.Component{
			{Package: "ui", Name: "Card", Source: "ui/card.cmp.gwdk"},
			{Package: "ui", Name: "A", Source: "ui/a.cmp.gwdk"},
			{Package: "ui", Name: "B", Source: "ui/b.cmp.gwdk"},
		},
		Templates: []gwdkir.Template{
			{
				OwnerKind: gwdkir.SourcePage,
				OwnerID:   "home",
				Package:   "pages",
				Source:    "pages/home.page.gwdk",
				Uses:      []gwdkir.Use{{Alias: "ui", Package: "ui"}},
				Body:      `<main><ui.Card /></main>`,
			},
			{
				OwnerKind: gwdkir.SourceComponent,
				OwnerID:   "A",
				Package:   "ui",
				Source:    "ui/a.cmp.gwdk",
				Body:      `<B />`,
			},
			{
				OwnerKind: gwdkir.SourceComponent,
				OwnerID:   "B",
				Package:   "ui",
				Source:    "ui/b.cmp.gwdk",
				Body:      `<A />`,
			},
		},
	}

	report := BuildTree(ir)
	if !hasGraphEdge(report.Edges, "view:page:pages:home:0:0", "component:ui:Card", "renders_component") {
		t.Fatalf("expected page component-call edge, got %#v", report.Edges)
	}
	if len(report.Diagnostics) != 1 || report.Diagnostics[0].Code != "component_composition_cycle" {
		t.Fatalf("expected component cycle diagnostic, got %#v", report.Diagnostics)
	}
}

func TestBuildEndpointGraphLinksStructuralDispatchNodes(t *testing.T) {
	ir := gwdkir.Program{
		Packages: []gwdkir.Package{{Name: "pages"}},
		Pages: []gwdkir.Page{{
			Package: "pages",
			ID:      "home",
			Source:  "pages/home.page.gwdk",
			Route:   "/",
		}},
		Routes: []gwdkir.Route{{
			Package: "pages",
			PageID:  "home",
			Path:    "/",
			Method:  "GET",
			Source:  "pages/home.page.gwdk",
		}},
		Endpoints: []gwdkir.Endpoint{{
			Kind:       gwdkir.EndpointAction,
			Source:     gwdkir.EndpointSourceGOWDK,
			Package:    "pages",
			PageID:     "home",
			Symbol:     "Save",
			Method:     "POST",
			Path:       "/save",
			SourceFile: "pages/home.page.gwdk",
		}},
		ContractRefs: []gwdkir.ContractReference{{
			Kind:      gwdkir.ContractQuery,
			Name:      "patients.List",
			Method:    "GET",
			Path:      "/patients",
			OwnerKind: gwdkir.SourcePage,
			OwnerID:   "home",
			Package:   "pages",
			Source:    "pages/home.page.gwdk",
		}},
		Templates: []gwdkir.Template{{
			OwnerKind: gwdkir.SourcePage,
			OwnerID:   "home",
			Package:   "pages",
			Source:    "pages/home.page.gwdk",
			Body:      `<main><form g:post={Save}></form><section g:query="patients.List"></section><button g:on:click={Save()}></button></main>`,
		}},
	}

	report := BuildEndpointGraph(gowdk.Config{}, ir)
	if !hasGraphNode(report.Nodes, "view:page:pages:home:0:0", "structural") {
		t.Fatalf("expected structural form node, got %#v", report.Nodes)
	}
	if !hasGraphNode(report.Nodes, "view:page:pages:home:0:2", "structural") {
		t.Fatalf("expected structural event node, got %#v", report.Nodes)
	}
	if !hasGraphEdge(report.Edges, "view:page:pages:home:0:0", "endpoint:action:home:Save:POST:/save", "dispatches") {
		t.Fatalf("expected form dispatch edge, got %#v", report.Edges)
	}
	if !hasGraphEdge(report.Edges, "view:page:pages:home:0:1", "endpoint:query:home:patients.List:GET:/patients", "dispatches") {
		t.Fatalf("expected query dispatch edge, got %#v", report.Edges)
	}
}

func hasGraphNode(nodes []GraphNode, id, kind string) bool {
	for _, node := range nodes {
		if node.ID == id && node.Kind == kind {
			return true
		}
	}
	return false
}

func hasGraphEdge(edges []GraphEdge, from, to, kind string) bool {
	for _, edge := range edges {
		if edge.From == from && edge.To == to && edge.Kind == kind {
			return true
		}
	}
	return false
}
