package trace

import (
	"context"
	"testing"
	"time"
)

func TestCollectorDropsFullSubscriberWithoutBlocking(t *testing.T) {
	collector := NewCollector(4)
	events := make(chan Snapshot, 1)
	events <- Snapshot{Name: "queued"}
	collector.mu.Lock()
	collector.subscribers[events] = struct{}{}
	collector.mu.Unlock()
	t.Cleanup(func() {
		collector.mu.Lock()
		delete(collector.subscribers, events)
		collector.mu.Unlock()
		close(events)
	})

	done := make(chan error, 1)
	go func() {
		done <- collector.RecordSpan(context.Background(), Snapshot{
			TraceID: NewTraceID(),
			SpanID:  NewSpanID(),
			Name:    "nonblocking",
		})
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("RecordSpan blocked on a full subscriber")
	}
	if collector.Dropped() == 0 {
		t.Fatal("full subscriber send was not counted as dropped")
	}
}
