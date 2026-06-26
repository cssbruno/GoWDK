package buildgen

import (
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

func TestBuildOpenAPISpecKeepsOriginalGOWDKRoute(t *testing.T) {
	spec := buildOpenAPISpec(gowdk.Config{}, gwdkir.Program{
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
	spec := buildOpenAPISpec(gowdk.Config{}, gwdkir.Program{
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

func TestBuildOpenAPISpecIncludesTypedAPIInputAndResultSchemas(t *testing.T) {
	spec := buildOpenAPISpec(gowdk.Config{}, gwdkir.Program{
		Endpoints: []gwdkir.Endpoint{{
			Kind:   gwdkir.EndpointAPI,
			Method: "POST",
			Path:   "/api/search",
			PageID: "search",
			Symbol: "Search",
			Binding: gwdkir.Binding{
				Status:    source.BackendBindingBound,
				Signature: source.BackendSignatureAPIInput,
				InputType: "SearchInput",
				InputFields: []source.BackendInputField{
					{FieldName: "Query", FormName: "q", Type: "string"},
					{FieldName: "Page", FormName: "page", Type: "int"},
				},
				ResultType: "SearchResult",
				ResultFields: []source.BackendResultField{
					{Path: "count", Selector: "Count", Type: "int"},
					{Path: "next", Selector: "Next", Type: "string"},
					{Path: "user", Selector: "User", Type: "SearchUser"},
					{Path: "user.name", Selector: "User.Name", Type: "string"},
				},
			},
		}},
	})

	operation := spec.Paths["/api/search"]["post"]
	if operation.RequestBody == nil {
		t.Fatalf("expected request body, got %#v", operation)
	}
	if _, ok := operation.RequestBody.Content["application/json"]; !ok {
		t.Fatalf("expected JSON request body, got %#v", operation.RequestBody.Content)
	}
	response := operation.Responses["200"]
	media, ok := response.Content["application/json"]
	if !ok {
		t.Fatalf("expected JSON response, got %#v", response.Content)
	}
	if media.Schema.Ref != "#/components/schemas/SearchResult" {
		t.Fatalf("unexpected response schema: %#v", media.Schema)
	}
	input := spec.Components.Schemas["SearchInput"]
	if input.Properties["q"].Type != "string" || input.Properties["page"].Type != "integer" {
		t.Fatalf("unexpected input schema: %#v", input)
	}
	result := spec.Components.Schemas["SearchResult"]
	if result.XGoType != "SearchResult" || result.Properties["count"].Type != "integer" || result.Properties["next"].Type != "string" {
		t.Fatalf("unexpected result schema: %#v", result)
	}
	user := result.Properties["user"]
	if user.Type != "object" || user.XGoType != "SearchUser" || user.Properties["name"].Type != "string" {
		t.Fatalf("unexpected nested user result schema: %#v", user)
	}
	if _, ok := result.Properties["user.name"]; ok {
		t.Fatalf("nested result path was emitted as a dotted top-level property: %#v", result.Properties)
	}
	if operation.XGOWDK.Signature != string(source.BackendSignatureAPIInput) || operation.XGOWDK.InputType != "SearchInput" {
		t.Fatalf("unexpected x-gowdk binding metadata: %#v", operation.XGOWDK)
	}
}

func TestBuildOpenAPISpecMapsSupportedEndpointRouteParamTypes(t *testing.T) {
	tests := []struct {
		paramType  string
		schemaType string
		format     string
	}{
		{paramType: "string", schemaType: "string"},
		{paramType: "int", schemaType: "integer", format: "int64"},
		{paramType: "int64", schemaType: "integer", format: "int64"},
		{paramType: "uint", schemaType: "integer", format: "uint64"},
		{paramType: "uint64", schemaType: "integer", format: "uint64"},
		{paramType: "bool", schemaType: "boolean"},
		{paramType: "float64", schemaType: "number", format: "double"},
	}
	for _, test := range tests {
		t.Run(test.paramType, func(t *testing.T) {
			spec := buildOpenAPISpec(gowdk.Config{}, gwdkir.Program{
				Endpoints: []gwdkir.Endpoint{{
					Kind:        gwdkir.EndpointAPI,
					Source:      gwdkir.EndpointSourceGOWDK,
					Method:      "GET",
					Path:        "/values/{value:" + test.paramType + "}",
					PageID:      "values",
					Symbol:      "Show",
					RouteParams: []source.RouteParam{{Name: "value", Type: test.paramType}},
				}},
			})

			operation := spec.Paths["/values/{value}"]["get"]
			if len(operation.Parameters) != 1 {
				t.Fatalf("expected one path parameter, got %#v", operation.Parameters)
			}
			schema := operation.Parameters[0].Schema
			if schema.Type != test.schemaType || schema.Format != test.format {
				t.Fatalf("route param %s schema = %#v, want type=%q format=%q", test.paramType, schema, test.schemaType, test.format)
			}
		})
	}
}

func TestBuildOpenAPISpecExpandsContractResultFields(t *testing.T) {
	spec := buildOpenAPISpec(gowdk.Config{}, gwdkir.Program{
		ContractRefs: []gwdkir.ContractReference{{
			Kind:   gwdkir.ContractQuery,
			Name:   "patients.GetPatient",
			Method: "GET",
			Path:   "/patients",
			Status: gwdkir.ContractBindingBound,
			Result: "patients.PatientPage",
			ResultFields: []source.BackendInputField{
				{FieldName: "ID", FormName: "id", Type: "string"},
				{FieldName: "Age", FormName: "age", Type: "int"},
				{FieldName: "Tags", FormName: "tags", Type: "[]string"},
			},
		}},
	})

	schema := spec.Components.Schemas["PatientPage"]
	if schema.Type != "object" || schema.XGoType != "patients.PatientPage" {
		t.Fatalf("unexpected result schema: %#v", schema)
	}
	if schema.Properties["id"].Type != "string" ||
		schema.Properties["age"].Type != "integer" ||
		schema.Properties["tags"].Type != "array" ||
		schema.Properties["tags"].Items == nil ||
		schema.Properties["tags"].Items.Type != "string" {
		t.Fatalf("unexpected result properties: %#v", schema.Properties)
	}
}

func TestBuildOpenAPISpecMarksCommandContractsCSRFProtectedByDefault(t *testing.T) {
	spec := buildOpenAPISpec(gowdk.Config{}, gwdkir.Program{
		ContractRefs: []gwdkir.ContractReference{{
			Kind:    gwdkir.ContractCommand,
			Name:    "patients.Create",
			Method:  "POST",
			Path:    "/_gowdk/commands/patients.Create",
			Status:  gwdkir.ContractBindingBound,
			OwnerID: "patients",
		}},
	})

	operation := spec.Paths["/_gowdk/commands/patients.Create"]["post"]
	if !operation.XGOWDK.CSRF {
		t.Fatalf("expected command contract OpenAPI metadata to preserve CSRF: %#v", operation.XGOWDK)
	}

	disabled := buildOpenAPISpec(gowdk.Config{Build: gowdk.BuildConfig{CSRF: gowdk.CSRFConfig{Disabled: true}}}, gwdkir.Program{
		ContractRefs: []gwdkir.ContractReference{{
			Kind:    gwdkir.ContractCommand,
			Name:    "patients.Create",
			Method:  "POST",
			Path:    "/_gowdk/commands/patients.Create",
			Status:  gwdkir.ContractBindingBound,
			OwnerID: "patients",
		}},
	})
	operation = disabled.Paths["/_gowdk/commands/patients.Create"]["post"]
	if operation.XGOWDK.CSRF {
		t.Fatalf("expected disabled CSRF config to be reflected in OpenAPI metadata: %#v", operation.XGOWDK)
	}
}
