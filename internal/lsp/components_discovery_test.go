package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/discover"
)

func TestConfiguredComponentDiscoveryMatchesSelectionAndOpenOverlays(t *testing.T) {
	root := t.TempDir()
	config := gowdk.Config{
		Source: gowdk.SourceConfig{
			Include: []string{"src/**/*.gwdk"},
			Exclude: []string{"src/excluded/**"},
		},
		Build: gowdk.BuildConfig{Output: "dist"},
	}
	pagePath := writeLSPDiscoverySource(t, root, "src/pages/home.page.gwdk", `package app

page home
route "/"

view {
  <main></main>
}
`)
	allowedPath := writeLSPDiscoverySource(t, root, "src/components/allowed.cmp.gwdk", `package app

component Allowed

view {
  <section></section>
}
`)
	excludedPath := writeLSPDiscoverySource(t, root, "src/excluded/hidden.cmp.gwdk", `package app

component Hidden

view {
  <section></section>
}
`)
	outputPath := writeLSPDiscoverySource(t, root, "dist/generated.cmp.gwdk", `package app

component Generated

view {
  <section></section>
}
`)

	pageDoc := document{URI: fileURI(pagePath), Path: pagePath, Version: 1, Text: mustReadLSPDiscoverySource(t, pagePath)}
	server := NewProjectServer(config, ProjectOptions{Root: root})
	server.log = nil
	server.documents[pageDoc.URI] = pageDoc
	server.documents[fileURI(allowedPath)] = document{
		URI:     fileURI(allowedPath),
		Path:    allowedPath,
		Version: 2,
		Text: `package app


component RenamedWhileOpen

view {
  <section></section>
}
`,
	}
	server.documents[fileURI(excludedPath)] = document{
		URI:     fileURI(excludedPath),
		Path:    excludedPath,
		Version: 2,
		Text:    strings.ReplaceAll(mustReadLSPDiscoverySource(t, excludedPath), "Hidden", "LeakedWhileOpen"),
	}
	unsavedPath := filepath.Join(root, "src", "components", "unsaved.cmp.gwdk")
	server.documents[fileURI(unsavedPath)] = document{
		URI:     fileURI(unsavedPath),
		Path:    unsavedPath,
		Version: 1,
		Text: `package app

component Unsaved

view {
  <section></section>
}
`,
	}

	selection, err := discover.ConfiguredSelection(config, config.Build.Output, nil, root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := selection.Files()
	if err != nil {
		t.Fatal(err)
	}
	if !containsCleanPath(files, pagePath) || !containsCleanPath(files, allowedPath) {
		t.Fatalf("configured discovery missed included sources: %#v", files)
	}
	if containsCleanPath(files, excludedPath) || containsCleanPath(files, outputPath) {
		t.Fatalf("configured discovery leaked excluded/generated sources: %#v", files)
	}

	definitions := server.componentDefinitions(pageDoc)
	for _, name := range []string{"RenamedWhileOpen", "Unsaved"} {
		if _, ok := definitions[componentDefinitionKey("app", name)]; !ok {
			t.Fatalf("expected %q definition, got %#v", name, definitions)
		}
	}
	for _, name := range []string{"Allowed", "Hidden", "LeakedWhileOpen", "Generated"} {
		if _, ok := definitions[componentDefinitionKey("app", name)]; ok {
			t.Fatalf("excluded or stale component %q leaked into definitions", name)
		}
	}
	if _, ok := server.resolveComponentDefinition(pageDoc, "RenamedWhileOpen"); !ok {
		t.Fatal("expected definition lookup to use the dirty open component")
	}
	if _, ok := server.resolveComponentDefinition(pageDoc, "LeakedWhileOpen"); ok {
		t.Fatal("excluded open component leaked into definition lookup")
	}

	completions := server.projectCompletions(pageDoc.URI)
	for _, name := range []string{"RenamedWhileOpen", "Unsaved"} {
		if !hasItemLabel(completions, name) {
			t.Fatalf("expected %q completion, got %#v", name, completions)
		}
	}
	for _, name := range []string{"Allowed", "Hidden", "LeakedWhileOpen", "Generated"} {
		if hasItemLabel(completions, name) {
			t.Fatalf("excluded or stale component %q leaked into completion", name)
		}
	}

	if server.workspaceComponentCache.selectionFingerprint == "" {
		t.Fatal("expected configured selection in the workspace cache identity")
	}
	for _, dir := range server.workspaceComponentCache.dirs {
		if pathWithinLSPDiscoveryRoot(dir, filepath.Join(root, "dist")) {
			t.Fatalf("generated output directory was scanned or cached: %q", dir)
		}
	}
	cacheKey := server.workspaceComponentCache.key
	if err := os.WriteFile(outputPath, []byte(strings.ReplaceAll(mustReadLSPDiscoverySource(t, outputPath), "Generated", "GeneratedChanged")), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = server.componentDefinitions(pageDoc)
	if server.workspaceComponentCache.key != cacheKey {
		t.Fatalf("excluded output change invalidated component cache: %q -> %q", cacheKey, server.workspaceComponentCache.key)
	}
}

func TestConfiguredComponentDiscoveryHonorsSelectedModules(t *testing.T) {
	root := t.TempDir()
	config := gowdk.Config{
		Modules: []gowdk.ModuleConfig{
			{Name: "admin"},
			{Name: "public"},
		},
	}
	pagePath := writeLSPDiscoverySource(t, root, "admin/home.page.gwdk", `package app

page home
route "/"

view {
  <main></main>
}
`)
	writeLSPDiscoverySource(t, root, "admin/admin-card.cmp.gwdk", `package app

component AdminCard

view {
  <section></section>
}
`)
	publicPath := writeLSPDiscoverySource(t, root, "public/public-card.cmp.gwdk", `package app

component PublicCard

view {
  <section></section>
}
`)
	pageDoc := document{URI: fileURI(pagePath), Path: pagePath, Version: 1, Text: mustReadLSPDiscoverySource(t, pagePath)}
	server := NewProjectServer(config, ProjectOptions{Root: root, Modules: []string{"admin"}})
	server.log = nil
	server.documents[pageDoc.URI] = pageDoc
	server.documents[fileURI(publicPath)] = document{
		URI:     fileURI(publicPath),
		Path:    publicPath,
		Version: 2,
		Text:    strings.ReplaceAll(mustReadLSPDiscoverySource(t, publicPath), "PublicCard", "OpenPublicCard"),
	}

	definitions := server.componentDefinitions(pageDoc)
	if _, ok := definitions[componentDefinitionKey("app", "AdminCard")]; !ok {
		t.Fatalf("selected module component missing: %#v", definitions)
	}
	for _, name := range []string{"PublicCard", "OpenPublicCard"} {
		if _, ok := definitions[componentDefinitionKey("app", name)]; ok {
			t.Fatalf("unselected module component %q leaked into definitions", name)
		}
	}
	completions := server.projectCompletions(pageDoc.URI)
	if !hasItemLabel(completions, "AdminCard") {
		t.Fatalf("selected module completion missing: %#v", completions)
	}
	if hasItemLabel(completions, "PublicCard") || hasItemLabel(completions, "OpenPublicCard") {
		t.Fatalf("unselected module leaked into completions: %#v", completions)
	}
}

func TestConfiguredComponentCompletionsRespectPackageVisibility(t *testing.T) {
	root := t.TempDir()
	pagePath := writeLSPDiscoverySource(t, root, "pages/home.page.gwdk", `package app
use ui "design"

page home
route "/"

view {
  <main></main>
}
`)
	writeLSPDiscoverySource(t, root, "components/local.cmp.gwdk", `package app

component LocalCard

view {
  <section></section>
}
`)
	writeLSPDiscoverySource(t, root, "components/button.cmp.gwdk", `package design

component Button

view {
  <button></button>
}
`)
	writeLSPDiscoverySource(t, root, "components/private.cmp.gwdk", `package private

component PrivateCard

view {
  <section></section>
}
`)
	pageDoc := document{URI: fileURI(pagePath), Path: pagePath, Version: 1, Text: mustReadLSPDiscoverySource(t, pagePath)}
	server := NewProjectServer(gowdk.Config{}, ProjectOptions{Root: root})
	server.log = nil
	server.documents[pageDoc.URI] = pageDoc

	completions := server.projectCompletions(pageDoc.URI)
	for _, label := range []string{"LocalCard", "ui.Button"} {
		if !hasItemLabel(completions, label) {
			t.Fatalf("expected visible component completion %q, got %#v", label, completions)
		}
	}
	for _, label := range []string{"Button", "PrivateCard", "private.PrivateCard"} {
		if hasItemLabel(completions, label) {
			t.Fatalf("unresolvable component completion %q leaked: %#v", label, completions)
		}
	}
}

func TestConfiguredComponentCompletionsSurviveMalformedCurrentPage(t *testing.T) {
	root := t.TempDir()
	writeLSPDiscoverySource(t, root, "components/local.cmp.gwdk", `package app

component LocalCard

view {
  <section></section>
}
`)
	writeLSPDiscoverySource(t, root, "components/button.cmp.gwdk", `package design

component Button

view {
  <button></button>
}
`)
	pagePath := filepath.Join(root, "pages", "home.page.gwdk")
	for name, source := range map[string]string{
		"parser error": `package app
use ui "design"

page home
route "/"

view {
  <main>
`,
		"lexer error": `package app
use ui "design"

page home
route "/"

view {
  <main title="unfinished></main>
}
`,
	} {
		t.Run(name, func(t *testing.T) {
			pageDoc := document{URI: fileURI(pagePath), Path: pagePath, Version: 1, Text: source}
			server := NewProjectServer(gowdk.Config{}, ProjectOptions{Root: root})
			server.log = nil
			server.documents[pageDoc.URI] = pageDoc

			completions := server.projectCompletions(pageDoc.URI)
			for _, label := range []string{"LocalCard", "ui.Button"} {
				if !hasItemLabel(completions, label) {
					t.Fatalf("expected %q completion for malformed page, got %#v", label, completions)
				}
			}
		})
	}
}

func writeLSPDiscoverySource(t *testing.T, root, relative, contents string) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func mustReadLSPDiscoverySource(t *testing.T, path string) string {
	t.Helper()
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(payload)
}

func containsCleanPath(paths []string, want string) bool {
	want = filepath.Clean(want)
	for _, path := range paths {
		if filepath.Clean(path) == want {
			return true
		}
	}
	return false
}

func pathWithinLSPDiscoveryRoot(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && (rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))))
}
