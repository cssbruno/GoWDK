package lang

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRedactMessageMasksSecrets(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		leaked  string
		wantSub string
	}{
		{
			name:    "attribute value dsn",
			in:      `g:bind:value="postgres://app:s3cr3tpw@db:5432" is invalid`,
			leaked:  "s3cr3tpw",
			wantSub: "postgres://app:[REDACTED]@db",
		},
		{
			name:    "store init password",
			in:      `page home store conn init is invalid: password=hunter2`,
			leaked:  "hunter2",
			wantSub: "password=[REDACTED]",
		},
		{
			name:    "api key in expression",
			in:      `g:for collection "api_key=live_sk_99aabb" is invalid`,
			leaked:  "live_sk_99aabb",
			wantSub: "api_key=[REDACTED]",
		},
		{
			name:    "csrf token in generated form field",
			in:      `invalid submitted field "_gowdk_csrf=csrf-secret-token"`,
			leaked:  "csrf-secret-token",
			wantSub: "_gowdk_csrf=[REDACTED]",
		},
		{
			name:    "cookie header",
			in:      `request Cookie: gowdk_session=signed-secret; theme=dark`,
			leaked:  "signed-secret",
			wantSub: "Cookie: [REDACTED]",
		},
		{
			name:    "authorization key value",
			in:      `request authorization=Bearer abc123secret`,
			leaked:  "abc123secret",
			wantSub: "Bearer [REDACTED]",
		},
		{
			name:    "sensitive query param",
			in:      `route query refresh_token=refresh-secret-token is invalid`,
			leaked:  "refresh-secret-token",
			wantSub: "refresh_token=[REDACTED]",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RedactMessage(tc.in)
			if strings.Contains(got, tc.leaked) {
				t.Fatalf("secret leaked through redaction: %q", got)
			}
			if !strings.Contains(got, tc.wantSub) {
				t.Fatalf("expected %q in redacted output, got %q", tc.wantSub, got)
			}
		})
	}
}

func TestRedactMessageLeavesOrdinaryDiagnosticsAlone(t *testing.T) {
	in := `route "/orders" must start with /`
	if got := RedactMessage(in); got != in {
		t.Fatalf("redaction altered ordinary diagnostic: %q", got)
	}
}

func TestDiagnosticStringRedactsSecret(t *testing.T) {
	d := Diagnostic{
		File:     "home.page.gwdk",
		Pos:      Position{Line: 3, Column: 5},
		Severity: "error",
		Message:  `store conn init is invalid: password=hunter2`,
	}
	got := d.String()
	if strings.Contains(got, "hunter2") {
		t.Fatalf("secret leaked into String(): %q", got)
	}
	if !strings.Contains(got, "password=[REDACTED]") {
		t.Fatalf("expected redacted message in String(): %q", got)
	}
	if !strings.Contains(got, "home.page.gwdk:3:5") {
		t.Fatalf("expected location preserved: %q", got)
	}
}

func TestDiagnosticMarshalJSONRedactsSecret(t *testing.T) {
	d := Diagnostic{
		Severity:   "error",
		Message:    `init is invalid: token=abc123secret`,
		Suggestion: `try password=plaintextpw`,
	}
	payload, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	body := string(payload)
	if strings.Contains(body, "abc123secret") || strings.Contains(body, "plaintextpw") {
		t.Fatalf("secret leaked into JSON: %s", body)
	}
	if !strings.Contains(body, "token=[REDACTED]") || !strings.Contains(body, "password=[REDACTED]") {
		t.Fatalf("expected redacted message and suggestion in JSON: %s", body)
	}
}
