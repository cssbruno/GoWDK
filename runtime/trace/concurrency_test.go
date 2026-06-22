package trace_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/cssbruno/gowdk/runtime/trace"
)

func TestRingSinkConcurrentRecordAndRead(t *testing.T) {
	sink := trace.NewRingSink(32)
	const workers = 12
	const iterations = 80

	var ready sync.WaitGroup
	var start sync.WaitGroup
	var done sync.WaitGroup
	start.Add(1)

	for worker := 0; worker < workers; worker++ {
		ready.Add(1)
		done.Add(1)
		go func() {
			defer done.Done()
			ready.Done()
			start.Wait()
			for iteration := 0; iteration < iterations; iteration++ {
				if err := sink.RecordSpan(context.Background(), trace.Snapshot{
					TraceID:    trace.NewTraceID(),
					SpanID:     trace.NewSpanID(),
					Name:       "race-span",
					Surface:    trace.SurfaceBackend,
					Lane:       trace.LaneHandler,
					StartTime:  time.Now(),
					EndTime:    time.Now(),
					DurationNS: 1,
				}); err != nil {
					t.Errorf("record span: %v", err)
				}
				_ = sink.Spans()
				_ = sink.Dropped()
			}
		}()
	}

	ready.Wait()
	start.Done()
	done.Wait()

	if got := len(sink.Spans()); got == 0 || got > 32 {
		t.Fatalf("stored spans = %d, want 1..32", got)
	}
}
