package contracts

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
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

func TestWorkerDispatchFailureLogRedactsSecrets(t *testing.T) {
	tests := []struct {
		name string
		err  string
		want string
		leak string
	}{
		{
			name: "bearer token",
			err:  "subscriber failed Authorization: Bearer abcdefgh1234567890",
			want: "Authorization: Bearer [REDACTED]",
			leak: "abcdefgh1234567890",
		},
		{
			name: "dsn password",
			err:  "subscriber failed opening postgres://app:supersecret@db.local/gowdk",
			want: "postgres://app:[REDACTED]@db.local/gowdk",
			leak: "supersecret",
		},
		{
			name: "key value secret",
			err:  "subscriber failed api_key=sk_live_123456789",
			want: "api_key=[REDACTED]",
			leak: "sk_live_123456789",
		},
		{
			name: "non secret",
			err:  "subscriber failed with temporary outage",
			want: "subscriber failed with temporary outage",
		},
	}

	previousLogger := WorkerLogger
	defer func() { WorkerLogger = previousLogger }()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logged []string
			WorkerLogger = func(message string) {
				logged = append(logged, message)
			}

			logWorkerDispatchFailure(errors.New(tt.err))

			if len(logged) != 1 {
				t.Fatalf("logged = %#v, want one message", logged)
			}
			if !strings.Contains(logged[0], tt.want) {
				t.Fatalf("logged message = %q, want it to contain %q", logged[0], tt.want)
			}
			if tt.leak != "" && strings.Contains(logged[0], tt.leak) {
				t.Fatalf("logged message leaked %q: %q", tt.leak, logged[0])
			}
		})
	}
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
	if err := seen.MarkSeen(context.Background(), "event-1"); err != nil {
		t.Fatalf("prime seen store: %v", err)
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
	if err := seen.MarkSeen(context.Background(), "event-1"); err != nil {
		t.Fatalf("prime seen store: %v", err)
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
	alreadySeen, err := seen.Seen(context.Background(), "event-2")
	if err != nil || !alreadySeen {
		t.Fatalf("event-2 should be recorded as seen, seen=%v err=%v", alreadySeen, err)
	}
}

func TestRunEventWorkerWithSeenStoreDoesNotMarkNackedDispatch(t *testing.T) {
	registry := NewRegistry()
	subscriberErr := errors.New("subscriber unavailable")
	var handled int
	must(t, RegisterDomainEvent[patientCreated](registry, func(ctx context.Context, event patientCreated) error {
		handled++
		if handled == 1 {
			return subscriberErr
		}
		return nil
	}, RoleWorker))
	var acked, nacked int
	seen := NewMemorySeenStore(10)
	event := EventEnvelope{
		ID:       "event-1",
		Category: DomainEvent,
		Type:     typeName[patientCreated](),
		Value:    patientCreated{ID: "patient-1"},
	}
	source := &scriptedEventSource{
		batches: []EventBatch{
			{
				Events: []EventEnvelope{event},
				Ack: func(ctx context.Context) error {
					acked++
					return nil
				},
				Nack: func(ctx context.Context, err error) error {
					nacked++
					return nil
				},
			},
			{
				Events: []EventEnvelope{event},
				Ack: func(ctx context.Context) error {
					acked++
					return nil
				},
				Nack: func(ctx context.Context, err error) error {
					nacked++
					return nil
				},
			},
		},
		err: ErrEventSourceClosed,
	}

	if err := RunEventWorkerWithSeenStore(context.Background(), registry, source, seen); err != nil {
		t.Fatalf("run event worker: %v", err)
	}
	if handled != 2 {
		t.Fatalf("handled = %d, want failed attempt plus redelivery", handled)
	}
	if acked != 1 || nacked != 1 {
		t.Fatalf("acked=%d nacked=%d, want acked=1 nacked=1", acked, nacked)
	}
	alreadySeen, err := seen.Seen(context.Background(), "event-1")
	if err != nil || !alreadySeen {
		t.Fatalf("event-1 should be marked after successful redelivery, seen=%v err=%v", alreadySeen, err)
	}
}

func TestRunEventWorkerWithSeenStoreDoesNotMarkAckFailure(t *testing.T) {
	registry := NewRegistry()
	var handled int
	must(t, RegisterDomainEvent[patientCreated](registry, func(ctx context.Context, event patientCreated) error {
		handled++
		return nil
	}, RoleWorker))
	seen := NewMemorySeenStore(10)
	ackErr := errors.New("ack unavailable")
	source := &scriptedEventSource{
		batches: []EventBatch{{
			Events: []EventEnvelope{{
				ID:       "event-1",
				Category: DomainEvent,
				Type:     typeName[patientCreated](),
				Value:    patientCreated{ID: "patient-1"},
			}},
			Ack: func(ctx context.Context) error {
				return ackErr
			},
		}},
	}

	err := RunEventWorkerWithSeenStore(context.Background(), registry, source, seen)
	if !errors.Is(err, ackErr) {
		t.Fatalf("run event worker error = %v, want ack error", err)
	}
	if handled != 1 {
		t.Fatalf("handled = %d, want 1", handled)
	}
	alreadySeen, seenErr := seen.Seen(context.Background(), "event-1")
	if seenErr != nil || alreadySeen {
		t.Fatalf("event-1 should not be marked after ack failure, seen=%v err=%v", alreadySeen, seenErr)
	}
}

func TestRunEventWorkerNacksSubscriberFailureAndContinues(t *testing.T) {
	registry := NewRegistry()
	subscriberErr := errors.New("subscriber unavailable password=hunter2")
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
	if len(logged) != 1 || !strings.Contains(logged[0], "subscriber unavailable") || !strings.Contains(logged[0], "password=[REDACTED]") {
		t.Fatalf("logged = %#v, want one redacted dispatch failure", logged)
	}
	if strings.Contains(logged[0], "hunter2") {
		t.Fatalf("worker log leaked secret: %q", logged[0])
	}
}

func TestRunEventWorkerWithOptionsAppliesBackoffAfterNack(t *testing.T) {
	registry := NewRegistry()
	subscriberErr := errors.New("subscriber unavailable")
	must(t, RegisterDomainEvent[patientCreated](registry, func(ctx context.Context, event patientCreated) error {
		return subscriberErr
	}, RoleWorker))
	previousLogger := WorkerLogger
	WorkerLogger = nil
	defer func() { WorkerLogger = previousLogger }()
	source := &scriptedEventSource{
		batches: []EventBatch{{
			Events: []EventEnvelope{{
				Category: DomainEvent,
				Type:     typeName[patientCreated](),
				Value:    patientCreated{ID: "patient-1"},
			}},
			Nack: func(ctx context.Context, err error) error {
				return nil
			},
		}},
		err: ErrEventSourceClosed,
	}
	var retries []EventWorkerRetry

	err := RunEventWorkerWithOptions(context.Background(), registry, source, WithEventWorkerBackoff(func(retry EventWorkerRetry) time.Duration {
		retries = append(retries, retry)
		return 0
	}))
	if err != nil {
		t.Fatalf("run event worker error = %v, want nil after successful nack", err)
	}
	if len(retries) != 1 {
		t.Fatalf("retries = %#v, want one retry callback", retries)
	}
	retry := retries[0]
	if retry.Role != RoleWorker || retry.Attempt != 1 || retry.EventCount != 1 || !Is(retry.Cause, ErrSubscriberFailed) {
		t.Fatalf("unexpected retry metadata: %#v", retry)
	}
}

func TestRunEventWorkerWithOptionsBackoffHonorsContextCancellation(t *testing.T) {
	registry := NewRegistry()
	must(t, RegisterDomainEvent[patientCreated](registry, func(ctx context.Context, event patientCreated) error {
		return errors.New("subscriber unavailable")
	}, RoleWorker))
	previousLogger := WorkerLogger
	WorkerLogger = nil
	defer func() { WorkerLogger = previousLogger }()
	source := &scriptedEventSource{
		batches: []EventBatch{{
			Events: []EventEnvelope{{
				Category: DomainEvent,
				Type:     typeName[patientCreated](),
				Value:    patientCreated{ID: "patient-1"},
			}},
			Nack: func(ctx context.Context, err error) error {
				return nil
			},
		}},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := RunEventWorkerWithOptions(ctx, registry, source, WithEventWorkerBackoff(func(EventWorkerRetry) time.Duration {
		cancel()
		return time.Hour
	}))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("run event worker error = %v, want context canceled", err)
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
