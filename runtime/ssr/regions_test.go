package ssr

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func boardRegion(queryType string) RegionRenderer {
	return RegionRenderer{
		QueryType: queryType,
		Template:  `<section data-gowdk-query-type="` + queryType + `"><ul>__LIST__</ul></section>`,
		Lists: []ListSpec{{
			Placeholder: "__LIST__",
			SourcePath:  "patients",
			RowTemplate: "<li>__NAME__</li>",
			Fields:      []ListField{{Placeholder: "__NAME__", Path: "Name"}},
		}},
		Load: func(*http.Request) (map[string]any, error) {
			return map[string]any{"patients": []map[string]any{{"Name": "Ada"}, {"Name": "Linus"}}}, nil
		},
	}
}

func TestRenderInvalidatedRegionsRendersRegisteredRegion(t *testing.T) {
	resetRegions()
	defer resetRegions()
	const queryType = "example.com/app/patients.GetPatientPage"
	RegisterRegion(boardRegion(queryType))

	patches := RenderInvalidatedRegions(httptest.NewRequest(http.MethodPost, "/patients", nil), []string{queryType})
	if len(patches) != 1 {
		t.Fatalf("expected 1 patch, got %d", len(patches))
	}
	if patches[0].Query != queryType {
		t.Fatalf("expected patch query %q, got %q", queryType, patches[0].Query)
	}
	if !strings.Contains(patches[0].HTML, "<li>Ada</li>") || !strings.Contains(patches[0].HTML, "<li>Linus</li>") {
		t.Fatalf("expected rendered rows in patch HTML, got %q", patches[0].HTML)
	}
}

func TestRegionRendererEscapesURLLoadFields(t *testing.T) {
	renderer := RegionRenderer{
		QueryType: "example.com/app/profile.Load",
		Template:  `<a href="/user/__SLUG__">Profile</a>`,
		LoadFields: []RegionLoadField{{
			Path:        "slug",
			Placeholder: "__SLUG__",
			URL:         true,
		}},
		Load: func(*http.Request) (map[string]any, error) {
			return map[string]any{"slug": `\\evil.com`}, nil
		},
	}
	got, ok := renderer.render(httptest.NewRequest(http.MethodPost, "/profile", nil))
	if !ok {
		t.Fatal("expected region to render")
	}
	const want = `<a href="/user/%5C%5Cevil.com">Profile</a>`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRenderInvalidatedRegionsSkipsUnregisteredAndAmbiguous(t *testing.T) {
	resetRegions()
	defer resetRegions()
	const single = "example.com/app/patients.GetPatientPage"
	const ambiguous = "example.com/app/patients.GetDashboard"
	RegisterRegion(boardRegion(single))
	// Registering the same query type twice marks it ambiguous: the command
	// request cannot tell which page's region the submitter is viewing.
	RegisterRegion(boardRegion(ambiguous))
	RegisterRegion(boardRegion(ambiguous))

	patches := RenderInvalidatedRegions(httptest.NewRequest(http.MethodPost, "/patients", nil), []string{single, ambiguous, "example.com/app/patients.Unregistered"})
	if len(patches) != 1 || patches[0].Query != single {
		t.Fatalf("expected only the unambiguous registered region, got %+v", patches)
	}
}

func TestRenderInvalidatedRegionsUsesRoutePathForAmbiguousQuery(t *testing.T) {
	resetRegions()
	defer resetRegions()
	const queryType = "example.com/app/patients.GetPatientPage"
	board := boardRegion(queryType)
	board.Route = "/board"
	dashboard := boardRegion(queryType)
	dashboard.Route = "/dashboard"
	dashboard.Load = func(*http.Request) (map[string]any, error) {
		return map[string]any{"patients": []map[string]any{{"Name": "Grace"}}}, nil
	}
	RegisterRegion(board)
	RegisterRegion(dashboard)

	if patches := RenderInvalidatedRegions(httptest.NewRequest(http.MethodPost, "/patients", nil), []string{queryType}); len(patches) != 0 {
		t.Fatalf("query-only rendering for ambiguous route should fall back, got %+v", patches)
	}

	request := httptest.NewRequest(http.MethodGet, "/_gowdk/realtime/query-refresh?path=%2Fdashboard", nil)
	patches := RenderInvalidatedRegions(request, []string{queryType})
	if len(patches) != 1 {
		t.Fatalf("expected one route-scoped patch, got %+v", patches)
	}
	if !strings.Contains(patches[0].HTML, "<li>Grace</li>") || strings.Contains(patches[0].HTML, "<li>Ada</li>") {
		t.Fatalf("expected dashboard route patch, got %q", patches[0].HTML)
	}

	request = httptest.NewRequest(http.MethodGet, "/_gowdk/realtime/query-refresh?path=%2Fsettings", nil)
	if patches := RenderInvalidatedRegions(request, []string{queryType}); len(patches) != 0 {
		t.Fatalf("wrong-route refresh should not render patches, got %+v", patches)
	}
}

func TestRenderInvalidatedRegionsHonorsRequestedRoute(t *testing.T) {
	resetRegions()
	defer resetRegions()
	const queryType = "example.com/app/patients.GetPatientPage"
	renderer := boardRegion(queryType)
	renderer.Route = "/patients"
	RegisterRegion(renderer)

	wrongRoute := httptest.NewRequest(http.MethodGet, "/_gowdk/realtime/query-refresh?path=%2Fdashboard&query="+queryType, nil)
	if patches := RenderInvalidatedRegions(wrongRoute, []string{queryType}); len(patches) != 0 {
		t.Fatalf("expected no patches for a mismatched route, got %+v", patches)
	}

	matchingRoute := httptest.NewRequest(http.MethodGet, "/_gowdk/realtime/query-refresh?path=%2Fpatients%3Fpage%3D2&query="+queryType, nil)
	patches := RenderInvalidatedRegions(matchingRoute, []string{queryType})
	if len(patches) != 1 || patches[0].Query != queryType {
		t.Fatalf("expected matching route patch, got %+v", patches)
	}
	if got := RegionRequestPath(matchingRoute); got != "/patients" {
		t.Fatalf("RegionRequestPath = %q, want /patients", got)
	}
	if got := RegionRequestRawQuery(matchingRoute); got != "page=2" {
		t.Fatalf("RegionRequestRawQuery = %q, want page=2", got)
	}
}

func TestRenderInvalidatedRegionsSelectsRouteFromSharedQueryType(t *testing.T) {
	resetRegions()
	defer resetRegions()
	const queryType = "example.com/app/patients.GetPatientPage"
	patients := boardRegion(queryType)
	patients.Route = "/patients"
	board := boardRegion(queryType)
	board.Route = "/board"
	board.Template = `<section data-gowdk-query-type="` + queryType + `"><p>Board</p></section>`
	RegisterRegion(patients)
	RegisterRegion(board)

	commandRequest := httptest.NewRequest(http.MethodPost, "/commands/create", nil)
	if patches := RenderInvalidatedRegions(commandRequest, []string{queryType}); len(patches) != 0 {
		t.Fatalf("expected ambiguous command patch to fall back, got %+v", patches)
	}

	refreshRequest := httptest.NewRequest(http.MethodGet, "/_gowdk/realtime/query-refresh?path=%2Fboard&query="+queryType, nil)
	patches := RenderInvalidatedRegions(refreshRequest, []string{queryType})
	if len(patches) != 1 || !strings.Contains(patches[0].HTML, "Board") {
		t.Fatalf("expected route-specific board patch, got %+v", patches)
	}
}

func TestRenderInvalidatedRegionsSkipsOnLoadError(t *testing.T) {
	resetRegions()
	defer resetRegions()
	const queryType = "example.com/app/patients.GetPatientPage"
	renderer := boardRegion(queryType)
	renderer.Load = func(*http.Request) (map[string]any, error) {
		return nil, errors.New("load failed")
	}
	RegisterRegion(renderer)

	patches := RenderInvalidatedRegions(httptest.NewRequest(http.MethodPost, "/patients", nil), []string{queryType})
	if len(patches) != 0 {
		t.Fatalf("expected no patches when load fails, got %+v", patches)
	}
}

func TestRenderInvalidatedRegionsSkipsPostForms(t *testing.T) {
	resetRegions()
	defer resetRegions()
	const queryType = "example.com/app/patients.GetPatientPage"
	renderer := boardRegion(queryType)
	renderer.Template = `<section data-gowdk-query-type="` + queryType + `"><form method="POST" action="/patients"><button>Save</button></form></section>`
	RegisterRegion(renderer)

	patches := RenderInvalidatedRegions(httptest.NewRequest(http.MethodPost, "/patients", nil), []string{queryType})
	if len(patches) != 0 {
		t.Fatalf("expected no patches for HTML containing POST forms, got %+v", patches)
	}
}

func TestRenderInvalidatedRegionsSkipsFormmethodPostControls(t *testing.T) {
	resetRegions()
	defer resetRegions()
	const queryType = "example.com/app/patients.GetPatientPage"
	renderer := boardRegion(queryType)
	// A nominally GET form that POSTs via a submit button override.
	renderer.Template = `<section data-gowdk-query-type="` + queryType + `"><form method="get"><button formmethod="post">Save</button></form></section>`
	RegisterRegion(renderer)

	patches := RenderInvalidatedRegions(httptest.NewRequest(http.MethodPost, "/patients", nil), []string{queryType})
	if len(patches) != 0 {
		t.Fatalf("expected no patches for HTML with formmethod=post controls, got %+v", patches)
	}
}

func TestRegisterRegionIgnoresIncomplete(t *testing.T) {
	resetRegions()
	defer resetRegions()
	RegisterRegion(RegionRenderer{QueryType: "", Load: func(*http.Request) (map[string]any, error) { return nil, nil }})
	RegisterRegion(RegionRenderer{QueryType: "example.com/app.Q", Load: nil})
	if patches := RenderInvalidatedRegions(httptest.NewRequest(http.MethodPost, "/", nil), []string{"example.com/app.Q"}); len(patches) != 0 {
		t.Fatalf("expected incomplete renderers to be ignored, got %+v", patches)
	}
}
