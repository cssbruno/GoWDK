package trace_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cssbruno/gowdk/runtime/trace"
)

func TestStartRecordsSpanToSink(t *testing.T) {
	ring := trace.NewRingSink(4)
	tracer := trace.NewTracer(trace.WithSink(ring))
	start := time.Unix(10, 0).UTC()
	ctx, span := tracer.Start(context.Background(), "GET /patients",
		trace.WithSurface(trace.SurfaceBackend),
		trace.WithLane(trace.LaneRoute),
		trace.WithSource(trace.SourceRef{File: "patients.page.gwdk", Line: 3, Column: 1, OwnerKind: "page", OwnerID: "patients"}),
		trace.WithAttributes(map[string]any{trace.AttrHTTPRoute: "/patients"}),
		trace.WithStartTime(start),
	)
	if span == nil {
		t.Fatal("expected sampled span")
	}
	span.Set("app.patient_count", 2)
	span.Event("info", "loaded patients", map[string]any{"count": 2})
	span.SetStatus(trace.StatusOK, "")
	span.EndTime(start.Add(1500 * time.Millisecond))

	traceContext, ok := trace.TraceContextFromContext(ctx)
	if !ok || !traceContext.TraceID.Valid() || !traceContext.SpanID.Valid() {
		t.Fatalf("expected trace context in returned context, got %#v", traceContext)
	}
	spans := waitForSpans(t, ring, 1)
	if len(spans) != 1 {
		t.Fatalf("expected one span, got %d", len(spans))
	}
	got := spans[0]
	if got.Name != "GET /patients" || got.Surface != trace.SurfaceBackend || got.Lane != trace.LaneRoute {
		t.Fatalf("unexpected span identity: %#v", got)
	}
	if got.Source.File != "patients.page.gwdk" || got.DurationNS != int64(1500*time.Millisecond) {
		t.Fatalf("unexpected source/duration: %#v", got)
	}
	if got.Status.Code != trace.StatusOK || len(got.Events) != 1 {
		t.Fatalf("unexpected status/events: %#v", got)
	}
}

func TestTraceparentInjectExtractRoundTrip(t *testing.T) {
	traceID := trace.TraceID("4bf92f3577b34da6a3ce929d0e0e4736")
	spanID := trace.SpanID("00f067aa0ba902b7")
	ctx := trace.ContextWithTraceContext(context.Background(), trace.TraceContext{TraceID: traceID, SpanID: spanID, Sampled: true})
	header := http.Header{}
	trace.Inject(ctx, header)
	if got := header.Get("traceparent"); got != "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01" {
		t.Fatalf("unexpected traceparent: %q", got)
	}

	extracted := trace.Extract(context.Background(), header)
	traceContext, ok := trace.TraceContextFromContext(extracted)
	if !ok {
		t.Fatal("expected extracted trace context")
	}
	if traceContext.TraceID != traceID || traceContext.SpanID != spanID || !traceContext.Sampled || !traceContext.Remote {
		t.Fatalf("unexpected extracted context: %#v", traceContext)
	}
}

func TestRingSinkDropsOldest(t *testing.T) {
	ring := trace.NewRingSink(2)
	for _, name := range []string{"one", "two", "three"} {
		if err := ring.RecordSpan(context.Background(), trace.Snapshot{Name: name}); err != nil {
			t.Fatal(err)
		}
	}
	if ring.Dropped() != 1 {
		t.Fatalf("expected one dropped span, got %d", ring.Dropped())
	}
	spans := ring.Spans()
	if len(spans) != 2 || spans[0].Name != "two" || spans[1].Name != "three" {
		t.Fatalf("expected oldest span to drop, got %#v", spans)
	}
}

func TestJSONLAndConsoleSinks(t *testing.T) {
	span := trace.Snapshot{Name: "unit", TraceID: trace.NewTraceID(), SpanID: trace.NewSpanID(), Status: trace.Status{Code: trace.StatusOK}}
	var jsonl bytes.Buffer
	if err := trace.NewJSONLSink(&jsonl).RecordSpan(context.Background(), span); err != nil {
		t.Fatal(err)
	}
	var decoded trace.Snapshot
	if err := json.Unmarshal(bytes.TrimSpace(jsonl.Bytes()), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Name != "unit" {
		t.Fatalf("unexpected jsonl span: %#v", decoded)
	}

	var console bytes.Buffer
	if err := trace.NewConsoleSink(&console).RecordSpan(context.Background(), span); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(console.String(), "unit trace=") {
		t.Fatalf("unexpected console output: %q", console.String())
	}
}

func TestSpanEndDoesNotBlockOnSink(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	tracer := trace.NewTracer(trace.WithSink(blockingSink{entered: entered, release: release}))
	_, span := tracer.Start(context.Background(), "blocked-export")
	done := make(chan struct{})

	go func() {
		span.EndTime(time.Unix(1, 0).UTC())
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		close(release)
		t.Fatal("span end blocked on sink export")
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("sink was not invoked")
	}
	close(release)
}

func TestMultiSinkAttemptsLaterSinksAfterFailure(t *testing.T) {
	expected := errors.New("first sink failed")
	recording := &recordingSink{}
	sink := trace.MultiSink(failingSink{err: expected}, recording)

	err := sink.RecordSpan(context.Background(), trace.Snapshot{Name: "fanout"})
	if !errors.Is(err, expected) {
		t.Fatalf("MultiSink error = %v, want %v", err, expected)
	}
	if len(recording.spans) != 1 || recording.spans[0].Name != "fanout" {
		t.Fatalf("later sink did not receive span: %#v", recording.spans)
	}
}

func TestCollectorHandlerServesJSONAndSSE(t *testing.T) {
	collector := trace.NewCollector(4)
	span := trace.Snapshot{Name: "collected", TraceID: trace.NewTraceID(), SpanID: trace.NewSpanID()}
	if err := collector.RecordSpan(context.Background(), span); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	collector.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("unexpected json status %d", response.Code)
	}
	var payload struct {
		Spans   []trace.Snapshot `json:"spans"`
		Dropped uint64           `json:"dropped"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Spans) != 1 || payload.Spans[0].Name != "collected" {
		t.Fatalf("unexpected collector json: %#v", payload)
	}

	server := httptest.NewServer(collector.Handler())
	defer server.Close()
	resp, err := server.Client().Get(server.URL + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if contentType := resp.Header.Get("Content-Type"); !strings.Contains(contentType, "text/event-stream") {
		t.Fatalf("expected event stream, got %q", contentType)
	}
	scanner := bufio.NewScanner(resp.Body)
	var lines []string
	for scanner.Scan() {
		line := scanner.Text()
		lines = append(lines, line)
		if line == "" {
			break
		}
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "event: gowdk-trace") || !strings.Contains(joined, `"name":"collected"`) {
		t.Fatalf("unexpected sse payload:\n%s", joined)
	}
}

func TestExporterSinkReceivesOTLPShape(t *testing.T) {
	exporter := &recordingExporter{}
	span := trace.Snapshot{
		TraceID:   trace.NewTraceID(),
		SpanID:    trace.NewSpanID(),
		Name:      "exported",
		StartTime: time.Unix(1, 0).UTC(),
		EndTime:   time.Unix(2, 0).UTC(),
	}
	if err := trace.ExporterSink(exporter).RecordSpan(context.Background(), span); err != nil {
		t.Fatal(err)
	}
	if len(exporter.spans) != 1 || exporter.spans[0].Name != "exported" || exporter.spans[0].StartTimeUnixNano != span.StartTime.UnixNano() {
		t.Fatalf("unexpected exported spans: %#v", exporter.spans)
	}
}

func TestAlwaysOffSamplerDoesNotAllocateSpanOrContext(t *testing.T) {
	tracer := trace.NewTracer(trace.WithSampler(trace.AlwaysOff()))
	ctx := context.Background()
	allocs := testing.AllocsPerRun(1000, func() {
		next, span := tracer.Start(ctx, "sampled-out")
		if next != ctx {
			t.Fatal("sampled-out start should return original context")
		}
		if span != nil {
			t.Fatal("sampled-out start should not allocate a span")
		}
		span.Event("info", "ignored", nil)
		span.Set("ignored", true)
		span.End()
	})
	if allocs != 0 {
		t.Fatalf("expected sampled-out start to allocate 0 objects, got %.2f", allocs)
	}
}

func TestRejectedDynamicSamplerDoesNotReturnTracerContext(t *testing.T) {
	tracer := trace.NewTracer(trace.WithSampler(denySampler{}))
	ctx, span := tracer.Start(context.Background(), "sampled-out", trace.WithLane(trace.LaneRoute))
	if span != nil {
		t.Fatal("rejected sampler should not allocate a span")
	}
	if _, ok := trace.TracerFromContext(ctx); ok {
		t.Fatal("rejected sampler returned a context carrying a tracer")
	}
}

func BenchmarkStartAlwaysOff(b *testing.B) {
	tracer := trace.NewTracer(trace.WithSampler(trace.AlwaysOff()))
	ctx := context.Background()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, span := tracer.Start(ctx, "sampled-out")
		span.End()
	}
}

type denySampler struct{}

func (denySampler) Sample(trace.SamplingContext) bool {
	return false
}

type recordingExporter struct {
	spans []trace.OTLPSpan
}

type blockingSink struct {
	entered chan<- struct{}
	release <-chan struct{}
}

func (sink blockingSink) RecordSpan(ctx context.Context, span trace.Snapshot) error {
	close(sink.entered)
	select {
	case <-sink.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type failingSink struct {
	err error
}

func (sink failingSink) RecordSpan(context.Context, trace.Snapshot) error {
	return sink.err
}

type recordingSink struct {
	spans []trace.Snapshot
}

func (sink *recordingSink) RecordSpan(ctx context.Context, span trace.Snapshot) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	sink.spans = append(sink.spans, span)
	return nil
}

func waitForSpans(t *testing.T, ring *trace.RingSink, want int) []trace.Snapshot {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		spans := ring.Spans()
		if len(spans) >= want {
			return spans
		}
		if time.Now().After(deadline) {
			return spans
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func (exporter *recordingExporter) ExportSpans(ctx context.Context, spans []trace.OTLPSpan) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	exporter.spans = append(exporter.spans, spans...)
	return nil
}
