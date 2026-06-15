package otel

import (
	"context"
	"testing"
	"time"

	gowdktrace "github.com/cssbruno/gowdk/runtime/trace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestSinkRecordsSnapshotWithProvider(t *testing.T) {
	sink := NewSinkWithProvider(sdktrace.NewTracerProvider())
	now := time.Now().UTC()
	err := sink.RecordSpan(context.Background(), gowdktrace.Snapshot{
		TraceID:      gowdktrace.NewTraceID(),
		SpanID:       gowdktrace.NewSpanID(),
		ParentSpanID: gowdktrace.NewSpanID(),
		Name:         "unit",
		Surface:      gowdktrace.SurfaceBackend,
		Lane:         gowdktrace.LaneHandler,
		StartTime:    now,
		EndTime:      now.Add(time.Millisecond),
		Status:       gowdktrace.Status{Code: gowdktrace.StatusOK},
	})
	if err != nil {
		t.Fatalf("RecordSpan returned error: %v", err)
	}
	if err := sink.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown returned error: %v", err)
	}
}
