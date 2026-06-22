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
		ID:       "event-1",
		Audience: []string{"tenant:clinic", "user:ada"},
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
	if records[0].EventID != "event-1" {
		t.Fatalf("record event ID = %q, want event-1", records[0].EventID)
	}
	if len(records[0].Audience) != 2 || records[0].Audience[0] != "tenant:clinic" || records[0].Audience[1] != "user:ada" {
		t.Fatalf("record audience = %#v, want tenant/user labels", records[0].Audience)
	}
	if records[0].ID == records[0].EventID {
		t.Fatalf("record ID should be distinct from event ID: %#v", records[0])
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

func TestStoreEventsFailedReplacementPreservesExistingRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "outbox.jsonl")
	store := New(path)
	if err := store.StoreEvents(context.Background(), []contracts.EventEnvelope{{
		ID:       "event-1",
		Category: contracts.DomainEvent,
		Type:     patientCreatedType,
		Value:    patientCreated{ID: "patient-1"},
	}}); err != nil {
		t.Fatalf("store initial event: %v", err)
	}

	renameErr := errors.New("rename failed")
	store.rename = func(_, _ string) error { return renameErr }
	err := store.StoreEvents(context.Background(), []contracts.EventEnvelope{{
		ID:       "event-2",
		Category: contracts.DomainEvent,
		Type:     patientCreatedType,
		Value:    patientCreated{ID: "patient-2"},
	}})
	if !errors.Is(err, renameErr) {
		t.Fatalf("store replacement error = %v, want %v", err, renameErr)
	}

	records, err := store.Records(context.Background())
	if err != nil {
		t.Fatalf("records after failed replacement: %v", err)
	}
	if len(records) != 1 || records[0].EventID != "event-1" {
		t.Fatalf("failed replacement should preserve only original record: %#v", records)
	}
}

func TestAppendRecordsToPathFailedReplacementPreservesExistingRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deadletter.jsonl")
	store := New(filepath.Join(t.TempDir(), "outbox.jsonl"))
	if err := store.appendRecordsToPathLocked(path, []Record{{
		ID:       "record-1",
		EventID:  "event-1",
		Category: contracts.DomainEvent,
		Type:     patientCreatedType,
		Value:    json.RawMessage(`{"id":"patient-1"}`),
	}}); err != nil {
		t.Fatalf("append initial record: %v", err)
	}

	renameErr := errors.New("rename failed")
	store.rename = func(_, _ string) error { return renameErr }
	err := store.appendRecordsToPathLocked(path, []Record{{
		ID:       "record-2",
		EventID:  "event-2",
		Category: contracts.DomainEvent,
		Type:     patientCreatedType,
		Value:    json.RawMessage(`{"id":"patient-2"}`),
	}})
	if !errors.Is(err, renameErr) {
		t.Fatalf("append replacement error = %v, want %v", err, renameErr)
	}

	records, err := store.readRecordsFromPathLocked(path)
	if err != nil {
		t.Fatalf("records after failed append replacement: %v", err)
	}
	if len(records) != 1 || records[0].EventID != "event-1" {
		t.Fatalf("failed append replacement should preserve only original record: %#v", records)
	}
}

func TestAppendRecordsToPathDeduplicatesByRecordID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deadletter.jsonl")
	store := New(filepath.Join(t.TempDir(), "outbox.jsonl"))
	first := Record{
		ID:       "record-1",
		EventID:  "event-1",
		Category: contracts.DomainEvent,
		Type:     patientCreatedType,
		Value:    json.RawMessage(`{"id":"patient-1"}`),
	}
	duplicate := first
	duplicate.EventID = "event-duplicate"
	duplicate.Value = json.RawMessage(`{"id":"patient-duplicate"}`)

	if err := store.appendRecordsToPathLocked(path, []Record{first}); err != nil {
		t.Fatalf("append initial record: %v", err)
	}
	if err := store.appendRecordsToPathLocked(path, []Record{duplicate}); err != nil {
		t.Fatalf("append duplicate record: %v", err)
	}

	records, err := store.readRecordsFromPathLocked(path)
	if err != nil {
		t.Fatalf("read records: %v", err)
	}
	if len(records) != 1 || records[0].EventID != "event-1" {
		t.Fatalf("duplicate dead-letter record should be ignored, got %#v", records)
	}
}

func TestReceiveEventBatchDecodesAndAcksRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "outbox.jsonl")
	store := New(path, WithJSONTypeDecoder[patientCreated]())
	if err := store.StoreEvents(context.Background(), []contracts.EventEnvelope{
		{Audience: []string{"tenant:clinic"}, Category: contracts.DomainEvent, Type: patientCreatedType, Value: patientCreated{ID: "patient-1"}},
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
	if len(batch.Events[0].Audience) != 1 || batch.Events[0].Audience[0] != "tenant:clinic" {
		t.Fatalf("first event audience = %#v, want tenant label", batch.Events[0].Audience)
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

func TestReceiveEventBatchNackRedactsLastError(t *testing.T) {
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
			err:  "subscriber failed password=hunter2",
			want: "password=[REDACTED]",
			leak: "hunter2",
		},
		{
			name: "non secret",
			err:  "subscriber failed with temporary outage",
			want: "subscriber failed with temporary outage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
			if err := batch.Nack(context.Background(), errors.New(tt.err)); err != nil {
				t.Fatalf("nack batch: %v", err)
			}
			records, err := store.Records(context.Background())
			if err != nil {
				t.Fatalf("records: %v", err)
			}
			if len(records) != 1 {
				t.Fatalf("len(records) = %d, want 1", len(records))
			}
			if !strings.Contains(records[0].LastError, tt.want) {
				t.Fatalf("last error = %q, want it to contain %q", records[0].LastError, tt.want)
			}
			if tt.leak != "" && strings.Contains(records[0].LastError, tt.leak) {
				t.Fatalf("last error leaked %q: %q", tt.leak, records[0].LastError)
			}
		})
	}
}

func TestReceiveEventBatchAckKeepsDuplicateEventIDRowsDistinct(t *testing.T) {
	path := filepath.Join(t.TempDir(), "outbox.jsonl")
	store := New(path, WithBatchSize(1), WithJSONTypeDecoder[patientCreated]())
	if err := store.StoreEvents(context.Background(), []contracts.EventEnvelope{
		{ID: "event-1", Category: contracts.DomainEvent, Type: patientCreatedType, Value: patientCreated{ID: "patient-1"}},
		{ID: "event-1", Category: contracts.DomainEvent, Type: patientCreatedType, Value: patientCreated{ID: "patient-2"}},
	}); err != nil {
		t.Fatalf("store events: %v", err)
	}

	first, err := store.ReceiveEventBatch(context.Background())
	if err != nil {
		t.Fatalf("receive first batch: %v", err)
	}
	if len(first.Events) != 1 || first.Events[0].ID != "event-1" {
		t.Fatalf("unexpected first batch: %#v", first.Events)
	}
	if err := first.Ack(context.Background()); err != nil {
		t.Fatalf("ack first batch: %v", err)
	}
	records, err := store.Records(context.Background())
	if err != nil {
		t.Fatalf("records after first ack: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("len(records) after first ack = %d, want 1: %#v", len(records), records)
	}
	if records[0].EventID != "event-1" || records[0].ID == "event-1" {
		t.Fatalf("remaining record should keep duplicate event ID with distinct record ID: %#v", records[0])
	}

	second, err := store.ReceiveEventBatch(context.Background())
	if err != nil {
		t.Fatalf("receive second batch: %v", err)
	}
	if len(second.Events) != 1 || second.Events[0].ID != "event-1" {
		t.Fatalf("unexpected second batch: %#v", second.Events)
	}
}

func TestReceiveEventBatchNackKeepsDuplicateEventIDRowsDistinct(t *testing.T) {
	path := filepath.Join(t.TempDir(), "outbox.jsonl")
	store := New(path, WithBatchSize(1), WithJSONTypeDecoder[patientCreated]())
	if err := store.StoreEvents(context.Background(), []contracts.EventEnvelope{
		{ID: "event-1", Category: contracts.DomainEvent, Type: patientCreatedType, Value: patientCreated{ID: "patient-1"}},
		{ID: "event-1", Category: contracts.DomainEvent, Type: patientCreatedType, Value: patientCreated{ID: "patient-2"}},
	}); err != nil {
		t.Fatalf("store events: %v", err)
	}

	first, err := store.ReceiveEventBatch(context.Background())
	if err != nil {
		t.Fatalf("receive first batch: %v", err)
	}
	nackErr := errors.New("subscriber failed")
	if err := first.Nack(context.Background(), nackErr); err != nil {
		t.Fatalf("nack first batch: %v", err)
	}
	records, err := store.Records(context.Background())
	if err != nil {
		t.Fatalf("records after first nack: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("len(records) after first nack = %d, want 2: %#v", len(records), records)
	}
	var retried, untouched int
	for _, record := range records {
		if record.EventID != "event-1" || record.ID == "event-1" {
			t.Fatalf("unexpected duplicate event record identity: %#v", record)
		}
		if record.Attempts == 1 && record.LastError == nackErr.Error() {
			retried++
		}
		if record.Attempts == 0 && record.LastError == "" {
			untouched++
		}
	}
	if retried != 1 || untouched != 1 {
		t.Fatalf("retry metadata should affect one row only, retried=%d untouched=%d records=%#v", retried, untouched, records)
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

func TestReceiveEventBatchDoesNotDuplicateDeadLetterAfterPendingRewriteFailure(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "outbox.jsonl")
	deadLetterPath := filepath.Join(root, "dead-letter.jsonl")
	store := New(path, WithBatchSize(1), WithJSONTypeDecoder[patientCreated](), WithDeadLetter(deadLetterPath, 1))
	if err := store.StoreEvents(context.Background(), []contracts.EventEnvelope{
		{
			Category: contracts.DomainEvent,
			Type:     patientCreatedType,
			Value:    patientCreated{ID: "patient-1"},
		},
		{
			Category: contracts.DomainEvent,
			Type:     patientCreatedType,
			Value:    patientCreated{ID: "patient-2"},
		},
	}); err != nil {
		t.Fatalf("store events: %v", err)
	}

	renameErr := errors.New("pending rewrite failed")
	var renames int
	store.rename = func(old, new string) error {
		renames++
		if renames == 2 {
			return renameErr
		}
		return os.Rename(old, new)
	}

	first, err := store.ReceiveEventBatch(context.Background())
	if err != nil {
		t.Fatalf("receive first batch: %v", err)
	}
	if err := first.Nack(context.Background(), errors.New("subscriber failed password=hunter2")); !errors.Is(err, renameErr) {
		t.Fatalf("first nack error = %v, want %v", err, renameErr)
	}
	records, err := store.Records(context.Background())
	if err != nil {
		t.Fatalf("records after failed pending rewrite: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("pending records after failed rewrite = %#v, want two", records)
	}
	dead, err := store.DeadLetterRecords(context.Background())
	if err != nil {
		t.Fatalf("dead letter after failed pending rewrite: %v", err)
	}
	if len(dead) != 1 {
		t.Fatalf("dead letter records after failed pending rewrite = %#v, want one", dead)
	}
	if strings.Contains(dead[0].LastError, "hunter2") || !strings.Contains(dead[0].LastError, "password=[REDACTED]") {
		t.Fatalf("dead letter error was not redacted: %q", dead[0].LastError)
	}

	second, err := store.ReceiveEventBatch(context.Background())
	if err != nil {
		t.Fatalf("receive retry batch: %v", err)
	}
	if err := second.Nack(context.Background(), errors.New("subscriber failed password=hunter2")); err != nil {
		t.Fatalf("retry nack: %v", err)
	}
	dead, err = store.DeadLetterRecords(context.Background())
	if err != nil {
		t.Fatalf("dead letter after retry: %v", err)
	}
	if len(dead) != 1 {
		t.Fatalf("dead letter records after retry = %#v, want one deduplicated record", dead)
	}
	records, err = store.Records(context.Background())
	if err != nil {
		t.Fatalf("records after retry: %v", err)
	}
	if len(records) != 1 || records[0].EventID == dead[0].EventID {
		t.Fatalf("pending records after retry = %#v, want only the untouched second record", records)
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

func TestStoreEventsRejectsRecordLargerThanReaderLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "outbox.jsonl")
	store := New(path)

	err := store.StoreEvents(context.Background(), []contracts.EventEnvelope{{
		Category: contracts.DomainEvent,
		Type:     patientCreatedType,
		Value:    strings.Repeat("x", maxJSONLineBytes),
	}})
	if err == nil || !strings.Contains(err.Error(), "file outbox record exceeds") {
		t.Fatalf("store oversized event error = %v, want record-size error", err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("oversized event should not create outbox file, stat err=%v", statErr)
	}
}

func TestStoreEventsRejectsOversizedRecordWithoutPoisoningLaterDelivery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "outbox.jsonl")
	store := New(path, WithJSONTypeDecoder[patientCreated]())

	err := store.StoreEvents(context.Background(), []contracts.EventEnvelope{{
		Category: contracts.DomainEvent,
		Type:     patientCreatedType,
		Value:    strings.Repeat("x", maxJSONLineBytes),
	}})
	if err == nil || !strings.Contains(err.Error(), "file outbox record exceeds") {
		t.Fatalf("store oversized event error = %v, want record-size error", err)
	}
	if err := store.StoreEvents(context.Background(), []contracts.EventEnvelope{{
		Category: contracts.DomainEvent,
		Type:     patientCreatedType,
		Value:    patientCreated{ID: "patient-1"},
	}}); err != nil {
		t.Fatalf("store later event: %v", err)
	}

	batch, err := store.ReceiveEventBatch(context.Background())
	if err != nil {
		t.Fatalf("receive batch after oversized attempt: %v", err)
	}
	if len(batch.Events) != 1 {
		t.Fatalf("len(batch.Events) = %d, want 1", len(batch.Events))
	}
	value, ok := batch.Events[0].Value.(patientCreated)
	if !ok || value.ID != "patient-1" {
		t.Fatalf("delivered event value = %#v, want patientCreated patient-1", batch.Events[0].Value)
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

func TestSeenStoreFailedReplacementPreservesExistingRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "seen.jsonl")
	store := NewSeenStore(path)
	if err := store.MarkSeen(context.Background(), "event-1"); err != nil {
		t.Fatalf("mark initial seen: %v", err)
	}

	renameErr := errors.New("rename failed")
	store.rename = func(_, _ string) error { return renameErr }
	err := store.MarkSeen(context.Background(), "event-2")
	if !errors.Is(err, renameErr) {
		t.Fatalf("mark replacement error = %v, want %v", err, renameErr)
	}

	reopened := NewSeenStore(path)
	event1Seen, err := reopened.Seen(context.Background(), "event-1")
	if err != nil {
		t.Fatalf("seen event-1 after failed replacement: %v", err)
	}
	event2Seen, err := reopened.Seen(context.Background(), "event-2")
	if err != nil {
		t.Fatalf("seen event-2 after failed replacement: %v", err)
	}
	if !event1Seen || event2Seen {
		t.Fatalf("failed replacement should preserve event-1 only, event1=%v event2=%v", event1Seen, event2Seen)
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
