package contracts

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync/atomic"
	"time"
)

var fallbackEventIDSeq uint64

// NewEventID returns a process-local unique event ID suitable for durable
// envelope storage and worker deduplication.
func NewEventID() string {
	var random [16]byte
	if _, err := rand.Read(random[:]); err == nil {
		return hex.EncodeToString(random[:])
	}
	seq := atomic.AddUint64(&fallbackEventIDSeq, 1)
	return fmt.Sprintf("%d-%d", time.Now().UTC().UnixNano(), seq)
}

// EnsureEventID returns event with a durable ID assigned when it is missing.
func EnsureEventID(event EventEnvelope) EventEnvelope {
	if event.ID == "" {
		event.ID = NewEventID()
	}
	return event
}

// EnsureEventIDs returns a copy of events where every envelope has a durable
// ID. Existing IDs are preserved.
func EnsureEventIDs(events []EventEnvelope) []EventEnvelope {
	if len(events) == 0 {
		return nil
	}
	out := make([]EventEnvelope, len(events))
	for index, event := range events {
		out[index] = EnsureEventID(event)
	}
	return out
}
