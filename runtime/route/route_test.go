package route

import "testing"

func TestMatchStaticRoute(t *testing.T) {
	params, ok := Match("/blog/about", "/blog/about")
	if !ok {
		t.Fatal("expected static route to match")
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
		t.Fatal("expected different static segment not to match")
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
