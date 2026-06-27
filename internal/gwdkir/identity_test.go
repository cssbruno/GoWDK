package gwdkir

import "testing"

func TestSemanticIdentitiesEscapeTupleSeparator(t *testing.T) {
	left := RouteIdentity("GET", "/foo:bar", "baz")
	right := RouteIdentity("GET", "/foo", "bar:baz")
	if left == right {
		t.Fatalf("RouteIdentity collapsed distinct tuples: %q", left)
	}
	if got, want := string(left), "GET:%2Ffoo%3Abar:baz"; got != want {
		t.Fatalf("RouteIdentity() = %q, want %q", got, want)
	}

	endpointLeft := EndpointIdentity(EndpointAPI, "page:one", "List", "GET", "/api/items")
	endpointRight := EndpointIdentity(EndpointAPI, "page", "one:List", "GET", "/api/items")
	if endpointLeft == endpointRight {
		t.Fatalf("EndpointIdentity collapsed distinct tuples: %q", endpointLeft)
	}
}
