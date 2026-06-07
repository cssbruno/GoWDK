package contracts

import (
	"context"
	"errors"
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

func TestRunEventWorkerNacksSubscriberFailure(t *testing.T) {
	registry := NewRegistry()
	subscriberErr := errors.New("subscriber unavailable")
	must(t, RegisterDomainEvent[patientCreated](registry, func(ctx context.Context, event patientCreated) error {
		return subscriberErr
	}, RoleWorker))
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
	}

	err := RunEventWorker(context.Background(), registry, source)
	if !Is(err, ErrSubscriberFailed) {
		t.Fatalf("run event worker error = %v, want %s", err, ErrSubscriberFailed)
	}
	if !errors.Is(err, subscriberErr) {
		t.Fatalf("run event worker error = %v, want subscriber cause", err)
	}
	if acked != 0 {
		t.Fatalf("acked = %d, want 0", acked)
	}
	if !Is(nackCause, ErrSubscriberFailed) {
		t.Fatalf("nack cause = %v, want subscriber failure", nackCause)
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
