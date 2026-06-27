package gwdkanalysis

import (
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkir"
)

func TestBuildProgramMarksStateChangingAPIsCSRFProtected(t *testing.T) {
	program := BuildProgram(gowdk.Config{}, Sources{Pages: []gwdkir.Page{{
		ID:      "status",
		Package: "pages",
		Source:  "status.page.gwdk",
		Route:   "/status",
		Blocks: gwdkir.Blocks{APIs: []gwdkir.API{
			{Name: "Read", Method: "GET", Route: "/api/status"},
			{Name: "Update", Method: "POST", Route: "/api/status"},
		}},
	}}})

	endpoints := map[string]gwdkir.Endpoint{}
	for _, endpoint := range program.Endpoints {
		endpoints[endpoint.Symbol] = endpoint
	}
	if endpoints["Read"].CSRF {
		t.Fatalf("expected safe API endpoint not to default to CSRF protected: %#v", endpoints["Read"])
	}
	if !endpoints["Update"].CSRF {
		t.Fatalf("expected state-changing API endpoint to default to CSRF protected: %#v", endpoints["Update"])
	}

	disabled := BuildProgram(gowdk.Config{Build: gowdk.BuildConfig{CSRF: gowdk.CSRFConfig{Disabled: true}}}, Sources{Pages: []gwdkir.Page{{
		ID:      "status",
		Package: "pages",
		Source:  "status.page.gwdk",
		Route:   "/status",
		Blocks:  gwdkir.Blocks{APIs: []gwdkir.API{{Name: "Update", Method: "POST", Route: "/api/status"}}},
	}}})
	for _, endpoint := range disabled.Endpoints {
		if endpoint.Symbol == "Update" && endpoint.CSRF {
			t.Fatalf("expected disabled CSRF config to opt out state-changing API endpoint: %#v", endpoint)
		}
	}
}

func TestBuildProgramAssignsStableSemanticIdentities(t *testing.T) {
	program := BuildProgram(gowdk.Config{}, Sources{Pages: []gwdkir.Page{{
		ID:      "newsletter",
		Package: "pages",
		Source:  "newsletter.page.gwdk",
		Route:   "/newsletter",
		Blocks: gwdkir.Blocks{Actions: []gwdkir.Action{
			{Name: "Subscribe", Method: "POST", Route: "/newsletter"},
		}},
	}}})

	if len(program.Routes) != 1 {
		t.Fatalf("expected one route, got %#v", program.Routes)
	}
	if got, want := program.Routes[0].ID, program.Routes[0].ExpectedID(); got != want {
		t.Fatalf("route ID = %q, want %q", got, want)
	}
	if len(program.Endpoints) != 1 {
		t.Fatalf("expected one endpoint, got %#v", program.Endpoints)
	}
	if got, want := program.Endpoints[0].ID, program.Endpoints[0].ExpectedID(); got != want {
		t.Fatalf("endpoint ID = %q, want %q", got, want)
	}
}
