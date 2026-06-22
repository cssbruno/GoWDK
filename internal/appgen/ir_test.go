package appgen

import (
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

func TestActionEndpointsFromIR(t *testing.T) {
	ir := gwdkir.Program{
		Pages: []gwdkir.Page{{
			ID:    "newsletter",
			Route: "/newsletter",
			Blocks: gwdkir.Blocks{
				ViewBody: `<form g:post={Subscribe}><input name="email" required /></form>`,
				Actions: []gwdkir.Action{{
					Name:           "Subscribe",
					InputName:      "input",
					InputType:      "SubscribeInput",
					ValidatesInput: true,
					Redirect:       "/newsletter?ok=1",
				}},
			},
		}},
		Endpoints: []gwdkir.Endpoint{{
			Kind:   gwdkir.EndpointAction,
			PageID: "newsletter",
			Symbol: "Subscribe",
			Method: "POST",
			Path:   "/newsletter",
			Binding: gwdkir.Binding{
				Status:       source.BackendBindingBound,
				ImportPath:   "example.com/app/newsletter",
				PackageName:  "newsletter",
				FunctionName: "Subscribe",
				Signature:    source.BackendSignatureAction0,
			},
		}},
	}

	endpoints, err := actionEndpointsFromIR(gowdk.Config{}, ir)
	if err != nil {
		t.Fatal(err)
	}
	if len(endpoints) != 1 {
		t.Fatalf("expected one action endpoint, got %#v", endpoints)
	}
	endpoint := endpoints[0]
	if endpoint.PageID != "newsletter" || endpoint.ActionName != "Subscribe" || endpoint.Route != "/newsletter" {
		t.Fatalf("unexpected endpoint: %#v", endpoint)
	}
	if len(endpoint.InputFields) != 1 || endpoint.InputFields[0] != "email" {
		t.Fatalf("expected form schema from IR page view, got %#v", endpoint.InputFields)
	}
	if endpoint.Binding.Status != source.BackendBindingBound || endpoint.Binding.FunctionName != "Subscribe" {
		t.Fatalf("expected backend binding from IR endpoint, got %#v", endpoint.Binding)
	}
}

func TestActionEndpointsFromIRLocalizesInheritedRoutes(t *testing.T) {
	ir := gwdkir.Program{
		Pages: []gwdkir.Page{{
			ID:    "contact",
			Route: "/contact",
			Blocks: gwdkir.Blocks{
				ViewBody: `<form g:post={Submit}><input name="email" required /></form>`,
				Actions: []gwdkir.Action{{
					Name: "Submit",
				}},
			},
		}},
		Endpoints: []gwdkir.Endpoint{{
			Kind:   gwdkir.EndpointAction,
			PageID: "contact",
			Symbol: "Submit",
			Method: "POST",
			Path:   "/contact",
			Binding: gwdkir.Binding{
				Status:       source.BackendBindingBound,
				ImportPath:   "example.com/app/contact",
				PackageName:  "contact",
				FunctionName: "Submit",
				Signature:    source.BackendSignatureAction0,
			},
		}},
	}

	endpoints, err := actionEndpointsFromIR(gowdk.Config{I18N: gowdk.I18NConfig{
		Locales: []gowdk.LocaleConfig{{Code: "en"}, {Code: "pt-BR", PathPrefix: "/br"}},
	}}, ir)
	if err != nil {
		t.Fatal(err)
	}
	if len(endpoints) != 2 {
		t.Fatalf("expected two localized action endpoints, got %#v", endpoints)
	}
	if endpoints[0].Route != "/en/contact" || endpoints[1].Route != "/br/contact" {
		t.Fatalf("unexpected localized action routes: %#v", endpoints)
	}
	for _, endpoint := range endpoints {
		if endpoint.Binding.Status != source.BackendBindingBound || endpoint.Binding.FunctionName != "Submit" {
			t.Fatalf("expected localized endpoint to preserve original binding, got %#v", endpoint)
		}
	}
}

func TestAPIEndpointsFromIR(t *testing.T) {
	endpoints, err := apiEndpointsFromIR(gwdkir.Program{
		Pages: []gwdkir.Page{{
			ID:    "status",
			Route: "/status",
			Blocks: gwdkir.Blocks{
				APIs: []gwdkir.API{{Name: "Health", Method: "GET", Route: "/api/health"}},
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(endpoints) != 1 || endpoints[0].APIName != "Health" || endpoints[0].Route != "/api/health" {
		t.Fatalf("unexpected API endpoints: %#v", endpoints)
	}
}

func TestFragmentEndpointsFromIR(t *testing.T) {
	endpoints, err := fragmentEndpointsFromIR(gwdkir.Program{
		Components: []gwdkir.Component{{
			Name:    "PatientCard",
			Package: "components",
			Props:   []gwdkir.Prop{{Name: "name", Type: "string"}},
			Blocks:  gwdkir.Blocks{View: true, ViewBody: `<article>{name}</article>`},
		}},
		Pages: []gwdkir.Page{{
			ID:      "patients",
			Route:   "/patients",
			Package: "pages",
			Uses:    []gwdkir.Use{{Alias: "ui", Package: "components"}},
			Guards:  []string{"auth.required"},
			Blocks: gwdkir.Blocks{
				Fragments: []gwdkir.FragmentEndpoint{{
					Name:   "List",
					Method: "GET",
					Route:  "/patients/list",
					Target: "#patients",
					Body:   `<section><ui.PatientCard name="Updated & safe" /></section>`,
				}},
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(endpoints) != 1 {
		t.Fatalf("expected one fragment endpoint, got %#v", endpoints)
	}
	endpoint := endpoints[0]
	if endpoint.FragmentName != "List" || endpoint.Route != "/patients/list" || endpoint.Target != "#patients" {
		t.Fatalf("unexpected fragment endpoint: %#v", endpoint)
	}
	if len(endpoint.RouteParams) != 0 {
		t.Fatalf("did not expect static fragment route params, got %#v", endpoint.RouteParams)
	}
	if endpoint.HTML != "<section><article>Updated &amp; safe</article></section>" {
		t.Fatalf("unexpected rendered fragment HTML: %q", endpoint.HTML)
	}
	if len(endpoint.Guards) != 1 || endpoint.Guards[0] != "auth.required" {
		t.Fatalf("expected inherited guards, got %#v", endpoint.Guards)
	}
}

func TestFragmentEndpointsFromIRPopulatesRouteParams(t *testing.T) {
	endpoints, err := fragmentEndpointsFromIR(gwdkir.Program{
		Pages: []gwdkir.Page{{
			ID:    "patients",
			Route: "/patients",
			Blocks: gwdkir.Blocks{
				Fragments: []gwdkir.FragmentEndpoint{{
					Name:   "Vitals",
					Method: "GET",
					Route:  "/patients/{id:int}/vitals/{section...}",
					Target: "#vitals",
					Body:   `<section>Vitals</section>`,
				}},
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(endpoints) != 1 {
		t.Fatalf("expected one fragment endpoint, got %#v", endpoints)
	}
	want := []source.RouteParam{{Name: "id", Type: "int"}, {Name: "section", Type: "string"}}
	got := endpoints[0].RouteParams
	if len(got) != len(want) {
		t.Fatalf("RouteParams = %#v, want %#v", got, want)
	}
	for index := range want {
		if got[index].Name != want[index].Name || got[index].Type != want[index].Type {
			t.Fatalf("RouteParams = %#v, want %#v", got, want)
		}
	}
}

func TestStandaloneGoEndpointsFromIR(t *testing.T) {
	ir := gwdkir.Program{
		Endpoints: []gwdkir.Endpoint{
			{
				Kind:   gwdkir.EndpointAction,
				Source: gwdkir.EndpointSourceGo,
				PageID: "auth.Login",
				Symbol: "Login",
				Method: "POST",
				Path:   "/login",
				Binding: gwdkir.Binding{
					Status:       source.BackendBindingBound,
					ImportPath:   "example.com/app/auth",
					PackageName:  "auth",
					FunctionName: "Login",
					Signature:    source.BackendSignatureAction0,
				},
			},
			{
				Kind:   gwdkir.EndpointAPI,
				Source: gwdkir.EndpointSourceGo,
				PageID: "api.Session",
				Symbol: "Session",
				Method: "GET",
				Path:   "/api/session",
			},
		},
	}
	actions, err := actionEndpointsFromIR(gowdk.Config{}, ir)
	if err != nil {
		t.Fatal(err)
	}
	apis, err := apiEndpointsFromIR(ir)
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 || actions[0].PageID != "auth.Login" || actions[0].ActionName != "Login" {
		t.Fatalf("unexpected standalone action endpoints: %#v", actions)
	}
	if len(apis) != 1 || apis[0].PageID != "api.Session" || apis[0].APIName != "Session" {
		t.Fatalf("unexpected standalone API endpoints: %#v", apis)
	}
}
