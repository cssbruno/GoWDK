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

// TestParseTopLevelMatchesLineParser anchors the recursive-descent declaration
// parser against the line-oriented parser: for valid source they must agree on
// the package name and the set of imports and uses.
func TestParseTopLevelMatchesLineParser(t *testing.T) {
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
		t.Fatalf("line parser failed: %v", err)
	}
	top := ParseTopLevel(src)

	if top.Package == nil || syntaxFile.Package == nil || top.Package.Name != syntaxFile.Package.Name {
		t.Fatalf("package mismatch: got %v, line parser %v", top.Package, syntaxFile.Package)
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
		t.Fatalf("page mismatch: got %v, line parser %v", top.Page, syntaxFile.Page)
	}
	if (top.Cache == nil) != (syntaxFile.Cache == nil) || (top.Cache != nil && top.Cache.Policy != syntaxFile.Cache.Policy) {
		t.Fatalf("cache mismatch: got %v, line parser %v", top.Cache, syntaxFile.Cache)
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
				t.Skipf("line parser rejects %s: %v", name, err)
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
				t.Fatalf("package mismatch for %s: got %q, line parser %q", name, topName, lineName)
			}
			if got, want := metadataPairs(top.Metadata), metadataPairs(syntaxFile.Metadata); !equalOrdered(got, want) {
				t.Fatalf("metadata mismatch for %s:\n got %v\nwant %v", name, got, want)
			}
		})
	}
}

// TestParseTopLevelRecoversPastError is the headline #306 capability: the
// recursive-descent parser surfaces declarations after a malformed line, where
// the line-oriented parser bails on the first error and returns nothing.
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
	// The line parser bails on the malformed use and returns nothing usable.
	if _, err := parser.ParseSyntax([]byte(src)); err == nil {
		t.Fatal("expected the line parser to bail on the malformed use line")
	}

	top := ParseTopLevel(src)
	if top.Package == nil || top.Package.Name != "pages" {
		t.Fatalf("recovery lost the package declaration: %v", top.Package)
	}
	keys := importKeys(top.Imports)
	if !keys["\x00fmt"] || !keys["alias\x00github.com/x/y"] {
		t.Fatalf("recovery lost an import past the malformed line; got %v", keys)
	}
	// Metadata after the malformed line is recovered too, where the line parser
	// surfaces nothing.
	if got := metadataPairs(top.Metadata); !equalOrdered(got, []string{`route="/"`, `title="Home"`}) {
		t.Fatalf("recovery lost metadata past the malformed line; got %v", got)
	}
}

// TestParseTopLevelRejectsMalformedDeclarations checks the cutover parser does
// not emit nodes for declarations the line parser rejects: trailing tokens, an
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
			t.Fatal("line parser unexpectedly accepted the malformed package")
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
			t.Fatal("line parser unexpectedly accepted the malformed import")
		}
	})

	t.Run("use with non-identifier package", func(t *testing.T) {
		src := "package pages\nuse widgets \"foo/bar\"\n"
		if top := ParseTopLevel(src); len(top.Uses) != 0 {
			t.Fatalf("emitted a use for malformed package string: %v", top.Uses)
		}
		if _, err := parser.ParseSyntax([]byte(src)); err == nil {
			t.Fatal("line parser unexpectedly accepted the malformed use")
		}
	})
}

// TestParseTopLevelRoutesTypedMetadata locks the three validation-free typed
// routings against the line parser: page and component carry the raw value as
// their identifier, cache strips surrounding quotes from its policy.
func TestParseTopLevelRoutesTypedMetadata(t *testing.T) {
	src := "package widgets\ncomponent Card\ncache \"no-store\"\n"
	syntaxFile, err := parser.ParseSyntax([]byte(src))
	if err != nil {
		t.Fatalf("line parser failed: %v", err)
	}
	top := ParseTopLevel(src)

	if syntaxFile.Component == nil || top.Component == nil || top.Component.Name != syntaxFile.Component.Name {
		t.Fatalf("component mismatch: got %v, line parser %v", top.Component, syntaxFile.Component)
	}
	if top.Component.Name != "Card" {
		t.Fatalf("component name = %q, want Card", top.Component.Name)
	}
	if syntaxFile.Cache == nil || top.Cache == nil || top.Cache.Policy != syntaxFile.Cache.Policy {
		t.Fatalf("cache mismatch: got %v, line parser %v", top.Cache, syntaxFile.Cache)
	}
	if top.Cache.Policy != "no-store" {
		t.Fatalf("cache policy = %q, want unquoted no-store", top.Cache.Policy)
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
