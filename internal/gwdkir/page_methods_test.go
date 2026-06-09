package gwdkir

import (
	"reflect"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/source"
)

func TestPageRenderModeResolvesRequestTime(t *testing.T) {
	cases := []struct {
		name string
		page Page
		def  gowdk.RenderMode
		want gowdk.RenderMode
	}{
		{"explicit", Page{Render: gowdk.SSR}, gowdk.SPA, gowdk.SSR},
		{"load_block", Page{Blocks: Blocks{Load: true}}, gowdk.SPA, gowdk.SSR},
		{"go_ssr_block", Page{Blocks: Blocks{GoBlocks: []GoBlock{{Target: "ssr"}}}}, gowdk.SPA, gowdk.SSR},
		{"default_spa", Page{}, "", gowdk.SPA},
		{"default_passthrough", Page{}, gowdk.Action, gowdk.Action},
	}
	for _, tc := range cases {
		if got := tc.page.RenderMode(tc.def); got != tc.want {
			t.Fatalf("%s: RenderMode = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestPageDynamicParamsFromExplicitAndPath(t *testing.T) {
	explicit := Page{RouteParams: []source.RouteParam{{Name: "slug"}, {Name: "id"}, {Name: "slug"}}}
	if got := explicit.DynamicParams(); !reflect.DeepEqual(got, []string{"id", "slug"}) {
		t.Fatalf("explicit DynamicParams = %v, want [id slug]", got)
	}

	fromPath := Page{Route: "/blog/{slug}/{id:int}"}
	if got := fromPath.DynamicParams(); !reflect.DeepEqual(got, []string{"id", "slug"}) {
		t.Fatalf("path DynamicParams = %v, want [id slug]", got)
	}

	if got := (Page{Route: "/"}).DynamicParams(); got != nil {
		t.Fatalf("static route DynamicParams = %v, want nil", got)
	}
}

func TestPageTypedRouteParamsDefaultsToString(t *testing.T) {
	page := Page{Route: "/blog/{slug}/{id:int}"}
	got := page.TypedRouteParams()
	want := []source.RouteParam{{Name: "slug", Type: "string"}, {Name: "id", Type: "int"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("TypedRouteParams = %#v, want %#v", got, want)
	}
}

func TestPageCachePolicy(t *testing.T) {
	if got := (Page{Cache: "public"}).CachePolicy(); got != "public" {
		t.Fatalf("CachePolicy = %q", got)
	}
	if got := (Page{Cache: "public", Revalidate: "60"}).CachePolicy(); got != "public, stale-while-revalidate=60" {
		t.Fatalf("CachePolicy with revalidate = %q", got)
	}
}
