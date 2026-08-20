package projectcompile

import (
	"context"
	"errors"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/gwdkir"
)

func TestCompileProducesValidatedSnapshot(t *testing.T) {
	snapshot, diagnostics, err := Compile(gowdk.Config{}, gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID: "home", Route: "/", Guards: []string{"public"},
		Blocks: gwdkir.Blocks{View: true, ViewBody: "<main>Home</main>"},
	}}}, Options{Mode: ProjectMode})
	if err != nil || diagnostics.HasErrors() {
		t.Fatalf("compile: diagnostics=%v err=%v", diagnostics, err)
	}
	if !snapshot.Validated.Valid() {
		t.Fatal("expected validated phase snapshot")
	}
}

func TestCompileReturnsStageCodedValidationDiagnostics(t *testing.T) {
	_, diagnostics, err := Compile(gowdk.Config{}, gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		Source: "admin.page.gwdk", ID: "admin", Route: "/admin", Guards: []string{"public"},
		Blocks: gwdkir.Blocks{Server: true, ServerBody: `=> { title: "Admin" }`, View: true, ViewBody: `<main>{title}</main>`},
	}}}, Options{Mode: ProjectMode})
	if err != nil {
		t.Fatal(err)
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.Stage == "validate" && diagnostic.Code == "missing_ssr_addon" && diagnostic.Source == "admin.page.gwdk" {
			return
		}
	}
	t.Fatalf("missing structured SSR diagnostic: %#v", diagnostics)
}

func TestCompileContextHonorsPreCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := CompileContext(ctx, gowdk.Config{}, gwdkanalysis.Sources{}, Options{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("CompileContext error = %v", err)
	}
}
