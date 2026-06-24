package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ErrSessionNotFound reports that a revocable session ID is unknown to the
// configured store.
var ErrSessionNotFound = errors.New("gowdk auth: session not found")

// SessionStore is the persistence boundary for revocable sessions. Applications
// can back it with a database, cache, or another durable service without adding
// a mandatory dependency to the GOWDK root module.
type SessionStore interface {
	CreateSession(context.Context, SessionRecord) error
	LookupSession(context.Context, string) (SessionRecord, error)
	RevokeSession(context.Context, string) error
	RevokePrincipal(context.Context, string) error
}

// SessionToucher lets a store slide idle expiry after a successful lookup.
type SessionToucher interface {
	TouchSession(context.Context, string, time.Time, time.Time) error
}

// SessionRecord is the server-owned state checked for every request in
// revocable mode. Principal roles and permissions are resolved from this record,
// not trusted from the signed cookie for the cookie's full lifetime.
type SessionRecord struct {
	ID                   string
	Principal            Principal
	AuthorizationVersion string
	CreatedAt            time.Time
	ExpiresAt            time.Time
	IdleExpiresAt        time.Time
	LastSeenAt           time.Time
	Revoked              bool
}

func (record SessionRecord) expired(now time.Time) bool {
	if !record.ExpiresAt.IsZero() && !now.Before(record.ExpiresAt) {
		return true
	}
	if !record.IdleExpiresAt.IsZero() && !now.Before(record.IdleExpiresAt) {
		return true
	}
	return false
}

// InMemorySessionStore is a dependency-free SessionStore for tests, examples,
// and single-process development. Production apps should use an app-owned
// durable store so revocation survives restarts and works across instances.
type InMemorySessionStore struct {
	mu       sync.RWMutex
	sessions map[string]SessionRecord
}

// NewInMemorySessionStore creates an empty in-memory revocation store.
func NewInMemorySessionStore() *InMemorySessionStore {
	return &InMemorySessionStore{
		sessions: map[string]SessionRecord{},
	}
}

// CreateSession records a new server-side session.
func (store *InMemorySessionStore) CreateSession(_ context.Context, record SessionRecord) error {
	if store == nil {
		return fmt.Errorf("gowdk auth: session store is nil")
	}
	record.ID = strings.TrimSpace(record.ID)
	record.Principal.ID = strings.TrimSpace(record.Principal.ID)
	if record.ID == "" {
		return fmt.Errorf("gowdk auth: session id is required")
	}
	if record.Principal.ID == "" {
		return fmt.Errorf("gowdk auth: principal id is required")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	store.sessions[record.ID] = cloneSessionRecord(record)
	return nil
}

// LookupSession returns the current server-side session state.
func (store *InMemorySessionStore) LookupSession(_ context.Context, id string) (SessionRecord, error) {
	if store == nil {
		return SessionRecord{}, fmt.Errorf("gowdk auth: session store is nil")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return SessionRecord{}, ErrSessionNotFound
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	record, ok := store.sessions[id]
	if !ok {
		return SessionRecord{}, ErrSessionNotFound
	}
	return cloneSessionRecord(record), nil
}

// RevokeSession marks one session ID invalid. It is idempotent.
func (store *InMemorySessionStore) RevokeSession(_ context.Context, id string) error {
	if store == nil {
		return fmt.Errorf("gowdk auth: session store is nil")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	record, ok := store.sessions[id]
	if !ok {
		return nil
	}
	record.Revoked = true
	store.sessions[id] = record
	return nil
}

// RevokePrincipal marks every current session for principalID invalid.
func (store *InMemorySessionStore) RevokePrincipal(_ context.Context, principalID string) error {
	if store == nil {
		return fmt.Errorf("gowdk auth: session store is nil")
	}
	principalID = strings.TrimSpace(principalID)
	if principalID == "" {
		return nil
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	for id, record := range store.sessions {
		if record.Principal.ID == principalID {
			record.Revoked = true
			store.sessions[id] = record
		}
	}
	return nil
}

// TouchSession updates LastSeenAt and IdleExpiresAt for a live session.
func (store *InMemorySessionStore) TouchSession(_ context.Context, id string, seenAt time.Time, idleExpiresAt time.Time) error {
	if store == nil {
		return fmt.Errorf("gowdk auth: session store is nil")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return ErrSessionNotFound
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	record, ok := store.sessions[id]
	if !ok {
		return ErrSessionNotFound
	}
	record.LastSeenAt = seenAt
	record.IdleExpiresAt = idleExpiresAt
	store.sessions[id] = record
	return nil
}

// UpdateSession applies update to one in-memory session record. It is intended
// for tests and examples that need to model role, permission, or account-state
// changes without a database.
func (store *InMemorySessionStore) UpdateSession(_ context.Context, id string, update func(*SessionRecord)) error {
	if store == nil {
		return fmt.Errorf("gowdk auth: session store is nil")
	}
	if update == nil {
		return nil
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return ErrSessionNotFound
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	record, ok := store.sessions[id]
	if !ok {
		return ErrSessionNotFound
	}
	record = cloneSessionRecord(record)
	update(&record)
	store.sessions[id] = cloneSessionRecord(record)
	return nil
}

func cloneSessionRecord(record SessionRecord) SessionRecord {
	record.Principal = clonePrincipal(record.Principal)
	return record
}

func clonePrincipal(principal Principal) Principal {
	principal.Roles = append([]string(nil), principal.Roles...)
	principal.Permissions = append([]string(nil), principal.Permissions...)
	return principal
}
