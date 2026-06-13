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
)

const defaultSeenLimit = 10000

// SeenRecord is one file-backed deduplication entry.
type SeenRecord struct {
	ID     string    `json:"id"`
	SeenAt time.Time `json:"seenAt"`
}

// SeenStore records delivered event IDs in a JSON Lines file. It is intended
// for local single-binary apps that also use fileoutbox.
type SeenStore struct {
	mu    sync.Mutex
	path  string
	limit int
	now   func() time.Time
}

// SeenOption configures a SeenStore.
type SeenOption func(*SeenStore)

// WithSeenLimit sets the maximum retained IDs. Non-positive values keep the
// default window.
func WithSeenLimit(limit int) SeenOption {
	return func(store *SeenStore) {
		if limit > 0 {
			store.limit = limit
		}
	}
}

// NewSeenStore creates a file-backed seen store at path.
func NewSeenStore(path string, options ...SeenOption) *SeenStore {
	store := &SeenStore{
		path:  path,
		limit: defaultSeenLimit,
		now:   time.Now,
	}
	for _, option := range options {
		if option != nil {
			option(store)
		}
	}
	return store
}

// MarkIfNew records id and reports whether it was not already present in the
// retained file window.
func (store *SeenStore) MarkIfNew(ctx context.Context, id string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if id == "" {
		return false, errors.New("event id is required")
	}
	store.mu.Lock()
	defer store.mu.Unlock()

	records, err := store.readRecordsLocked()
	if err != nil {
		return false, err
	}
	for _, record := range records {
		if record.ID == id {
			return false, nil
		}
	}
	records = append(records, SeenRecord{ID: id, SeenAt: store.now().UTC()})
	if len(records) > store.limit {
		records = append([]SeenRecord(nil), records[len(records)-store.limit:]...)
	}
	if err := os.MkdirAll(filepath.Dir(store.path), 0o755); err != nil {
		return false, err
	}
	return true, store.writeRecordsLocked(records)
}

func (store *SeenStore) readRecordsLocked() ([]SeenRecord, error) {
	file, err := os.Open(store.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	var records []SeenRecord
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		text := bytes.TrimSpace(scanner.Bytes())
		if len(text) == 0 {
			continue
		}
		var record SeenRecord
		if err := json.Unmarshal(text, &record); err != nil {
			return nil, fmt.Errorf("file seen store %s line %d is invalid: %w", store.path, line, err)
		}
		records = append(records, record)
	}
	return records, scanner.Err()
}

func (store *SeenStore) writeRecordsLocked(records []SeenRecord) error {
	if len(records) == 0 {
		if err := os.Remove(store.path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	file, err := os.OpenFile(store.path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
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
