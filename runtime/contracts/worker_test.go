package contracts

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type scriptedEventSource struct {
	batches  []EventBatch
	err      error
	received int
}

func (source *scriptedEventSource) ReceiveEventBatch(ctx context.Context) (EventBatch, error) {
	source.received++
	if len(source.batches) > 0 {
		batch := source.batches[0]
		source.batches = source.batches[1:]
		return batch, nil
	}
	if source.err != nil {
		return EventBatch{}, source.err
	}
	<-ctx.Done()
	return EventBatch{}, ctx.Err()
}

func TestRunEventWorkerDispatchesAndAcks(t *testing.T) {
	registry := NewRegistry()
	var webHandled, workerHandled, rolelessHandled int
	must(t, RegisterDomainEvent[patientCreated](registry, func(ctx context.Context, event patientCreated) error {
		webHandled++
		return nil
	}, RoleWeb))
	must(t, RegisterDomainEvent[patientCreated](registry, func(ctx context.Context, event patientCreated) error {
		workerHandled++
		return nil
	}, RoleWorker))
	must(t, RegisterDomainEvent[patientCreated](registry, func(ctx context.Context, event patientCreated) error {
		rolelessHandled++
		return nil
	}))
	var acked, nacked int
	source := &scriptedEventSource{
		batches: []EventBatch{{
			Events: []EventEnvelope{{
				Category: DomainEvent,
				Type:     typeName[patientCreated](),
				Value:    patientCreated{ID: "patient-1"},
			}},
			Ack: func(ctx context.Context) error {
				acked++
				return nil
			},
			Nack: func(ctx context.Context, err error) error {
				nacked++
				return nil
			},
		}},
		err: ErrEventSourceClosed,
	}

	if err := RunEventWorker(context.Background(), registry, source); err != nil {
		t.Fatalf("run event worker: %v", err)
	}
	if webHandled != 0 || workerHandled != 1 || rolelessHandled != 1 {
		t.Fatalf("unexpected role dispatch counts: web=%d worker=%d roleless=%d", webHandled, workerHandled, rolelessHandled)
	}
	if acked != 1 || nacked != 0 {
		t.Fatalf("acked=%d nacked=%d, want acked=1 nacked=0", acked, nacked)
	}
}

func TestRunEventWorkerWithSeenStoreAcksDuplicateWithoutDispatch(t *testing.T) {
	registry := NewRegistry()
	var handled int
	must(t, RegisterDomainEvent[patientCreated](registry, func(ctx context.Context, event patientCreated) error {
		handled++
		return nil
	}, RoleWorker))
	seen := NewMemorySeenStore(10)
	if fresh, err := seen.MarkIfNew(context.Background(), "event-1"); err != nil || !fresh {
		t.Fatalf("prime seen store fresh=%v err=%v, want true nil", fresh, err)
	}
	var acked, nacked int
	source := &scriptedEventSource{
		batches: []EventBatch{{
			Events: []EventEnvelope{{
				ID:       "event-1",
				Category: DomainEvent,
				Type:     typeName[patientCreated](),
				Value:    patientCreated{ID: "patient-1"},
			}},
			Ack: func(ctx context.Context) error {
				acked++
				return nil
			},
			Nack: func(ctx context.Context, err error) error {
				nacked++
				return nil
			},
		}},
		err: ErrEventSourceClosed,
	}

	if err := RunEventWorkerWithSeenStore(context.Background(), registry, source, seen); err != nil {
		t.Fatalf("run event worker: %v", err)
	}
	if handled != 0 {
		t.Fatalf("handled duplicate count = %d, want 0", handled)
	}
	if acked != 1 || nacked != 0 {
		t.Fatalf("acked=%d nacked=%d, want acked=1 nacked=0", acked, nacked)
	}
}

func TestRunEventWorkerWithSeenStoreDispatchesOnlyNewEvents(t *testing.T) {
	registry := NewRegistry()
	var handled []string
	must(t, RegisterDomainEvent[patientCreated](registry, func(ctx context.Context, event patientCreated) error {
		handled = append(handled, event.ID)
		return nil
	}, RoleWorker))
	seen := NewMemorySeenStore(10)
	if fresh, err := seen.MarkIfNew(context.Background(), "event-1"); err != nil || !fresh {
		t.Fatalf("prime seen store fresh=%v err=%v, want true nil", fresh, err)
	}
	source := &scriptedEventSource{
		batches: []EventBatch{{
			Events: []EventEnvelope{
				{ID: "event-1", Category: DomainEvent, Type: typeName[patientCreated](), Value: patientCreated{ID: "duplicate"}},
				{ID: "event-2", Category: DomainEvent, Type: typeName[patientCreated](), Value: patientCreated{ID: "new"}},
			},
			Ack: func(ctx context.Context) error { return nil },
		}},
		err: ErrEventSourceClosed,
	}

	if err := RunEventWorkerWithSeenStore(context.Background(), registry, source, seen); err != nil {
		t.Fatalf("run event worker: %v", err)
	}
	if strings.Join(handled, ",") != "new" {
		t.Fatalf("handled = %#v, want only new event", handled)
	}
	fresh, err := seen.MarkIfNew(context.Background(), "event-2")
	if err != nil || fresh {
		t.Fatalf("event-2 should be recorded as seen, fresh=%v err=%v", fresh, err)
	}
}

func TestRunEventWorkerNacksSubscriberFailureAndContinues(t *testing.T) {
	registry := NewRegistry()
	subscriberErr := errors.New("subscriber unavailable")
	must(t, RegisterDomainEvent[patientCreated](registry, func(ctx context.Context, event patientCreated) error {
		return subscriberErr
	}, RoleWorker))
	var logged []string
	previousLogger := WorkerLogger
	WorkerLogger = func(message string) {
		logged = append(logged, message)
	}
	defer func() { WorkerLogger = previousLogger }()
	var acked int
	var nackCause error
	source := &scriptedEventSource{
		batches: []EventBatch{{
			Events: []EventEnvelope{{
				Category: DomainEvent,
				Type:     typeName[patientCreated](),
				Value:    patientCreated{ID: "patient-1"},
			}},
			Ack: func(ctx context.Context) error {
				acked++
				return nil
			},
			Nack: func(ctx context.Context, err error) error {
				nackCause = err
				return nil
			},
		}},
		err: ErrEventSourceClosed,
	}

	if err := RunEventWorker(context.Background(), registry, source); err != nil {
		t.Fatalf("run event worker error = %v, want nil after successful nack", err)
	}
	if source.received != 2 {
		t.Fatalf("source.received = %d, want worker to keep consuming after nack", source.received)
	}
	if acked != 0 {
		t.Fatalf("acked = %d, want 0", acked)
	}
	if !Is(nackCause, ErrSubscriberFailed) {
		t.Fatalf("nack cause = %v, want subscriber failure", nackCause)
	}
	if len(logged) != 1 || !strings.Contains(logged[0], subscriberErr.Error()) {
		t.Fatalf("logged = %#v, want one recovered dispatch failure", logged)
	}
}

func TestRunEventWorkerReturnsSubscriberFailureWithoutNack(t *testing.T) {
	registry := NewRegistry()
	subscriberErr := errors.New("subscriber unavailable")
	must(t, RegisterDomainEvent[patientCreated](registry, func(ctx context.Context, event patientCreated) error {
		return subscriberErr
	}, RoleWorker))
	source := &scriptedEventSource{
		batches: []EventBatch{{
			Events: []EventEnvelope{{
				Category: DomainEvent,
				Type:     typeName[patientCreated](),
				Value:    patientCreated{ID: "patient-1"},
			}},
		}},
	}

	err := RunEventWorker(context.Background(), registry, source)
	if !Is(err, ErrSubscriberFailed) {
		t.Fatalf("run event worker error = %v, want %s", err, ErrSubscriberFailed)
	}
	if !errors.Is(err, subscriberErr) {
		t.Fatalf("run event worker error = %v, want subscriber cause", err)
	}
}

func TestRunEventWorkerReturnsNackFailure(t *testing.T) {
	registry := NewRegistry()
	must(t, RegisterDomainEvent[patientCreated](registry, func(ctx context.Context, event patientCreated) error {
		return errors.New("subscriber unavailable")
	}, RoleWorker))
	nackErr := errors.New("nack unavailable")
	source := &scriptedEventSource{
		batches: []EventBatch{{
			Events: []EventEnvelope{{
				Category: DomainEvent,
				Type:     typeName[patientCreated](),
				Value:    patientCreated{ID: "patient-1"},
			}},
			Nack: func(ctx context.Context, err error) error {
				return nackErr
			},
		}},
	}

	err := RunEventWorker(context.Background(), registry, source)
	if !Is(err, ErrSubscriberFailed) {
		t.Fatalf("run event worker error = %v, want subscriber failure", err)
	}
	if !errors.Is(err, nackErr) {
		t.Fatalf("run event worker error = %v, want nack cause", err)
	}
}

func TestRunEventWorkerStopsOnClosedSource(t *testing.T) {
	source := &scriptedEventSource{err: ErrEventSourceClosed}

	if err := RunEventWorker(context.Background(), NewRegistry(), source); err != nil {
		t.Fatalf("run event worker: %v", err)
	}
	if source.received != 1 {
		t.Fatalf("source.received = %d, want 1", source.received)
	}
}

func TestRunEventWorkerReturnsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := RunEventWorker(ctx, NewRegistry(), &scriptedEventSource{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("run event worker error = %v, want context canceled", err)
	}
}

func TestRunEventWorkerRejectsNilSource(t *testing.T) {
	err := RunEventWorker(context.Background(), NewRegistry(), nil)
	if !Is(err, ErrNilHandler) {
		t.Fatalf("run event worker error = %v, want %s", err, ErrNilHandler)
	}
}
