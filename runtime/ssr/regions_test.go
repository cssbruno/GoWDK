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

func TestRegisterRegionIgnoresIncomplete(t *testing.T) {
	resetRegions()
	defer resetRegions()
	RegisterRegion(RegionRenderer{QueryType: "", Load: func(*http.Request) (map[string]any, error) { return nil, nil }})
	RegisterRegion(RegionRenderer{QueryType: "example.com/app.Q", Load: nil})
	if patches := RenderInvalidatedRegions(httptest.NewRequest(http.MethodPost, "/", nil), []string{"example.com/app.Q"}); len(patches) != 0 {
		t.Fatalf("expected incomplete renderers to be ignored, got %+v", patches)
	}
}
