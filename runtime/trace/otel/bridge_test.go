package otel

import (
	"context"
	"testing"
	"time"

	gowdktrace "github.com/cssbruno/gowdk/runtime/trace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestSinkRecordsSnapshotWithProvider(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	sink := NewSinkWithProvider(sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(recorder),
		sdktrace.WithIDGenerator(SnapshotIDGenerator{}),
	))
	now := time.Now().UTC()
	snapshot := gowdktrace.Snapshot{
		TraceID:      gowdktrace.NewTraceID(),
		SpanID:       gowdktrace.NewSpanID(),
		ParentSpanID: gowdktrace.NewSpanID(),
		Name:         "unit",
		Surface:      gowdktrace.SurfaceBackend,
		Lane:         gowdktrace.LaneHandler,
		StartTime:    now,
		EndTime:      now.Add(time.Millisecond),
		Status:       gowdktrace.Status{Code: gowdktrace.StatusOK},
	}
	err := sink.RecordSpan(context.Background(), snapshot)
	if err != nil {
		t.Fatalf("RecordSpan returned error: %v", err)
	}
	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("ended spans = %d, want 1", len(spans))
	}
	if got := spans[0].SpanContext().TraceID().String(); got != string(snapshot.TraceID) {
		t.Fatalf("TraceID = %s, want %s", got, snapshot.TraceID)
	}
	if got := spans[0].SpanContext().SpanID().String(); got != string(snapshot.SpanID) {
		t.Fatalf("SpanID = %s, want %s", got, snapshot.SpanID)
	}
	if got := spans[0].Parent().SpanID().String(); got != string(snapshot.ParentSpanID) {
		t.Fatalf("parent SpanID = %s, want %s", got, snapshot.ParentSpanID)
	}
	if err := sink.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown returned error: %v", err)
	}
}
