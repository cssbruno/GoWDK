package contracts

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type raceQueryA struct{}
type raceQueryB struct{}
type raceCommandA struct{}
type raceCommandB struct{}
type raceEventA struct{}
type raceEventB struct{}
type raceJobA struct{}
type raceJobB struct{}
type raceResult struct{}

func TestRegistryConcurrentRegistrationAndReads(t *testing.T) {
	registry := NewRegistry()
	registrations := []func() error{
		func() error {
			return RegisterQuery[raceQueryA, raceResult](registry, func(context.Context, raceQueryA) (raceResult, error) { return raceResult{}, nil }, RoleWeb)
		},
		func() error {
			return RegisterQuery[raceQueryB, raceResult](registry, func(context.Context, raceQueryB) (raceResult, error) { return raceResult{}, nil }, RoleWorker)
		},
		func() error {
			return RegisterCommand[raceCommandA, raceResult](registry, func(context.Context, raceCommandA) (raceResult, error) { return raceResult{}, nil }, RoleWeb)
		},
		func() error {
			return RegisterCommand[raceCommandB, raceResult](registry, func(context.Context, raceCommandB) (raceResult, error) { return raceResult{}, nil }, RoleWorker)
		},
		func() error {
			return RegisterDomainEvent[raceEventA](registry, func(context.Context, raceEventA) error { return nil }, RoleWorker)
		},
		func() error {
			return RegisterPresentationEvent[raceEventB](registry, func(context.Context, raceEventB) error { return nil }, RoleWeb)
		},
		func() error {
			return RegisterJob[raceJobA](registry, func(context.Context, raceJobA) error { return nil }, RoleWorker)
		},
		func() error {
			return RegisterJob[raceJobB](registry, func(context.Context, raceJobB) error { return nil }, RoleWeb)
		},
		func() error { return RegisterInvalidation[raceEventA, raceQueryA](registry) },
	}

	var readersDone atomic.Bool
	var readers sync.WaitGroup
	for i := 0; i < 8; i++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for !readersDone.Load() {
				_ = registry.Contracts()
				_ = registry.ContractsForRole(RoleWeb)
				_ = registry.Invalidations()
			}
		}()
	}

	var writers sync.WaitGroup
	for _, registration := range registrations {
		registration := registration
		writers.Add(1)
		go func() {
			defer writers.Done()
			if err := registration(); err != nil {
				t.Errorf("register contract: %v", err)
			}
		}()
	}
	writers.Wait()
	readersDone.Store(true)
	readers.Wait()

	if got := len(registry.Contracts()); got != 8 {
		t.Fatalf("registered contracts = %d, want 8", got)
	}
	if got := len(registry.Invalidations()); got != 1 {
		t.Fatalf("invalidations = %d, want 1", got)
	}
}

func TestMemorySeenStoreConcurrentAccessEvictsSafely(t *testing.T) {
	store := NewMemorySeenStore(32)
	const workers = 12
	const iterations = 80

	var done sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		worker := worker
		done.Add(1)
		go func() {
			defer done.Done()
			for iteration := 0; iteration < iterations; iteration++ {
				id := "event-" + string(rune('a'+worker)) + "-" + string(rune('a'+iteration%26))
				if _, err := store.MarkIfNew(context.Background(), id); err != nil {
					t.Errorf("mark if new: %v", err)
				}
				if _, err := store.Seen(context.Background(), id); err != nil {
					t.Errorf("seen: %v", err)
				}
				if err := store.MarkSeen(context.Background(), id); err != nil {
					t.Errorf("mark seen: %v", err)
				}
			}
		}()
	}
	done.Wait()
}

func TestEventWorkerConcurrentRetryBookkeeping(t *testing.T) {
	registry := NewRegistry()
	var handled atomic.Int64
	if err := RegisterDomainEvent[raceEventA](registry, func(context.Context, raceEventA) error {
		handled.Add(1)
		return nil
	}, RoleWorker); err != nil {
		t.Fatal(err)
	}

	source := &retryRaceSource{
		batches: []EventBatch{
			{
				Events: []EventEnvelope{{ID: "one", Category: DomainEvent, Type: typeName[raceEventA](), Value: raceEventA{}}},
				Nack:   func(context.Context, error) error { return nil },
			},
			{
				Events: []EventEnvelope{{ID: "two", Category: DomainEvent, Type: typeName[raceEventA](), Value: raceEventA{}}},
				Ack:    func(context.Context) error { return nil },
			},
		},
	}
	if err := RegisterDomainEvent[raceEventB](registry, func(context.Context, raceEventB) error {
		return errors.New("force first batch nack")
	}, RoleWorker); err != nil {
		t.Fatal(err)
	}
	source.batches[0].Events = append(source.batches[0].Events, EventEnvelope{ID: "fail", Category: DomainEvent, Type: typeName[raceEventB](), Value: raceEventB{}})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := RunEventWorkerForRoleWithOptions(ctx, registry, RoleWorker, source, WithEventWorkerBackoff(ConstantEventWorkerBackoff(time.Nanosecond))); err != nil {
		t.Fatal(err)
	}
	if handled.Load() == 0 {
		t.Fatal("expected successful event batch to run after retry")
	}
}

type retryRaceSource struct {
	mu      sync.Mutex
	batches []EventBatch
}

func (source *retryRaceSource) ReceiveEventBatch(context.Context) (EventBatch, error) {
	source.mu.Lock()
	defer source.mu.Unlock()
	if len(source.batches) == 0 {
		return EventBatch{}, ErrEventSourceClosed
	}
	batch := source.batches[0]
	source.batches = source.batches[1:]
	return batch, nil
}
