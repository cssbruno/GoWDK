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

var patientCreatedType = typeName[patientCreated]()

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
