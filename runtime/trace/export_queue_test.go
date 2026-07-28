package trace_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cssbruno/gowdk/runtime/trace"
)

func TestTracerExportQueuePreservesOrderAndFlushes(t *testing.T) {
	sink := &recordingSink{}
	tracer := trace.NewTracer(
		trace.WithSink(sink),
		trace.WithExportQueueSize(4),
	)
	for _, name := range []string{"first", "second", "third"} {
		endNamedSpan(t, tracer, name)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := tracer.Flush(ctx); err != nil {
		t.Fatal(err)
	}

	if len(sink.spans) != 3 {
		t.Fatalf("expected three exported spans, got %#v", sink.spans)
	}
	for index, name := range []string{"first", "second", "third"} {
		if sink.spans[index].Name != name {
			t.Fatalf("export order = %#v", sink.spans)
		}
	}
	health := tracer.HealthSnapshot()
	if health.ExportedSpans != 3 || health.ExportFailures != 0 || health.ExportDroppedSpans != 0 {
		t.Fatalf("unexpected export health: %#v", health)
	}
	if health.ExportQueueDepth != 0 || health.ExportQueueCapacity != 4 || health.ExportInFlight {
		t.Fatalf("unexpected drained queue health: %#v", health)
	}
}

func TestTracerExportQueueCountsTimeout(t *testing.T) {
	logged := captureSinkLogs(t)
	sink := &deadlineSink{entered: make(chan struct{})}
	tracer := trace.NewTracer(
		trace.WithSink(sink),
		trace.WithExportTimeout(10*time.Millisecond),
	)
	endNamedSpan(t, tracer, "timeout")

	select {
	case <-sink.entered:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for sink")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := tracer.Flush(ctx); err != nil {
		t.Fatal(err)
	}

	health := tracer.HealthSnapshot()
	if health.ExportedSpans != 0 || health.ExportFailures != 1 || health.ExportTimeouts != 1 {
		t.Fatalf("unexpected timeout health: %#v", health)
	}
	_ = waitForSinkLog(t, logged)
}

func TestTracerExportQueueBoundsBlockedSinkAndDropsNewest(t *testing.T) {
	sink := newGatedSink()
	tracer := trace.NewTracer(
		trace.WithSink(sink),
		trace.WithExportQueueSize(1),
		trace.WithExportTimeout(time.Second),
	)
	endNamedSpan(t, tracer, "first")
	select {
	case name := <-sink.entered:
		if name != "first" {
			t.Fatalf("first sink call = %q", name)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for blocked sink")
	}

	endNamedSpan(t, tracer, "second")
	for index := 0; index < 18; index++ {
		endNamedSpan(t, tracer, "dropped")
	}

	health := waitForTracerHealth(t, tracer, func(health trace.TracerHealthSnapshot) bool {
		return health.ExportDroppedSpans == 18
	})
	if health.ExportQueueDepth != 1 || !health.ExportInFlight || health.ExportQueueCapacity != 1 {
		t.Fatalf("unexpected blocked queue health: %#v", health)
	}
	if sink.maxActive.Load() != 1 {
		t.Fatalf("expected one concurrent sink call, got %d", sink.maxActive.Load())
	}

	blockedCtx, blockedCancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer blockedCancel()
	if err := tracer.Flush(blockedCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("blocked Flush error = %v, want deadline exceeded", err)
	}

	close(sink.release)
	flushCtx, flushCancel := context.WithTimeout(context.Background(), time.Second)
	defer flushCancel()
	if err := tracer.Flush(flushCtx); err != nil {
		t.Fatal(err)
	}

	names := sink.Names()
	if len(names) != 2 || names[0] != "first" || names[1] != "second" {
		t.Fatalf("drop-newest export order = %#v", names)
	}
	health = tracer.HealthSnapshot()
	if health.ExportedSpans != 2 || health.ExportDroppedSpans != 18 || health.ExportFailures != 0 {
		t.Fatalf("unexpected final queue health: %#v", health)
	}
	if sink.maxActive.Load() != 1 {
		t.Fatalf("sink concurrency exceeded one: %d", sink.maxActive.Load())
	}
}

func endNamedSpan(t *testing.T, tracer *trace.Tracer, name string) {
	t.Helper()
	_, span := tracer.Start(context.Background(), name)
	if span == nil {
		t.Fatalf("expected sampled span %q", name)
	}
	span.End()
}

type deadlineSink struct {
	entered chan struct{}
	once    sync.Once
}

func (sink *deadlineSink) RecordSpan(ctx context.Context, _ trace.Snapshot) error {
	sink.once.Do(func() {
		close(sink.entered)
	})
	<-ctx.Done()
	return ctx.Err()
}

type gatedSink struct {
	entered   chan string
	release   chan struct{}
	active    atomic.Int64
	maxActive atomic.Int64
	mu        sync.Mutex
	names     []string
}

func newGatedSink() *gatedSink {
	return &gatedSink{
		entered: make(chan string, 4),
		release: make(chan struct{}),
	}
}

func (sink *gatedSink) RecordSpan(_ context.Context, span trace.Snapshot) error {
	active := sink.active.Add(1)
	updateAtomicMax(&sink.maxActive, active)
	defer sink.active.Add(-1)

	sink.mu.Lock()
	sink.names = append(sink.names, span.Name)
	sink.mu.Unlock()
	sink.entered <- span.Name
	<-sink.release
	return nil
}

func (sink *gatedSink) Names() []string {
	sink.mu.Lock()
	defer sink.mu.Unlock()
	return append([]string(nil), sink.names...)
}

func updateAtomicMax(target *atomic.Int64, value int64) {
	for {
		current := target.Load()
		if value <= current || target.CompareAndSwap(current, value) {
			return
		}
	}
}
