package playground

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCollectFilesExcludesGeneratedAndSecretFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "gowdk.config.go"), "package app\n")
	writeFile(t, filepath.Join(root, "src", "pages", "home.page.gwdk"), "package app\n")
	writeFile(t, filepath.Join(root, ".env"), "SECRET=value")
	writeFile(t, filepath.Join(root, ".gowdk", "app", "main.go"), "package main\n")
	writeFile(t, filepath.Join(root, "dist", "index.html"), "<html></html>")
	writeFile(t, filepath.Join(root, "secrets", "private.pem"), "secret")
	writeFile(t, filepath.Join(root, "api_token.json"), "{}")
	writeFile(t, filepath.Join(root, "config", "credentials.txt"), "secret")
	writeFile(t, filepath.Join(root, "src", "private_session.gwdk"), "package app\n")

	files, err := CollectFiles(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, file := range files {
		paths = append(paths, file.Path)
	}
	joined := strings.Join(paths, ",")
	for _, expected := range []string{"gowdk.config.go", "src/pages/home.page.gwdk"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("expected %s in collected files: %#v", expected, paths)
		}
	}
	for _, forbidden := range []string{".env", ".gowdk", "dist", "secrets", "api_token.json", "credentials.txt", "private_session.gwdk"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("did not expect %s in collected files: %#v", forbidden, paths)
		}
	}
}

func TestExportArchiveWritesNormalProjectFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "gowdk.config.go"), "package app\n")
	writeFile(t, filepath.Join(root, "src", "pages", "home.page.gwdk"), "package app\n")
	writeFile(t, filepath.Join(root, ".env.local"), "SECRET=value")
	archivePath := filepath.Join(t.TempDir(), "playground.zip")

	result, err := ExportArchive(root, archivePath, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Archive != archivePath || len(result.Files) != 2 {
		t.Fatalf("unexpected export result: %#v", result)
	}
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = reader.Close()
	}()
	var names []string
	for _, file := range reader.File {
		names = append(names, file.Name)
	}
	joined := strings.Join(names, ",")
	if !strings.Contains(joined, "gowdk.config.go") || !strings.Contains(joined, "src/pages/home.page.gwdk") {
		t.Fatalf("missing project files in archive: %#v", names)
	}
	if strings.Contains(joined, ".env") {
		t.Fatalf("archive included env file: %#v", names)
	}
}

func TestStageWorkspaceCopiesAllowedFilesOnly(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "gowdk.config.go"), "package app\n")
	writeFile(t, filepath.Join(root, "src", "pages", "home.page.gwdk"), "package app\n")
	writeFile(t, filepath.Join(root, "bin", "site"), "binary")

	workspace, cleanup, err := StageWorkspace(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = cleanup()
	}()
	if _, err := os.Stat(filepath.Join(workspace.Root, "src", "pages", "home.page.gwdk")); err != nil {
		t.Fatalf("expected staged source file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspace.Root, "bin", "site")); !os.IsNotExist(err) {
		t.Fatalf("generated bin should not be staged, err=%v", err)
	}
}

func TestRejectSecretEnvironment(t *testing.T) {
	if err := RejectSecretEnvironment([]string{"PATH=/bin", "GOWDK_TOKEN=secret"}); err == nil || !strings.Contains(err.Error(), "looks like a secret") {
		t.Fatalf("expected secret env rejection, got %v", err)
	}
	if err := RejectSecretEnvironment(SanitizedEnvironment(t.TempDir())); err != nil {
		t.Fatalf("sanitized env should pass: %v", err)
	}
}

func TestPolicyJSONDocumentsExecutionDisabledByDefault(t *testing.T) {
	policy := DefaultPolicy()
	if policy.HostedExecutionEnabled {
		t.Fatal("hosted execution must be disabled by default")
	}
	payload, err := PolicyJSON(policy)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), `"hostedExecutionEnabled": false`) || !strings.Contains(string(payload), "GOPROXY=off") {
		t.Fatalf("policy JSON missing sandbox details:\n%s", payload)
	}
}

func writeFile(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
