package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSiteRootRequiresGeneratedIndex(t *testing.T) {
	if _, err := siteRoot(t.TempDir()); err == nil {
		t.Fatal("expected missing generated site output error")
	}
}

func TestSiteRootUsesGeneratedOutputDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<!doctype html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := siteRoot(dir); err != nil {
		t.Fatalf("expected generated site output to be accepted: %v", err)
	}
}
