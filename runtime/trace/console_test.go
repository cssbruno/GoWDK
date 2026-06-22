package trace_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk/runtime/trace"
)

func TestConsoleSinkEscapesControlCharacters(t *testing.T) {
	span := trace.Snapshot{
		Name:    "GET /p\n[forged] injected=line",
		TraceID: trace.NewTraceID(),
		SpanID:  trace.NewSpanID(),
		Surface: trace.SurfaceBackend,
		Lane:    trace.LaneRoute,
		Status:  trace.Status{Code: trace.StatusError, Message: "boom\r\nlevel=fatal"},
		Source:  trace.SourceRef{File: "pages/\x1b[31mhome.gwdk", Line: 2},
		Events: []trace.Event{
			{Message: "loaded\tpatients\nfake=event", Level: "info"},
		},
	}

	var buffer bytes.Buffer
	if err := trace.NewConsoleSink(&buffer).RecordSpan(context.Background(), span); err != nil {
		t.Fatal(err)
	}
	out := buffer.String()

	if count := strings.Count(out, "\n"); count != 1 {
		t.Fatalf("console output forged extra log lines (%d newlines): %q", count, out)
	}
	if !strings.HasSuffix(out, "\n") {
		t.Fatalf("console output is not newline-terminated: %q", out)
	}
	for _, forbidden := range []string{"\r", "\x1b", "\t"} {
		if strings.ContainsRune(out, []rune(forbidden)[0]) {
			t.Fatalf("console output contains a raw control character %q: %q", forbidden, out)
		}
	}
	// Span name, status message, source field, and event message must all be
	// present but escaped.
	for _, want := range []string{`GET /p\n[forged]`, `boom\r\nlevel=fatal`, `\x1b[31mhome.gwdk`, `loaded\tpatients\nfake=event`} {
		if !strings.Contains(out, want) {
			t.Fatalf("console output missing escaped field %q in %q", want, out)
		}
	}
}

func TestConsoleSinkKeepsNormalSpansReadable(t *testing.T) {
	span := trace.Snapshot{
		Name:    "GET /patients",
		TraceID: trace.NewTraceID(),
		SpanID:  trace.NewSpanID(),
		Status:  trace.Status{Code: trace.StatusOK},
	}
	var buffer bytes.Buffer
	if err := trace.NewConsoleSink(&buffer).RecordSpan(context.Background(), span); err != nil {
		t.Fatal(err)
	}
	if got := buffer.String(); !strings.HasPrefix(got, "GET /patients trace=") {
		t.Fatalf("normal span lost its readable form: %q", got)
	}
}
