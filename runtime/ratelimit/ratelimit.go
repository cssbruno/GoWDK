// Package ratelimit provides HTTP rate limiting for generated or user-owned
// request-time handlers.
package ratelimit

import (
	"context"
	"fmt"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cssbruno/gowdk/runtime/response"
)

const (
	defaultCleanupInterval = time.Minute

	// HeaderLimit reports the configured request limit for the current window.
	HeaderLimit = "X-RateLimit-Limit"
	// HeaderRemaining reports how many requests remain in the current window.
	HeaderRemaining = "X-RateLimit-Remaining"
	// HeaderReset reports the Unix timestamp when the current window resets.
	HeaderReset = "X-RateLimit-Reset"
)

// KeyFunc returns the storage key for a request.
type KeyFunc func(*http.Request) string

// Store records fixed-window request hits. Redis-backed implementations should
// make Take atomic for one key.
type Store interface {
	Take(ctx context.Context, key string, limit int, window time.Duration, now time.Time) (Result, error)
}

// Result describes one rate-limit decision.
type Result struct {
	Key        string
	Limit      int
	Remaining  int
	Reset      time.Time
	RetryAfter time.Duration
	Allowed    bool
}

// LimitHandler writes the response for blocked requests.
type LimitHandler func(http.ResponseWriter, *http.Request, Result)

// ErrorHandler writes the response for rate-limit store or configuration
// failures.
type ErrorHandler func(http.ResponseWriter, *http.Request, error)

// Options configures a Limiter.
type Options struct {
	Limit        int
	Window       time.Duration
	Store        Store
	KeyFunc      KeyFunc
	LimitHandler LimitHandler
	ErrorHandler ErrorHandler
	Now          func() time.Time
}

// Limiter applies fixed-window rate limits to HTTP requests.
type Limiter struct {
	limit        int
	window       time.Duration
	store        Store
	keyFunc      KeyFunc
	limitHandler LimitHandler
	errorHandler ErrorHandler
	now          func() time.Time
}

// New creates a fixed-window limiter.
func New(options Options) (*Limiter, error) {
	if options.Limit < 1 {
		return nil, fmt.Errorf("rate limit must be at least 1")
	}
	if options.Window <= 0 {
		return nil, fmt.Errorf("rate limit window must be greater than zero")
	}
	if options.Store == nil {
		return nil, fmt.Errorf("rate limit store is required")
	}
	keyFunc := options.KeyFunc
	if keyFunc == nil {
		keyFunc = KeyByRemoteAddr
	}
	limitHandler := options.LimitHandler
	if limitHandler == nil {
		limitHandler = DefaultLimitHandler
	}
	errorHandler := options.ErrorHandler
	if errorHandler == nil {
		errorHandler = DefaultErrorHandler
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &Limiter{
		limit:        options.Limit,
		window:       options.Window,
		store:        options.Store,
		keyFunc:      keyFunc,
		limitHandler: limitHandler,
		errorHandler: errorHandler,
		now:          now,
	}, nil
}

// AllowRequest records a request and returns the rate-limit decision.
func (limiter *Limiter) AllowRequest(request *http.Request) (Result, error) {
	if request == nil {
		return Result{}, fmt.Errorf("rate limit request is required")
	}
	key := strings.TrimSpace(limiter.keyFunc(request))
	if key == "" {
		return Result{}, fmt.Errorf("rate limit key is empty")
	}
	return limiter.store.Take(request.Context(), key, limiter.limit, limiter.window, limiter.now())
}

// HandleError writes the configured rate-limit store-error response.
func (limiter *Limiter) HandleError(writer http.ResponseWriter, request *http.Request, err error) {
	if limiter == nil || limiter.errorHandler == nil {
		DefaultErrorHandler(writer, request, err)
		return
	}
	limiter.errorHandler(writer, request, err)
}

// HandleLimit writes the configured blocked-request response.
func (limiter *Limiter) HandleLimit(writer http.ResponseWriter, request *http.Request, result Result) {
	if limiter == nil || limiter.limitHandler == nil {
		DefaultLimitHandler(writer, request, result)
		return
	}
	limiter.limitHandler(writer, request, result)
}

// Middleware wraps an HTTP handler with rate limiting.
func (limiter *Limiter) Middleware(next http.Handler) http.Handler {
	if next == nil {
		next = http.NotFoundHandler()
	}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		result, err := limiter.AllowRequest(request)
		if err != nil {
			limiter.errorHandler(writer, request, err)
			return
		}
		WriteHeaders(writer, result)
		if !result.Allowed {
			limiter.limitHandler(writer, request, result)
			return
		}
		next.ServeHTTP(writer, request)
	})
}

// KeyByRemoteAddr returns the request RemoteAddr host. It intentionally ignores
// forwarded headers because those are only safe behind trusted proxy handling.
func KeyByRemoteAddr(request *http.Request) string {
	if request == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err == nil {
		return host
	}
	return strings.TrimSpace(request.RemoteAddr)
}

// WriteHeaders writes rate-limit metadata response headers.
func WriteHeaders(writer http.ResponseWriter, result Result) {
	header := writer.Header()
	header.Set(HeaderLimit, strconv.Itoa(result.Limit))
	header.Set(HeaderRemaining, strconv.Itoa(max(result.Remaining, 0)))
	if !result.Reset.IsZero() {
		header.Set(HeaderReset, strconv.FormatInt(result.Reset.Unix(), 10))
	}
	if !result.Allowed {
		header.Set("Retry-After", strconv.FormatInt(retryAfterSeconds(result.RetryAfter), 10))
	}
}

// DefaultLimitHandler writes HTTP 429 for blocked requests.
func DefaultLimitHandler(writer http.ResponseWriter, _ *http.Request, _ Result) {
	http.Error(writer, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
}

// DefaultErrorHandler writes HTTP 500 for limiter failures.
func DefaultErrorHandler(writer http.ResponseWriter, _ *http.Request, err error) {
	_ = err
	response.WriteNoStoreError(writer, http.StatusInternalServerError, "GOWDK rate limit error")
}

// InMemoryOptions configures the in-memory store.
type InMemoryOptions struct {
	CleanupInterval time.Duration
}

// InMemoryStore stores fixed-window counters in the current process.
type InMemoryStore struct {
	mu              sync.Mutex
	entries         map[string]memoryEntry
	cleanupInterval time.Duration
	nextCleanup     time.Time
}

type memoryEntry struct {
	count int
	reset time.Time
}

// NewInMemoryStore creates a concurrency-safe process-local store.
func NewInMemoryStore(options InMemoryOptions) *InMemoryStore {
	cleanupInterval := options.CleanupInterval
	if cleanupInterval <= 0 {
		cleanupInterval = defaultCleanupInterval
	}
	return &InMemoryStore{
		entries:         map[string]memoryEntry{},
		cleanupInterval: cleanupInterval,
	}
}

// Take records one hit against a fixed window.
func (store *InMemoryStore) Take(ctx context.Context, key string, limit int, window time.Duration, now time.Time) (Result, error) {
	select {
	case <-ctx.Done():
		return Result{}, ctx.Err()
	default:
	}
	if strings.TrimSpace(key) == "" {
		return Result{}, fmt.Errorf("rate limit key is empty")
	}
	if limit < 1 {
		return Result{}, fmt.Errorf("rate limit must be at least 1")
	}
	if window <= 0 {
		return Result{}, fmt.Errorf("rate limit window must be greater than zero")
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	if store.entries == nil {
		store.entries = map[string]memoryEntry{}
	}
	if store.cleanupInterval <= 0 {
		store.cleanupInterval = defaultCleanupInterval
	}

	entry := store.entries[key]
	if entry.count == 0 || !now.Before(entry.reset) {
		entry = memoryEntry{reset: now.Add(window)}
	}
	entry.count++
	store.entries[key] = entry
	store.cleanupExpired(now)

	remaining := limit - entry.count
	if remaining < 0 {
		remaining = 0
	}
	result := Result{
		Key:       key,
		Limit:     limit,
		Remaining: remaining,
		Reset:     entry.reset,
		Allowed:   entry.count <= limit,
	}
	if !result.Allowed {
		result.RetryAfter = entry.reset.Sub(now)
		if result.RetryAfter < 0 {
			result.RetryAfter = 0
		}
	}
	return result, nil
}

func (store *InMemoryStore) cleanupExpired(now time.Time) {
	if !store.nextCleanup.IsZero() && now.Before(store.nextCleanup) {
		return
	}
	for key, entry := range store.entries {
		if !now.Before(entry.reset) {
			delete(store.entries, key)
		}
	}
	store.nextCleanup = now.Add(store.cleanupInterval)
}

func retryAfterSeconds(duration time.Duration) int64 {
	if duration <= 0 {
		return 0
	}
	return int64(math.Ceil(duration.Seconds()))
}
