package redisstream

import (
	"context"
	"errors"
	"time"

	redis "github.com/redis/go-redis/v9"
)

const (
	defaultSeenKeyPrefix = "gowdk:contracts:seen:"
	defaultSeenTTL       = 24 * time.Hour
)

type seenClient interface {
	Exists(ctx context.Context, keys ...string) *redis.IntCmd
	Set(ctx context.Context, key string, value any, expiration time.Duration) *redis.StatusCmd
	SetNX(ctx context.Context, key string, value any, expiration time.Duration) *redis.BoolCmd
}

// SeenStore records delivered event IDs in Redis with a TTL-backed
// deduplication window.
type SeenStore struct {
	client seenClient
	prefix string
	ttl    time.Duration
}

// NewSeenStore creates a Redis-backed SeenStore. Empty prefix and non-positive
// TTL values use local defaults.
func NewSeenStore(client *redis.Client, prefix string, ttl time.Duration) *SeenStore {
	return newSeenStore(client, prefix, ttl)
}

func newSeenStore(client seenClient, prefix string, ttl time.Duration) *SeenStore {
	if prefix == "" {
		prefix = defaultSeenKeyPrefix
	}
	if ttl <= 0 {
		ttl = defaultSeenTTL
	}
	return &SeenStore{client: client, prefix: prefix, ttl: ttl}
}

// Seen reports whether id is present inside the TTL window.
func (store *SeenStore) Seen(ctx context.Context, id string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if store.client == nil {
		return false, errors.New("redis seen store client is required")
	}
	if id == "" {
		return false, errors.New("event id is required")
	}
	count, err := store.client.Exists(ctx, store.prefix+id).Result()
	return count > 0, err
}

// MarkSeen records id inside the TTL window.
func (store *SeenStore) MarkSeen(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if store.client == nil {
		return errors.New("redis seen store client is required")
	}
	if id == "" {
		return errors.New("event id is required")
	}
	return store.client.Set(ctx, store.prefix+id, "1", store.ttl).Err()
}

// MarkIfNew records id with SETNX and returns false when the ID already exists
// inside the TTL window.
func (store *SeenStore) MarkIfNew(ctx context.Context, id string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if store.client == nil {
		return false, errors.New("redis seen store client is required")
	}
	if id == "" {
		return false, errors.New("event id is required")
	}
	return store.client.SetNX(ctx, store.prefix+id, "1", store.ttl).Result()
}
