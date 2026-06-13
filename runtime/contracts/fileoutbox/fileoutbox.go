// Package fileoutbox provides a dependency-free JSON Lines outbox adapter for
// runtime/contracts.
package fileoutbox

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/cssbruno/gowdk/runtime/contracts"
)

const defaultBatchSize = 100

// Decoder converts one persisted JSON payload back into the typed Go event
// value expected by runtime/contracts subscribers.
type Decoder = contracts.EventDecoder

// Record is one durable outbox row stored as a JSON Lines object.
type Record struct {
	ID            string                  `json:"id"`
	StoredAt      time.Time               `json:"storedAt"`
	Category      contracts.EventCategory `json:"category"`
	Type          string                  `json:"type"`
	Value         json.RawMessage         `json:"value"`
	Attempts      int                     `json:"attempts,omitempty"`
	LastAttemptAt *time.Time              `json:"lastAttemptAt,omitempty"`
	LastError     string                  `json:"lastError,omitempty"`
}

// Store appends event envelopes to a JSON Lines file and can replay them as an
// EventSource. Ack removes delivered records; Nack records retry metadata and
// leaves records for later delivery.
//
// Store synchronizes access within a single process only. Pointing multiple
// processes at the same outbox file is not supported and can lose or
// double-deliver records.
type Store struct {
	mu             sync.Mutex
	path           string
	batchSize      int
	deadLetterPath string
	maxAttempts    int
	decoders       map[string]Decoder
	now            func() time.Time
}

// Option configures a Store.
type Option func(*Store)

// WithBatchSize sets the maximum number of records returned by one worker
// batch. Non-positive values keep the default.
func WithBatchSize(size int) Option {
	return func(store *Store) {
		if size > 0 {
			store.batchSize = size
		}
	}
}

// WithDeadLetter moves records to deadLetterPath after maxAttempts failed
// deliveries. Non-positive maxAttempts or an empty path disables dead-lettering.
func WithDeadLetter(deadLetterPath string, maxAttempts int) Option {
	return func(store *Store) {
		if deadLetterPath != "" && maxAttempts > 0 {
			store.deadLetterPath = deadLetterPath
			store.maxAttempts = maxAttempts
		}
	}
}

// WithDecoder registers a decoder for one stored event type.
func WithDecoder(eventType string, decoder Decoder) Option {
	return func(store *Store) {
		if eventType != "" && decoder != nil {
			store.decoders[eventType] = decoder
		}
	}
}

// WithJSONDecoder registers a JSON decoder for one stored event type.
func WithJSONDecoder[T any](eventType string) Option {
	return WithDecoder(eventType, contracts.JSONEventDecoder[T]())
}

// WithJSONTypeDecoder registers a JSON decoder using the same Go type name
// stored by runtime/contracts when T is emitted.
func WithJSONTypeDecoder[T any]() Option {
	return WithJSONDecoder[T](contracts.ContractName[T]())
}

// New creates a file-backed outbox at path.
func New(path string, options ...Option) *Store {
	store := &Store{
		path:      path,
		batchSize: defaultBatchSize,
		decoders:  map[string]Decoder{},
		now:       time.Now,
	}
	for _, option := range options {
		if option != nil {
			option(store)
		}
	}
	return store
}

// StoreEvents appends events to the outbox file.
func (store *Store) StoreEvents(ctx context.Context, events []contracts.EventEnvelope) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(events) == 0 {
		return nil
	}
	store.mu.Lock()
	defer store.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(store.path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(store.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}

	encoder := json.NewEncoder(file)
	for _, event := range events {
		event = contracts.EnsureEventID(event)
		value, err := json.Marshal(event.Value)
		if err != nil {
			file.Close()
			return err
		}
		record := Record{
			ID:       event.ID,
			StoredAt: store.now().UTC(),
			Category: event.Category,
			Type:     event.Type,
			Value:    value,
		}
		if err := encoder.Encode(record); err != nil {
			file.Close()
			return err
		}
	}
	return file.Close()
}

// Records returns all currently pending outbox records.
func (store *Store) Records(ctx context.Context) ([]Record, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.readRecordsLocked()
}

// DeadLetterRecords returns records moved out of the pending outbox by the
// configured dead-letter policy.
func (store *Store) DeadLetterRecords(ctx context.Context) ([]Record, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if store.deadLetterPath == "" {
		return nil, nil
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.readRecordsFromPathLocked(store.deadLetterPath)
}

// ReceiveEventBatch returns the next pending records as typed event envelopes.
// It returns contracts.ErrEventSourceClosed when the outbox is empty.
func (store *Store) ReceiveEventBatch(ctx context.Context) (contracts.EventBatch, error) {
	if err := ctx.Err(); err != nil {
		return contracts.EventBatch{}, err
	}
	store.mu.Lock()
	records, err := store.readRecordsLocked()
	if err != nil {
		store.mu.Unlock()
		return contracts.EventBatch{}, err
	}
	if len(records) == 0 {
		store.mu.Unlock()
		return contracts.EventBatch{}, contracts.ErrEventSourceClosed
	}
	limit := store.batchSize
	if limit > len(records) {
		limit = len(records)
	}
	selected := append([]Record(nil), records[:limit]...)
	events, acked, failed, decodeErr := store.decodeRecordsLocked(selected)
	if len(failed) > 0 {
		if err := store.markRecordsFailedLocked(failed, decodeErr); err != nil {
			store.mu.Unlock()
			return contracts.EventBatch{}, err
		}
	}
	store.mu.Unlock()
	if len(events) == 0 {
		return contracts.EventBatch{}, decodeErr
	}

	return contracts.EventBatch{
		Events: events,
		Ack: func(ctx context.Context) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			store.mu.Lock()
			defer store.mu.Unlock()
			return store.removeRecordsLocked(acked)
		},
		Nack: func(ctx context.Context, cause error) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			store.mu.Lock()
			defer store.mu.Unlock()
			return store.markRecordsFailedLocked(acked, cause)
		},
	}, nil
}

// decodeRecordsLocked decodes records into typed envelopes. Records that have
// no decoder or fail to decode are reported as failed instead of failing the
// whole batch so one poison record cannot wedge delivery of the rest; failed
// records flow through the regular Nack retry and dead-letter machinery.
func (store *Store) decodeRecordsLocked(records []Record) ([]contracts.EventEnvelope, map[string]bool, map[string]bool, error) {
	events := make([]contracts.EventEnvelope, 0, len(records))
	decoded := map[string]bool{}
	failed := map[string]bool{}
	var failure error
	for _, record := range records {
		decoder := store.decoders[record.Type]
		if decoder == nil {
			failed[record.ID] = true
			failure = errors.Join(failure, fmt.Errorf("file outbox event %s has no decoder", record.Type))
			continue
		}
		value, err := decoder(record.Value)
		if err != nil {
			failed[record.ID] = true
			failure = errors.Join(failure, fmt.Errorf("file outbox event %s cannot be decoded: %w", record.Type, err))
			continue
		}
		decoded[record.ID] = true
		events = append(events, contracts.EventEnvelope{
			ID:       record.ID,
			Category: record.Category,
			Type:     record.Type,
			Value:    value,
		})
	}
	return events, decoded, failed, failure
}

func (store *Store) readRecordsLocked() ([]Record, error) {
	return store.readRecordsFromPathLocked(store.path)
}

func (store *Store) readRecordsFromPathLocked(path string) ([]Record, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	var records []Record
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		text := bytes.TrimSpace(scanner.Bytes())
		if len(text) == 0 {
			continue
		}
		var record Record
		if err := json.Unmarshal(text, &record); err != nil {
			return nil, fmt.Errorf("file outbox %s line %d is invalid: %w", path, line, err)
		}
		record.Value = append(json.RawMessage(nil), record.Value...)
		records = append(records, record)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

func (store *Store) removeRecordsLocked(remove map[string]bool) error {
	records, err := store.readRecordsLocked()
	if err != nil {
		return err
	}
	var kept []Record
	for _, record := range records {
		if !remove[record.ID] {
			kept = append(kept, record)
		}
	}
	return store.writeRecordsLocked(kept)
}

func (store *Store) markRecordsFailedLocked(mark map[string]bool, cause error) error {
	records, err := store.readRecordsLocked()
	if err != nil {
		return err
	}
	now := store.now().UTC()
	message := ""
	if cause != nil {
		message = cause.Error()
	}
	var kept []Record
	var dead []Record
	for _, record := range records {
		if !mark[record.ID] {
			kept = append(kept, record)
			continue
		}
		record.Attempts++
		record.LastAttemptAt = &now
		record.LastError = message
		if store.deadLetterPath != "" && store.maxAttempts > 0 && record.Attempts >= store.maxAttempts {
			dead = append(dead, record)
			continue
		}
		kept = append(kept, record)
	}
	if len(dead) > 0 {
		if err := store.appendRecordsToPathLocked(store.deadLetterPath, dead); err != nil {
			return err
		}
	}
	return store.writeRecordsLocked(kept)
}

func (store *Store) writeRecordsLocked(records []Record) error {
	if len(records) == 0 {
		if err := os.Remove(store.path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(store.path), 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(store.path), ".gowdk-outbox-*")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	encoder := json.NewEncoder(temp)
	for _, record := range records {
		if err := encoder.Encode(record); err != nil {
			temp.Close()
			os.Remove(tempName)
			return err
		}
	}
	if err := temp.Close(); err != nil {
		os.Remove(tempName)
		return err
	}
	return os.Rename(tempName, store.path)
}

func (store *Store) appendRecordsToPathLocked(path string, records []Record) error {
	if len(records) == 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(file)
	for _, record := range records {
		if err := encoder.Encode(record); err != nil {
			file.Close()
			return err
		}
	}
	return file.Close()
}
