package gowdk

import (
	"strings"
	"testing"
)

func TestCSRFConfigSecretEnvNames(t *testing.T) {
	config := CSRFConfig{
		SecretEnv:              " PRIMARY_CSRF_SECRET ",
		VerificationSecretEnvs: []string{" NEXT_CSRF_SECRET ", "PREVIOUS_CSRF_SECRET"},
	}

	if got := config.SecretEnvName(); got != "PRIMARY_CSRF_SECRET" {
		t.Fatalf("primary CSRF secret env = %q", got)
	}
	if got := strings.Join(config.SecretEnvNames(), ","); got != "PRIMARY_CSRF_SECRET,NEXT_CSRF_SECRET,PREVIOUS_CSRF_SECRET" {
		t.Fatalf("CSRF secret env names = %q", got)
	}
}

func TestCSRFConfigValidateRejectsBlankAndDuplicateVerificationSecretEnvs(t *testing.T) {
	tests := []struct {
		name   string
		config CSRFConfig
		want   string
	}{
		{
			name:   "blank",
			config: CSRFConfig{VerificationSecretEnvs: []string{" "}},
			want:   "VerificationSecretEnvs[0]",
		},
		{
			name: "duplicates primary",
			config: CSRFConfig{
				SecretEnv:              "PRIMARY_CSRF_SECRET",
				VerificationSecretEnvs: []string{"PRIMARY_CSRF_SECRET"},
			},
			want: "declared more than once",
		},
		{
			name: "duplicates verification key",
			config: CSRFConfig{
				VerificationSecretEnvs: []string{"OLD_CSRF_SECRET", " OLD_CSRF_SECRET "},
			},
			want: "declared more than once",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.config.Validate()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Validate() error = %v, want %q", err, test.want)
			}
		})
	}
}
