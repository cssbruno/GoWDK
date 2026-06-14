package buildgen

import (
	"testing"

	"github.com/cssbruno/gowdk/internal/gwdkir"
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
