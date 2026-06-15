package app

import (
	"os"
	"strings"
	"testing"
)

// TestMain silences recovered-panic logging by default so the many panic
// boundary tests do not spew stack traces to stderr. Tests that assert on the
// log set BoundaryLogger explicitly and restore it via t.Cleanup.
func TestMain(m *testing.M) {
	BoundaryLogger = nil
	os.Exit(m.Run())
}

func TestRedactSecretsMasksCredentials(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		leaked  string
		wantSub string
	}{
		{
			name:    "dsn password",
			in:      "dial postgres://app:s3cr3tpw@db.internal:5432/app failed",
			leaked:  "s3cr3tpw",
			wantSub: "postgres://app:[REDACTED]@db.internal",
		},
		{
			name:    "password key=value",
			in:      "config error: password=hunter2 invalid",
			leaked:  "hunter2",
			wantSub: "password=[REDACTED]",
		},
		{
			name:    "token colon value",
			in:      "auth failed token: abc123def456",
			leaked:  "abc123def456",
			wantSub: "[REDACTED]",
		},
		{
			name:    "bearer header",
			in:      "request had Authorization: Bearer eyJhbGciOiJIUzI1NiJ9 attached",
			leaked:  "eyJhbGciOiJIUzI1NiJ9",
			wantSub: "Bearer [REDACTED]",
		},
		{
			name:    "api key",
			in:      "rejected api_key=live_sk_9921aabbcc here",
			leaked:  "live_sk_9921aabbcc",
			wantSub: "api_key=[REDACTED]",
		},
		{
			name:    "csrf token",
			in:      "invalid form _gowdk_csrf=csrf-secret-token",
			leaked:  "csrf-secret-token",
			wantSub: "_gowdk_csrf=[REDACTED]",
		},
		{
			name:    "cookie header",
			in:      "request Cookie: gowdk_session=signed-secret; theme=dark",
			leaked:  "signed-secret",
			wantSub: "Cookie: [REDACTED]",
		},
		{
			name:    "set cookie header",
			in:      "response Set-Cookie: gowdk_session=signed-secret; HttpOnly",
			leaked:  "signed-secret",
			wantSub: "Set-Cookie: [REDACTED]",
		},
		{
			name:    "session query value",
			in:      "request query session_id=abc123secret path=/account",
			leaked:  "abc123secret",
			wantSub: "session_id=[REDACTED]",
		},
		{
			name:    "refresh token form value",
			in:      "form refresh_token=refresh-secret-token rejected",
			leaked:  "refresh-secret-token",
			wantSub: "refresh_token=[REDACTED]",
		},
		{
			name:    "bare session token in panic message",
			in:      "panic: rejected session eyJpZCI6ImFsaWNlIiwiZXhwIjoxNzB9.c2lnbmF0dXJlYnl0ZXNoZXJlMDEyMzQ1Njc4OQ at handler",
			leaked:  "eyJpZCI6ImFsaWNlIiwiZXhwIjoxNzB9.c2lnbmF0dXJlYnl0ZXNoZXJlMDEyMzQ1Njc4OQ",
			wantSub: "[REDACTED]",
		},
		{
			name:    "bare jwt in stack frame",
			in:      "token eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dBjftJeZ4CVPmB92K27uhbUJU1p1r wong",
			leaked:  "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dBjftJeZ4CVPmB92K27uhbUJU1p1r",
			wantSub: "[REDACTED]",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := redactSecrets(tc.in)
			if strings.Contains(got, tc.leaked) {
				t.Fatalf("secret leaked through redaction: %q", got)
			}
			if !strings.Contains(got, tc.wantSub) {
				t.Fatalf("expected %q in redacted output, got %q", tc.wantSub, got)
			}
		})
	}
}

func TestRedactSecretsLeavesOrdinaryTextAlone(t *testing.T) {
	in := "handler failed: row not found for user 42 at /accounts"
	if got := redactSecrets(in); got != in {
		t.Fatalf("redaction altered ordinary text: %q", got)
	}
}

func TestRedactSecretsEmpty(t *testing.T) {
	if got := redactSecrets(""); got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}
