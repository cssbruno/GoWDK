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
	ctx := trace.ContextWithTraceContext(context.Background(), trace.TraceContext{
		TraceID:    traceID,
		SpanID:     spanID,
		Sampled:    true,
		TraceState: "rojo=00f067aa0ba902b7,congo=t61rcWkgMzE",
	})
	header := http.Header{}
	trace.Inject(ctx, header)
	if got := header.Get("traceparent"); got != "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01" {
		t.Fatalf("unexpected traceparent: %q", got)
	}
	if got := header.Get("tracestate"); got != "rojo=00f067aa0ba902b7,congo=t61rcWkgMzE" {
		t.Fatalf("unexpected tracestate: %q", got)
	}

	extracted := trace.Extract(context.Background(), header)
	traceContext, ok := trace.TraceContextFromContext(extracted)
	if !ok {
		t.Fatal("expected extracted trace context")
	}
	if traceContext.TraceID != traceID || traceContext.SpanID != spanID || !traceContext.Sampled || !traceContext.Remote || traceContext.TraceState != "rojo=00f067aa0ba902b7,congo=t61rcWkgMzE" {
		t.Fatalf("unexpected extracted context: %#v", traceContext)
	}
}

func TestTraceContextRejectsMalformedTraceparent(t *testing.T) {
	validTraceID := "4bf92f3577b34da6a3ce929d0e0e4736"
	validSpanID := "00f067aa0ba902b7"
	tests := []struct {
		name        string
		traceparent string
	}{
		{name: "malformed", traceparent: "not-a-traceparent"},
		{name: "zero trace id", traceparent: "00-00000000000000000000000000000000-" + validSpanID + "-01"},
		{name: "zero span id", traceparent: "00-" + validTraceID + "-0000000000000000-01"},
		{name: "uppercase trace id", traceparent: "00-4BF92F3577B34DA6A3CE929D0E0E4736-" + validSpanID + "-01"},
		{name: "uppercase flags", traceparent: "00-" + validTraceID + "-" + validSpanID + "-0A"},
		{name: "unsupported version", traceparent: "fe-" + validTraceID + "-" + validSpanID + "-01"},
		{name: "oversized", traceparent: strings.Repeat("0", trace.MaxTraceparentHeaderBytes+1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := trace.ParseTraceparent(tt.traceparent); err == nil {
				t.Fatal("expected ParseTraceparent to reject invalid input")
			}
			header := http.Header{"Traceparent": []string{tt.traceparent}}
			if _, ok := trace.TraceContextFromContext(trace.Extract(context.Background(), header)); ok {
				t.Fatal("invalid traceparent should not enter context")
			}
		})
	}
}

func TestTraceContextDropsInvalidTracestate(t *testing.T) {
	header := http.Header{}
	header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	header.Set("tracestate", "bad key=value")

	ctx := trace.Extract(context.Background(), header)
	traceContext, ok := trace.TraceContextFromContext(ctx)
	if !ok {
		t.Fatal("valid traceparent should still be extracted")
	}
	if traceContext.TraceState != "" {
		t.Fatalf("invalid tracestate should be dropped, got %q", traceContext.TraceState)
	}

	header.Set("tracestate", strings.Repeat("a", trace.MaxTracestateHeaderBytes+1))
	ctx = trace.Extract(context.Background(), header)
	traceContext, ok = trace.TraceContextFromContext(ctx)
	if !ok {
		t.Fatal("valid traceparent should survive oversized tracestate")
	}
	if traceContext.TraceState != "" {
		t.Fatalf("oversized tracestate should be dropped, got %q", traceContext.TraceState)
	}
}

func TestTracestatePropagatesThroughChildSpan(t *testing.T) {
	header := http.Header{}
	header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	header.Set("tracestate", "rojo=00f067aa0ba902b7")
	ctx := trace.Extract(context.Background(), header)

	tracer := trace.NewTracer()
	ctx, span := tracer.Start(ctx, "child")
	if span == nil {
		t.Fatal("expected sampled child span")
	}
	out := http.Header{}
	trace.Inject(ctx, out)
	if got := out.Get("tracestate"); got != "rojo=00f067aa0ba902b7" {
		t.Fatalf("child span lost tracestate: %q", got)
	}
}

func TestSlogAttrsExposeTraceAndSpanIDs(t *testing.T) {
	ctx := trace.ContextWithTraceContext(context.Background(), trace.TraceContext{
		TraceID: "4bf92f3577b34da6a3ce929d0e0e4736",
		SpanID:  "00f067aa0ba902b7",
	})

	attrs := trace.SlogAttrs(ctx)
	if len(attrs) != 2 {
		t.Fatalf("len(attrs) = %d, want 2: %#v", len(attrs), attrs)
	}
	if attrs[0].Key != trace.SlogTraceIDKey || attrs[0].Value.String() != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("unexpected trace attr: %#v", attrs[0])
	}
	if attrs[1].Key != trace.SlogSpanIDKey || attrs[1].Value.String() != "00f067aa0ba902b7" {
		t.Fatalf("unexpected span attr: %#v", attrs[1])
	}
	args := trace.SlogArgs(ctx)
	if len(args) != 4 || args[0] != trace.SlogTraceIDKey || args[2] != trace.SlogSpanIDKey {
		t.Fatalf("unexpected slog args: %#v", args)
	}
	if got := trace.SlogAttrs(context.Background()); got != nil {
		t.Fatalf("SlogAttrs without context = %#v, want nil", got)
	}
}

func TestRemoteSampledFlagDoesNotOverrideLocalSampler(t *testing.T) {
	header := http.Header{}
	header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	ctx := trace.Extract(context.Background(), header)

	tracer := trace.NewTracer(trace.WithSampler(trace.AlwaysOff()))
	next, span := tracer.Start(ctx, "sampled-remote")
	if span != nil {
		t.Fatal("remote sampled flag should not override local sampler")
	}
	if next != ctx {
		t.Fatal("sampled-out start should return the extracted context unchanged")
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
		Spans  []trace.Snapshot              `json:"spans"`
		Health trace.CollectorHealthSnapshot `json:"health"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Spans) != 1 || payload.Spans[0].Name != "collected" {
		t.Fatalf("unexpected collector json: %#v", payload)
	}
	if payload.Health.Spans != 1 || payload.Health.SSELimit == 0 || payload.Health.IngestRateLimit == 0 {
		t.Fatalf("unexpected collector health: %#v", payload.Health)
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
	handler.ServeHTTP(response, jsonPostRequest(http.MethodPost, "/", singlePayload))
	if response.Code != http.StatusNoContent {
		t.Fatalf("single POST status = %d body=%q, want 204", response.Code, response.Body.String())
	}

	batchPayload, err := json.Marshal([]trace.Snapshot{validSnapshot("batch-one"), validSnapshot("batch-two")})
	if err != nil {
		t.Fatal(err)
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, jsonPostRequest(http.MethodPost, "/", batchPayload))
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
			collector.Handler().ServeHTTP(response, jsonPostRequest(http.MethodPost, "/", payload))
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
			collector.Handler().ServeHTTP(response, jsonPostRequest(http.MethodPost, "/", []byte(tt.body)))
			if response.Code != tt.status {
				t.Fatalf("POST status = %d body=%q, want %d", response.Code, response.Body.String(), tt.status)
			}
			if got := collector.Spans(); len(got) != 0 {
				t.Fatalf("rejected payload recorded spans: %#v", got)
			}
		})
	}
}

func TestCollectorRejectsMissingOrNonJSONContentType(t *testing.T) {
	payload := []byte(snapshotJSON(t, validSnapshot("single")))
	tests := []struct {
		name        string
		contentType string
	}{
		{name: "missing"},
		{name: "plain text", contentType: "text/plain"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector := trace.NewCollector(4)
			request := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(payload))
			if tt.contentType != "" {
				request.Header.Set("Content-Type", tt.contentType)
			}
			response := httptest.NewRecorder()

			collector.Handler().ServeHTTP(response, request)

			if response.Code != http.StatusUnsupportedMediaType {
				t.Fatalf("POST status = %d body=%q, want 415", response.Code, response.Body.String())
			}
			if collector.Rejected() != 1 {
				t.Fatalf("collector rejected count = %d, want 1", collector.Rejected())
			}
		})
	}
}

func TestCollectorRejectsUnsupportedMethods(t *testing.T) {
	collector := trace.NewCollector(4)
	response := httptest.NewRecorder()

	collector.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/", nil))

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("method status = %d body=%q, want 405", response.Code, response.Body.String())
	}
	if allow := response.Header().Get("Allow"); allow != "GET, POST" {
		t.Fatalf("Allow = %q, want GET, POST", allow)
	}
	if collector.Rejected() != 1 {
		t.Fatalf("collector rejected count = %d, want 1", collector.Rejected())
	}
}

func TestCollectorRejectsCrossOriginBrowserIngest(t *testing.T) {
	collector := trace.NewCollector(4)
	payload := []byte(snapshotJSON(t, validSnapshot("browser")))
	request := jsonPostRequest(http.MethodPost, "http://trace.local/browser", payload)
	request.Header.Set("Origin", "http://evil.local")
	response := httptest.NewRecorder()

	collector.ViewerHandler().ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("browser ingest status = %d body=%q, want 403", response.Code, response.Body.String())
	}
	if len(collector.Spans()) != 0 {
		t.Fatalf("cross-origin request recorded spans: %#v", collector.Spans())
	}
}

func TestCollectorAcceptsSameOriginAndMissingOriginBrowserIngest(t *testing.T) {
	for _, tt := range []struct {
		name   string
		origin string
	}{
		{name: "same origin", origin: "http://trace.local"},
		{name: "missing origin"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			collector := trace.NewCollector(4)
			payload := []byte(snapshotJSON(t, validSnapshot("browser")))
			request := jsonPostRequest(http.MethodPost, "http://trace.local/browser", payload)
			if tt.origin != "" {
				request.Header.Set("Origin", tt.origin)
			}
			response := httptest.NewRecorder()

			collector.ViewerHandler().ServeHTTP(response, request)

			if response.Code != http.StatusNoContent {
				t.Fatalf("browser ingest status = %d body=%q, want 204", response.Code, response.Body.String())
			}
			if got := collector.Spans(); len(got) != 1 || got[0].Name != "browser" {
				t.Fatalf("browser ingest stored unexpected spans: %#v", got)
			}
		})
	}
}

func TestCollectorRateLimitsIngest(t *testing.T) {
	collector := trace.NewCollector(4, trace.WithCollectorIngestRate(1, time.Minute))
	handler := collector.Handler()
	payload := []byte(snapshotJSON(t, validSnapshot("limited")))

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, jsonPostRequest(http.MethodPost, "/", payload))
	if response.Code != http.StatusNoContent {
		t.Fatalf("first POST status = %d body=%q, want 204", response.Code, response.Body.String())
	}

	response = httptest.NewRecorder()
	handler.ServeHTTP(response, jsonPostRequest(http.MethodPost, "/", payload))
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("second POST status = %d body=%q, want 429", response.Code, response.Body.String())
	}
	if collector.Rejected() != 1 {
		t.Fatalf("collector rejected count = %d, want 1", collector.Rejected())
	}
}

func TestCollectorSSELimit(t *testing.T) {
	collector := trace.NewCollector(4, trace.WithCollectorSSELimit(1))
	if err := collector.RecordSpan(context.Background(), validSnapshot("existing")); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(collector.Handler())
	defer server.Close()

	first, err := server.Client().Get(server.URL + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer first.Body.Close()
	if first.StatusCode != http.StatusOK {
		t.Fatalf("first SSE status = %d, want 200", first.StatusCode)
	}

	second, err := server.Client().Get(server.URL + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer second.Body.Close()
	if second.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("second SSE status = %d, want 429", second.StatusCode)
	}
	if collector.Rejected() != 1 {
		t.Fatalf("collector rejected count = %d, want 1", collector.Rejected())
	}
}

func TestCollectorRejectedCounterIsExposedInJSON(t *testing.T) {
	collector := trace.NewCollector(4)
	collector.Handler().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`)))

	response := httptest.NewRecorder()
	collector.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("json status = %d body=%q, want 200", response.Code, response.Body.String())
	}
	var payload struct {
		Rejected uint64 `json:"rejected"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Rejected != 1 {
		t.Fatalf("json rejected = %d, want 1", payload.Rejected)
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

func TestTracerHealthSnapshotRecordsSamplingAndExports(t *testing.T) {
	ring := trace.NewRingSink(4)
	tracer := trace.NewTracer(
		trace.WithSink(ring),
		trace.WithSampler(trace.RatioSampler(0.5)),
		trace.WithIDGenerator(staticIDGenerator{
			traceID: "00000000000000000000000000000001",
			spanID:  "00f067aa0ba902b7",
		}),
	)
	_, span := tracer.Start(context.Background(), "health")
	if span == nil {
		t.Fatal("expected deterministic ratio sampler to sample valid trace")
	}
	span.EndTime(time.Unix(2, 0).UTC())

	health := waitForTracerHealth(t, tracer, func(health trace.TracerHealthSnapshot) bool {
		return health.ExportedSpans == 1
	})
	if health.Sampler != "ratio" || health.SamplingRatio != "0.5" || health.SampledSpans != 1 {
		t.Fatalf("unexpected sampling health: %#v", health)
	}
	if health.LastExportLatencyNS <= 0 || health.MaxExportLatencyNS <= 0 {
		t.Fatalf("expected export latency in health snapshot: %#v", health)
	}
}

func TestTracerHealthSnapshotRecordsExportFailure(t *testing.T) {
	logged := captureSinkLogs(t)
	tracer := trace.NewTracer(trace.WithSink(failingSink{err: errors.New("export failed")}))
	_, span := tracer.Start(context.Background(), "failed-export")
	span.EndTime(time.Unix(2, 0).UTC())

	health := waitForTracerHealth(t, tracer, func(health trace.TracerHealthSnapshot) bool {
		return health.ExportFailures == 1
	})
	if health.ExportedSpans != 0 || health.LastExportLatencyNS <= 0 {
		t.Fatalf("unexpected export failure health: %#v", health)
	}
	_ = waitForSinkLog(t, logged)
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

func jsonPostRequest(method string, target string, payload []byte) *http.Request {
	request := httptest.NewRequest(method, target, bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	return request
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

func waitForTracerHealth(t *testing.T, tracer *trace.Tracer, ready func(trace.TracerHealthSnapshot) bool) trace.TracerHealthSnapshot {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		health := tracer.HealthSnapshot()
		if ready(health) {
			return health
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for tracer health, last=%#v", health)
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
