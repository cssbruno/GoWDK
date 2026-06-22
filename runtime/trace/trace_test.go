package trace_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

func TestRingSinkDropsOldestWhenByteBudgetExceeded(t *testing.T) {
	ring := trace.NewRingSink(100)
	attrs := make([]trace.Attribute, 20)
	for index := range attrs {
		attrs[index] = trace.Attribute{
			Key:   fmt.Sprintf("attr.%02d", index),
			Value: strings.Repeat("x", 1800),
		}
	}
	for index := range 50 {
		if err := ring.RecordSpan(context.Background(), trace.Snapshot{
			TraceID:    trace.NewTraceID(),
			SpanID:     trace.NewSpanID(),
			Name:       fmt.Sprintf("span-%02d", index),
			Attributes: attrs,
		}); err != nil {
			t.Fatal(err)
		}
	}
	spans := ring.Spans()
	if ring.Dropped() == 0 || len(spans) >= 50 {
		t.Fatalf("expected byte budget to drop oldest spans, dropped=%d spans=%d", ring.Dropped(), len(spans))
	}
	if spans[0].Name == "span-00" || spans[len(spans)-1].Name != "span-49" {
		t.Fatalf("expected newest byte-bounded window, got first=%q last=%q", spans[0].Name, spans[len(spans)-1].Name)
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

func TestSpanEndRecoversPanickingSinkAndRedactsLog(t *testing.T) {
	logged := captureSinkLogs(t)
	tracer := trace.NewTracer(trace.WithSink(panickingSink{value: "token=super-secret-token-value"}))
	_, span := tracer.Start(context.Background(), "panic-export")

	span.EndTime(time.Unix(1, 0).UTC())

	message := waitForSinkLog(t, logged)
	if !strings.Contains(message, "gowdk trace: sink failed: panic:") {
		t.Fatalf("unexpected sink panic log: %q", message)
	}
	if strings.Contains(message, "super-secret-token-value") || !strings.Contains(message, "[REDACTED]") {
		t.Fatalf("sink panic log was not redacted: %q", message)
	}
}

func TestSpanEndRedactsSinkFailureLog(t *testing.T) {
	logged := captureSinkLogs(t)
	tracer := trace.NewTracer(trace.WithSink(failingSink{err: errors.New("export failed: Authorization: Bearer abcdefghijklmnop")}))
	_, span := tracer.Start(context.Background(), "failed-export")

	span.EndTime(time.Unix(1, 0).UTC())

	message := waitForSinkLog(t, logged)
	if strings.Contains(message, "abcdefghijklmnop") || !strings.Contains(message, "Bearer [REDACTED]") {
		t.Fatalf("sink failure log was not redacted: %q", message)
	}
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

func TestCollectorAcceptsValidSingleAndBatchPayloads(t *testing.T) {
	collector := trace.NewCollector(4)
	handler := collector.Handler()

	singlePayload, err := json.Marshal(validSnapshot("single"))
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(singlePayload)))
	if response.Code != http.StatusNoContent {
		t.Fatalf("single POST status = %d body=%q, want 204", response.Code, response.Body.String())
	}

	batchPayload, err := json.Marshal([]trace.Snapshot{validSnapshot("batch-one"), validSnapshot("batch-two")})
	if err != nil {
		t.Fatal(err)
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(batchPayload)))
	if response.Code != http.StatusNoContent {
		t.Fatalf("batch POST status = %d body=%q, want 204", response.Code, response.Body.String())
	}
	if got := collector.Spans(); len(got) != 3 {
		t.Fatalf("collector stored %d spans, want 3: %#v", len(got), got)
	}
}

func TestCollectorRejectsInvalidBatchWithoutPartialRecord(t *testing.T) {
	tests := []struct {
		name  string
		spans []trace.Snapshot
	}{
		{name: "invalid first", spans: []trace.Snapshot{invalidSnapshot("bad"), validSnapshot("ok")}},
		{name: "invalid middle", spans: []trace.Snapshot{validSnapshot("ok-1"), invalidSnapshot("bad"), validSnapshot("ok-2")}},
		{name: "invalid last", spans: []trace.Snapshot{validSnapshot("ok"), invalidSnapshot("bad")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector := trace.NewCollector(4)
			payload, err := json.Marshal(tt.spans)
			if err != nil {
				t.Fatal(err)
			}
			response := httptest.NewRecorder()
			collector.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(payload)))
			if response.Code != http.StatusBadRequest {
				t.Fatalf("POST status = %d body=%q, want 400", response.Code, response.Body.String())
			}
			if got := collector.Spans(); len(got) != 0 {
				t.Fatalf("invalid batch partially recorded spans: %#v", got)
			}
		})
	}
}

func TestCollectorRejectsAmbiguousOrOversizedPayloads(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		status int
	}{
		{name: "oversized body", body: strings.Repeat("x", 1<<20+1), status: http.StatusRequestEntityTooLarge},
		{name: "trailing json", body: snapshotJSON(t, validSnapshot("single")) + `{}`, status: http.StatusBadRequest},
		{name: "empty batch", body: `[]`, status: http.StatusBadRequest},
		{name: "unknown field", body: `{"traceId":"` + string(trace.NewTraceID()) + `","spanId":"` + string(trace.NewSpanID()) + `","unknown":true}`, status: http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector := trace.NewCollector(4)
			response := httptest.NewRecorder()
			collector.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.body)))
			if response.Code != tt.status {
				t.Fatalf("POST status = %d body=%q, want %d", response.Code, response.Body.String(), tt.status)
			}
			if got := collector.Spans(); len(got) != 0 {
				t.Fatalf("rejected payload recorded spans: %#v", got)
			}
		})
	}
}

func TestSpanSnapshotsAndRingResultsDeepCopyAttributes(t *testing.T) {
	ring := trace.NewRingSink(2)
	tracer := trace.NewTracer(trace.WithSink(ring))
	tags := []string{"original"}
	eventTags := []string{"event-original"}
	_, span := tracer.Start(context.Background(), "immutable",
		trace.WithAttributes(map[string]any{
			"tags":        tags,
			"unsupported": map[string]string{"kept": "no"},
		}),
	)
	span.Event("info", "loaded", map[string]any{"event_tags": eventTags})
	tags[0] = "caller-mutated"
	eventTags[0] = "event-caller-mutated"

	snapshot := span.Snapshot()
	if got := stringAttributeSlice(t, snapshot.Attributes, "tags"); got[0] != "original" {
		t.Fatalf("snapshot retained caller-owned attribute slice: %#v", got)
	}
	if hasAttribute(snapshot.Attributes, "unsupported") {
		t.Fatalf("snapshot retained unsupported map attribute: %#v", snapshot.Attributes)
	}
	if got := stringAttributeSlice(t, snapshot.Events[0].Attributes, "event_tags"); got[0] != "event-original" {
		t.Fatalf("snapshot retained caller-owned event attribute slice: %#v", got)
	}

	stringAttributeSlice(t, snapshot.Attributes, "tags")[0] = "snapshot-mutated"
	if got := stringAttributeSlice(t, span.Snapshot().Attributes, "tags"); got[0] != "original" {
		t.Fatalf("span snapshot reused returned attribute slice: %#v", got)
	}

	if err := ring.RecordSpan(context.Background(), span.Snapshot()); err != nil {
		t.Fatal(err)
	}
	spans := ring.Spans()
	stringAttributeSlice(t, spans[0].Events[0].Attributes, "event_tags")[0] = "ring-result-mutated"
	if got := stringAttributeSlice(t, ring.Spans()[0].Events[0].Attributes, "event_tags"); got[0] != "event-original" {
		t.Fatalf("ring reused returned event attribute slice: %#v", got)
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

func validSnapshot(name string) trace.Snapshot {
	return trace.Snapshot{
		TraceID:   trace.NewTraceID(),
		SpanID:    trace.NewSpanID(),
		Name:      name,
		StartTime: time.Unix(1, 0).UTC(),
		EndTime:   time.Unix(2, 0).UTC(),
		Attributes: []trace.Attribute{{
			Key:   "component",
			Value: "test",
		}},
	}
}

func invalidSnapshot(name string) trace.Snapshot {
	span := validSnapshot(name)
	span.TraceID = "not-valid"
	return span
}

func snapshotJSON(t *testing.T, span trace.Snapshot) string {
	t.Helper()
	payload, err := json.Marshal(span)
	if err != nil {
		t.Fatal(err)
	}
	return string(payload)
}

func stringAttributeSlice(t *testing.T, attrs []trace.Attribute, key string) []string {
	t.Helper()
	for _, attr := range attrs {
		if attr.Key != key {
			continue
		}
		values, ok := attr.Value.([]string)
		if !ok {
			t.Fatalf("attribute %q = %#v, want []string", key, attr.Value)
		}
		return values
	}
	t.Fatalf("attribute %q not found in %#v", key, attrs)
	return nil
}

func hasAttribute(attrs []trace.Attribute, key string) bool {
	for _, attr := range attrs {
		if attr.Key == key {
			return true
		}
	}
	return false
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

type panickingSink struct {
	value any
}

func (sink panickingSink) RecordSpan(context.Context, trace.Snapshot) error {
	panic(sink.value)
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

func captureSinkLogs(t *testing.T) <-chan string {
	t.Helper()
	logged := make(chan string, 1)
	previous := trace.SinkLogger
	trace.SinkLogger = func(message string) {
		logged <- message
	}
	t.Cleanup(func() {
		trace.SinkLogger = previous
	})
	return logged
}

func waitForSinkLog(t *testing.T, logged <-chan string) string {
	t.Helper()
	select {
	case message := <-logged:
		return message
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for sink log")
		return ""
	}
}

func (exporter *recordingExporter) ExportSpans(ctx context.Context, spans []trace.OTLPSpan) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	exporter.spans = append(exporter.spans, spans...)
	return nil
}
