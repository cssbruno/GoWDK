package contracts

import (
	"container/list"
	"context"
	"errors"
	"sync"
)

const defaultSeenStoreLimit = 10000

// MemorySeenStore is a bounded in-memory SeenStore for single-process apps,
// local development, and tests. It is process-local and intentionally not a
// durable delivery guarantee.
type MemorySeenStore struct {
	mu    sync.Mutex
	limit int
	order *list.List
	items map[string]*list.Element
}

// NewMemorySeenStore creates a bounded in-memory SeenStore. Non-positive
// limits use the default window.
func NewMemorySeenStore(limit int) *MemorySeenStore {
	if limit <= 0 {
		limit = defaultSeenStoreLimit
	}
	return &MemorySeenStore{
		limit: limit,
		order: list.New(),
		items: map[string]*list.Element{},
	}
}

// Seen reports whether id is present in the current memory window.
func (store *MemorySeenStore) Seen(ctx context.Context, id string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if id == "" {
		return false, errors.New("event id is required")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if element := store.items[id]; element != nil {
		store.order.MoveToBack(element)
		return true, nil
	}
	return false, nil
}

// MarkSeen records id in the current memory window.
func (store *MemorySeenStore) MarkSeen(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if id == "" {
		return errors.New("event id is required")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	store.markSeenLocked(id)
	return nil
}

// MarkIfNew records id and reports whether it had not been seen inside the
// current memory window.
func (store *MemorySeenStore) MarkIfNew(ctx context.Context, id string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if id == "" {
		return false, errors.New("event id is required")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if element := store.items[id]; element != nil {
		store.order.MoveToBack(element)
		return false, nil
	}
	store.markSeenLocked(id)
	return true, nil
}

func (store *MemorySeenStore) markSeenLocked(id string) {
	if element := store.items[id]; element != nil {
		store.order.MoveToBack(element)
		return
	}
	element := store.order.PushBack(id)
	store.items[id] = element
	for len(store.items) > store.limit {
		oldest := store.order.Front()
		if oldest == nil {
			break
		}
		delete(store.items, oldest.Value.(string))
		store.order.Remove(oldest)
	}
}
