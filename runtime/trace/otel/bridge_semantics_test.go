package otel

import (
	"context"
	"testing"
	"time"

	gowdktrace "github.com/cssbruno/gowdk/runtime/trace"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// shutdownRecorder wraps a span processor and records whether Shutdown was
// called so ownership behavior can be asserted.
type shutdownRecorder struct {
	sdktrace.SpanProcessor
	shutdownCalled bool
}

func (recorder *shutdownRecorder) Shutdown(ctx context.Context) error {
	recorder.shutdownCalled = true
	return recorder.SpanProcessor.Shutdown(ctx)
}

func newRecorderProvider() (*sdktrace.TracerProvider, *tracetest.SpanRecorder, *shutdownRecorder) {
	recorder := tracetest.NewSpanRecorder()
	wrapper := &shutdownRecorder{SpanProcessor: recorder}
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(wrapper),
		sdktrace.WithIDGenerator(SnapshotIDGenerator{}),
	)
	return provider, recorder, wrapper
}

func attributeMap(keyValues []attribute.KeyValue) map[string]attribute.Value {
	out := make(map[string]attribute.Value, len(keyValues))
	for _, keyValue := range keyValues {
		out[string(keyValue.Key)] = keyValue.Value
	}
	return out
}

func semanticsSnapshot() gowdktrace.Snapshot {
	now := time.Unix(1, 0).UTC()
	return gowdktrace.Snapshot{
		TraceID: gowdktrace.NewTraceID(),
		SpanID:  gowdktrace.NewSpanID(),
		Name:    "GET /patients",
		Surface: gowdktrace.SurfaceBackend,
		Lane:    gowdktrace.LaneRoute,
		Source:  gowdktrace.SourceRef{File: "/abs/app/home.page.gwdk", Line: 3, Column: 1, OwnerKind: "page", OwnerID: "home"},
		Attributes: []gowdktrace.Attribute{
			{Key: "tags", Value: []string{"a", "b"}},
			{Key: "weird", Value: struct{}{}},
		},
		Events: []gowdktrace.Event{
			{Message: "loaded", Level: "warn", Time: now},
		},
		Status:    gowdktrace.Status{Code: gowdktrace.StatusOK},
		StartTime: now,
		EndTime:   now.Add(time.Millisecond),
	}
}

func TestRecordSpanPreservesSemantics(t *testing.T) {
	provider, recorder, _ := newRecorderProvider()
	sink := NewSinkWithProvider(provider, WithNativeIdentity())

	snapshot := semanticsSnapshot()
	beforeUnsupported := UnsupportedAttributeCount()
	if err := sink.RecordSpan(context.Background(), snapshot); err != nil {
		t.Fatalf("RecordSpan: %v", err)
	}

	ended := recorder.Ended()
	if len(ended) != 1 {
		t.Fatalf("ended spans = %d, want 1", len(ended))
	}
	span := ended[0]

	if span.SpanKind() != oteltrace.SpanKindServer {
		t.Fatalf("span kind = %v, want server", span.SpanKind())
	}
	if scope := span.InstrumentationScope(); scope.Name != instrumentationName || scope.Version != instrumentationVersion {
		t.Fatalf("instrumentation scope = %+v, want %s/%s", scope, instrumentationName, instrumentationVersion)
	}

	attrs := attributeMap(span.Attributes())
	if _, ok := attrs[attrGOWDKTraceID]; ok {
		t.Fatal("gowdk.trace_id must not be duplicated as an attribute when native identity is preserved")
	}
	if got := attrs[gowdktrace.AttrGOWDKSourceFile].AsString(); got != "abs/app/home.page.gwdk" {
		t.Fatalf("source file attribute not normalized: %q", got)
	}
	if got := attrs["tags"].AsStringSlice(); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("string slice attribute not preserved: %#v", got)
	}
	if _, ok := attrs["weird"]; ok {
		t.Fatal("unsupported attribute value should be dropped, not stringified")
	}
	if UnsupportedAttributeCount() <= beforeUnsupported {
		t.Fatal("unsupported attribute counter did not increase")
	}
	if dropped := attrs[AttrGOWDKDroppedAttributes].AsStringSlice(); len(dropped) != 1 || dropped[0] != "weird" {
		t.Fatalf("dropped-attribute marker = %#v, want [weird]", dropped)
	}

	events := span.Events()
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}
	if level := attributeMap(events[0].Attributes)[AttrGOWDKEventLevel].AsString(); level != "warn" {
		t.Fatalf("event level attribute = %q, want warn", level)
	}
}

func TestRecordSpanKindByLane(t *testing.T) {
	cases := []struct {
		lane gowdktrace.Lane
		want oteltrace.SpanKind
	}{
		{gowdktrace.LaneRoute, oteltrace.SpanKindServer},
		{gowdktrace.LaneAPI, oteltrace.SpanKindServer},
		{gowdktrace.LaneContract, oteltrace.SpanKindClient},
		{gowdktrace.LaneJob, oteltrace.SpanKindInternal},
		{gowdktrace.LaneNav, oteltrace.SpanKindInternal},
	}
	for _, testCase := range cases {
		provider, recorder, _ := newRecorderProvider()
		sink := NewSinkWithProvider(provider, WithNativeIdentity())
		snapshot := semanticsSnapshot()
		snapshot.Lane = testCase.lane
		if err := sink.RecordSpan(context.Background(), snapshot); err != nil {
			t.Fatalf("RecordSpan(%s): %v", testCase.lane, err)
		}
		if got := recorder.Ended()[0].SpanKind(); got != testCase.want {
			t.Fatalf("lane %s span kind = %v, want %v", testCase.lane, got, testCase.want)
		}
	}
}

func TestBorrowedProviderPreservesIDsAsAttributes(t *testing.T) {
	provider, recorder, _ := newRecorderProvider()
	sink := NewSinkWithProvider(provider) // borrowed default: native identity not asserted

	snapshot := semanticsSnapshot()
	snapshot.Attributes = nil
	if err := sink.RecordSpan(context.Background(), snapshot); err != nil {
		t.Fatalf("RecordSpan: %v", err)
	}
	span := recorder.Ended()[0]
	// Native identity still matches because the provider uses SnapshotIDGenerator...
	if got := span.SpanContext().TraceID().String(); got != string(snapshot.TraceID) {
		t.Fatalf("native trace id = %s, want %s", got, snapshot.TraceID)
	}
	// ...and the GOWDK IDs are also retained as attributes for borrowed providers.
	if got := attributeMap(span.Attributes())[attrGOWDKTraceID].AsString(); got != string(snapshot.TraceID) {
		t.Fatalf("gowdk.trace_id attribute = %q, want %q", got, snapshot.TraceID)
	}
}

func TestProviderOwnershipControlsShutdown(t *testing.T) {
	borrowedProvider, _, borrowedShutdown := newRecorderProvider()
	borrowed := NewSinkWithProvider(borrowedProvider)
	if err := borrowed.Shutdown(context.Background()); err != nil {
		t.Fatalf("borrowed Shutdown: %v", err)
	}
	if borrowedShutdown.shutdownCalled {
		t.Fatal("borrowed (app-owned) provider must not be shut down by the sink")
	}
	_ = borrowedProvider.Shutdown(context.Background())

	ownedProvider, _, ownedShutdown := newRecorderProvider()
	owned := NewSinkWithProvider(ownedProvider, WithProviderShutdown())
	if err := owned.Shutdown(context.Background()); err != nil {
		t.Fatalf("owned Shutdown: %v", err)
	}
	if !ownedShutdown.shutdownCalled {
		t.Fatal("owned provider must be shut down by the sink")
	}
}

func TestNilProviderIsOwnedAndIdentityPreserving(t *testing.T) {
	sink := NewSinkWithProvider(nil)
	if !sink.ownsProvider || !sink.preservesIdentity {
		t.Fatalf("nil provider should yield an owned, identity-preserving sink: %+v", sink)
	}
	if err := sink.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
}

func TestDefaultResourceHasServiceName(t *testing.T) {
	resource := defaultResource()
	for _, keyValue := range resource.Attributes() {
		if string(keyValue.Key) == "service.name" && keyValue.Value.AsString() == defaultServiceName {
			return
		}
	}
	t.Fatalf("default resource missing service.name=%q: %#v", defaultServiceName, resource.Attributes())
}
