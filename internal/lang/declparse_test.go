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

// TestParseTopLevelMatchesLineParser anchors the recursive-descent declaration
// parser against the line-oriented parser: for valid source they must agree on
// the package name and the set of imports and uses.
func TestParseTopLevelMatchesLineParser(t *testing.T) {
	src := `package pages

import "fmt"
import alias "github.com/x/y"

use widgets "components"
use forms "forms"

route "/"
title "Home"

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
