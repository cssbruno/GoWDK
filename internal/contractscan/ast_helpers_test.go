package contractscan

import (
	goparser "go/parser"
	"testing"
)

// TestRoleNameNormalizesKnownRoles guards that every runtime role constant maps
// to its stable role value, so scanned metadata, route reports, and the audit
// manifest match the role the runtime uses for authorization.
func TestRoleNameNormalizesKnownRoles(t *testing.T) {
	cases := []struct {
		src  string
		want string
	}{
		{"contracts.RoleWeb", "web"},
		{"contracts.RoleWorker", "worker"},
		{"contracts.RoleCron", "cron"},
		{"contracts.RoleAPI", "api"},
		{"contracts.RoleAdmin", "admin"},
		{"contracts.RoleAny", "any"},
		{`"custom"`, "custom"},
	}
	for _, tc := range cases {
		expr, err := goparser.ParseExpr(tc.src)
		if err != nil {
			t.Fatalf("parse %q: %v", tc.src, err)
		}
		if got := roleName(expr); got != tc.want {
			t.Fatalf("roleName(%q) = %q, want %q", tc.src, got, tc.want)
		}
	}
}
