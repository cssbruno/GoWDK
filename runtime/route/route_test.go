package route

import (
	"errors"
	"strings"
	"testing"
)

func TestMatchSPARoute(t *testing.T) {
	params, ok := Match("/blog/about", "/blog/about")
	if !ok {
		t.Fatal("expected SPA route to match")
	}
	if len(params) != 0 {
		t.Fatalf("did not expect params, got %#v", params)
	}
}

func TestMatchDynamicRoute(t *testing.T) {
	params, ok := Match("/blog/{slug}", "/blog/hello")
	if !ok {
		t.Fatal("expected dynamic route to match")
	}
	if params["slug"] != "hello" {
		t.Fatalf("unexpected slug: %#v", params)
	}
}

func TestMatchTypedDynamicRoute(t *testing.T) {
	params, ok := Match("/patients/{id:int}", "/patients/42")
	if !ok {
		t.Fatal("expected typed dynamic route to match")
	}
	if params["id"] != "42" {
		t.Fatalf("unexpected id: %#v", params)
	}
}

func TestMatchRejectsDifferentShape(t *testing.T) {
	if _, ok := Match("/blog/{slug}", "/blog/hello/edit"); ok {
		t.Fatal("expected different shape not to match")
	}
	if _, ok := Match("/blog/{slug}", "/docs/hello"); ok {
		t.Fatal("expected different literal segment not to match")
	}
}

func TestMatchRestRouteMultipleSegments(t *testing.T) {
	params, ok := Match("/docs/{path...}", "/docs/guides/routing/rest")
	if !ok {
		t.Fatal("expected rest route to match multiple segments")
	}
	if params["path"] != "guides/routing/rest" {
		t.Fatalf("unexpected path: %#v", params)
	}
}

func TestMatchRestRouteSingleSegment(t *testing.T) {
	params, ok := Match("/docs/{path...}", "/docs/intro")
	if !ok {
		t.Fatal("expected rest route to match a single segment")
	}
	if params["path"] != "intro" {
		t.Fatalf("unexpected path: %#v", params)
	}
}

func TestMatchRestRouteRejectsZeroSegments(t *testing.T) {
	if _, ok := Match("/docs/{path...}", "/docs"); ok {
		t.Fatal("expected rest route to require at least one segment")
	}
	if _, ok := Match("/docs/{path...}", "/docs/"); ok {
		t.Fatal("expected rest route to require at least one segment after cleaning")
	}
}

func TestMatchRestRouteRejectsOtherPrefix(t *testing.T) {
	if _, ok := Match("/docs/{path...}", "/blog/guides/routing"); ok {
		t.Fatal("expected rest route to require its fixed prefix")
	}
}

func TestMatchRestRouteWithFixedAndDynamicPrefix(t *testing.T) {
	params, ok := Match("/docs/{lang}/{path...}", "/docs/en/guides/routing")
	if !ok {
		t.Fatal("expected mixed dynamic and rest route to match")
	}
	if params["lang"] != "en" || params["path"] != "guides/routing" {
		t.Fatalf("unexpected params: %#v", params)
	}
}

func TestMatchRestRouteRejectsNonCanonicalPaths(t *testing.T) {
	for _, requestPath := range []string{
		"/docs/a/../admin",
		"/docs/../docs/a",
		"/docs//a",
		"/docs/./a",
		"/docs/a/..",
	} {
		if _, ok := Match("/docs/{path...}", requestPath); ok {
			t.Fatalf("expected non-canonical rest path %q not to match", requestPath)
		}
	}
}

func TestMatchRestRouteTypedHelpers(t *testing.T) {
	params, ok := Match("/docs/{path...}", "/docs/guides/routing")
	if !ok {
		t.Fatal("expected rest route to match")
	}
	value, err := Required(params, "path")
	if err != nil {
		t.Fatalf("unexpected Required error: %v", err)
	}
	if value != "guides/routing" {
		t.Fatalf("unexpected Required value: %q", value)
	}
}

func TestMatchRejectsUnsafeParamValues(t *testing.T) {
	if _, ok := Match("/blog/{slug}", "/blog/."); ok {
		t.Fatal("expected dot param not to match")
	}
	if _, ok := Match("/blog/{slug}", "/blog/.."); ok {
		t.Fatal("expected dot-dot param not to match")
	}
}

func TestMatchRejectsNonCanonicalDynamicPaths(t *testing.T) {
	for _, requestPath := range []string{
		"/blog/a/../b",
		"/blog/./b",
		"/blog//b",
	} {
		if _, ok := Match("/blog/{slug}", requestPath); ok {
			t.Fatalf("expected non-canonical dynamic path %q not to match", requestPath)
		}
	}
}

func TestTypedParamHelpersDecodeScalars(t *testing.T) {
	params := map[string]string{
		"slug":  "hello",
		"count": "42",
		"flag":  "true",
		"ratio": "3.5",
	}

	if got, err := Required(params, "slug"); err != nil || got != "hello" {
		t.Fatalf("Required() = %q, %v", got, err)
	}
	if got, ok, err := String(params, "slug"); err != nil || !ok || got != "hello" {
		t.Fatalf("String() = %q, %v, %v", got, ok, err)
	}
	if got, ok, err := Int(params, "count"); err != nil || !ok || got != 42 {
		t.Fatalf("Int() = %d, %v, %v", got, ok, err)
	}
	if got, ok, err := Int64(params, "count"); err != nil || !ok || got != 42 {
		t.Fatalf("Int64() = %d, %v, %v", got, ok, err)
	}
	if got, ok, err := Uint(params, "count"); err != nil || !ok || got != 42 {
		t.Fatalf("Uint() = %d, %v, %v", got, ok, err)
	}
	if got, ok, err := Uint64(params, "count"); err != nil || !ok || got != 42 {
		t.Fatalf("Uint64() = %d, %v, %v", got, ok, err)
	}
	if got, ok, err := Bool(params, "flag"); err != nil || !ok || !got {
		t.Fatalf("Bool() = %v, %v, %v", got, ok, err)
	}
	if got, ok, err := Float64(params, "ratio"); err != nil || !ok || got != 3.5 {
		t.Fatalf("Float64() = %f, %v, %v", got, ok, err)
	}
}

func TestTypedParamHelpersAcceptOptionalMissing(t *testing.T) {
	params := map[string]string{}

	if got, ok, err := String(params, "slug"); err != nil || ok || got != "" {
		t.Fatalf("String() = %q, %v, %v", got, ok, err)
	}
	if got, ok, err := Int(params, "id"); err != nil || ok || got != 0 {
		t.Fatalf("Int() = %d, %v, %v", got, ok, err)
	}
}

func TestTypedParamHelpersReportMissingAndInvalidWithoutLeakingValue(t *testing.T) {
	_, err := Required(map[string]string{}, "id")
	if err == nil {
		t.Fatal("expected missing param error")
	}
	var paramErr ParamError
	if !errors.As(err, &paramErr) {
		t.Fatalf("expected ParamError, got %T", err)
	}
	if !paramErr.Missing || paramErr.Name != "id" || paramErr.Type != "string" {
		t.Fatalf("unexpected missing ParamError: %#v", paramErr)
	}

	params := map[string]string{"id": "secret-value"}
	_, _, err = Int(params, "id")
	if err == nil {
		t.Fatal("expected invalid int error")
	}
	if !errors.As(err, &paramErr) {
		t.Fatalf("expected ParamError, got %T", err)
	}
	if paramErr.Missing || paramErr.Name != "id" || paramErr.Type != "int" {
		t.Fatalf("unexpected invalid ParamError: %#v", paramErr)
	}
	if strings.Contains(err.Error(), "secret-value") {
		t.Fatalf("error leaked raw param value: %v", err)
	}
}
