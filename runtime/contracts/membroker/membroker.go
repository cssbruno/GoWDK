// Package membroker provides an in-memory broker adapter for runtime/contracts.
package membroker

import (
	"context"
	"sync"

	"github.com/cssbruno/gowdk/runtime/contracts"
)

const defaultBatchSize = 100

// Broker stores published events in memory and exposes them as worker batches.
// It is intended for tests, local development, and single-process apps.
type Broker struct {
	mu        sync.Mutex
	batchSize int
	nextID    uint64
	records   []record
}

type record struct {
	id    uint64
	event contracts.EventEnvelope
}

// Option configures a Broker.
type Option func(*Broker)

// WithBatchSize sets the maximum events returned by one worker batch.
func WithBatchSize(size int) Option {
	return func(broker *Broker) {
		if size > 0 {
			broker.batchSize = size
		}
	}
}

// New creates an in-memory broker.
func New(options ...Option) *Broker {
	broker := &Broker{batchSize: defaultBatchSize}
	for _, option := range options {
		if option != nil {
			option(broker)
		}
	}
	return broker
}

// PublishEvents appends events to the broker queue.
func (broker *Broker) PublishEvents(ctx context.Context, events []contracts.EventEnvelope) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(events) == 0 {
		return nil
	}
	broker.mu.Lock()
	defer broker.mu.Unlock()
	for _, event := range events {
		broker.nextID++
		broker.records = append(broker.records, record{id: broker.nextID, event: contracts.EnsureEventID(event)})
	}
	return nil
}

// ReceiveEventBatch returns the next queued batch. Ack removes the delivered
// records; Nack leaves them queued for retry.
func (broker *Broker) ReceiveEventBatch(ctx context.Context) (contracts.EventBatch, error) {
	if err := ctx.Err(); err != nil {
		return contracts.EventBatch{}, err
	}
	broker.mu.Lock()
	if len(broker.records) == 0 {
		broker.mu.Unlock()
		return contracts.EventBatch{}, contracts.ErrEventSourceClosed
	}
	limit := broker.batchSize
	if limit > len(broker.records) {
		limit = len(broker.records)
	}
	selected := append([]record(nil), broker.records[:limit]...)
	broker.mu.Unlock()

	events := make([]contracts.EventEnvelope, 0, len(selected))
	acked := map[uint64]bool{}
	for _, record := range selected {
		events = append(events, record.event)
		acked[record.id] = true
	}
	return contracts.EventBatch{
		Events: events,
		Ack: func(ctx context.Context) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			broker.mu.Lock()
			defer broker.mu.Unlock()
			kept := broker.records[:0]
			for _, record := range broker.records {
				if !acked[record.id] {
					kept = append(kept, record)
				}
			}
			broker.records = kept
			return nil
		},
	}, nil
}

// Len returns the queued event count.
func (broker *Broker) Len() int {
	broker.mu.Lock()
	defer broker.mu.Unlock()
	return len(broker.records)
}
