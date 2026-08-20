package gowdk

import (
	"net/http"
	"strings"
	"testing"

	gowdkauth "github.com/cssbruno/gowdk/runtime/auth"
	gowdkguard "github.com/cssbruno/gowdk/runtime/guard"
	"github.com/cssbruno/gowdk/runtime/ssr"
)

func InteropTestLoad(ssr.LoadContext) map[string]any { return nil }
func InteropTestGuards() gowdkguard.Registry         { return nil }
func InteropTestAuth() gowdkauth.Provider {
	return gowdkauth.ProviderFunc(func(*http.Request) (*gowdkauth.Principal, error) { return nil, nil })
}

func TestTypedInteropRegistrationsCaptureNavigableGoSymbols(t *testing.T) {
	config := Config{Interop: InteropConfig{
		Loads:        []LoadRegistration{RegisterLoad("dashboard", InteropTestLoad)},
		Guards:       RegisterGuards(InteropTestGuards),
		AuthProvider: RegisterAuthProvider(InteropTestAuth),
	}}
	if err := config.ValidateStructural(); err != nil {
		t.Fatal(err)
	}
	load, ok := config.Interop.LoadForPage("dashboard")
	if !ok || load.Hook.Function != "InteropTestLoad" || load.Hook.ImportPath != "github.com/cssbruno/gowdk" || load.Hook.SourceFile == "" {
		t.Fatalf("unexpected load registration: %#v", load)
	}
}

func TestInteropRegistrationsRejectDuplicatesAndNonFunctions(t *testing.T) {
	config := Config{Interop: InteropConfig{Loads: []LoadRegistration{
		RegisterLoad("dashboard", InteropTestLoad),
		RegisterLoad("dashboard", InteropTestLoad),
	}}}
	if err := config.ValidateStructural(); err == nil || !strings.Contains(err.Error(), "registered more than once") {
		t.Fatalf("expected duplicate page registration error, got %v", err)
	}
	config = Config{Interop: InteropConfig{Loads: []LoadRegistration{RegisterLoad("dashboard", "not a function")}}}
	if err := config.ValidateStructural(); err == nil || !strings.Contains(err.Error(), "non-nil package-level function") {
		t.Fatalf("expected typed function error, got %v", err)
	}
}
