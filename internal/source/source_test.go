package source

import "testing"

func TestValidateBackendRoutePath(t *testing.T) {
	valid := []string{
		"/",
		"/patients",
		"/patients/list",
	}
	for _, path := range valid {
		if err := ValidateBackendRoutePath(path); err != nil {
			t.Fatalf("expected %q to be valid, got %v", path, err)
		}
	}

	invalid := []string{
		"",
		"patients",
		"//example.com/pay",
		"https://example.com/pay",
		"/https://example.com/pay",
		"/patients?filter=active",
		"/patients#form",
		"/patients/{id}",
		"/patients\nadmin",
		"/patients\\admin",
		"/patients/../admin",
		"/patients//active",
		"/patients/./active",
		"/patients/",
		" /patients",
	}
	for _, path := range invalid {
		if err := ValidateBackendRoutePath(path); err == nil {
			t.Fatalf("expected %q to be invalid", path)
		}
	}
}

func TestBackendRouteMethod(t *testing.T) {
	if got := BackendRouteMethod(" post "); got != "POST" {
		t.Fatalf("expected normalized method POST, got %q", got)
	}
}
