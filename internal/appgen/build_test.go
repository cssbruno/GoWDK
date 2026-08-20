package appgen

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPackagingIsExplicitNonMutatingAndAtomic(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test helper is a POSIX shell script")
	}
	root := t.TempDir()
	fakeBin := filepath.Join(root, "bin")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatal(err)
	}
	goPath := filepath.Join(fakeBin, "go")
	script := `#!/bin/sh
if [ "$1" = "env" ]; then
  printf '{"GOVERSION":"go-test","GOOS":"linux","GOARCH":"amd64","CGO_ENABLED":"0"}\n'
  exit 0
fi
printf '%s\n' "$@" > "$FAKE_GO_ARGS"
if [ "$FAKE_GO_FAIL" = "1" ]; then
  printf 'intentional build failure\n' >&2
  exit 1
fi
previous=""
for argument in "$@"; do
  if [ "$previous" = "-o" ]; then
    printf 'stable artifact bytes\n' > "$argument"
    exit 0
  fi
  previous="$argument"
done
exit 2
`
	if err := os.WriteFile(goPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	appDir := filepath.Join(root, "generated")
	if err := os.MkdirAll(filepath.Join(appDir, "cmd", "server"), 0o755); err != nil {
		t.Fatal(err)
	}
	goModPath := filepath.Join(appDir, "go.mod")
	goSumPath := filepath.Join(appDir, "go.sum")
	goMod := []byte("module example.com/generated\n\ngo 1.26.4\n")
	goSum := []byte("sentinel sum\n")
	if err := os.WriteFile(goModPath, goMod, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(goSumPath, goSum, 0o644); err != nil {
		t.Fatal(err)
	}
	argsPath := filepath.Join(root, "args.txt")
	artifactPath := filepath.Join(root, "site")
	environment := []string{
		"PATH=" + fakeBin,
		"FAKE_GO_ARGS=" + argsPath,
		"GOFLAGS=-tags=ambient -ldflags=-X=unstable",
	}
	result, err := BuildBinaryWithOptions(appDir, artifactPath, PackagingOptions{
		Environment: environment,
		Tags:        []string{"sqlite", "enterprise", "sqlite"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Path != artifactPath {
		t.Fatalf("path = %q, want %q", result.Path, artifactPath)
	}
	args, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"-trimpath", "-buildvcs=false", "-mod=readonly", "-tags=enterprise,sqlite"} {
		if !strings.Contains(string(args), expected) {
			t.Fatalf("go build args missing %q:\n%s", expected, args)
		}
	}
	if strings.Contains(string(args), "ambient") || strings.Contains(string(args), "unstable") {
		t.Fatalf("ambient GOFLAGS leaked into explicit build args:\n%s", args)
	}
	payload, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(payload)
	if result.Metadata.ArtifactSHA256 != hex.EncodeToString(digest[:]) {
		t.Fatalf("artifact hash = %q", result.Metadata.ArtifactSHA256)
	}
	if result.Metadata.GoVersion != "go-test" || result.Metadata.ModuleMode != "readonly" || !result.Metadata.Trimpath || result.Metadata.BuildVCS {
		t.Fatalf("unexpected packaging metadata: %#v", result.Metadata)
	}
	assertFileBytes(t, goModPath, goMod)
	assertFileBytes(t, goSumPath, goSum)

	previous := []byte("previous release\n")
	if err := os.WriteFile(artifactPath, previous, 0o755); err != nil {
		t.Fatal(err)
	}
	failingEnvironment := append(append([]string(nil), environment...), "FAKE_GO_FAIL=1")
	if _, err := BuildBinaryWithOptions(appDir, artifactPath, PackagingOptions{Environment: failingEnvironment}); err == nil || !strings.Contains(err.Error(), "intentional build failure") {
		t.Fatalf("expected intentional failure, got %v", err)
	}
	assertFileBytes(t, artifactPath, previous)
	assertFileBytes(t, goModPath, goMod)
	assertFileBytes(t, goSumPath, goSum)
}

func TestPackagingHashIsIndependentOfProjectPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test helper is a POSIX shell script")
	}
	root := t.TempDir()
	fakeBin := filepath.Join(root, "bin")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatal(err)
	}
	goPath := filepath.Join(fakeBin, "go")
	script := `#!/bin/sh
if [ "$1" = "env" ]; then
  printf '{"GOVERSION":"go-test","GOOS":"linux","GOARCH":"amd64","CGO_ENABLED":"0"}\n'
  exit 0
fi
previous=""
for argument in "$@"; do
  if [ "$previous" = "-o" ]; then
    printf 'same content for every root\n' > "$argument"
    exit 0
  fi
  previous="$argument"
done
exit 2
`
	if err := os.WriteFile(goPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	environment := []string{"PATH=" + fakeBin}
	var hashes []string
	for _, name := range []string{"first/deep/root", "second/root"} {
		appDir := filepath.Join(root, name, "app")
		if err := os.MkdirAll(filepath.Join(appDir, "cmd", "server"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(appDir, "go.mod"), []byte("module example.com/generated\n\ngo 1.26.4\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		result, err := BuildBinaryWithOptions(appDir, filepath.Join(root, name, "site"), PackagingOptions{Environment: environment})
		if err != nil {
			t.Fatal(err)
		}
		hashes = append(hashes, result.Metadata.ArtifactSHA256)
	}
	if hashes[0] != hashes[1] {
		t.Fatalf("path-independent hashes differ: %q != %q", hashes[0], hashes[1])
	}
}

func assertFileBytes(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("%s changed:\ngot:  %q\nwant: %q", path, got, want)
	}
}
