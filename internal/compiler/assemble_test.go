package compiler

import (
	"path/filepath"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

// TestAssembleProgramEnrichesBindings proves the canonical pipeline runs the
// discover -> bind phases, so a source-taking caller cannot end up with an
// under-enriched program (the bug behind #386): assembling from sources and
// reading bindings out of the bare program leaves the inline go-block action
// unbound, whereas AssembleProgram binds it.
func TestAssembleProgramEnrichesBindings(t *testing.T) {
	root := t.TempDir()
	sources := gwdkanalysis.Sources{
		Pages: []gwdkir.Page{{
			ID:      "home",
			Package: "pages",
			Source:  filepath.Join(root, "home.page.gwdk"),
			Route:   "/",
			Blocks: gwdkir.Blocks{
				Actions: []gwdkir.Action{{Name: "Subscribe", Method: "POST", Route: "/newsletter"}},
				GoBlocks: []gwdkir.GoBlock{{Body: `import (
	"context"

	"github.com/cssbruno/gowdk/runtime/response"
)

func Subscribe(context.Context) (response.Response, error) {
	return response.RedirectTo("/?subscribed=1"), nil
}`}},
			},
		}},
	}
	config := gowdk.Config{}

	// The old under-enriched path: assemble, then read bindings out of the bare
	// program. Discovery and binding never ran, so Subscribe is not bound.
	base := gwdkanalysis.BuildProgram(config, sources)
	if binding := compilerBindingsByBlock(BackendBindingsFromIR(base))["Subscribe"]; binding.Status == source.BackendBindingBound {
		t.Fatalf("expected bare BuildProgram to leave Subscribe unbound, got %#v", binding)
	}

	// The canonical pipeline runs discover -> bind, so Subscribe is bound.
	_, bindings, err := AssembleProgram(config, sources)
	if err != nil {
		t.Fatalf("AssembleProgram: %v", err)
	}
	binding, ok := compilerBindingsByBlock(bindings)["Subscribe"]
	if !ok {
		t.Fatalf("expected AssembleProgram to return a Subscribe binding, got %#v", bindings)
	}
	if binding.Status != source.BackendBindingBound {
		t.Fatalf("expected AssembleProgram to bind Subscribe, got status %q (%#v)", binding.Status, binding)
	}
}
