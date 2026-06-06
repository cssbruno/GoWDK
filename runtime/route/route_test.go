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

func TestMatchRejectsDifferentShape(t *testing.T) {
	if _, ok := Match("/blog/{slug}", "/blog/hello/edit"); ok {
		t.Fatal("expected different shape not to match")
	}
	if _, ok := Match("/blog/{slug}", "/docs/hello"); ok {
		t.Fatal("expected different literal segment not to match")
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
