package buildgen

import (
	"testing"

	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

func TestBuildOpenAPISpecKeepsOriginalGOWDKRoute(t *testing.T) {
	spec := buildOpenAPISpec(gwdkir.Program{
		Endpoints: []gwdkir.Endpoint{{
			Kind:   gwdkir.EndpointFragment,
			Method: "GET",
			Path:   "/docs/{path...}",
			PageID: "docs",
			Symbol: "Docs",
		}},
	})

	pathItem := spec.Paths["/docs/{path}"]
	if pathItem == nil {
		t.Fatalf("expected OpenAPI path /docs/{path}, got %#v", spec.Paths)
	}
	operation, ok := pathItem["get"]
	if !ok {
		t.Fatalf("expected GET operation, got %#v", pathItem)
	}
	if operation.XGOWDK.Route != "/docs/{path...}" {
		t.Fatalf("x-gowdk route = %q, want %q", operation.XGOWDK.Route, "/docs/{path...}")
	}
}

func TestBuildOpenAPISpecIncludesTypedEndpointRouteMetadata(t *testing.T) {
	spec := buildOpenAPISpec(gwdkir.Program{
		Endpoints: []gwdkir.Endpoint{{
			Kind:        gwdkir.EndpointFragment,
			Source:      gwdkir.EndpointSourceGOWDK,
			Method:      "GET",
			Path:        "/patients/{id:int}/vitals",
			PageID:      "patients",
			Symbol:      "Vitals",
			Cache:       "no-store",
			Guards:      []string{"auth.required"},
			CSRF:        true,
			RouteParams: []source.RouteParam{{Name: "id", Type: "int"}},
		}},
	})

	operation := spec.Paths["/patients/{id}/vitals"]["get"]
	if len(operation.Parameters) != 1 {
		t.Fatalf("expected path parameter, got %#v", operation.Parameters)
	}
	param := operation.Parameters[0]
	if param.Name != "id" || param.In != "path" || !param.Required || param.Schema.Type != "integer" || param.Schema.Format != "int64" {
		t.Fatalf("unexpected path parameter: %#v", param)
	}
	if operation.XGOWDK.EndpointSource != "gwdk" ||
		operation.XGOWDK.Cache != "no-store" ||
		!operation.XGOWDK.CSRF ||
		len(operation.XGOWDK.Guards) != 1 ||
		operation.XGOWDK.Guards[0] != "auth.required" {
		t.Fatalf("unexpected x-gowdk metadata: %#v", operation.XGOWDK)
	}
}
