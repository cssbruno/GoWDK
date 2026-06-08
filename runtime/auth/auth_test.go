package auth

import "testing"

func TestPrincipalRoleAndPermissionChecks(t *testing.T) {
	principal := Principal{
		Roles:       []string{"admin"},
		Permissions: []string{"patients.read"},
	}
	if !principal.HasRole("admin") {
		t.Fatal("expected admin role")
	}
	if !principal.HasPermission("patients.read") {
		t.Fatal("expected patients.read permission")
	}
	if principal.HasRole("") || principal.HasPermission("") {
		t.Fatal("empty role or permission must not match")
	}
	if principal.HasRole("viewer") || principal.HasPermission("patients.write") {
		t.Fatal("unexpected role or permission match")
	}
}

func TestIsNativeGuard(t *testing.T) {
	for _, name := range []string{"role:admin", "permission:patients.read"} {
		if !IsNativeGuard(name) {
			t.Fatalf("expected native guard %q", name)
		}
	}
	if IsNativeGuard("auth.required") {
		t.Fatal("custom guard must not be native")
	}
}

func TestIsPublicGuard(t *testing.T) {
	if !IsPublicGuard("public") {
		t.Fatal("expected public guard")
	}
	for _, name := range []string{"role:public", "permission:public", "auth.public"} {
		if IsPublicGuard(name) {
			t.Fatalf("did not expect public guard %q", name)
		}
	}
}
