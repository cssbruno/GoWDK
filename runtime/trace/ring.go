package trace

import (
	"context"
	"sync"
)

const defaultRingLimit = 256

// RingSink stores the most recent completed spans in memory. On overflow it
// drops the oldest span and increments Dropped.
type RingSink struct {
	mu      sync.RWMutex
	limit   int
	next    int
	filled  bool
	dropped uint64
	spans   []Snapshot
}

// NewRingSink creates a bounded in-memory sink.
func NewRingSink(limit int) *RingSink {
	if limit <= 0 {
		limit = defaultRingLimit
	}
	return &RingSink{limit: limit, spans: make([]Snapshot, limit)}
}

// RecordSpan implements Sink. It never blocks on external I/O.
func (sink *RingSink) RecordSpan(ctx context.Context, span Snapshot) error {
	if sink == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	sink.mu.Lock()
	defer sink.mu.Unlock()
	if sink.filled {
		sink.dropped++
	}
	sink.spans[sink.next] = span
	sink.next = (sink.next + 1) % sink.limit
	if sink.next == 0 {
		sink.filled = true
	}
	return nil
}

// Spans returns completed spans from oldest to newest.
func (sink *RingSink) Spans() []Snapshot {
	if sink == nil {
		return nil
	}
	sink.mu.RLock()
	defer sink.mu.RUnlock()
	var out []Snapshot
	if !sink.filled {
		out = append(out, sink.spans[:sink.next]...)
		return out
	}
	out = append(out, sink.spans[sink.next:]...)
	out = append(out, sink.spans[:sink.next]...)
	return out
}

// Dropped returns the number of spans dropped because the ring was full.
func (sink *RingSink) Dropped() uint64 {
	if sink == nil {
		return 0
	}
	sink.mu.RLock()
	defer sink.mu.RUnlock()
	return sink.dropped
}
