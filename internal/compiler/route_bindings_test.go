package compiler

import (
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/addons/ssr"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

func TestBuildRouteMetadataSeparatesRoutesFromEndpoints(t *testing.T) {
	app := appFixture{
		Pages: []gwdkir.Page{
			{
				ID:     "newsletter",
				Route:  "/newsletter",
				Render: gowdk.SPA,
				Blocks: gwdkir.Blocks{
					View:    true,
					Actions: []gwdkir.Action{{Name: "Subscribe"}},
				},
			},
			{
				ID:     "dashboard",
				Route:  "/dashboard",
				Render: gowdk.SSR,
				Blocks: gwdkir.Blocks{
					Load: true,
					View: true,
				},
			},
			{
				ID:     "patients.index",
				Route:  "/patients",
				Render: gowdk.SPA,
				Blocks: gwdkir.Blocks{
					View:      true,
					APIs:      []gwdkir.API{{Name: "List", Method: "GET", Route: "/api/patients"}},
					Fragments: []gwdkir.FragmentEndpoint{{Name: "Table", Method: "GET", Route: "/patients/table", Target: "#patients", Body: "<section>Patients</section>"}},
				},
			},
		},
	}

	metadata, err := buildRouteMetadata(gowdk.Config{Addons: []gowdk.Addon{ssr.Addon()}}, app)
	if err != nil {
		t.Fatal(err)
	}

	assertRoute(t, metadata.Routes, RouteSPA, "GET", "/newsletter", `embedded.SPA("pages/newsletter.html")`)
	assertRoute(t, metadata.Routes, RouteSSR, "GET", "/dashboard", "ssr.RenderDashboard")
	assertEndpoint(t, metadata.Endpoints, EndpointAction, "POST", "/newsletter", "actions.NewsletterSubscribe")
	assertEndpoint(t, metadata.Endpoints, EndpointAPI, "GET", "/api/patients", "api.PatientsIndexList")
	assertEndpoint(t, metadata.Endpoints, EndpointFragment, "GET", "/patients/table", "fragments.PatientsIndexTable")
	assertInfo(t, metadata.Info, "ssr_disabled", "newsletter")
	assertInfo(t, metadata.Info, "spa_disabled", "dashboard")
}

func TestBuildRouteMetadataRejectsSSRWithoutAddon(t *testing.T) {
	app := appFixture{
		Pages: []gwdkir.Page{{
			ID:     "dashboard",
			Route:  "/dashboard",
			Render: gowdk.SSR,
			Blocks: gwdkir.Blocks{
				View: true,
			},
		}},
	}

	_, err := buildRouteMetadata(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected Missing SSR addon error")
	}
}

func TestBuildRouteMetadataRejectsHybridWithoutAddon(t *testing.T) {
	app := appFixture{Pages: []gwdkir.Page{{
		ID:     "dashboard",
		Route:  "/dashboard",
		Render: gowdk.Hybrid,
		Blocks: gwdkir.Blocks{
			View: true,
		},
	}}}

	_, err := buildRouteMetadata(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected missing SSR addon error")
	}
}

func TestBuildRouteMetadataMapsHybridWithoutExplicitLoadToHybridRoute(t *testing.T) {
	app := appFixture{Pages: []gwdkir.Page{{
		ID:     "dashboard",
		Route:  "/dashboard",
		Render: gowdk.Hybrid,
		Blocks: gwdkir.Blocks{
			View: true,
		},
	}}}

	metadata, err := buildRouteMetadata(gowdk.Config{Addons: []gowdk.Addon{ssr.Addon()}}, app)
	if err != nil {
		t.Fatal(err)
	}
	assertRoute(t, metadata.Routes, RouteHybrid, "GET", "/dashboard", "hybrid.RenderDashboard")
}

func TestBuildRouteMetadataMapsHybridWithLoadToHybridRoute(t *testing.T) {
	app := appFixture{Pages: []gwdkir.Page{{
		ID:     "dashboard",
		Route:  "/dashboard",
		Render: gowdk.Hybrid,
		Blocks: gwdkir.Blocks{
			Load: true,
			View: true,
		},
	}}}

	metadata, err := buildRouteMetadata(gowdk.Config{Addons: []gowdk.Addon{ssr.Addon()}}, app)
	if err != nil {
		t.Fatal(err)
	}
	assertRoute(t, metadata.Routes, RouteHybrid, "GET", "/dashboard", "hybrid.RenderDashboard")
}

func TestBuildRouteMetadataFromIR(t *testing.T) {
	metadata := BuildRouteMetadataFromIR(gowdk.Config{}, gwdkir.Program{
		Version: gwdkir.Version,
		Routes: []gwdkir.Route{
			{Kind: gwdkir.RouteSPA, Method: "GET", Path: "/newsletter", PageID: "newsletter", Render: gowdk.SPA},
			{Kind: gwdkir.RouteSSR, Method: "GET", Path: "/dashboard", PageID: "dashboard", Render: gowdk.SSR},
		},
		Endpoints: []gwdkir.Endpoint{
			{
				Kind:       gwdkir.EndpointAction,
				Source:     gwdkir.EndpointSourceGOWDK,
				PageID:     "newsletter",
				Symbol:     "Subscribe",
				Method:     "POST",
				Path:       "/newsletter",
				SourceFile: "newsletter.page.gwdk",
				Binding: gwdkir.Binding{
					Status:       source.BackendBindingBound,
					ImportPath:   "example.com/app/newsletter",
					PackageName:  "newsletter",
					FunctionName: "Subscribe",
					Signature:    source.BackendSignatureAction0,
				},
			},
			{
				Kind:       gwdkir.EndpointFragment,
				Source:     gwdkir.EndpointSourceGOWDK,
				PageID:     "newsletter",
				Symbol:     "List",
				Method:     "GET",
				Path:       "/newsletter/list",
				SourceFile: "newsletter.page.gwdk",
			},
		},
		ContractRefs: []gwdkir.ContractReference{{
			Kind:        gwdkir.ContractCommand,
			Name:        "patients.CreatePatient",
			ImportAlias: "patients",
			Type:        "CreatePatient",
			Result:      "CreatePatientResult",
			Roles:       []string{"web"},
			Method:      "POST",
			Path:        "/patients",
			Status:      gwdkir.ContractBindingBound,
			Handler:     "HandleCreatePatient",
			Register:    "Register",
			OwnerKind:   gwdkir.SourcePage,
			OwnerID:     "patients",
			Package:     "pages",
			Source:      "patients.page.gwdk",
		}},
	})

	assertRoute(t, metadata.Routes, RouteSPA, "GET", "/newsletter", `embedded.SPA("pages/newsletter.html")`)
	assertRoute(t, metadata.Routes, RouteSSR, "GET", "/dashboard", "ssr.RenderDashboard")
	assertEndpoint(t, metadata.Endpoints, EndpointAction, "POST", "/newsletter", "actions.NewsletterSubscribe")
	assertEndpoint(t, metadata.Endpoints, EndpointFragment, "GET", "/newsletter/list", "fragments.NewsletterList")
	assertEndpoint(t, metadata.Endpoints, EndpointCommand, "POST", "/patients", "contracts.command.patients.CreatePatient")
	if metadata.Endpoints[0].BindingStatus != source.BackendBindingBound {
		t.Fatalf("expected binding status from IR, got %#v", metadata.Endpoints[0])
	}
	command := findEndpoint(t, metadata.Endpoints, EndpointCommand, "POST", "/patients")
	if command.Contract.Name != "patients.CreatePatient" ||
		command.Contract.Status != gwdkir.ContractBindingBound ||
		command.Contract.Handler != "HandleCreatePatient" ||
		len(command.Contract.Roles) != 1 ||
		command.Contract.Roles[0] != "web" {
		t.Fatalf("unexpected command contract endpoint metadata: %#v", command.Contract)
	}
}

func assertRoute(t *testing.T, routes []RouteBinding, kind RouteKind, method, route, handler string) {
	t.Helper()
	for _, binding := range routes {
		if binding.Kind == kind && binding.Method == method && binding.Route == route && binding.Handler == handler {
			return
		}
	}
	t.Fatalf("Missing route kind=%s method=%s route=%s handler=%s in %#v", kind, method, route, handler, routes)
}

func assertEndpoint(t *testing.T, endpoints []EndpointBinding, kind EndpointKind, method, route, handler string) {
	t.Helper()
	_ = findEndpoint(t, endpoints, kind, method, route, handler)
}

func findEndpoint(t *testing.T, endpoints []EndpointBinding, kind EndpointKind, method, route string, handler ...string) EndpointBinding {
	t.Helper()
	for _, binding := range endpoints {
		if binding.Kind != kind || binding.Method != method || binding.Route != route {
			continue
		}
		if len(handler) > 0 && binding.Handler != handler[0] {
			continue
		}
		return binding
	}
	t.Fatalf("Missing endpoint kind=%s method=%s route=%s handler=%v in %#v", kind, method, route, handler, endpoints)
	return EndpointBinding{}
}

func assertInfo(t *testing.T, infos []RouteInfo, code string, pageID string) {
	t.Helper()
	for _, info := range infos {
		if info.Code == code && info.PageID == pageID {
			return
		}
	}
	t.Fatalf("Missing info code=%s page=%s in %#v", code, pageID, infos)
}

// buildRouteMetadata validates and binds a manifest fixture through the
// production IR path before deriving route metadata, mirroring the deleted
// manifest-typed BuildRouteMetadata entrypoint these tests were written
// against.
func buildRouteMetadata(config gowdk.Config, app appFixture) (RouteMetadata, error) {
	ir := app.program(config)
	if err := ValidateProgram(config, ir); err != nil {
		return RouteMetadata{}, err
	}
	BindBackendHandlers(&ir)
	return BuildRouteMetadataFromIR(config, ir), nil
}
