package source

import "testing"

func TestExportedIdentifierPreservesUnderscores(t *testing.T) {
	if got := ExportedIdentifier("Save_User", "Action"); got != "Save_User" {
		t.Fatalf("ExportedIdentifier(Save_User) = %q, want Save_User", got)
	}
	if left, right := ExportedIdentifier("Save_User", "Action"), ExportedIdentifier("SaveUser", "Action"); left == right {
		t.Fatalf("expected underscore to distinguish generated names, got %q and %q", left, right)
	}
}

func TestExportedIdentifierFallbackAndDigitPrefix(t *testing.T) {
	if got := ExportedIdentifier("!!!", "Action"); got != "Action" {
		t.Fatalf("ExportedIdentifier fallback = %q, want Action", got)
	}
	if got := ExportedIdentifier("1-save", "Action"); got != "P1Save" {
		t.Fatalf("ExportedIdentifier digit prefix = %q, want P1Save", got)
	}
}
