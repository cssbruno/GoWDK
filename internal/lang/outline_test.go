package lang

import "testing"

func symbolNames(symbols []OutlineSymbol) []string {
	names := make([]string, 0, len(symbols))
	for _, symbol := range symbols {
		names = append(names, symbol.Name)
	}
	return names
}

func findSymbol(symbols []OutlineSymbol, name string) (OutlineSymbol, bool) {
	for _, symbol := range symbols {
		if symbol.Name == name {
			return symbol, true
		}
	}
	return OutlineSymbol{}, false
}

func TestOutlineParsesTopLevelDeclarations(t *testing.T) {
	src := `package pages

route "/"
title "Home"

view {
  <main>
    <h1>{title}</h1>
  </main>
}

style {
  main { padding: 1rem; }
}
`
	symbols := Outline(src)

	for _, want := range []string{"package pages", "route", "title", "view", "style"} {
		if _, ok := findSymbol(symbols, want); !ok {
			t.Errorf("expected outline symbol %q; got %v", want, symbolNames(symbols))
		}
	}

	// The view block's range must extend past the interpolation braces to the
	// real closing brace, not stop at the {title} interpolation.
	view, _ := findSymbol(symbols, "view")
	if view.Span.End.Line < 10 {
		t.Fatalf("view block range ended too early at line %d (interpolation miscounted?)", view.Span.End.Line)
	}
}

func TestOutlineRecoversFromUnknownLines(t *testing.T) {
	// A junk line sits between valid declarations; the parser must skip it and
	// still surface the declarations after it.
	src := `package pages

@@@ not valid !!!

route "/"

view {
  <main></main>
}
`
	symbols := Outline(src)
	for _, want := range []string{"package pages", "route", "view"} {
		if _, ok := findSymbol(symbols, want); !ok {
			t.Errorf("recovery failed: expected %q after a junk line; got %v", want, symbolNames(symbols))
		}
	}
}

func TestOutlineIncludesEndpointsAndComponents(t *testing.T) {
	src := `package widgets

component Counter

api Items GET "/items"
`
	symbols := Outline(src)
	if symbol, ok := findSymbol(symbols, "component Counter"); !ok || symbol.Kind != OutlineKindComponent {
		t.Errorf("expected a component symbol; got %v", symbolNames(symbols))
	}
	if symbol, ok := findSymbol(symbols, "api Items"); !ok || symbol.Kind != OutlineKindEndpoint {
		t.Errorf("expected an endpoint symbol; got %v", symbolNames(symbols))
	}
}

func TestOutlineSpansCarryOffsets(t *testing.T) {
	src := "package pages\nroute \"/\"\n"
	symbols := Outline(src)
	pkg, ok := findSymbol(symbols, "package pages")
	if !ok {
		t.Fatalf("expected package symbol; got %v", symbolNames(symbols))
	}
	if pkg.Span.Start.Offset != 0 {
		t.Fatalf("package symbol start offset = %d, want 0", pkg.Span.Start.Offset)
	}
}
