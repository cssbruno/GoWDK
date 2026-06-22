package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const (
	// defaultAuditRunTimeout bounds how long `gowdk audit --run` lets the
	// generated app test process run before it is killed. Override it with
	// --run-timeout=<duration>.
	defaultAuditRunTimeout = 2 * time.Minute
	// defaultAuditRunOutputLimit bounds the combined stdout/stderr captured from
	// the test process so a runaway test cannot overwhelm CI logs or memory.
	defaultAuditRunOutputLimit = 1 << 20 // 1 MiB
	// auditRunTruncationMarker is printed after bounded output when the test
	// process produced more than the limit, so truncation is explicit.
	auditRunTruncationMarker = "... [gowdk audit] test output truncated at the configured limit ..."
	// auditRunCommandGrace lets the generated test binary handle Go's own
	// -timeout panic before the parent go command is killed by context.
	auditRunCommandGrace = 5 * time.Second
)

// auditRunOptions configures the execution boundary for `gowdk audit --run`.
type auditRunOptions struct {
	Timeout        time.Duration
	MaxOutputBytes int
	// Seeds are environment variables explicitly seeded into the otherwise
	// minimized test environment (for example a deterministic audit-run marker).
	Seeds map[string]string
}

func defaultAuditRunOptions() auditRunOptions {
	return auditRunOptions{
		Timeout:        defaultAuditRunTimeout,
		MaxOutputBytes: defaultAuditRunOutputLimit,
		Seeds:          map[string]string{"GOWDK_AUDIT_RUN": "1"},
	}
}

// auditRunResult reports the outcome of a single audit test execution. Timeout,
// process failure, and output truncation are distinct so callers can report
// them distinctly.
type auditRunResult struct {
	Output    string
	Truncated bool
	TimedOut  bool
	ExitErr   error
}

// runAuditTestCommand runs `go test` for the generated audit app under a strict
// execution boundary: a deadline (exec.CommandContext), a sanitized environment
// with dependency downloads disabled, and a bounded combined output buffer.
func runAuditTestCommand(ctx context.Context, dir string, options auditRunOptions) auditRunResult {
	if options.Timeout <= 0 {
		options.Timeout = defaultAuditRunTimeout
	}
	if options.MaxOutputBytes <= 0 {
		options.MaxOutputBytes = defaultAuditRunOutputLimit
	}
	runCtx, cancel := context.WithTimeout(ctx, options.Timeout+auditRunCommandGrace)
	defer cancel()

	output := &boundedBuffer{limit: options.MaxOutputBytes}
	command := exec.CommandContext(runCtx, "go", "test", "-count=1", "-timeout="+options.Timeout.String(), "./gowdkapp")
	command.Dir = dir
	command.Env = auditTestEnv(os.Environ(), options.Seeds)
	command.Stdout = output
	command.Stderr = output

	err := command.Run()
	result := auditRunResult{Output: output.String(), Truncated: output.Truncated()}
	if runCtx.Err() == context.DeadlineExceeded || auditRunOutputTimedOut(result.Output) {
		result.TimedOut = true
		return result
	}
	result.ExitErr = err
	return result
}

func auditRunOutputTimedOut(output string) bool {
	return strings.Contains(output, "panic: test timed out")
}

// auditTestEnv builds a minimized environment for the audit test process. It
// keeps only the Go toolchain and GOWDK variables required to compile and run
// the generated app, drops everything else so local secrets are not leaked into
// the test process, and forces dependency downloads and toolchain fetches off so
// an audit run cannot reach the network.
func auditTestEnv(base []string, seeds map[string]string) []string {
	allowed := map[string]bool{
		"PATH": true, "HOME": true,
		"GOROOT": true, "GOPATH": true, "GOCACHE": true,
		"GOMODCACHE": true, "GOTMPDIR": true,
		"CGO_ENABLED": true, "GOOS": true, "GOARCH": true,
		// Windows toolchain and cache discovery.
		"SYSTEMROOT": true, "SYSTEMDRIVE": true, "TEMP": true, "TMP": true,
		"USERPROFILE": true, "LOCALAPPDATA": true, "APPDATA": true,
	}
	env := make([]string, 0, len(allowed)+len(seeds)+6)
	for _, entry := range base {
		key, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		upper := strings.ToUpper(key)
		if allowed[upper] || strings.HasPrefix(upper, "GOWDK_") {
			env = append(env, entry)
		}
	}
	// Deterministic overrides: resolve modules from the local cache only and
	// never fetch from the network, so an audit run is hermetic.
	env = append(env,
		"GOPROXY=off",
		"GOSUMDB=off",
		"GOTOOLCHAIN=local",
		"GOWORK=off",
		"GOFLAGS="+auditRunGOFlags(base),
	)
	for key, value := range seeds {
		env = append(env, key+"="+value)
	}
	return env
}

func auditRunGOFlags(base []string) string {
	flags := auditSafeGOFlags(base)
	flags = append(flags, "-mod=mod")
	return strings.Join(flags, " ")
}

func auditSafeGOFlags(base []string) []string {
	for _, entry := range base {
		key, value, ok := strings.Cut(entry, "=")
		if !ok || !strings.EqualFold(key, "GOFLAGS") {
			continue
		}
		fields := strings.Fields(value)
		flags := make([]string, 0, len(fields))
		for index := 0; index < len(fields); index++ {
			field := fields[index]
			switch {
			case field == "-tags" || field == "--tags":
				if index+1 < len(fields) {
					flags = append(flags, field, fields[index+1])
					index++
				}
			case strings.HasPrefix(field, "-tags="), strings.HasPrefix(field, "--tags="):
				flags = append(flags, field)
			}
		}
		return flags
	}
	return nil
}

// boundedBuffer captures up to limit bytes of combined output and records
// whether more was produced. Writes past the limit are discarded but reported
// as fully written so the child process is not killed by a short write.
type boundedBuffer struct {
	mu        sync.Mutex
	buffer    bytes.Buffer
	limit     int
	truncated bool
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if remaining := b.limit - b.buffer.Len(); remaining > 0 {
		if len(p) > remaining {
			b.buffer.Write(p[:remaining])
			b.truncated = true
		} else {
			b.buffer.Write(p)
		}
	} else if len(p) > 0 {
		b.truncated = true
	}
	return len(p), nil
}

func (b *boundedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

func (b *boundedBuffer) Truncated() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.truncated
}
