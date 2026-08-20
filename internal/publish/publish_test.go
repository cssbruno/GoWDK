package publish

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTransactionCommitsDirectoryGenerationAndRemovesStaleFiles(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "output")
	writeTestFile(t, filepath.Join(target, "old.txt"), "old")

	var tx Transaction
	stage, err := tx.StageDirectory(target)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Abort()
	writeTestFile(t, filepath.Join(stage, "new.txt"), "new")
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(target, "old.txt")); !os.IsNotExist(err) {
		t.Fatalf("stale file survived publication: %v", err)
	}
	if got := readTestFile(t, filepath.Join(target, "new.txt")); got != "new" {
		t.Fatalf("new generation = %q", got)
	}
}

func TestRecoverChoosesNewestBackupDeterministically(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "output")
	older := filepath.Join(root, ".output.gowdk-backup-z")
	newer := filepath.Join(root, ".output.gowdk-backup-a")
	writeTestFile(t, filepath.Join(older, "value.txt"), "older")
	writeTestFile(t, filepath.Join(newer, "value.txt"), "newer")
	now := time.Now()
	if err := os.Chtimes(older, now.Add(-time.Hour), now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newer, now, now); err != nil {
		t.Fatal(err)
	}
	if err := Recover(target); err != nil {
		t.Fatal(err)
	}
	if got := readTestFile(t, filepath.Join(target, "value.txt")); got != "newer" {
		t.Fatalf("restored contents = %q", got)
	}
}

func TestTransactionRollsBackEarlierTargetWhenLaterStageIsMissing(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "first")
	second := filepath.Join(root, "second")
	writeTestFile(t, filepath.Join(first, "value.txt"), "old-first")
	writeTestFile(t, filepath.Join(second, "value.txt"), "old-second")

	var tx Transaction
	firstStage, err := tx.StageDirectory(first)
	if err != nil {
		t.Fatal(err)
	}
	secondStage, err := tx.StageDirectory(second)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Abort()
	writeTestFile(t, filepath.Join(firstStage, "value.txt"), "new-first")
	writeTestFile(t, filepath.Join(secondStage, "value.txt"), "new-second")
	if err := os.RemoveAll(secondStage); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err == nil {
		t.Fatal("Commit() = nil, want missing-stage error")
	}
	if got := readTestFile(t, filepath.Join(first, "value.txt")); got != "old-first" {
		t.Fatalf("first target was not rolled back: %q", got)
	}
	if got := readTestFile(t, filepath.Join(second, "value.txt")); got != "old-second" {
		t.Fatalf("second target changed: %q", got)
	}
}

func TestRecoverRestoresInterruptedBackupAndRemovesStage(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "output")
	backup := filepath.Join(root, ".output.gowdk-backup-interrupted")
	stage := filepath.Join(root, ".output.gowdk-stage-interrupted")
	writeTestFile(t, filepath.Join(backup, "value.txt"), "old")
	writeTestFile(t, filepath.Join(stage, "value.txt"), "partial")
	if err := Recover(target); err != nil {
		t.Fatal(err)
	}
	if got := readTestFile(t, filepath.Join(target, "value.txt")); got != "old" {
		t.Fatalf("restored contents = %q", got)
	}
	if _, err := os.Stat(stage); !os.IsNotExist(err) {
		t.Fatalf("stale stage survived recovery: %v", err)
	}
}

func writeTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(payload)
}
