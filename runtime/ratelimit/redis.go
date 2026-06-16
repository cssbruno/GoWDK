package ratelimit

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	defaultRedisKeyPrefix = "gowdk:ratelimit:"

	redisFixedWindowScript = `
local limit = tonumber(ARGV[1])
local window_ms = tonumber(ARGV[2])
local now_ms = tonumber(ARGV[3])
local reset_ms = now_ms + window_ms
local current = redis.call("INCR", KEYS[1])

if current == 1 then
	redis.call("PEXPIREAT", KEYS[1], reset_ms)
else
	local ttl = redis.call("PTTL", KEYS[1])
	if ttl < 0 then
		redis.call("PEXPIREAT", KEYS[1], reset_ms)
	else
		reset_ms = now_ms + ttl
	end
end

local remaining = limit - current
if remaining < 0 then
	remaining = 0
end

local allowed = 0
if current <= limit then
	allowed = 1
end

return {allowed, remaining, reset_ms}
`
)

// RedisClient adapts a Redis client to the rate limiter's fixed-window script.
// Implementations should run script atomically and return the Lua integer array.
type RedisClient interface {
	EvalInt64s(ctx context.Context, script string, keys []string, args ...string) ([]int64, error)
}

// RedisOptions configures a Redis-backed store.
type RedisOptions struct {
	Client    RedisClient
	KeyPrefix string
}

// RedisStore stores fixed-window counters in Redis.
type RedisStore struct {
	client    RedisClient
	keyPrefix string
}

// NewRedisStore creates a Redis-backed store without depending on a concrete
// Redis client package.
func NewRedisStore(options RedisOptions) (*RedisStore, error) {
	if options.Client == nil {
		return nil, fmt.Errorf("redis rate limit client is required")
	}
	keyPrefix := options.KeyPrefix
	if keyPrefix == "" {
		keyPrefix = defaultRedisKeyPrefix
	}
	return &RedisStore{
		client:    options.Client,
		keyPrefix: keyPrefix,
	}, nil
}

// Take records one hit against a fixed window using one Redis Lua evaluation.
func (store *RedisStore) Take(ctx context.Context, key string, limit int, window time.Duration, now time.Time) (Result, error) {
	if strings.TrimSpace(key) == "" {
		return Result{}, fmt.Errorf("rate limit key is empty")
	}
	if limit < 1 {
		return Result{}, fmt.Errorf("rate limit must be at least 1")
	}
	if window <= 0 {
		return Result{}, fmt.Errorf("rate limit window must be greater than zero")
	}

	windowMillis := durationMillis(window)
	resetMillis := now.UnixMilli() + windowMillis
	values, err := store.client.EvalInt64s(
		ctx,
		redisFixedWindowScript,
		[]string{store.keyPrefix + key},
		strconv.Itoa(limit),
		strconv.FormatInt(windowMillis, 10),
		strconv.FormatInt(now.UnixMilli(), 10),
	)
	if err != nil {
		return Result{}, err
	}
	if len(values) < 3 {
		return Result{}, fmt.Errorf("redis rate limit reply must include allowed, remaining, and reset values")
	}
	if values[2] > 0 {
		resetMillis = values[2]
	}

	remaining := int(values[1])
	if remaining < 0 {
		remaining = 0
	}
	result := Result{
		Key:       key,
		Limit:     limit,
		Remaining: remaining,
		Reset:     time.UnixMilli(resetMillis),
		Allowed:   values[0] == 1,
	}
	if !result.Allowed {
		result.RetryAfter = result.Reset.Sub(now)
		if result.RetryAfter < 0 {
			result.RetryAfter = 0
		}
	}
	return result, nil
}

func durationMillis(duration time.Duration) int64 {
	millis := duration / time.Millisecond
	if duration%time.Millisecond != 0 {
		millis++
	}
	if millis < 1 {
		return 1
	}
	return int64(millis)
}
