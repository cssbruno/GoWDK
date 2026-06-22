package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeAuditRunModule creates a throwaway module whose ./gowdkapp package holds
// the given test file body, mirroring the layout runAuditTestCommand expects.
func writeAuditRunModule(t *testing.T, testBody string) string {
	t.Helper()
	dir := t.TempDir()
	writeAuditRunFile(t, filepath.Join(dir, "go.mod"), "module gowdkaudittest\n\ngo 1.26\n")
	pkgDir := filepath.Join(dir, "gowdkapp")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeAuditRunFile(t, filepath.Join(pkgDir, "app.go"), "package gowdkapp\n")
	writeAuditRunFile(t, filepath.Join(pkgDir, "app_test.go"), testBody)
	return dir
}

func writeAuditRunFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRunAuditTestCommandTimesOut(t *testing.T) {
	dir := writeAuditRunModule(t, `package gowdkapp

import (
	"testing"
	"time"
)

func TestSleeps(t *testing.T) { time.Sleep(60 * time.Second) }
`)
	options := defaultAuditRunOptions()
	options.Timeout = time.Second

	result := runAuditTestCommand(context.Background(), dir, options)
	if !result.TimedOut {
		t.Fatalf("expected timeout, got %#v", result)
	}
	if !strings.Contains(result.Output, "panic: test timed out") {
		t.Fatalf("expected Go test timeout output, got %q", result.Output)
	}
}

func TestRunAuditTestCommandReportsFailure(t *testing.T) {
	dir := writeAuditRunModule(t, `package gowdkapp

import "testing"

func TestFails(t *testing.T) { t.Fatal("intentional failure") }
`)
	result := runAuditTestCommand(context.Background(), dir, defaultAuditRunOptions())
	if result.TimedOut {
		t.Fatalf("did not expect timeout: %#v", result)
	}
	if result.ExitErr == nil {
		t.Fatalf("expected non-zero test exit, got %#v", result)
	}
	if !strings.Contains(result.Output, "intentional failure") {
		t.Fatalf("expected failure output captured, got %q", result.Output)
	}
}

func TestRunAuditTestCommandBoundsOutput(t *testing.T) {
	// The test fails so go test relays its (large) output back to the runner;
	// a passing test's output would be buffered and hidden.
	dir := writeAuditRunModule(t, `package gowdkapp

import (
	"strings"
	"testing"
)

func TestLoud(t *testing.T) {
	t.Log(strings.Repeat("x", 200000))
	t.Fatal("loud test fails so its output is relayed")
}
`)
	options := defaultAuditRunOptions()
	options.MaxOutputBytes = 4096

	result := runAuditTestCommand(context.Background(), dir, options)
	if !result.Truncated {
		t.Fatalf("expected truncation, got %#v", result.Truncated)
	}
	if len(result.Output) > options.MaxOutputBytes {
		t.Fatalf("output not bounded: len=%d limit=%d", len(result.Output), options.MaxOutputBytes)
	}
}

func TestRunAuditTestCommandSeedsAndMinimizesEnv(t *testing.T) {
	t.Setenv("MY_CLOUD_SECRET", "leak")
	dir := writeAuditRunModule(t, `package gowdkapp

import (
	"os"
	"testing"
)

func TestEnv(t *testing.T) {
	if os.Getenv("GOWDK_AUDIT_RUN") != "1" {
		t.Fatalf("missing seed GOWDK_AUDIT_RUN=%q", os.Getenv("GOWDK_AUDIT_RUN"))
	}
	if leaked := os.Getenv("MY_CLOUD_SECRET"); leaked != "" {
		t.Fatalf("secret leaked into test env: %q", leaked)
	}
}
`)
	result := runAuditTestCommand(context.Background(), dir, defaultAuditRunOptions())
	if result.TimedOut || result.ExitErr != nil {
		t.Fatalf("expected env test to pass, got %#v\noutput:\n%s", result, result.Output)
	}
}

func TestAuditTestEnvMinimizesAndSeeds(t *testing.T) {
	base := []string{
		"PATH=/usr/bin",
		"HOME=/home/u",
		"AWS_SECRET=shh",
		"GOWDK_CUSTOM=keep",
		"GOPROXY=https://proxy.example",
		"GOFLAGS=-tags=enterprise -mod=vendor -ldflags=-X=secret",
	}
	env := envMap(auditTestEnv(base, map[string]string{"GOWDK_AUDIT_RUN": "1"}))

	if env["PATH"] != "/usr/bin" || env["HOME"] != "/home/u" {
		t.Fatalf("required Go variables were dropped: %#v", env)
	}
	if env["GOWDK_CUSTOM"] != "keep" {
		t.Fatal("GOWDK_-prefixed variable should pass through")
	}
	if _, leaked := env["AWS_SECRET"]; leaked {
		t.Fatal("non-allowlisted secret should be dropped")
	}
	if env["GOPROXY"] != "off" || env["GOTOOLCHAIN"] != "local" || env["GOFLAGS"] != "-tags=enterprise -mod=mod" {
		t.Fatalf("download-disabling overrides missing: GOPROXY=%q GOTOOLCHAIN=%q GOFLAGS=%q", env["GOPROXY"], env["GOTOOLCHAIN"], env["GOFLAGS"])
	}
	if env["GOWDK_AUDIT_RUN"] != "1" {
		t.Fatal("explicit seed missing")
	}
}

func TestAuditSafeGOFlagsPreservesSeparateTagsValue(t *testing.T) {
	flags := auditSafeGOFlags([]string{"GOFLAGS=-tags enterprise,sqlite -trimpath"})
	if got := strings.Join(flags, " "); got != "-tags enterprise,sqlite" {
		t.Fatalf("safe GOFLAGS = %q, want -tags enterprise,sqlite", got)
	}
}

func TestBoundedBufferTruncates(t *testing.T) {
	buffer := &boundedBuffer{limit: 10}
	if n, _ := buffer.Write([]byte("hello")); n != 5 {
		t.Fatalf("short write reported n=%d", n)
	}
	if n, _ := buffer.Write([]byte("world!!!")); n != 8 {
		t.Fatalf("over-limit write must report full length, got n=%d", n)
	}
	if !buffer.Truncated() {
		t.Fatal("expected buffer to report truncation")
	}
	if got := buffer.String(); got != "helloworld" {
		t.Fatalf("bounded content = %q, want %q", got, "helloworld")
	}
}

func TestParseAuditCommandOptionsRunTimeout(t *testing.T) {
	options, _, err := parseAuditCommandOptions([]string{"--run", "--run-timeout=90s"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !options.RunTests || options.RunTimeout != 90*time.Second {
		t.Fatalf("unexpected options: %#v", options)
	}
	if _, _, err := parseAuditCommandOptions([]string{"--run-timeout=nonsense"}); err == nil {
		t.Fatal("expected error for invalid duration")
	}
	if _, _, err := parseAuditCommandOptions([]string{"--run-timeout=-5s"}); err == nil {
		t.Fatal("expected error for non-positive duration")
	}
}

func envMap(env []string) map[string]string {
	out := make(map[string]string, len(env))
	for _, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		out[key] = value
	}
	return out
}
