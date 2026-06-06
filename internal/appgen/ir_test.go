package appgen

import (
	"testing"

	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/manifest"
)

func TestActionEndpointsFromIR(t *testing.T) {
	ir := gwdkir.Program{
		Version: gwdkir.Version,
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
				Status:       manifest.BackendBindingBound,
				ImportPath:   "example.com/app/newsletter",
				PackageName:  "newsletter",
				FunctionName: "Subscribe",
				Signature:    manifest.BackendSignatureAction0,
			},
		}},
	}

	endpoints, err := actionEndpointsFromIR(ir)
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
	if endpoint.Binding.Status != manifest.BackendBindingBound || endpoint.Binding.FunctionName != "Subscribe" {
		t.Fatalf("expected backend binding from IR endpoint, got %#v", endpoint.Binding)
	}
}

func TestAPIEndpointsFromIR(t *testing.T) {
	endpoints, err := apiEndpointsFromIR(gwdkir.Program{
		Version: gwdkir.Version,
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
