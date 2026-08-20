package gowdkcmd

import (
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	fixture "github.com/cssbruno/gowdk/testfixture/interop"
)

func TestGoBindingsReportIncludesTypedInteropProviders(t *testing.T) {
	config := gowdk.Config{Interop: gowdk.InteropConfig{
		Guards:       gowdk.RegisterGuards(fixture.Guards),
		AuthProvider: gowdk.RegisterAuthProvider(fixture.AuthProvider),
	}}
	report := buildGoBindingsReport(config, gwdkir.Program{})
	for _, expected := range []struct {
		kind   string
		symbol string
	}{{"guards", "Guards"}, {"auth_provider", "AuthProvider"}} {
		binding, ok := findGoBinding(report.Bindings, expected.kind, expected.symbol)
		if !ok || binding.Status != "bound" || binding.PackagePath != "github.com/cssbruno/gowdk/testfixture/interop" || binding.Source == "" {
			t.Fatalf("missing inspectable %s provider: %#v", expected.kind, report.Bindings)
		}
	}
}
