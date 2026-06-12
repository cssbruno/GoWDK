package ratelimit

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cssbruno/gowdk"
)

func TestAddonRegistersRateLimitFeature(t *testing.T) {
	addon := Addon()
	if addon.Name() != "ratelimit" {
		t.Fatalf("unexpected addon name: %q", addon.Name())
	}
	if !(gowdk.Config{Addons: []gowdk.Addon{addon}}).HasFeature(gowdk.FeatureRateLimit) {
		t.Fatal("expected ratelimit feature")
	}
}

func TestNewRejectsInvalidOptions(t *testing.T) {
	cases := []struct {
		name    string
		options Options
		message string
	}{
		{
			name:    "missing limit",
			options: Options{Window: time.Minute, Store: NewInMemoryStore(InMemoryOptions{})},
			message: "rate limit must be at least 1",
		},
		{
			name:    "missing window",
			options: Options{Limit: 1, Store: NewInMemoryStore(InMemoryOptions{})},
			message: "rate limit window must be greater than zero",
		},
		{
			name:    "missing store",
			options: Options{Limit: 1, Window: time.Minute},
			message: "rate limit store is required",
		},
	}

	for _, tc := range cases {
		_, err := New(tc.options)
		if err == nil || !strings.Contains(err.Error(), tc.message) {
			t.Fatalf("%s: expected %q error, got %v", tc.name, tc.message, err)
		}
	}
}

func TestInMemoryStoreLimitsAndResetsFixedWindow(t *testing.T) {
	store := NewInMemoryStore(InMemoryOptions{})
	now := time.Unix(1000, 0)
	ctx := context.Background()

	first, err := store.Take(ctx, "client", 2, time.Minute, now)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Allowed || first.Remaining != 1 || first.Reset != now.Add(time.Minute) {
		t.Fatalf("unexpected first result: %#v", first)
	}

	second, err := store.Take(ctx, "client", 2, time.Minute, now.Add(10*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if !second.Allowed || second.Remaining != 0 {
		t.Fatalf("unexpected second result: %#v", second)
	}

	blocked, err := store.Take(ctx, "client", 2, time.Minute, now.Add(20*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if blocked.Allowed || blocked.Remaining != 0 || blocked.RetryAfter != 40*time.Second {
		t.Fatalf("unexpected blocked result: %#v", blocked)
	}

	reset, err := store.Take(ctx, "client", 2, time.Minute, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if !reset.Allowed || reset.Remaining != 1 || reset.Reset != now.Add(2*time.Minute) {
		t.Fatalf("unexpected reset result: %#v", reset)
	}
}

func TestMiddlewareAllowsThenBlocksRequests(t *testing.T) {
	now := time.Unix(1000, 0)
	limiter, err := New(Options{
		Limit:  1,
		Window: time.Minute,
		Store:  NewInMemoryStore(InMemoryOptions{}),
		Now: func() time.Time {
			return now
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := limiter.Middleware(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))

	first := httptest.NewRecorder()
	firstRequest := httptest.NewRequest(http.MethodGet, "/api/patients", nil)
	firstRequest.RemoteAddr = "198.51.100.10:44000"
	handler.ServeHTTP(first, firstRequest)

	if first.Code != http.StatusNoContent {
		t.Fatalf("expected allowed request, got %d", first.Code)
	}
	if first.Header().Get(HeaderLimit) != "1" || first.Header().Get(HeaderRemaining) != "0" || first.Header().Get(HeaderReset) != "1060" {
		t.Fatalf("missing rate-limit headers: %#v", first.Header())
	}

	second := httptest.NewRecorder()
	secondRequest := httptest.NewRequest(http.MethodGet, "/api/patients", nil)
	secondRequest.RemoteAddr = "198.51.100.10:44001"
	handler.ServeHTTP(second, secondRequest)

	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("expected blocked request, got %d", second.Code)
	}
	if second.Header().Get("Retry-After") != "60" {
		t.Fatalf("unexpected retry header: %#v", second.Header())
	}
	if !strings.Contains(second.Body.String(), http.StatusText(http.StatusTooManyRequests)) {
		t.Fatalf("unexpected response body: %q", second.Body.String())
	}
}

func TestMiddlewareHidesStoreErrorDetails(t *testing.T) {
	expected := errors.New("redis unavailable password=secret")
	limiter, err := New(Options{
		Limit:  1,
		Window: time.Minute,
		Store:  failingStore{err: expected},
	})
	if err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "198.51.100.10:44000"
	limiter.Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next handler should not run")
	})).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "GOWDK rate limit error") {
		t.Fatalf("unexpected error body: %q", recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "redis unavailable") || strings.Contains(recorder.Body.String(), "secret") {
		t.Fatalf("internal error leaked in response body: %q", recorder.Body.String())
	}
	if cache := recorder.Header().Get("Cache-Control"); cache != "no-store" {
		t.Fatalf("expected no-store error response, got %q", cache)
	}
}

func TestKeyByRemoteAddrIgnoresForwardedHeaders(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "198.51.100.10:44000"
	request.Header.Set("X-Forwarded-For", "203.0.113.99")

	if got := KeyByRemoteAddr(request); got != "198.51.100.10" {
		t.Fatalf("unexpected key: %q", got)
	}
}

func TestNewRedisStoreRejectsMissingClient(t *testing.T) {
	_, err := NewRedisStore(RedisOptions{})
	if err == nil || !strings.Contains(err.Error(), "redis rate limit client is required") {
		t.Fatalf("expected missing client error, got %v", err)
	}
}

func TestRedisStoreRunsAtomicFixedWindowScript(t *testing.T) {
	client := &recordingRedisClient{values: []int64{1, 2, 2500}}
	store, err := NewRedisStore(RedisOptions{
		Client:    client,
		KeyPrefix: "test:",
	})
	if err != nil {
		t.Fatal(err)
	}

	now := time.UnixMilli(1000)
	result, err := store.Take(context.Background(), "client", 3, 1500*time.Millisecond, now)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(client.script, `redis.call("INCR", KEYS[1])`) || !strings.Contains(client.script, `redis.call("PEXPIREAT", KEYS[1], reset_ms)`) {
		t.Fatalf("expected atomic fixed-window script, got %s", client.script)
	}
	if len(client.keys) != 1 || client.keys[0] != "test:client" {
		t.Fatalf("unexpected redis keys: %#v", client.keys)
	}
	if strings.Join(client.args, ",") != "3,1500,1000" {
		t.Fatalf("unexpected redis args: %#v", client.args)
	}
	if !result.Allowed || result.Remaining != 2 || !result.Reset.Equal(time.UnixMilli(2500)) {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestRedisStoreBlocksWithRetryAfter(t *testing.T) {
	client := &recordingRedisClient{values: []int64{0, 0, 1600}}
	store, err := NewRedisStore(RedisOptions{Client: client})
	if err != nil {
		t.Fatal(err)
	}

	result, err := store.Take(context.Background(), "client", 1, time.Second, time.UnixMilli(1000))
	if err != nil {
		t.Fatal(err)
	}

	if result.Allowed || result.Remaining != 0 || result.RetryAfter != 600*time.Millisecond {
		t.Fatalf("unexpected blocked result: %#v", result)
	}
	if len(client.keys) != 1 || client.keys[0] != "gowdk:ratelimit:client" {
		t.Fatalf("unexpected default redis key: %#v", client.keys)
	}
}

func TestRedisStoreReportsInvalidReplies(t *testing.T) {
	store, err := NewRedisStore(RedisOptions{Client: &recordingRedisClient{values: []int64{1}}})
	if err != nil {
		t.Fatal(err)
	}

	_, err = store.Take(context.Background(), "client", 1, time.Second, time.UnixMilli(1000))
	if err == nil || !strings.Contains(err.Error(), "redis rate limit reply must include allowed, remaining, and reset values") {
		t.Fatalf("expected invalid reply error, got %v", err)
	}
}

type failingStore struct {
	err error
}

func (store failingStore) Take(context.Context, string, int, time.Duration, time.Time) (Result, error) {
	return Result{}, store.err
}

type recordingRedisClient struct {
	script string
	keys   []string
	args   []string
	values []int64
	err    error
}

func (client *recordingRedisClient) EvalInt64s(_ context.Context, script string, keys []string, args ...string) ([]int64, error) {
	client.script = script
	client.keys = append([]string(nil), keys...)
	client.args = append([]string(nil), args...)
	return append([]int64(nil), client.values...), client.err
}
