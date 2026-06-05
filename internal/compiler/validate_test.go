package compiler

import (
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/addons/ssr"
	"github.com/cssbruno/gowdk/internal/manifest"
)

func TestValidatePageRejectsSSRWithoutAddon(t *testing.T) {
	page := manifest.Page{
		ID:     "dashboard",
		Route:  "/dashboard",
		Render: gowdk.SSR,
		Blocks: manifest.Blocks{
			View: true,
		},
	}

	diagnostics := ValidatePage(gowdk.Config{}, page)
	if len(diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diagnostics))
	}
	if diagnostics[0].Code != "missing_ssr_addon" {
		t.Fatalf("unexpected diagnostic code: %s", diagnostics[0].Code)
	}
	if !strings.Contains(diagnostics[0].Message, "enable ssr.Addon()") {
		t.Fatalf("diagnostic should suggest enabling ssr addon: %s", diagnostics[0].Message)
	}
}

func TestValidatePageAllowsSSRWithAddon(t *testing.T) {
	page := manifest.Page{
		ID:     "dashboard",
		Route:  "/dashboard",
		Render: gowdk.SSR,
		Blocks: manifest.Blocks{
			Load: true,
			View: true,
		},
	}

	diagnostics := ValidatePage(gowdk.Config{Addons: []gowdk.Addon{ssr.Addon()}}, page)
	if len(diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %#v", diagnostics)
	}
}

func TestValidateManifestRejectsDuplicatePageIDsAndComponentNames(t *testing.T) {
	app := manifest.Manifest{
		Pages: []manifest.Page{
			{ID: "home", Route: "/", Source: "pages/home.page.gwdk", Blocks: manifest.Blocks{View: true}},
			{ID: "home", Route: "/again", Source: "pages/home-again.page.gwdk", Blocks: manifest.Blocks{View: true}},
		},
		Components: []manifest.Component{
			{Name: "Hero", Source: "components/hero.cmp.gwdk"},
			{Name: "Hero", Source: "components/hero-copy.cmp.gwdk"},
		},
	}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected duplicate identity diagnostics")
	}
	diagnostics, ok := err.(ValidationErrors)
	if !ok {
		t.Fatalf("expected ValidationErrors, got %T", err)
	}

	codes := map[string]bool{}
	for _, diagnostic := range diagnostics {
		codes[diagnostic.Code] = true
		if diagnostic.Source == "" {
			t.Fatalf("expected source on duplicate diagnostic: %#v", diagnostic)
		}
	}
	if !codes["duplicate_page_id"] {
		t.Fatalf("missing duplicate_page_id diagnostic: %#v", diagnostics)
	}
	if !codes["duplicate_component_name"] {
		t.Fatalf("missing duplicate_component_name diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestAllowsPageStoreDeclaration(t *testing.T) {
	app := manifest.Manifest{Pages: []manifest.Page{{
		ID:     "cart",
		Route:  "/cart",
		Source: "pages/cart.page.gwdk",
		Imports: []manifest.Import{{
			Alias: "ui",
			Path:  "github.com/cssbruno/gowdk/testfixture/islands",
		}},
		Stores: []manifest.Store{{
			Name: "cart",
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		}},
		Blocks: manifest.Blocks{View: true, ViewBody: `<main>Cart</main>`},
	}}}

	if err := ValidateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected valid store declaration, got %v", err)
	}
}

func TestValidateManifestRejectsDuplicatePageStore(t *testing.T) {
	app := manifest.Manifest{Pages: []manifest.Page{{
		ID:     "cart",
		Route:  "/cart",
		Source: "pages/cart.page.gwdk",
		Stores: []manifest.Store{
			{
				Name: "cart",
				Span: manifest.SourceSpan{Start: manifest.SourcePosition{Line: 5, Column: 1}, End: manifest.SourcePosition{Line: 5, Column: 40}},
			},
			{
				Name: "cart",
				Span: manifest.SourceSpan{Start: manifest.SourcePosition{Line: 6, Column: 1}, End: manifest.SourcePosition{Line: 6, Column: 40}},
			},
		},
		Blocks: manifest.Blocks{View: true, ViewBody: `<main>Cart</main>`},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected duplicate store diagnostic")
	}
	diagnostic := firstDiagnostic(err.(ValidationErrors), "duplicate_page_store")
	if diagnostic == nil {
		t.Fatalf("missing duplicate_page_store diagnostic: %v", err)
	}
	assertSourceSpan(t, diagnostic.Span, 6, 1, 6, 40)
}

func TestValidateManifestRejectsUnknownComponentStoreUse(t *testing.T) {
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:     "cart",
			Route:  "/cart",
			Blocks: manifest.Blocks{View: true, ViewBody: `<main><CartButton /></main>`},
		}},
		Components: []manifest.Component{{
			Name:   "CartButton",
			Source: "components/cart-button.cmp.gwdk",
			Blocks: manifest.Blocks{
				Client:     true,
				ClientBody: "use cart",
				Spans: manifest.BlockSpans{
					Client: manifest.SourceSpan{Start: manifest.SourcePosition{Line: 4, Column: 1}, End: manifest.SourcePosition{Line: 4, Column: 9}},
				},
				View:     true,
				ViewBody: `<button>Cart</button>`,
			},
		}},
	}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected unknown store diagnostic")
	}
	diagnostic := firstDiagnostic(err.(ValidationErrors), "unknown_component_store")
	if diagnostic == nil {
		t.Fatalf("missing unknown_component_store diagnostic: %v", err)
	}
	assertSourceSpan(t, diagnostic.Span, 5, 1, 5, 2)
}

func TestValidateManifestRejectsRedundantComponentImplementations(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{
		{
			Name:   "Hero",
			Source: "components/hero.cmp.gwdk",
			Props:  []manifest.Prop{{Name: "title", Type: "string"}},
			Blocks: manifest.Blocks{View: true, ViewBody: `<section><h1>{title}</h1></section>`},
		},
		{
			Name:   "Feature",
			Source: "components/feature.cmp.gwdk",
			Props:  []manifest.Prop{{Name: "title", Type: "string"}},
			Blocks: manifest.Blocks{View: true, ViewBody: `<section>
  // ignored by fingerprint
  <h1>{title}</h1>
</section>`},
		},
	}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected redundant component diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "redundant_component_implementation") {
		t.Fatalf("missing redundant component diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsRedundantComponentImplementationsWithNormalizedAttrs(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{
		{
			Name:   "PrimaryButton",
			Source: "components/primary-button.cmp.gwdk",
			Props:  []manifest.Prop{{Name: "label", Type: "string"}},
			Blocks: manifest.Blocks{View: true, ViewBody: `<button id="save" class="primary large">{label}</button>`},
		},
		{
			Name:   "SaveButton",
			Source: "components/save-button.cmp.gwdk",
			Props:  []manifest.Prop{{Name: "label", Type: "string"}},
			Blocks: manifest.Blocks{View: true, ViewBody: `<button class="large primary" id="save">{label}</button>`},
		},
	}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected redundant component diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "redundant_component_implementation") {
		t.Fatalf("missing redundant component diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsRedundantTypedComponentsWithCanonicalImportsAndEvents(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{
		{
			Name:    "Counter",
			Source:  "components/counter.cmp.gwdk",
			Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
			PropsType: manifest.GoTypeRef{
				Alias: "ui",
				Name:  "CounterProps",
			},
			State: manifest.StateContract{
				Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
				Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
			},
			Blocks: manifest.Blocks{View: true, ViewBody: `<button g:on:click={Count=Count+1}>{Label}:{Count}</button>`},
		},
		{
			Name:    "Stepper",
			Source:  "components/stepper.cmp.gwdk",
			Imports: []manifest.Import{{Alias: "widgets", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
			PropsType: manifest.GoTypeRef{
				Alias: "widgets",
				Name:  "CounterProps",
			},
			State: manifest.StateContract{
				Type: manifest.GoTypeRef{Alias: "widgets", Name: "CounterState"},
				Init: manifest.GoFuncRef{Alias: "widgets", Name: "NewCounterState"},
			},
			Blocks: manifest.Blocks{View: true, ViewBody: `<button g:on:click={Count = Count + 1}>{Label}:{Count}</button>`},
		},
	}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected redundant component diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "redundant_component_implementation") {
		t.Fatalf("missing redundant component diagnostic: %#v", diagnostics)
	}
	if !hasDiagnosticMessage(diagnostics, "redundant_component_implementation", "components/counter.cmp.gwdk", "components/stepper.cmp.gwdk") {
		t.Fatalf("redundant diagnostic should point to both component sources: %#v", diagnostics)
	}
}

func TestValidateManifestAllowsSameViewWithDifferentContracts(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{
		{
			Name:   "Hero",
			Source: "components/hero.cmp.gwdk",
			Props:  []manifest.Prop{{Name: "title", Type: "string"}},
			Blocks: manifest.Blocks{View: true, ViewBody: `<section>Same</section>`},
		},
		{
			Name:   "Feature",
			Source: "components/feature.cmp.gwdk",
			Props:  []manifest.Prop{{Name: "subtitle", Type: "string"}},
			Blocks: manifest.Blocks{View: true, ViewBody: `<section>Same</section>`},
		},
	}}

	if err := ValidateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected different contracts to be allowed, got %v", err)
	}
}

func TestValidateManifestAllowsSameViewWithDifferentTypedContracts(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{
		{
			Name:    "CounterShell",
			Source:  "components/counter-shell.cmp.gwdk",
			Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
			State: manifest.StateContract{
				Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
				Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
			},
			Blocks: manifest.Blocks{View: true, ViewBody: `<section>Same</section>`},
		},
		{
			Name:    "OtherShell",
			Source:  "components/other-shell.cmp.gwdk",
			Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
			State: manifest.StateContract{
				Type: manifest.GoTypeRef{Alias: "ui", Name: "OtherState"},
				Init: manifest.GoFuncRef{Alias: "ui", Name: "NewOtherState"},
			},
			Blocks: manifest.Blocks{View: true, ViewBody: `<section>Same</section>`},
		},
	}}

	if err := ValidateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected different typed contracts to be allowed, got %v", err)
	}
}

func TestValidateManifestResolvesGoTypedComponentContracts(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		PropsType: manifest.GoTypeRef{
			Alias: "ui",
			Name:  "CounterProps",
		},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<button g:on:click={Count++}>{Label}: {Count}</button>`,
		},
	}}}

	if err := ValidateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected typed component contracts to validate, got %v", err)
	}
}

func TestValidateManifestAllowsEventModifiers(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<button g:on:click.prevent.stop.once.capture.debounce(1s)={Count++}>{Count}</button><button g:on:input.throttle(250ms)={Count++}>{Count}</button>`,
		},
	}}}

	if err := ValidateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected event modifiers to validate, got %v", err)
	}
}

func TestValidateManifestRejectsBadEventModifier(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<button g:on:click.passive={Count++}>{Count}</button>`,
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected unsupported event modifier diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_field_error") {
		t.Fatalf("missing component_field_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsBadDebounceDuration(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<button g:on:click.debounce(soon)={Count++}>{Count}</button>`,
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected invalid debounce duration diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_field_error") {
		t.Fatalf("missing component_field_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsDebounceThrottleCombination(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<button g:on:click.debounce(100ms).throttle(100ms)={Count++}>{Count}</button>`,
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected debounce/throttle compatibility diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_field_error") {
		t.Fatalf("missing component_field_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestResolvesUnaliasedGoTypedComponentImports(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		PropsType: manifest.GoTypeRef{
			Alias: "islands",
			Name:  "CounterProps",
		},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "islands", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "islands", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<button g:on:click={Count++}>{Label}: {Count}</button>`,
		},
	}}}

	if err := ValidateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected unaliased Go imports to validate, got %v", err)
	}
}

func TestValidateManifestRejectsMissingGoTypedComponentField(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<button g:on:click={Missing++}>{Missing}</button>`,
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected missing field diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_field_error") {
		t.Fatalf("missing component_field_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsMissingGoTypedComponentPackage(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/missing"}},
		PropsType: manifest.GoTypeRef{
			Alias: "ui",
			Name:  "CounterProps",
		},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<p>{Label}</p>`,
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected missing package diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_contract_error") {
		t.Fatalf("missing component_contract_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsMissingGoTypedComponentType(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		PropsType: manifest.GoTypeRef{
			Alias: "ui",
			Name:  "MissingProps",
		},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<p>{Label}</p>`,
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected missing type diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_contract_error") {
		t.Fatalf("missing component_contract_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestAllowsClientFunctionEventCall(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			Client: true,
			ClientBody: `fn Increment() {
  Count++
}`,
			View:     true,
			ViewBody: `<button g:on:click={Increment()}>{Count}</button>`,
		},
	}}}

	if err := ValidateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected client function event call to validate, got %v", err)
	}
}

func TestValidateManifestAllowsDeclaredComponentEmit(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Emits: []manifest.Emit{{
			Name:   "select",
			Params: []manifest.EmitParam{{Name: "id", Type: "int"}},
		}},
		Blocks: manifest.Blocks{
			Client: true,
			ClientBody: `fn Select() {
  emit select(Count)
}`,
			View:     true,
			ViewBody: `<button g:on:click={Select()}>{Count}</button>`,
		},
	}}}

	if err := ValidateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected declared component emit to validate, got %v", err)
	}
}

func TestValidateManifestRejectsDuplicateComponentEmitNames(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:   "Picker",
		Source: "components/picker.cmp.gwdk",
		Emits: []manifest.Emit{
			{
				Name: "select",
				Span: manifest.SourceSpan{
					Start: manifest.SourcePosition{Line: 4, Column: 3},
					End:   manifest.SourcePosition{Line: 4, Column: 16},
				},
			},
			{
				Name: "select",
				Span: manifest.SourceSpan{
					Start: manifest.SourcePosition{Line: 5, Column: 3},
					End:   manifest.SourcePosition{Line: 5, Column: 20},
				},
			},
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected duplicate component emit diagnostic")
	}
	diagnostic := firstDiagnostic(err.(ValidationErrors), "duplicate_component_emit")
	if diagnostic == nil {
		t.Fatalf("missing duplicate_component_emit diagnostic: %v", err)
	}
	if !strings.Contains(diagnostic.Message, `duplicate emit "select"`) {
		t.Fatalf("unexpected diagnostic message: %s", diagnostic.Message)
	}
	assertSourceSpan(t, diagnostic.Span, 5, 3, 5, 20)
}

func TestValidateManifestRejectsUnknownComponentEmit(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			Client: true,
			ClientBody: `fn Select() {
  emit select(Count)
}`,
			View:     true,
			ViewBody: `<button g:on:click={Select()}>{Count}</button>`,
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected unknown component emit diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_client_error") || !strings.Contains(err.Error(), `unknown component event "select"`) {
		t.Fatalf("unexpected diagnostics: %v", err)
	}
}

func TestValidateManifestClientParseErrorPointsToClientLine(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:   "Counter",
		Source: "components/counter.cmp.gwdk",
		Blocks: manifest.Blocks{
			Client:     true,
			ClientBody: "fn Bad() {\n  if Count {\n  }\n}",
			Spans: manifest.BlockSpans{
				Client: manifest.SourceSpan{
					Start: manifest.SourcePosition{Line: 10, Column: 1},
					End:   manifest.SourcePosition{Line: 14, Column: 1},
				},
			},
			View:     true,
			ViewBody: `<button>Bad</button>`,
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected client parse diagnostic")
	}
	diagnostic := firstDiagnostic(err.(ValidationErrors), "component_client_error")
	if diagnostic == nil {
		t.Fatalf("missing component_client_error diagnostic: %v", err)
	}
	if !strings.Contains(diagnostic.Message, "unsupported syntax") {
		t.Fatalf("unexpected diagnostic message: %s", diagnostic.Message)
	}
	assertSourceSpan(t, diagnostic.Span, 12, 1, 12, 2)
}

func TestValidateManifestRejectsComponentEmitPayloadTypeMismatch(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Emits: []manifest.Emit{{
			Name:   "select",
			Params: []manifest.EmitParam{{Name: "id", Type: "string"}},
		}},
		Blocks: manifest.Blocks{
			Client: true,
			ClientBody: `fn Select() {
  emit select(Count)
}`,
			View:     true,
			ViewBody: `<button g:on:click={Select()}>{Count}</button>`,
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected component emit payload type diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_client_error") || !strings.Contains(err.Error(), "component event select argument 1 expects string, got int") {
		t.Fatalf("unexpected diagnostics: %v", err)
	}
}

func TestValidateManifestAllowsClientFunctionParams(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			Client: true,
			ClientBody: `fn Add(step int) {
  Count = Count + step
}`,
			View:     true,
			ViewBody: `<button g:on:click={Add(Count + 1)}>{Count}</button>`,
		},
	}}}

	if err := ValidateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected client function params to validate, got %v", err)
	}
}

func TestValidateManifestAllowsClientHelperFunctionReturns(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			Client: true,
			ClientBody: `fn Next(value int) int {
  return value + 1
}

fn Add() {
  Count = Next(Count)
}`,
			View:     true,
			ViewBody: `<button g:on:click={Add()}>{Count}</button>`,
		},
	}}}

	if err := ValidateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected client helper function to validate, got %v", err)
	}
}

func TestValidateManifestAllowsClientBuiltins(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Nested",
		Source:  "components/nested.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "NestedState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewNestedState"},
		},
		Blocks: manifest.Blocks{
			Client: true,
			ClientBody: `computed ItemCount string {
  return string(len(Items))
}

fn SetCount() {
  Count = len(Items) + int("1")
}`,
			View:     true,
			ViewBody: `<button g:on:click={SetCount()}>{ItemCount}</button>`,
		},
	}}}

	if err := ValidateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected client built-ins to validate, got %v", err)
	}
}

func TestValidateManifestAllowsAsyncFetchJSONClientFunction(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Nested",
		Source:  "components/nested.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "NestedState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewNestedState"},
		},
		Blocks: manifest.Blocks{
			Client: true,
			ClientBody: `async fn Refresh() {
  Items = await fetchJSON[[]ui.Item]("/api/items")
}`,
			View:     true,
			ViewBody: `<button g:on:click={Refresh()}>{len(Items)}</button>`,
		},
	}}}

	if err := ValidateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected async fetchJSON function to validate, got %v", err)
	}
}

func TestValidateManifestRejectsAwaitOutsideAsyncClientFunction(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Nested",
		Source:  "components/nested.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "NestedState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewNestedState"},
		},
		Blocks: manifest.Blocks{
			Client: true,
			ClientBody: `fn Refresh() {
  Items = await fetchJSON[[]ui.Item]("/api/items")
}`,
			View:     true,
			ViewBody: `<button g:on:click={Refresh()}>{len(Items)}</button>`,
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected await outside async diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_client_error") || !strings.Contains(err.Error(), "await is only supported inside async client functions") {
		t.Fatalf("missing async await diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsAsyncFetchJSONNonStringURL(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Nested",
		Source:  "components/nested.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "NestedState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewNestedState"},
		},
		Blocks: manifest.Blocks{
			Client: true,
			ClientBody: `async fn Refresh() {
  Items = await fetchJSON[[]ui.Item](Count)
}`,
			View:     true,
			ViewBody: `<button g:on:click={Refresh()}>{len(Items)}</button>`,
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected fetchJSON URL diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_client_error") || !strings.Contains(err.Error(), "fetchJSON url must be string") {
		t.Fatalf("missing fetchJSON URL diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsBadClientBuiltinArg(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			Client: true,
			ClientBody: `fn Bad() {
  Count = len(Count)
}`,
			View:     true,
			ViewBody: `<button g:on:click={Bad()}>{Count}</button>`,
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected bad built-in argument diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_client_error") {
		t.Fatalf("missing component_client_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsHelperAsEventHandler(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			Client: true,
			ClientBody: `fn Next(value int) int {
  return value + 1
}`,
			View:     true,
			ViewBody: `<button g:on:click={Next(Count)}>{Count}</button>`,
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected helper event handler diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_field_error") {
		t.Fatalf("missing component_field_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsHelperReturnMismatch(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			Client: true,
			ClientBody: `fn Bad() int {
  return Open
}

fn Add() {
  Count = Bad()
}`,
			View:     true,
			ViewBody: `<button g:on:click={Add()}>{Count}</button>`,
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected helper return mismatch diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_client_error") {
		t.Fatalf("missing component_client_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsHelperCallCycle(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			Client: true,
			ClientBody: `fn A() int {
  return B()
}

fn B() int {
  return A()
}

fn Add() {
  Count = A()
}`,
			View:     true,
			ViewBody: `<button g:on:click={Add()}>{Count}</button>`,
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected helper call cycle diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_client_error") {
		t.Fatalf("missing component_client_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsClientExpressionTypeMismatch(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			Client: true,
			ClientBody: `fn Bad(step int) {
  Open = Count + step
}`,
			View:     true,
			ViewBody: `<button g:on:click={Bad(1)}>{Count}</button>`,
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected expression type mismatch diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_client_error") {
		t.Fatalf("missing component_client_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsClientFunctionArgumentMismatch(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			Client: true,
			ClientBody: `fn Add(step int) {
  Count = step
}`,
			View:     true,
			ViewBody: `<button g:on:click={Add()}>{Count}</button>`,
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected argument mismatch diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_field_error") {
		t.Fatalf("missing component_field_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsUnknownClientFunctionEventCall(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			Client: true,
			ClientBody: `fn Increment() {
  Count++
}`,
			View:     true,
			ViewBody: `<button g:on:click={Missing()}>{Count}</button>`,
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected unknown client function diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_field_error") {
		t.Fatalf("missing component_field_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsClientFunctionUnknownStateField(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			Client: true,
			ClientBody: `fn Increment() {
  Missing++
}`,
			View:     true,
			ViewBody: `<button g:on:click={Increment()}>{Count}</button>`,
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected client function field diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_client_error") {
		t.Fatalf("missing component_client_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsClientFunctionMutatingProp(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		PropsType: manifest.GoTypeRef{
			Alias: "ui",
			Name:  "CounterProps",
		},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			Client: true,
			ClientBody: `fn Rename() {
  Label = "changed"
}`,
			View:     true,
			ViewBody: `<button g:on:click={Rename()}>{Label}</button>`,
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected prop mutation diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_client_error") {
		t.Fatalf("missing component_client_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestAllowsLifecycleAndEffects(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			Client: true,
			ClientBody: `on mount {
  Open = true
}

effect when Count {
  Open = false
  return {
    Open = true
  }
}

on destroy {
  Open = false
}`,
			View:     true,
			ViewBody: `<button g:on:click={Count++}>{Count}</button>`,
		},
	}}}

	if err := ValidateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected lifecycle/effects to validate, got %v", err)
	}
}

func TestValidateManifestRejectsEffectUnknownDependency(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			Client: true,
			ClientBody: `effect when Missing {
  Open = true
}`,
			View:     true,
			ViewBody: `<button>{Count}</button>`,
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected unknown effect dependency diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_client_error") {
		t.Fatalf("missing component_client_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestAllowsDOMRefFocusCall(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Search",
		Source:  "components/search.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "TextState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewTextState"},
		},
		Blocks: manifest.Blocks{
			Client: true,
			ClientBody: `ref searchInput HTMLInputElement

fn FocusSearch() {
  searchInput.Focus()
}`,
			View:     true,
			ViewBody: `<input g:ref={searchInput} /><button g:on:click={FocusSearch()}>Focus</button>`,
		},
	}}}

	if err := ValidateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected DOM ref focus call to validate, got %v", err)
	}
}

func TestValidateManifestRejectsUnknownDOMRefBinding(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Search",
		Source:  "components/search.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "TextState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewTextState"},
		},
		Blocks: manifest.Blocks{
			Client:     true,
			ClientBody: `ref searchInput HTMLInputElement`,
			View:       true,
			ViewBody:   `<input g:ref={missingInput} />`,
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected unknown DOM ref binding diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_field_error") {
		t.Fatalf("missing component_field_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsDuplicateDOMRefBinding(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Search",
		Source:  "components/search.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "TextState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewTextState"},
		},
		Blocks: manifest.Blocks{
			Client:     true,
			ClientBody: `ref searchInput HTMLInputElement`,
			View:       true,
			ViewBody:   `<input g:ref={searchInput} /><input g:ref={searchInput} />`,
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected duplicate DOM ref binding diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_field_error") {
		t.Fatalf("missing component_field_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsUnboundUsedDOMRef(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Search",
		Source:  "components/search.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "TextState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewTextState"},
		},
		Blocks: manifest.Blocks{
			Client: true,
			ClientBody: `ref searchInput HTMLInputElement

fn FocusSearch() {
  searchInput.Focus()
}`,
			View:     true,
			ViewBody: `<button g:on:click={FocusSearch()}>Focus</button>`,
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected unbound DOM ref diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_client_error") {
		t.Fatalf("missing component_client_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestAllowsGIfBoolExpression(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<p g:if={Count > 0 && !Open}>{Count}</p>`,
		},
	}}}

	if err := ValidateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected g:if bool expression to validate, got %v", err)
	}
}

func TestValidateManifestAllowsGElseIfExpression(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<p g:if={Open}>{Count}</p><p g:else-if={Count > 0}>{Count}</p><p g:else>Closed</p>`,
		},
	}}}

	if err := ValidateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected g:else-if expression to validate, got %v", err)
	}
}

func TestValidateManifestRejectsGElseIfNonBoolExpression(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<p g:if={Open}>{Count}</p><p g:else-if={Count}>{Count}</p>`,
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected g:else-if non-bool diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_field_error") {
		t.Fatalf("missing component_field_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsGIfNonBoolExpression(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<p g:if={Count}>{Count}</p>`,
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected g:if non-bool diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_field_error") {
		t.Fatalf("missing component_field_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestAllowsNestedAndIndexExpressions(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Nested",
		Source:  "components/nested.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "NestedState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewNestedState"},
		},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<section g:if={User.Open && Items[0].Name == "first" && Flags[Count]}>{Count}</section>`,
		},
	}}}

	if err := ValidateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected nested/index expressions to validate, got %v", err)
	}
}

func TestValidateManifestAllowsGForListRendering(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Nested",
		Source:  "components/nested.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "NestedState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewNestedState"},
		},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<ul><li g:for={item in Items} g:key={item.ID}>{item.Name}</li></ul>`,
		},
	}}}

	if err := ValidateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected g:for list rendering to validate, got %v", err)
	}
}

func TestValidateManifestAllowsGForIndexVariable(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Nested",
		Source:  "components/nested.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "NestedState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewNestedState"},
		},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<ol><li aria-posinset={i} g:for={item, i in Items} g:key={item.ID}>{i}: {item.Name}</li></ol>`,
		},
	}}}

	if err := ValidateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected g:for index variable to validate, got %v", err)
	}
}

func TestValidateManifestAllowsListMutationBuiltins(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Nested",
		Source:  "components/nested.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "NestedState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewNestedState"},
		},
		Blocks: manifest.Blocks{
			Client: true,
			ClientBody: `fn AddItem() {
  append(Items, { ID: "third", Name: "third", Done: false })
}

fn RemoveFirst() {
  remove(Items, 0)
}

fn SwapFirstTwo() {
  move(Items, 1, 0)
}`,
			View:     true,
			ViewBody: `<ul><li g:for={item, i in Items} g:key={item.ID}><button g:on:click={remove(Items, i)}>{item.Name}</button></li></ul><button g:on:click={AddItem()}>Add</button><button g:on:click={RemoveFirst()}>Remove</button><button g:on:click={SwapFirstTwo()}>Swap</button>`,
		},
	}}}

	if err := ValidateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected list mutation built-ins to validate, got %v", err)
	}
}

func TestValidateManifestRejectsBadAppendItemField(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Nested",
		Source:  "components/nested.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "NestedState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewNestedState"},
		},
		Blocks: manifest.Blocks{
			Client: true,
			ClientBody: `fn AddItem() {
  append(Items, { Missing: "third" })
}`,
			View:     true,
			ViewBody: `<ul><li g:for={item in Items} g:key={item.ID}>{item.Name}</li></ul><button g:on:click={AddItem()}>Add</button>`,
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected bad append item diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_client_error") || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("missing bad append field diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsGForWithoutKey(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Nested",
		Source:  "components/nested.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "NestedState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewNestedState"},
		},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<ul><li g:for={item in Items}>{item.Name}</li></ul>`,
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected missing g:key diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_field_error") || !strings.Contains(err.Error(), "g:for requires g:key") {
		t.Fatalf("missing g:for missing key diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsUnknownGForItemField(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Nested",
		Source:  "components/nested.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "NestedState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewNestedState"},
		},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<ul><li g:for={item in Items} g:key={item.ID}>{item.Missing}</li></ul>`,
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected unknown item field diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_field_error") || !strings.Contains(err.Error(), "item.Missing") {
		t.Fatalf("missing unknown item field diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestViewEventDiagnosticPointsToExpression(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<button g:on:click={Missing()}>{Count}</button>`,
			Spans: manifest.BlockSpans{
				View: manifest.SourceSpan{Start: manifest.SourcePosition{Line: 9, Column: 1}, End: manifest.SourcePosition{Line: 9, Column: 7}},
			},
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected invalid event expression diagnostic")
	}
	diagnostic := firstDiagnostic(err.(ValidationErrors), "component_field_error")
	if diagnostic == nil || !strings.Contains(diagnostic.Message, "Missing()") {
		t.Fatalf("missing event expression diagnostic: %#v", err)
	}
	assertSourceSpan(t, diagnostic.Span, 10, 21, 10, 30)
}

func TestValidateManifestUnknownViewFieldDiagnosticPointsToIdentifier(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:   "Counter",
		Source: "components/counter.cmp.gwdk",
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<button>{Missing}</button>`,
			Spans: manifest.BlockSpans{
				View: manifest.SourceSpan{Start: manifest.SourcePosition{Line: 4, Column: 1}, End: manifest.SourcePosition{Line: 4, Column: 7}},
			},
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected unknown field diagnostic")
	}
	diagnostic := firstDiagnostic(err.(ValidationErrors), "component_field_error")
	if diagnostic == nil || !strings.Contains(diagnostic.Message, `"Missing"`) {
		t.Fatalf("missing unknown field diagnostic: %#v", err)
	}
	assertSourceSpan(t, diagnostic.Span, 5, 10, 5, 17)
}

func TestValidateManifestBadGForDiagnosticPointsToDirectiveValue(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:   "Nested",
		Source: "components/nested.cmp.gwdk",
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<ul><li g:for={item of Items}>{item.Name}</li></ul>`,
			Spans: manifest.BlockSpans{
				View: manifest.SourceSpan{Start: manifest.SourcePosition{Line: 12, Column: 1}, End: manifest.SourcePosition{Line: 12, Column: 7}},
			},
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected bad g:for diagnostic")
	}
	diagnostic := firstDiagnostic(err.(ValidationErrors), "component_field_error")
	if diagnostic == nil || !strings.Contains(diagnostic.Message, `g:for must use`) {
		t.Fatalf("missing bad g:for diagnostic: %#v", err)
	}
	assertSourceSpan(t, diagnostic.Span, 13, 16, 13, 29)
}

func TestValidateManifestAllowsGoishConditionalExpressions(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			Client: true,
			ClientBody: `fn ToggleCount() {
  Count = if Open { Count + 1 } else { 0 }
}`,
			View:     true,
			ViewBody: `<section g:if={if Open { Count > 0 } else { false }}><button g:on:click={ToggleCount()}>{Count}</button></section>`,
		},
	}}}

	if err := ValidateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected Go-ish conditional expressions to validate, got %v", err)
	}
}

func TestValidateManifestAllowsClientLocalVariables(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			Client: true,
			ClientBody: `fn Add(step int) {
  let next int = Count + step
  Count = next
}`,
			View:     true,
			ViewBody: `<button g:on:click={Add(2)}>{Count}</button>`,
		},
	}}}

	if err := ValidateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected local variables to validate, got %v", err)
	}
}

func TestValidateManifestRejectsLocalVariableBeforeDeclaration(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			Client: true,
			ClientBody: `fn Bad() {
  Count = next
  let next int = Count + 1
}`,
			View:     true,
			ViewBody: `<button g:on:click={Bad()}>{Count}</button>`,
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected local-before-declaration diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_client_error") || !strings.Contains(err.Error(), "next") {
		t.Fatalf("missing local-before-declaration diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsGoishConditionalTypeMismatch(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			Client: true,
			ClientBody: `fn Bad() {
  Count = if Open { Count + 1 } else { "closed" }
}`,
			View:     true,
			ViewBody: `<button g:on:click={Bad()}>{Count}</button>`,
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected conditional branch mismatch diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_client_error") {
		t.Fatalf("missing component_client_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestAllowsComputedState(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			Client: true,
			ClientBody: `computed Label string {
  return if Open { "open" } else { "closed" }
}

computed Visible bool {
  return Label == "open"
}`,
			View:     true,
			ViewBody: `<section g:if={Visible}>{Label}</section>`,
		},
	}}}

	if err := ValidateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected computed state to validate, got %v", err)
	}
}

func TestValidateManifestAllowsComputedOutOfOrderDependencies(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			Client: true,
			ClientBody: `computed Visible bool {
  return Label == "open"
}

computed Label string {
  return if Open { "open" } else { "closed" }
}`,
			View:     true,
			ViewBody: `<section g:if={Visible}>{Label}</section>`,
		},
	}}}

	if err := ValidateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected out-of-order computed state to validate, got %v", err)
	}
}

func TestValidateManifestRejectsComputedCycle(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			Client: true,
			ClientBody: `computed First string {
  return Second
}

computed Second string {
  return First
}`,
			View:     true,
			ViewBody: `<section>{First}</section>`,
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected computed cycle diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_client_error") {
		t.Fatalf("missing component_client_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsComputedMutation(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			Client: true,
			ClientBody: `computed Label string {
  Count = Count + 1
}`,
			View:     true,
			ViewBody: `<section>{Count}</section>`,
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected computed mutation diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_client_error") {
		t.Fatalf("missing component_client_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsUnknownNestedField(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Nested",
		Source:  "components/nested.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "NestedState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewNestedState"},
		},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<section g:if={User.Missing}>{Count}</section>`,
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected unknown nested field diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_field_error") {
		t.Fatalf("missing component_field_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestAllowsValueBindingToStringState(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Search",
		Source:  "components/search.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "TextState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewTextState"},
		},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<input g:bind:value={Query} />`,
		},
	}}}

	if err := ValidateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected string state value binding to validate, got %v", err)
	}
}

func TestValidateManifestRejectsValueBindingToNonStringState(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<input g:bind:value={Count} />`,
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected non-string value binding diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_field_error") {
		t.Fatalf("missing component_field_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestAllowsNumberInputValueBindingToNumericState(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<input type="number" g:bind:value={Count} />`,
		},
	}}}

	if err := ValidateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected numeric value binding to validate, got %v", err)
	}
}

func TestValidateManifestRejectsNumericValueBindingOutsideNumberInput(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<textarea g:bind:value={Count}></textarea>`,
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected numeric value binding target diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_field_error") {
		t.Fatalf("missing component_field_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestAllowsRadioValueBindingToStringState(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Search",
		Source:  "components/search.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "TextState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewTextState"},
		},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<input type="radio" value="initial" g:bind:value={Query} />`,
		},
	}}}

	if err := ValidateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected radio value binding to validate, got %v", err)
	}
}

func TestValidateManifestRejectsRadioValueBindingWithoutValue(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Search",
		Source:  "components/search.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "TextState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewTextState"},
		},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<input type="radio" g:bind:value={Query} />`,
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected radio missing value diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_field_error") {
		t.Fatalf("missing component_field_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsValueBindingToProp(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Search",
		Source:  "components/search.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		PropsType: manifest.GoTypeRef{
			Alias: "ui",
			Name:  "CounterProps",
		},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<input g:bind:value={Label} />`,
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected prop value binding diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_field_error") {
		t.Fatalf("missing component_field_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestAllowsCheckedBindingToBoolState(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<input type="checkbox" g:bind:checked={Open} />`,
		},
	}}}

	if err := ValidateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected bool state checked binding to validate, got %v", err)
	}
}

func TestValidateManifestRejectsCheckedBindingToNonBoolState(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Search",
		Source:  "components/search.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "TextState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewTextState"},
		},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<input type="checkbox" g:bind:checked={Query} />`,
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected non-bool checked binding diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_field_error") {
		t.Fatalf("missing component_field_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestAllowsReactiveAttributes(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<button disabled={Open} aria-expanded={Open}>{Count}</button>`,
		},
	}}}

	if err := ValidateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected reactive attributes to validate, got %v", err)
	}
}

func TestValidateManifestRejectsNonBoolReactiveBooleanAttribute(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<button disabled={Count}>{Count}</button>`,
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected non-bool boolean attr diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_field_error") {
		t.Fatalf("missing component_field_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsUnsafeReactiveURLAttribute(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Link",
		Source:  "components/link.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "TextState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewTextState"},
		},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<a href={Query}>Link</a>`,
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected unsafe reactive URL attr diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_field_error") {
		t.Fatalf("missing component_field_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestAllowsClassToggle(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<button class:active={Open}>{Count}</button>`,
		},
	}}}

	if err := ValidateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected class toggle to validate, got %v", err)
	}
}

func TestValidateManifestRejectsNonBoolClassToggle(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<button class:active={Count}>{Count}</button>`,
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected non-bool class toggle diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_field_error") {
		t.Fatalf("missing component_field_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestAllowsStyleBinding(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<div style:height.px={Count}>{Count}</div>`,
		},
	}}}

	if err := ValidateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected style binding to validate, got %v", err)
	}
}

func TestValidateManifestRejectsBoolStyleBinding(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<div style:height.px={Open}>{Count}</div>`,
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected bool style binding diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_field_error") {
		t.Fatalf("missing component_field_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsRelativeGoTypedImportPath(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "./ui"}},
		PropsType: manifest.GoTypeRef{
			Alias: "ui",
			Name:  "CounterProps",
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected invalid import diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "invalid_go_import") {
		t.Fatalf("missing invalid_go_import diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsStateInitReturnMismatch(t *testing.T) {
	app := manifest.Manifest{Components: []manifest.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewOtherState"},
		},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<p>{Count}</p>`,
		},
	}}}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected state init mismatch diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_contract_error") {
		t.Fatalf("missing component_contract_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestResolvesLayoutsByID(t *testing.T) {
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:      "dashboard",
			Route:   "/dashboard",
			Layouts: []string{"root", "missing"},
			Source:  "pages/dashboard.page.gwdk",
			Blocks:  manifest.Blocks{View: true},
		}},
		Layouts: []manifest.Layout{{
			ID:     "root",
			Source: "layouts/root.layout.gwdk",
		}},
	}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected unknown layout diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "unknown_layout_id") {
		t.Fatalf("missing unknown_layout_id diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsDuplicateLayoutIDs(t *testing.T) {
	app := manifest.Manifest{
		Layouts: []manifest.Layout{
			{ID: "root", Source: "layouts/root.layout.gwdk"},
			{ID: "root", Source: "layouts/root-copy.layout.gwdk"},
		},
	}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected duplicate layout diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "duplicate_layout_id") {
		t.Fatalf("missing duplicate_layout_id diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsDuplicatePageRoutes(t *testing.T) {
	app := manifest.Manifest{
		Pages: []manifest.Page{
			{ID: "blog.post", Route: "/blog/{slug}", Paths: true, Source: "pages/blog-post.page.gwdk", Blocks: manifest.Blocks{View: true}},
			{ID: "blog.entry", Route: "/blog/{id}", Paths: true, Source: "pages/blog-entry.page.gwdk", Blocks: manifest.Blocks{View: true}},
		},
	}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected duplicate route diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "duplicate_route") {
		t.Fatalf("missing duplicate_route diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsAmbiguousDynamicPageRoutes(t *testing.T) {
	tests := []struct {
		name  string
		left  string
		right string
	}{
		{
			name:  "literal tail can also be a param",
			left:  "/blog/{category}/{slug}",
			right: "/blog/{slug}/edit",
		},
		{
			name:  "literal head can also be a param",
			left:  "/{section}/settings",
			right: "/admin/{page}",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := manifest.Manifest{
				Pages: []manifest.Page{
					{ID: "left", Route: test.left, Paths: true, Blocks: manifest.Blocks{View: true}},
					{ID: "right", Route: test.right, Paths: true, Blocks: manifest.Blocks{View: true}},
				},
			}

			err := ValidateManifest(gowdk.Config{}, app)
			if err == nil {
				t.Fatal("expected ambiguous dynamic route diagnostic")
			}
			diagnostics := err.(ValidationErrors)
			if !hasDiagnosticCode(diagnostics, "ambiguous_dynamic_route") {
				t.Fatalf("missing ambiguous_dynamic_route diagnostic: %#v", diagnostics)
			}
		})
	}
}

func TestValidateManifestAllowsConcreteRouteBesideDynamicPageRoute(t *testing.T) {
	app := manifest.Manifest{
		Pages: []manifest.Page{
			{ID: "blog.about", Route: "/blog/about", Blocks: manifest.Blocks{View: true}},
			{ID: "blog.post", Route: "/blog/{slug}", Paths: true, Blocks: manifest.Blocks{View: true}},
		},
	}

	if err := ValidateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected concrete and dynamic routes to be valid, got %v", err)
	}
}

func TestValidateManifestRejectsRouteMethodConflicts(t *testing.T) {
	t.Run("multiple actions on one route", func(t *testing.T) {
		app := manifest.Manifest{
			Pages: []manifest.Page{{
				ID:    "profile",
				Route: "/profile",
				Blocks: manifest.Blocks{
					View:    true,
					Actions: []manifest.Action{{Name: "save"}, {Name: "updateAvatar"}},
				},
			}},
		}

		err := ValidateManifest(gowdk.Config{}, app)
		if err == nil {
			t.Fatal("expected route method conflict")
		}
		diagnostics := err.(ValidationErrors)
		if !hasDiagnosticCode(diagnostics, "route_method_conflict") {
			t.Fatalf("missing route_method_conflict diagnostic: %#v", diagnostics)
		}
	})

	t.Run("api default route conflicts with page get", func(t *testing.T) {
		app := manifest.Manifest{
			Pages: []manifest.Page{{
				ID:    "patients.index",
				Route: "/patients",
				Blocks: manifest.Blocks{
					View: true,
					APIs: []manifest.API{{Name: "index"}},
				},
			}},
		}

		err := ValidateManifest(gowdk.Config{}, app)
		if err == nil {
			t.Fatal("expected route method conflict")
		}
		diagnostics := err.(ValidationErrors)
		if !hasDiagnosticCode(diagnostics, "route_method_conflict") {
			t.Fatalf("missing route_method_conflict diagnostic: %#v", diagnostics)
		}
	})
}

func TestValidateManifestAllowsSameRouteWithDifferentMethods(t *testing.T) {
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "newsletter",
			Route: "/newsletter",
			Blocks: manifest.Blocks{
				View:    true,
				Actions: []manifest.Action{{Name: "subscribe"}},
			},
		}},
	}

	if err := ValidateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected GET page plus POST action to be valid, got %v", err)
	}
}

func TestValidatePageRejectsMalformedRoutes(t *testing.T) {
	tests := []struct {
		name  string
		route string
	}{
		{name: "relative route", route: "patients"},
		{name: "query string", route: "/patients?page=1"},
		{name: "trailing slash", route: "/patients/"},
		{name: "dot segment", route: "/patients/../admin"},
		{name: "embedded param", route: "/blog/{slug}.html"},
		{name: "invalid param name", route: "/blog/{123}"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			page := manifest.Page{ID: "patients", Route: test.route, Paths: true, Blocks: manifest.Blocks{View: true}}

			diagnostics := ValidatePage(gowdk.Config{}, page)
			if !hasDiagnosticCode(diagnostics, "malformed_route") {
				t.Fatalf("missing malformed_route diagnostic for %q: %#v", test.route, diagnostics)
			}
			if hasDiagnosticCode(diagnostics, "spa_dynamic_route_missing_paths") {
				t.Fatalf("malformed route should not cascade into missing paths: %#v", diagnostics)
			}
		})
	}
}

func TestValidatePageRejectsDuplicateRouteParams(t *testing.T) {
	page := manifest.Page{ID: "blog.post", Route: "/blog/{slug}/{slug}", Paths: true, Blocks: manifest.Blocks{View: true}}

	diagnostics := ValidatePage(gowdk.Config{}, page)
	if !hasDiagnosticCode(diagnostics, "duplicate_route_param") {
		t.Fatalf("missing duplicate_route_param diagnostic: %#v", diagnostics)
	}
}

func TestValidatePageRequiresPathsForSPADynamicRoutes(t *testing.T) {
	page := manifest.Page{ID: "patients.show", Route: "/patients/{id}", Render: gowdk.SPA, Blocks: manifest.Blocks{View: true}}

	diagnostics := ValidatePage(gowdk.Config{}, page)
	if len(diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diagnostics))
	}
	if diagnostics[0].Code != "spa_dynamic_route_missing_paths" {
		t.Fatalf("unexpected diagnostic code: %s", diagnostics[0].Code)
	}
	if !strings.Contains(diagnostics[0].Message, "add paths") {
		t.Fatalf("diagnostic should suggest paths block: %s", diagnostics[0].Message)
	}
}

func TestValidatePageAllowsSPADynamicRoutesWithPaths(t *testing.T) {
	page := manifest.Page{ID: "blog.post", Route: "/blog/{slug}", Render: gowdk.SPA, Paths: true, Blocks: manifest.Blocks{View: true}}

	diagnostics := ValidatePage(gowdk.Config{}, page)
	if len(diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %#v", diagnostics)
	}
}

func TestValidatePageAllowsSPAActionsWithoutSSR(t *testing.T) {
	page := manifest.Page{
		ID:     "newsletter",
		Route:  "/newsletter",
		Render: gowdk.SPA,
		Blocks: manifest.Blocks{
			View:    true,
			Actions: []manifest.Action{{Name: "subscribe"}},
		},
	}

	diagnostics := ValidatePage(gowdk.Config{}, page)
	if len(diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %#v", diagnostics)
	}
}

func TestValidatePageRejectsLoadOnSPAPage(t *testing.T) {
	page := manifest.Page{
		ID:     "newsletter",
		Route:  "/newsletter",
		Render: gowdk.SPA,
		Blocks: manifest.Blocks{
			View: true,
			Load: true,
		},
	}

	diagnostics := ValidatePage(gowdk.Config{}, page)
	if len(diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diagnostics))
	}
	if diagnostics[0].Code != "load_requires_request_render" {
		t.Fatalf("unexpected diagnostic code: %s", diagnostics[0].Code)
	}
}

func TestValidatePageRejectsAmbiguousHybridWithoutLoad(t *testing.T) {
	page := manifest.Page{
		ID:     "dashboard",
		Route:  "/dashboard",
		Render: gowdk.Hybrid,
		Blocks: manifest.Blocks{
			View: true,
		},
	}

	diagnostics := ValidatePage(gowdk.Config{Addons: []gowdk.Addon{ssr.Addon()}}, page)
	if len(diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %#v", diagnostics)
	}
	if diagnostics[0].Code != "hybrid_requires_explicit_request_policy" {
		t.Fatalf("unexpected diagnostic code: %s", diagnostics[0].Code)
	}
	if !strings.Contains(diagnostics[0].Message, "implicit SSR") {
		t.Fatalf("expected implicit SSR guidance: %s", diagnostics[0].Message)
	}
}

func TestValidatePageAllowsHybridWithExplicitLoadAndSSRAddon(t *testing.T) {
	page := manifest.Page{
		ID:     "dashboard",
		Route:  "/dashboard",
		Render: gowdk.Hybrid,
		Blocks: manifest.Blocks{
			Load: true,
			View: true,
		},
	}

	diagnostics := ValidatePage(gowdk.Config{Addons: []gowdk.Addon{ssr.Addon()}}, page)
	if len(diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %#v", diagnostics)
	}
}

func TestValidatePageRejectsMissingViewBlock(t *testing.T) {
	page := manifest.Page{ID: "home", Route: "/", Render: gowdk.SPA}

	diagnostics := ValidatePage(gowdk.Config{}, page)
	if len(diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %#v", diagnostics)
	}
	if diagnostics[0].Code != "missing_view_block" {
		t.Fatalf("unexpected diagnostic code: %s", diagnostics[0].Code)
	}
}

func TestValidatePageRejectsInvalidCSSSelection(t *testing.T) {
	page := manifest.Page{
		ID:    "embed",
		Route: "/embed",
		CSS:   []string{"none", "forms"},
		Blocks: manifest.Blocks{
			View: true,
		},
	}

	diagnostics := ValidatePage(gowdk.Config{}, page)
	if !hasDiagnosticCode(diagnostics, "invalid_css_selection") {
		t.Fatalf("missing invalid_css_selection diagnostic: %#v", diagnostics)
	}
}

func TestValidatePageRejectsDuplicateCSSSelection(t *testing.T) {
	page := manifest.Page{
		ID:    "home",
		Route: "/",
		CSS:   []string{"default", "forms", "forms"},
		Blocks: manifest.Blocks{
			View: true,
		},
	}

	diagnostics := ValidatePage(gowdk.Config{}, page)
	if !hasDiagnosticCode(diagnostics, "duplicate_css_selection") {
		t.Fatalf("missing duplicate_css_selection diagnostic: %#v", diagnostics)
	}
}

func hasDiagnosticCode(diagnostics []ValidationError, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}

func firstDiagnostic(diagnostics []ValidationError, code string) *ValidationError {
	for index := range diagnostics {
		if diagnostics[index].Code == code {
			return &diagnostics[index]
		}
	}
	return nil
}

func assertSourceSpan(t *testing.T, span manifest.SourceSpan, startLine, startColumn, endLine, endColumn int) {
	t.Helper()
	if span.Start.Line != startLine || span.Start.Column != startColumn || span.End.Line != endLine || span.End.Column != endColumn {
		t.Fatalf("unexpected source span: got %#v, want %d:%d-%d:%d", span, startLine, startColumn, endLine, endColumn)
	}
}

func hasDiagnosticMessage(diagnostics []ValidationError, code string, parts ...string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code != code {
			continue
		}
		matches := true
		for _, part := range parts {
			if !strings.Contains(diagnostic.Message, part) {
				matches = false
				break
			}
		}
		if matches {
			return true
		}
	}
	return false
}
