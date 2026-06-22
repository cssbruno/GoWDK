package main

import (
	"reflect"
	"testing"

	"github.com/cssbruno/gowdk/internal/compiler"
	"github.com/cssbruno/gowdk/internal/source"
)

func TestEndpointSharedFieldsMatchRouteReport(t *testing.T) {
	metadata := compiler.RouteMetadata{Endpoints: []compiler.EndpointBinding{{
		Kind:           compiler.EndpointAction,
		EndpointSource: "gwdk",
		Source:         "pages/newsletter.page.gwdk",
		SourceSpan: source.SourceSpan{
			Start: source.SourcePosition{Line: 3, Column: 1},
			End:   source.SourcePosition{Line: 3, Column: 32},
		},
		Package:       "pages",
		PackagePath:   "github.com/acme/app/pages",
		PackageName:   "pages",
		Symbol:        "Subscribe",
		Method:        "POST",
		Route:         "/newsletter/{slug}",
		Cache:         "no-store",
		DynamicParams: []string{"slug"},
		RouteParams:   []source.RouteParam{{Name: "slug", Type: "string"}},
		Guards:        []string{"public"},
		CSRF:          true,
		PageID:        "newsletter",
		Handler:       "Subscribe",
		BindingStatus: source.BackendBindingBound,
	}}}

	routeReport := routeMetadataJSON(metadata)
	endpointReport := endpointMetadataJSON(metadata)
	if len(routeReport.Routes) != 1 || len(endpointReport.Endpoints) != 1 {
		t.Fatalf("reports = %#v %#v, want one route endpoint and one endpoint", routeReport, endpointReport)
	}
	route := routeReport.Routes[0]
	endpoint := endpointReport.Endpoints[0]
	if route.EndpointSource != endpoint.EndpointSource ||
		route.Directive != endpoint.Directive ||
		route.Source != endpoint.Source ||
		!reflect.DeepEqual(route.SourceSpan, endpoint.SourceSpan) ||
		route.Package != endpoint.Package ||
		route.PackagePath != endpoint.PackagePath ||
		route.PackageName != endpoint.PackageName ||
		route.Symbol != endpoint.Symbol ||
		route.Method != endpoint.Method ||
		route.Route != endpoint.Route ||
		route.Cache != endpoint.Cache ||
		!reflect.DeepEqual(route.DynamicParams, endpoint.DynamicParams) ||
		!reflect.DeepEqual(route.RouteParams, endpoint.RouteParams) ||
		!reflect.DeepEqual(route.Guards, endpoint.Guards) ||
		route.CSRF != endpoint.CSRF ||
		route.PageID != endpoint.PageID ||
		route.Handler != endpoint.Handler ||
		route.BindingStatus != endpoint.BindingStatus {
		t.Fatalf("route endpoint fields = %#v, endpoint fields = %#v", route, endpoint)
	}
}
