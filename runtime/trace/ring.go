package trace

import (
	"context"
	"sync"
)

const (
	defaultRingLimit   = 256
	maxRingStoredBytes = 1 << 20
)

// RingSink stores the most recent completed spans in memory. On overflow it
// drops the oldest span and increments Dropped.
type RingSink struct {
	mu         sync.RWMutex
	limit      int
	start      int
	count      int
	dropped    uint64
	totalBytes int
	spans      []Snapshot
	sizes      []int
}

// NewRingSink creates a bounded in-memory sink.
func NewRingSink(limit int) *RingSink {
	if limit <= 0 {
		limit = defaultRingLimit
	}
	return &RingSink{limit: limit, spans: make([]Snapshot, limit), sizes: make([]int, limit)}
}

// RecordSpan implements Sink. It never blocks on external I/O.
func (sink *RingSink) RecordSpan(ctx context.Context, span Snapshot) error {
	if sink == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	span = cloneSnapshot(span)
	size := snapshotEncodedSize(span)
	sink.mu.Lock()
	defer sink.mu.Unlock()
	if size > maxSnapshotEncodedBytes {
		sink.dropped++
		return nil
	}
	for sink.count > 0 && (sink.count == sink.limit || sink.totalBytes+size > maxRingStoredBytes) {
		sink.dropOldestLocked()
	}
	index := (sink.start + sink.count) % sink.limit
	sink.spans[index] = span
	sink.sizes[index] = size
	sink.totalBytes += size
	sink.count++
	return nil
}

func (sink *RingSink) dropOldestLocked() {
	sink.totalBytes -= sink.sizes[sink.start]
	sink.spans[sink.start] = Snapshot{}
	sink.sizes[sink.start] = 0
	sink.start = (sink.start + 1) % sink.limit
	sink.count--
	sink.dropped++
}

// Spans returns completed spans from oldest to newest.
func (sink *RingSink) Spans() []Snapshot {
	if sink == nil {
		return nil
	}
	sink.mu.RLock()
	defer sink.mu.RUnlock()
	out := make([]Snapshot, 0, sink.count)
	for offset := 0; offset < sink.count; offset++ {
		index := (sink.start + offset) % sink.limit
		out = append(out, cloneSnapshot(sink.spans[index]))
	}
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
