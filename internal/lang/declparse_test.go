package lang

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cssbruno/gowdk/internal/gwdkast"
	"github.com/cssbruno/gowdk/internal/parser"
)

func importKeys(imports []gwdkast.Import) map[string]bool {
	keys := map[string]bool{}
	for _, item := range imports {
		keys[item.Alias+"\x00"+item.Path] = true
	}
	return keys
}

func useKeys(uses []gwdkast.Use) map[string]bool {
	keys := map[string]bool{}
	for _, item := range uses {
		keys[item.Alias+"\x00"+item.Package] = true
	}
	return keys
}

// metadataPairs reduces metadata declarations to ordered Name=Value strings, the
// surface both parsers must agree on (spans differ by construction).
func metadataPairs(decls []gwdkast.MetadataDecl) []string {
	pairs := make([]string, 0, len(decls))
	for _, decl := range decls {
		pairs = append(pairs, decl.Name+"="+decl.Value)
	}
	return pairs
}

func equalOrdered(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for index := range a {
		if a[index] != b[index] {
			return false
		}
	}
	return true
}

// TestParseTopLevelMatchesCompilerParser anchors the recursive-descent declaration
// parser against ParseSyntax: for valid source they must agree on
// the package name and the set of imports and uses.
func TestParseTopLevelMatchesCompilerParser(t *testing.T) {
	src := `package pages

import "fmt"
import alias "github.com/x/y"

use widgets "components"
use forms "forms"

page home
route "/"
title "Home"
description "Welcome"
cache "public, max-age=60"

view {
  <main>{title}</main>
}
`
	syntaxFile, err := parser.ParseSyntax([]byte(src))
	if err != nil {
		t.Fatalf("compiler parser failed: %v", err)
	}
	top := ParseTopLevel(src)

	if top.Package == nil || syntaxFile.Package == nil || top.Package.Name != syntaxFile.Package.Name {
		t.Fatalf("package mismatch: got %v, compiler parser %v", top.Package, syntaxFile.Package)
	}
	if got, want := importKeys(top.Imports), importKeys(syntaxFile.Imports); !equalKeys(got, want) {
		t.Fatalf("import mismatch:\n got %v\nwant %v", got, want)
	}
	if got, want := useKeys(top.Uses), useKeys(syntaxFile.Uses); !equalKeys(got, want) {
		t.Fatalf("use mismatch:\n got %v\nwant %v", got, want)
	}
	if got, want := metadataPairs(top.Metadata), metadataPairs(syntaxFile.Metadata); !equalOrdered(got, want) {
		t.Fatalf("metadata mismatch:\n got %v\nwant %v", got, want)
	}
	if (top.Page == nil) != (syntaxFile.Page == nil) || (top.Page != nil && top.Page.ID != syntaxFile.Page.ID) {
		t.Fatalf("page mismatch: got %v, compiler parser %v", top.Page, syntaxFile.Page)
	}
	if (top.Cache == nil) != (syntaxFile.Cache == nil) || (top.Cache != nil && top.Cache.Policy != syntaxFile.Cache.Policy) {
		t.Fatalf("cache mismatch: got %v, compiler parser %v", top.Cache, syntaxFile.Cache)
	}
}

// TestParseTopLevelMatchesPackageOnCorpus runs the equivalence check across the
// accept corpus, comparing the package declaration each parser recovers.
func TestParseTopLevelMatchesPackageOnCorpus(t *testing.T) {
	dir := filepath.FromSlash("testdata/conformance/accept")
	for _, name := range conformanceFiles(t, dir) {
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			syntaxFile, err := parser.ParseSyntax(source)
			if err != nil {
				t.Skipf("compiler parser rejects %s: %v", name, err)
			}
			top := ParseTopLevel(string(source))
			lineName := ""
			if syntaxFile.Package != nil {
				lineName = syntaxFile.Package.Name
			}
			topName := ""
			if top.Package != nil {
				topName = top.Package.Name
			}
			if topName != lineName {
				t.Fatalf("package mismatch for %s: got %q, compiler parser %q", name, topName, lineName)
			}
			if got, want := metadataPairs(top.Metadata), metadataPairs(syntaxFile.Metadata); !equalOrdered(got, want) {
				t.Fatalf("metadata mismatch for %s:\n got %v\nwant %v", name, got, want)
			}
			if got, want := endpointKeys(top.Actions), endpointKeys(syntaxFile.Actions); !equalKeys(got, want) {
				t.Fatalf("action mismatch for %s:\n got %v\nwant %v", name, got, want)
			}
			if got, want := endpointKeys(top.APIs), endpointKeys(syntaxFile.APIs); !equalKeys(got, want) {
				t.Fatalf("api mismatch for %s:\n got %v\nwant %v", name, got, want)
			}
		})
	}
}

// TestParseTopLevelRecoversPastError is the headline #306 capability: the
// recursive-descent parser surfaces declarations after a malformed line.
func TestParseTopLevelRecoversPastError(t *testing.T) {
	src := `package pages

import "fmt"

use widgets

route "/"
title "Home"

import alias "github.com/x/y"

view {
  <main></main>
}
`
	if _, err := parser.ParseSyntax([]byte(src)); err == nil {
		t.Fatal("expected the compiler parser to report the malformed use line")
	}

	top := ParseTopLevel(src)
	if top.Package == nil || top.Package.Name != "pages" {
		t.Fatalf("recovery lost the package declaration: %v", top.Package)
	}
	keys := importKeys(top.Imports)
	if !keys["\x00fmt"] || !keys["alias\x00github.com/x/y"] {
		t.Fatalf("recovery lost an import past the malformed line; got %v", keys)
	}
	// Metadata after the malformed line is recovered too.
	if got := metadataPairs(top.Metadata); !equalOrdered(got, []string{`route="/"`, `title="Home"`}) {
		t.Fatalf("recovery lost metadata past the malformed line; got %v", got)
	}
}

// TestParseTopLevelRejectsMalformedDeclarations checks the cutover parser does
// not emit nodes for declarations the compiler parser rejects: trailing tokens, an
// extra import identifier, a non-identifier use package, and a non-strict
// package name. Emitting these would let recovery surface invalid declarations
// as valid AST.
func TestParseTopLevelRejectsMalformedDeclarations(t *testing.T) {
	t.Run("package with trailing tokens", func(t *testing.T) {
		top := ParseTopLevel("package pages extra\n")
		if top.Package != nil {
			t.Fatalf("emitted a package for malformed declaration: %v", top.Package)
		}
		if _, err := parser.ParseSyntax([]byte("package pages extra\n")); err == nil {
			t.Fatal("compiler parser unexpectedly accepted the malformed package")
		}
	})

	t.Run("non-strict package name", func(t *testing.T) {
		if top := ParseTopLevel("package my.pkg\n"); top.Package != nil {
			t.Fatalf("emitted a package for non-strict name: %v", top.Package)
		}
	})

	t.Run("import with extra identifier", func(t *testing.T) {
		src := "package pages\nimport ui extra \"github.com/acme/ui\"\n"
		if top := ParseTopLevel(src); len(top.Imports) != 0 {
			t.Fatalf("emitted an import for malformed declaration: %v", top.Imports)
		}
		if _, err := parser.ParseSyntax([]byte(src)); err == nil {
			t.Fatal("compiler parser unexpectedly accepted the malformed import")
		}
	})

	t.Run("use with non-identifier package", func(t *testing.T) {
		src := "package pages\nuse widgets \"foo/bar\"\n"
		if top := ParseTopLevel(src); len(top.Uses) != 0 {
			t.Fatalf("emitted a use for malformed package string: %v", top.Uses)
		}
		if _, err := parser.ParseSyntax([]byte(src)); err == nil {
			t.Fatal("compiler parser unexpectedly accepted the malformed use")
		}
	})
}

// TestParseTopLevelRoutesTypedMetadata locks the three validation-free typed
// routings against the compiler parser: page and component carry the raw value as
// their identifier, cache strips surrounding quotes from its policy.
func TestParseTopLevelRoutesTypedMetadata(t *testing.T) {
	src := "package widgets\ncomponent Card\ncache \"no-store\"\n"
	syntaxFile, err := parser.ParseSyntax([]byte(src))
	if err != nil {
		t.Fatalf("compiler parser failed: %v", err)
	}
	top := ParseTopLevel(src)

	if syntaxFile.Component == nil || top.Component == nil || top.Component.Name != syntaxFile.Component.Name {
		t.Fatalf("component mismatch: got %v, compiler parser %v", top.Component, syntaxFile.Component)
	}
	if top.Component.Name != "Card" {
		t.Fatalf("component name = %q, want Card", top.Component.Name)
	}
	if syntaxFile.Cache == nil || top.Cache == nil || top.Cache.Policy != syntaxFile.Cache.Policy {
		t.Fatalf("cache mismatch: got %v, compiler parser %v", top.Cache, syntaxFile.Cache)
	}
	if top.Cache.Policy != "no-store" {
		t.Fatalf("cache policy = %q, want unquoted no-store", top.Cache.Policy)
	}
}

// TestParseTopLevelMatchesContractsOnComponent anchors the Go-typed contract
// recovery (store/props/state/wasm) against the compiler parser, including the
// go/parser-backed pkg.Type and pkg.NewFn() references.
func TestParseTopLevelMatchesContractsOnComponent(t *testing.T) {
	src := `package widgets

use cart "cart"

component Cart

state cart.State = cart.NewState()
props cart.Props

store Items cart.Items = cart.NewItems()

wasm "github.com/acme/cart/wasm"

view {
  <main></main>
}
`
	syntaxFile, err := parser.ParseSyntax([]byte(src))
	if err != nil {
		t.Fatalf("compiler parser failed: %v", err)
	}
	top := ParseTopLevel(src)

	if syntaxFile.State == nil || top.State == nil {
		t.Fatalf("state contract missing: got %v, compiler parser %v", top.State, syntaxFile.State)
	}
	if top.State.Type.Alias != syntaxFile.State.Type.Alias || top.State.Type.Name != syntaxFile.State.Type.Name {
		t.Fatalf("state type mismatch: got %+v, compiler parser %+v", top.State.Type, syntaxFile.State.Type)
	}
	if top.State.Init.Alias != syntaxFile.State.Init.Alias || top.State.Init.Name != syntaxFile.State.Init.Name {
		t.Fatalf("state init mismatch: got %+v, compiler parser %+v", top.State.Init, syntaxFile.State.Init)
	}
	if syntaxFile.PropsType == nil || top.PropsType == nil ||
		top.PropsType.Alias != syntaxFile.PropsType.Alias || top.PropsType.Name != syntaxFile.PropsType.Name {
		t.Fatalf("props type mismatch: got %v, compiler parser %v", top.PropsType, syntaxFile.PropsType)
	}
	if len(top.Stores) != len(syntaxFile.Stores) || len(top.Stores) != 1 {
		t.Fatalf("store count mismatch: got %d, compiler parser %d", len(top.Stores), len(syntaxFile.Stores))
	}
	if top.Stores[0].Name != syntaxFile.Stores[0].Name ||
		top.Stores[0].Type.Name != syntaxFile.Stores[0].Type.Name ||
		top.Stores[0].Init.Name != syntaxFile.Stores[0].Init.Name {
		t.Fatalf("store mismatch: got %+v, compiler parser %+v", top.Stores[0], syntaxFile.Stores[0])
	}
	if syntaxFile.WASM == nil || top.WASM == nil || top.WASM.Package != syntaxFile.WASM.Package {
		t.Fatalf("wasm mismatch: got %v, compiler parser %v", top.WASM, syntaxFile.WASM)
	}
}

// TestParseTopLevelRejectsMalformedContracts checks the go/parser-constrained
// recovery does not emit nodes for contract references the compiler parser does
// not accept: multi-segment selectors, generics, a constructor with arguments, props
// with an initializer, and state without one. The compiler parser handles these two
// ways — some it errors on (props-with-init, state-without-init), others its
// pattern simply ignores (multi-dot, generics, args) — so the equivalence is
// that neither parser emits a contract node, not that the compiler parser errors.
func TestParseTopLevelRejectsMalformedContracts(t *testing.T) {
	cases := map[string]string{
		"store init with args":   "package p\nstore X a.T = a.New(1)\n",
		"store multi-dot type":   "package p\nstore X a.b.C = a.New()\n",
		"props with initializer": "package p\nprops a.T = a.New()\n",
		"state without init":     "package p\nstate a.T\n",
		"generic type":           "package p\nstate a.T[int] = a.New()\n",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			top := ParseTopLevel(src)
			if len(top.Stores) != 0 || top.PropsType != nil || top.State != nil {
				t.Fatalf("emitted a contract node for malformed source: stores=%v props=%v state=%v", top.Stores, top.PropsType, top.State)
			}
			// Equivalence: where the compiler parser succeeds, it emits no such
			// contract either (where it errors, it definitionally has none).
			if syntaxFile, err := parser.ParseSyntax([]byte(src)); err == nil {
				if len(syntaxFile.Stores) != 0 || syntaxFile.PropsType != nil || syntaxFile.State != nil {
					t.Fatalf("compiler parser emitted a contract for %q", src)
				}
			}
		})
	}
}

func endpointKeys(endpoints []gwdkast.Endpoint) map[string]bool {
	keys := map[string]bool{}
	for _, e := range endpoints {
		keys[e.Kind+"\x00"+e.Name+"\x00"+e.Method+"\x00"+e.Route+"\x00"+e.ErrorPage] = true
	}
	return keys
}

// TestParseTopLevelMatchesEndpoints anchors act/api endpoint recovery against the
// compiler parser, including the optional error page.
func TestParseTopLevelMatchesEndpoints(t *testing.T) {
	src := `package pages

route "/"

act Submit POST "/submit"
act Save POST "/save" error "/oops.html"
api List GET "/api/list"
api Remove DELETE "/api/remove"

view {
  <main></main>
}
`
	syntaxFile, err := parser.ParseSyntax([]byte(src))
	if err != nil {
		t.Fatalf("compiler parser failed: %v", err)
	}
	top := ParseTopLevel(src)

	if got, want := endpointKeys(top.Actions), endpointKeys(syntaxFile.Actions); !equalKeys(got, want) {
		t.Fatalf("action mismatch:\n got %v\nwant %v", got, want)
	}
	if got, want := endpointKeys(top.APIs), endpointKeys(syntaxFile.APIs); !equalKeys(got, want) {
		t.Fatalf("api mismatch:\n got %v\nwant %v", got, want)
	}
	if len(top.Actions) != 2 || len(top.APIs) != 2 {
		t.Fatalf("expected 2 actions and 2 apis, got %d and %d", len(top.Actions), len(top.APIs))
	}
}

// TestParseTopLevelRejectsMalformedEndpoints checks recovery does not emit
// endpoints the compiler parser rejects: a non-exported handler name, an action with
// a non-POST method, an API with an unknown verb, and a bareword (unquoted)
// route. None should produce a node.
func TestParseTopLevelRejectsMalformedEndpoints(t *testing.T) {
	cases := map[string]string{
		"non-exported action": "package p\nact submit POST \"/x\"\n",
		"action non-POST":     "package p\nact Submit GET \"/x\"\n",
		"api unknown verb":    "package p\napi List FETCH \"/x\"\n",
		"unquoted route":      "package p\nact Submit POST /x\n",
		"non-exported api":    "package p\napi list GET \"/x\"\n",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			top := ParseTopLevel(src)
			if len(top.Actions) != 0 || len(top.APIs) != 0 {
				t.Fatalf("emitted an endpoint for malformed source: actions=%v apis=%v", top.Actions, top.APIs)
			}
			if syntaxFile, err := parser.ParseSyntax([]byte(src)); err == nil {
				if len(syntaxFile.Actions) != 0 || len(syntaxFile.APIs) != 0 {
					t.Fatalf("compiler parser emitted an endpoint for %q", src)
				}
			}
		})
	}
}

func equalKeys(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for key := range a {
		if !b[key] {
			return false
		}
	}
	return true
}

// TestParseTopLevelRejectsTrailingComments locks the equivalence fix for the
// shared tokenizer stripping // comments. The compiler parser requires eof() after
// every identifier-led declaration, so a trailing comment makes it emit nothing
// (it errors on package/import or simply drops the line for act). Recovery must
// match: the stripped comment must not let a phantom node through. Metadata is
// exempt because the compiler parser keeps the raw remainder, comment included.
func TestParseTopLevelRejectsTrailingComments(t *testing.T) {
	t.Run("package", func(t *testing.T) {
		if top := ParseTopLevel("package home // c\n"); top.Package != nil {
			t.Fatalf("emitted package for trailing-comment line: %#v", top.Package)
		}
	})
	t.Run("import", func(t *testing.T) {
		if top := ParseTopLevel("package p\nimport \"fmt\" // c\n"); len(top.Imports) != 0 {
			t.Fatalf("emitted import for trailing-comment line: %#v", top.Imports)
		}
	})
	t.Run("use", func(t *testing.T) {
		if top := ParseTopLevel("package p\nuse ui \"ui\" // c\n"); len(top.Uses) != 0 {
			t.Fatalf("emitted use for trailing-comment line: %#v", top.Uses)
		}
	})
	t.Run("store", func(t *testing.T) {
		// go/parser would silently ignore the comment in the initializer slice;
		// the guard rejects the line first, matching the compiler parser's eof().
		src := "package p\nstore Cart cart.Cart = cart.NewCart() // c\n"
		if top := ParseTopLevel(src); len(top.Stores) != 0 {
			t.Fatalf("emitted store for trailing-comment line: %#v", top.Stores)
		}
	})
	t.Run("endpoint anchored to compiler parser", func(t *testing.T) {
		// The compiler parser parses this successfully but emits no action; recovery
		// must agree rather than report a phantom endpoint.
		src := "package p\nact Submit POST \"/x\" // c\n"
		syntaxFile, err := parser.ParseSyntax([]byte(src))
		if err != nil {
			t.Fatalf("compiler parser failed: %v", err)
		}
		if len(syntaxFile.Actions) != 0 {
			t.Fatalf("precondition: compiler parser emitted an action: %#v", syntaxFile.Actions)
		}
		if top := ParseTopLevel(src); len(top.Actions) != 0 {
			t.Fatalf("emitted endpoint for trailing-comment line: %#v", top.Actions)
		}
	})
	t.Run("metadata keeps the comment in its raw value", func(t *testing.T) {
		src := "package p\nroute \"/\" // c\n"
		syntaxFile, err := parser.ParseSyntax([]byte(src))
		if err != nil {
			t.Fatalf("compiler parser failed: %v", err)
		}
		top := ParseTopLevel(src)
		if got, want := metadataPairs(top.Metadata), metadataPairs(syntaxFile.Metadata); !equalOrdered(got, want) {
			t.Fatalf("metadata mismatch:\n got %v\nwant %v", got, want)
		}
	})
}

// TestParseTopLevelDecodesStringEscapes locks the second equivalence fix: string
// values the compiler parser pulls through stringValue()/identString() are decoded
// (\t, \n, \", \\), so recovery must decode them too rather than keep the raw
// backslash escapes.
func TestParseTopLevelDecodesStringEscapes(t *testing.T) {
	src := "package p\nimport \"a\\tb\"\nact Tab POST \"/x\\ty\"\n"
	syntaxFile, err := parser.ParseSyntax([]byte(src))
	if err != nil {
		t.Fatalf("compiler parser failed: %v", err)
	}
	top := ParseTopLevel(src)

	if len(top.Actions) != 1 {
		t.Fatalf("expected one action, got %#v", top.Actions)
	}
	if top.Actions[0].Route != "/x\ty" {
		t.Fatalf("route = %q, want %q (a decoded tab)", top.Actions[0].Route, "/x\ty")
	}
	if len(syntaxFile.Actions) != 1 || top.Actions[0].Route != syntaxFile.Actions[0].Route {
		t.Fatalf("route diverged from compiler parser: got %q want %q", top.Actions[0].Route, syntaxFile.Actions[0].Route)
	}
	if len(top.Imports) != 1 || top.Imports[0].Path != "a\tb" {
		t.Fatalf("import path = %#v, want decoded \"a\\tb\"", top.Imports)
	}
	if len(syntaxFile.Imports) != 1 || top.Imports[0].Path != syntaxFile.Imports[0].Path {
		t.Fatalf("import path diverged from compiler parser: got %q want %q", top.Imports[0].Path, syntaxFile.Imports[0].Path)
	}
}
