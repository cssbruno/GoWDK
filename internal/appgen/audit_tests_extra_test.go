package appgen

import (
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkir"
)

func TestGeneratedAuditTestSourceExpandsRuntimeScenarioMatrix(t *testing.T) {
	config := gowdk.Config{
		Build: gowdk.BuildConfig{
			SecurityHeaders: gowdk.SecurityHeadersConfig{
				Enabled: true,
				Headers: map[string]string{"X-Frame-Options": "DENY"},
			},
		},
	}
	source, err := GeneratedAuditTestSource(Options{
		Config: config,
		IR: &gwdkir.Program{
			Routes: []gwdkir.Route{
				{Kind: gwdkir.RouteSPA, Method: "GET", Path: "/blog/{slug}", PageID: "blog", Render: gowdk.SPA},
				{Kind: gwdkir.RouteSPA, Method: "GET", Path: "/", PageID: "home", Render: gowdk.SPA, Guards: []string{"public"}},
			},
			Endpoints: []gwdkir.Endpoint{{
				Kind:   gwdkir.EndpointAction,
				Symbol: "Submit",
				Method: "POST",
				Path:   "/submit",
				Guards: []string{"public"},
				CSRF:   true,
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	payload := string(source)
	for _, expected := range []string{
		`Name:       "default-deny /blog/gowdk-audit"`,
		`Name:       "action csrf rejection Submit"`,
		`Path:       "/submit"`,
		`Name:       "security header X-Frame-Options on route /"`,
		`WantHeader: map[string]string{`,
		`"X-Frame-Options": "DENY"`,
		`GOWDK audit guard fixture: public not-required`,
	} {
		if !strings.Contains(payload, expected) {
			t.Fatalf("expected generated audit test source to contain %q:\n%s", expected, payload)
		}
	}
}
