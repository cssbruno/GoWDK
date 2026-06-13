package fileoutbox

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cssbruno/gowdk/runtime/contracts"
)

var patientCreatedType = contracts.ContractName[patientCreated]()

type patientCreated struct {
	ID string `json:"id"`
}

func TestStoreEventsAppendsDurableRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "outbox.jsonl")
	store := New(path)
	store.now = func() time.Time { return time.Unix(123, 0).UTC() }

	err := store.StoreEvents(context.Background(), []contracts.EventEnvelope{{
		Category: contracts.DomainEvent,
		Type:     patientCreatedType,
		Value:    patientCreated{ID: "patient-1"},
	}})
	if err != nil {
		t.Fatalf("store events: %v", err)
	}

	records, err := store.Records(context.Background())
	if err != nil {
		t.Fatalf("records: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("len(records) = %d, want 1", len(records))
	}
	if records[0].Category != contracts.DomainEvent || records[0].Type != patientCreatedType {
		t.Fatalf("unexpected record metadata: %#v", records[0])
	}
	if records[0].ID == "" {
		t.Fatalf("expected durable record ID to be assigned: %#v", records[0])
	}
	var value patientCreated
	if err := json.Unmarshal(records[0].Value, &value); err != nil {
		t.Fatalf("unmarshal record value: %v", err)
	}
	if value.ID != "patient-1" {
		t.Fatalf("record value ID = %q, want patient-1", value.ID)
	}
	if info, err := os.Stat(path); err != nil || info.Size() == 0 {
		t.Fatalf("expected durable file at %s, info=%v err=%v", path, info, err)
	}
}

func TestReceiveEventBatchDecodesAndAcksRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "outbox.jsonl")
	store := New(path, WithJSONTypeDecoder[patientCreated]())
	if err := store.StoreEvents(context.Background(), []contracts.EventEnvelope{
		{Category: contracts.DomainEvent, Type: patientCreatedType, Value: patientCreated{ID: "patient-1"}},
		{Category: contracts.DomainEvent, Type: patientCreatedType, Value: patientCreated{ID: "patient-2"}},
	}); err != nil {
		t.Fatalf("store events: %v", err)
	}

	batch, err := store.ReceiveEventBatch(context.Background())
	if err != nil {
		t.Fatalf("receive batch: %v", err)
	}
	if len(batch.Events) != 2 {
		t.Fatalf("len(batch.Events) = %d, want 2", len(batch.Events))
	}
	if batch.Events[0].ID == "" || batch.Events[1].ID == "" || batch.Events[0].ID == batch.Events[1].ID {
		t.Fatalf("expected replayed events to carry unique IDs: %#v", batch.Events)
	}
	first, ok := batch.Events[0].Value.(patientCreated)
	if !ok || first.ID != "patient-1" {
		t.Fatalf("first event value = %#v, want patientCreated patient-1", batch.Events[0].Value)
	}
	if err := batch.Ack(context.Background()); err != nil {
		t.Fatalf("ack batch: %v", err)
	}
	_, err = store.ReceiveEventBatch(context.Background())
	if !errors.Is(err, contracts.ErrEventSourceClosed) {
		t.Fatalf("receive after ack error = %v, want closed source", err)
	}
}

func TestReceiveEventBatchNackKeepsRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "outbox.jsonl")
	store := New(path, WithJSONTypeDecoder[patientCreated]())
	if err := store.StoreEvents(context.Background(), []contracts.EventEnvelope{{
		Category: contracts.DomainEvent,
		Type:     patientCreatedType,
		Value:    patientCreated{ID: "patient-1"},
	}}); err != nil {
		t.Fatalf("store events: %v", err)
	}

	batch, err := store.ReceiveEventBatch(context.Background())
	if err != nil {
		t.Fatalf("receive batch: %v", err)
	}
	nackErr := errors.New("subscriber failed")
	if err := batch.Nack(context.Background(), nackErr); err != nil {
		t.Fatalf("nack batch: %v", err)
	}
	records, err := store.Records(context.Background())
	if err != nil {
		t.Fatalf("records: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("len(records) = %d, want 1", len(records))
	}
	if records[0].Attempts != 1 || records[0].LastError != nackErr.Error() || records[0].LastAttemptAt == nil {
		t.Fatalf("unexpected retry metadata after nack: %#v", records[0])
	}
}

func TestReceiveEventBatchMovesRecordToDeadLetterAfterMaxAttempts(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "outbox.jsonl")
	deadLetterPath := filepath.Join(root, "dead-letter.jsonl")
	store := New(path, WithJSONTypeDecoder[patientCreated](), WithDeadLetter(deadLetterPath, 2))
	store.now = func() time.Time { return time.Unix(123, 0).UTC() }
	if err := store.StoreEvents(context.Background(), []contracts.EventEnvelope{{
		Category: contracts.DomainEvent,
		Type:     patientCreatedType,
		Value:    patientCreated{ID: "patient-1"},
	}}); err != nil {
		t.Fatalf("store events: %v", err)
	}

	first, err := store.ReceiveEventBatch(context.Background())
	if err != nil {
		t.Fatalf("receive first batch: %v", err)
	}
	if err := first.Nack(context.Background(), errors.New("first failure")); err != nil {
		t.Fatalf("first nack: %v", err)
	}
	records, err := store.Records(context.Background())
	if err != nil {
		t.Fatalf("records after first nack: %v", err)
	}
	if len(records) != 1 || records[0].Attempts != 1 {
		t.Fatalf("unexpected pending records after first nack: %#v", records)
	}
	dead, err := store.DeadLetterRecords(context.Background())
	if err != nil {
		t.Fatalf("dead letter records after first nack: %v", err)
	}
	if len(dead) != 0 {
		t.Fatalf("dead letter records after first nack = %#v, want empty", dead)
	}

	second, err := store.ReceiveEventBatch(context.Background())
	if err != nil {
		t.Fatalf("receive second batch: %v", err)
	}
	if err := second.Nack(context.Background(), errors.New("second failure")); err != nil {
		t.Fatalf("second nack: %v", err)
	}
	records, err = store.Records(context.Background())
	if err != nil {
		t.Fatalf("records after second nack: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("pending records after second nack = %#v, want empty", records)
	}
	dead, err = store.DeadLetterRecords(context.Background())
	if err != nil {
		t.Fatalf("dead letter records after second nack: %v", err)
	}
	if len(dead) != 1 {
		t.Fatalf("len(dead) = %d, want 1: %#v", len(dead), dead)
	}
	if dead[0].Attempts != 2 || dead[0].LastError != "second failure" || dead[0].LastAttemptAt == nil {
		t.Fatalf("unexpected dead letter retry metadata: %#v", dead[0])
	}
}

func TestReceiveEventBatchHonorsBatchSize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "outbox.jsonl")
	store := New(path, WithBatchSize(1), WithJSONTypeDecoder[patientCreated]())
	if err := store.StoreEvents(context.Background(), []contracts.EventEnvelope{
		{Category: contracts.DomainEvent, Type: patientCreatedType, Value: patientCreated{ID: "patient-1"}},
		{Category: contracts.DomainEvent, Type: patientCreatedType, Value: patientCreated{ID: "patient-2"}},
	}); err != nil {
		t.Fatalf("store events: %v", err)
	}

	first, err := store.ReceiveEventBatch(context.Background())
	if err != nil {
		t.Fatalf("receive first batch: %v", err)
	}
	if len(first.Events) != 1 {
		t.Fatalf("len(first.Events) = %d, want 1", len(first.Events))
	}
	if err := first.Ack(context.Background()); err != nil {
		t.Fatalf("ack first batch: %v", err)
	}
	second, err := store.ReceiveEventBatch(context.Background())
	if err != nil {
		t.Fatalf("receive second batch: %v", err)
	}
	if len(second.Events) != 1 {
		t.Fatalf("len(second.Events) = %d, want 1", len(second.Events))
	}
}

func TestRunEventWorkerConsumesFileOutbox(t *testing.T) {
	path := filepath.Join(t.TempDir(), "outbox.jsonl")
	store := New(path, WithJSONTypeDecoder[patientCreated]())
	if err := store.StoreEvents(context.Background(), []contracts.EventEnvelope{{
		Category: contracts.DomainEvent,
		Type:     patientCreatedType,
		Value:    patientCreated{ID: "patient-1"},
	}}); err != nil {
		t.Fatalf("store events: %v", err)
	}

	registry := contracts.NewRegistry()
	var handled string
	if err := contracts.RegisterDomainEvent[patientCreated](registry, func(ctx context.Context, event patientCreated) error {
		handled = event.ID
		return nil
	}, contracts.RoleWorker); err != nil {
		t.Fatalf("register event: %v", err)
	}
	if err := contracts.RunEventWorker(context.Background(), registry, store); err != nil {
		t.Fatalf("run event worker: %v", err)
	}
	if handled != "patient-1" {
		t.Fatalf("handled = %q, want patient-1", handled)
	}
	records, err := store.Records(context.Background())
	if err != nil {
		t.Fatalf("records: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("len(records) = %d, want 0", len(records))
	}
}

func TestReceiveEventBatchDeliversDecodableRecordsAroundPoisonRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "outbox.jsonl")
	store := New(path, WithJSONTypeDecoder[patientCreated]())
	if err := store.StoreEvents(context.Background(), []contracts.EventEnvelope{
		{Category: contracts.DomainEvent, Type: patientCreatedType, Value: 42},
		{Category: contracts.DomainEvent, Type: patientCreatedType, Value: patientCreated{ID: "patient-1"}},
	}); err != nil {
		t.Fatalf("store events: %v", err)
	}

	batch, err := store.ReceiveEventBatch(context.Background())
	if err != nil {
		t.Fatalf("receive batch: %v", err)
	}
	if len(batch.Events) != 1 {
		t.Fatalf("len(batch.Events) = %d, want 1", len(batch.Events))
	}
	if value, ok := batch.Events[0].Value.(patientCreated); !ok || value.ID != "patient-1" {
		t.Fatalf("event value = %#v, want patientCreated patient-1", batch.Events[0].Value)
	}
	if err := batch.Ack(context.Background()); err != nil {
		t.Fatalf("ack batch: %v", err)
	}

	records, err := store.Records(context.Background())
	if err != nil {
		t.Fatalf("records: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("len(records) = %d, want 1 pending poison record", len(records))
	}
	if records[0].Attempts != 1 || !strings.Contains(records[0].LastError, "cannot be decoded") || records[0].LastAttemptAt == nil {
		t.Fatalf("unexpected poison record retry metadata: %#v", records[0])
	}
}

func TestReceiveEventBatchDeadLettersPoisonRecordAfterMaxAttempts(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "outbox.jsonl")
	deadLetterPath := filepath.Join(root, "dead-letter.jsonl")
	store := New(path, WithJSONTypeDecoder[patientCreated](), WithDeadLetter(deadLetterPath, 2))
	if err := store.StoreEvents(context.Background(), []contracts.EventEnvelope{
		{Category: contracts.DomainEvent, Type: patientCreatedType, Value: 42},
		{Category: contracts.DomainEvent, Type: patientCreatedType, Value: patientCreated{ID: "patient-1"}},
	}); err != nil {
		t.Fatalf("store events: %v", err)
	}

	for attempt := 1; attempt <= 2; attempt++ {
		batch, err := store.ReceiveEventBatch(context.Background())
		if err != nil {
			t.Fatalf("receive batch attempt %d: %v", attempt, err)
		}
		if len(batch.Events) != 1 {
			t.Fatalf("len(batch.Events) attempt %d = %d, want 1", attempt, len(batch.Events))
		}
	}

	records, err := store.Records(context.Background())
	if err != nil {
		t.Fatalf("records: %v", err)
	}
	for _, record := range records {
		if record.LastError != "" {
			t.Fatalf("expected poison record to leave pending outbox, still have %#v", record)
		}
	}
	dead, err := store.DeadLetterRecords(context.Background())
	if err != nil {
		t.Fatalf("dead letter records: %v", err)
	}
	if len(dead) != 1 {
		t.Fatalf("len(dead) = %d, want 1: %#v", len(dead), dead)
	}
	if dead[0].Attempts != 2 || !strings.Contains(dead[0].LastError, "cannot be decoded") {
		t.Fatalf("unexpected dead letter metadata: %#v", dead[0])
	}

	batch, err := store.ReceiveEventBatch(context.Background())
	if err != nil {
		t.Fatalf("receive after dead-letter: %v", err)
	}
	if len(batch.Events) != 1 {
		t.Fatalf("len(batch.Events) after dead-letter = %d, want 1", len(batch.Events))
	}
	if err := batch.Ack(context.Background()); err != nil {
		t.Fatalf("ack after dead-letter: %v", err)
	}
	if _, err := store.ReceiveEventBatch(context.Background()); !errors.Is(err, contracts.ErrEventSourceClosed) {
		t.Fatalf("receive after drain error = %v, want closed source", err)
	}
}

func TestReceiveEventBatchRequiresDecoder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "outbox.jsonl")
	store := New(path)
	if err := store.StoreEvents(context.Background(), []contracts.EventEnvelope{{
		Category: contracts.DomainEvent,
		Type:     patientCreatedType,
		Value:    patientCreated{ID: "patient-1"},
	}}); err != nil {
		t.Fatalf("store events: %v", err)
	}

	_, err := store.ReceiveEventBatch(context.Background())
	if err == nil || !strings.Contains(err.Error(), "no decoder") {
		t.Fatalf("expected decoder error, got %v", err)
	}
}

func TestSeenStoreSeenMarkSeenAndPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "seen.jsonl")
	store := NewSeenStore(path)
	store.now = func() time.Time { return time.Unix(123, 0).UTC() }

	alreadySeen, err := store.Seen(context.Background(), "event-1")
	if err != nil || alreadySeen {
		t.Fatalf("initial seen event-1 seen=%v err=%v, want false nil", alreadySeen, err)
	}
	if err := store.MarkSeen(context.Background(), "event-1"); err != nil {
		t.Fatalf("mark seen: %v", err)
	}
	alreadySeen, err = store.Seen(context.Background(), "event-1")
	if err != nil || !alreadySeen {
		t.Fatalf("seen event-1 after mark seen=%v err=%v, want true nil", alreadySeen, err)
	}

	reopened := NewSeenStore(path)
	alreadySeen, err = reopened.Seen(context.Background(), "event-1")
	if err != nil || !alreadySeen {
		t.Fatalf("reopened seen event-1 seen=%v err=%v, want true nil", alreadySeen, err)
	}
	records, err := reopened.readRecordsLocked()
	if err != nil {
		t.Fatalf("read seen records: %v", err)
	}
	if len(records) != 1 || records[0].ID != "event-1" {
		t.Fatalf("unexpected seen records: %#v", records)
	}
}

func TestSeenStoreMarkIfNew(t *testing.T) {
	path := filepath.Join(t.TempDir(), "seen.jsonl")
	store := NewSeenStore(path)

	fresh, err := store.MarkIfNew(context.Background(), "event-1")
	if err != nil || !fresh {
		t.Fatalf("first mark fresh=%v err=%v, want true nil", fresh, err)
	}
	fresh, err = store.MarkIfNew(context.Background(), "event-1")
	if err != nil || fresh {
		t.Fatalf("second mark fresh=%v err=%v, want false nil", fresh, err)
	}
}

func TestSeenStoreEvictsOldestRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "seen.jsonl")
	store := NewSeenStore(path, WithSeenLimit(1))

	if err := store.MarkSeen(context.Background(), "event-1"); err != nil {
		t.Fatalf("mark event-1: %v", err)
	}
	if err := store.MarkSeen(context.Background(), "event-2"); err != nil {
		t.Fatalf("mark event-2: %v", err)
	}
	alreadySeen, err := store.Seen(context.Background(), "event-1")
	if err != nil || alreadySeen {
		t.Fatalf("event-1 should be evicted, seen=%v err=%v", alreadySeen, err)
	}
}
