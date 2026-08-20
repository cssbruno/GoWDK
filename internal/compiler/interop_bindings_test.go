package compiler

import (
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
	fixture "github.com/cssbruno/gowdk/testfixture/interop"
)

func TestBindBackendHandlersWithConfigUsesExplicitLoadRegistration(t *testing.T) {
	ir := gwdkir.Program{Pages: []gwdkir.Page{{
		ID: "dashboard", Package: "app", Source: "dashboard.page.gwdk",
		Route: "/dashboard", Render: gowdk.SSR,
		Blocks: gwdkir.Blocks{Server: true},
	}}}
	config := gowdk.Config{Interop: gowdk.InteropConfig{Loads: []gowdk.LoadRegistration{
		gowdk.RegisterLoad("dashboard", fixture.LoadDashboard),
	}}}
	bindings := BindBackendHandlersWithConfig(config, &ir)
	if len(bindings) != 1 || bindings[0].Status != source.BackendBindingBound || bindings[0].FunctionName != "LoadDashboard" || bindings[0].ImportPath != "github.com/cssbruno/gowdk/testfixture/interop" {
		t.Fatalf("unexpected explicit load binding: %#v", bindings)
	}
}

func TestBindBackendHandlersWithConfigRejectsMagicLoadName(t *testing.T) {
	ir := gwdkir.Program{Pages: []gwdkir.Page{{
		ID: "dashboard", Source: "dashboard.page.gwdk", Route: "/dashboard",
		Render: gowdk.SSR, Blocks: gwdkir.Blocks{Server: true},
	}}}
	bindings := BindBackendHandlersWithConfig(gowdk.Config{}, &ir)
	diagnostics := BackendBindingDiagnostics(bindings)
	if len(diagnostics) != 1 || diagnostics[0].Code != "missing_load_registration" || diagnostics[0].Severity != SeverityError {
		t.Fatalf("expected early explicit-registration diagnostic, got %#v", diagnostics)
	}
}

func TestValidateInteropRegistrationsRequiresGuardAndAuthProviders(t *testing.T) {
	ir := gwdkir.Program{Pages: []gwdkir.Page{{
		ID: "dashboard", Source: "dashboard.page.gwdk", Route: "/dashboard", Render: gowdk.SSR,
		Guards: []string{"session", "role:admin"}, Blocks: gwdkir.Blocks{Server: true, View: true, ViewBody: "<main></main>"},
	}}}
	diagnostics := validateInteropRegistrations(gowdk.Config{}, ir)
	if !hasDiagnosticCode(diagnostics, "missing_guard_registration") || !hasDiagnosticCode(diagnostics, "missing_auth_registration") {
		t.Fatalf("expected guard and auth registration diagnostics, got %#v", diagnostics)
	}
	config := gowdk.Config{Interop: gowdk.InteropConfig{
		Guards:       gowdk.RegisterGuards(fixture.Guards),
		AuthProvider: gowdk.RegisterAuthProvider(fixture.AuthProvider),
	}}
	if diagnostics := validateInteropRegistrations(config, ir); len(diagnostics) != 0 {
		t.Fatalf("expected typed providers to satisfy validation, got %#v", diagnostics)
	}
}

func TestValidateInteropRegistrationsCoversRealtimeGuards(t *testing.T) {
	ir := gwdkir.Program{RealtimeSubscriptions: []gwdkir.RealtimeSubscription{{
		OwnerID: "dashboard", Source: "dashboard.page.gwdk",
		Guards: []string{"session", "permission:events.read"},
	}}}
	diagnostics := validateInteropRegistrations(gowdk.Config{}, ir)
	if !hasDiagnosticCode(diagnostics, "missing_guard_registration") || !hasDiagnosticCode(diagnostics, "missing_auth_registration") {
		t.Fatalf("expected realtime guard and auth registration diagnostics, got %#v", diagnostics)
	}
}
