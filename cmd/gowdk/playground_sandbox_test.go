package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateOutputDirRejectsRoot(t *testing.T) {
	if err := validateOutputDir("/"); err == nil {
		t.Fatal("expected the filesystem root to be rejected as an output directory")
	}
}

func TestValidateOutputDirRejectsNonEmpty(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "existing.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := validateOutputDir(dir)
	if err == nil || !strings.Contains(err.Error(), "must be empty") {
		t.Fatalf("expected non-empty output directory to be rejected, got %v", err)
	}
}

func TestValidateOutputDirAllowsFreshOrAbsent(t *testing.T) {
	dir := t.TempDir() // exists and is empty
	if err := validateOutputDir(dir); err != nil {
		t.Fatalf("expected empty directory to be allowed, got %v", err)
	}
	absent := filepath.Join(dir, "not-created-yet")
	if err := validateOutputDir(absent); err != nil {
		t.Fatalf("expected absent directory to be allowed, got %v", err)
	}
}

func TestResolveSandboxModuleCacheFailsClosed(t *testing.T) {
	// Neither a per-session cache nor the explicit shared opt-in: must refuse.
	_, err := resolveSandboxModuleCache(playgroundFileOptions{}, "/unused")
	if err == nil || !strings.Contains(err.Error(), "shared host module cache") {
		t.Fatalf("expected fail-closed without a module-cache choice, got %v", err)
	}
}

func TestResolveSandboxModuleCacheUsesProvidedDir(t *testing.T) {
	dir := t.TempDir()
	got, err := resolveSandboxModuleCache(playgroundFileOptions{ModuleCache: dir}, "/unused")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != dir {
		t.Fatalf("expected resolved cache %q, got %q", dir, got)
	}
}

func TestResolveSandboxModuleCacheRejectsNonDir(t *testing.T) {
	file := filepath.Join(t.TempDir(), "afile")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveSandboxModuleCache(playgroundFileOptions{ModuleCache: file}, "/unused"); err == nil {
		t.Fatal("expected a non-directory module cache to be rejected")
	}
}
