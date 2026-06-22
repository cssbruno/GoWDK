package trace_test

import (
	"context"
	"testing"
	"time"

	"github.com/cssbruno/gowdk/runtime/trace"
)

func TestNormalizeSourceFileRelativeMode(t *testing.T) {
	cases := []struct {
		name   string
		file   string
		policy trace.SourcePolicy
		want   string
	}{
		{name: "empty", file: "", want: ""},
		{name: "relative unchanged", file: "pages/home.page.gwdk", want: "pages/home.page.gwdk"},
		{name: "absolute posix stripped", file: "/Users/me/app/home.page.gwdk", want: "Users/me/app/home.page.gwdk"},
		{name: "under project root", file: "/Users/me/app/home.page.gwdk", policy: trace.SourcePolicy{ProjectRoot: "/Users/me/app"}, want: "home.page.gwdk"},
		{name: "windows drive absolute", file: `C:\Users\me\app\home.page.gwdk`, want: "Users/me/app/home.page.gwdk"},
		{name: "windows relative", file: `pages\home.page.gwdk`, want: "pages/home.page.gwdk"},
		{name: "unc path", file: `\\host\share\app\home.page.gwdk`, want: "host/share/app/home.page.gwdk"},
		{name: "traversal collapsed", file: "../../etc/passwd", want: "etc/passwd"},
		{name: "leading dot slash", file: "./pages/home.page.gwdk", want: "pages/home.page.gwdk"},
		{name: "generated absolute source", file: "/tmp/gowdk-build/output/home.page.go", want: "tmp/gowdk-build/output/home.page.go"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := trace.NormalizeSourceFile(testCase.file, testCase.policy)
			if got != testCase.want {
				t.Fatalf("NormalizeSourceFile(%q) = %q, want %q", testCase.file, got, testCase.want)
			}
		})
	}
}

func TestNormalizeSourceFileAbsoluteModeKeepsPath(t *testing.T) {
	policy := trace.SourcePolicy{Mode: trace.SourcePathAbsolute}
	const absolute = "/Users/me/app/home.page.gwdk"
	if got := trace.NormalizeSourceFile(absolute, policy); got != absolute {
		t.Fatalf("absolute mode = %q, want %q preserved", got, absolute)
	}
}

func TestSnapshotNormalizesAbsoluteSourcePath(t *testing.T) {
	ring := trace.NewRingSink(1)
	tracer := trace.NewTracer(trace.WithSink(ring))
	_, span := tracer.Start(context.Background(), "unit",
		trace.WithSource(trace.SourceRef{File: "/Users/me/secret/app/home.page.gwdk", Line: 3}))
	if span == nil {
		t.Fatal("expected sampled span")
	}
	span.EndTime(time.Unix(1, 0).UTC())

	spans := waitForSpans(t, ring, 1)
	if len(spans) != 1 {
		t.Fatalf("expected one span, got %d", len(spans))
	}
	if got := spans[0].Source.File; got != "Users/me/secret/app/home.page.gwdk" {
		t.Fatalf("completed-span source not normalized: %q", got)
	}
}

func TestCollectorNormalizesIngestedSource(t *testing.T) {
	collector := trace.NewCollector(1)
	if err := collector.RecordSpan(context.Background(), trace.Snapshot{
		TraceID: trace.NewTraceID(),
		SpanID:  trace.NewSpanID(),
		Name:    "ingested",
		Source:  trace.SourceRef{File: "/abs/host/path/file.gwdk"},
	}); err != nil {
		t.Fatal(err)
	}
	spans := collector.Spans()
	if len(spans) != 1 || spans[0].Source.File != "abs/host/path/file.gwdk" {
		t.Fatalf("collector did not normalize ingested source: %#v", spans)
	}
}
