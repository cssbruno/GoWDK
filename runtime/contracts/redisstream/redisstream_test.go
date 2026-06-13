package redisstream

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/cssbruno/gowdk/runtime/contracts"
	redis "github.com/redis/go-redis/v9"
)

type patientCreated struct {
	ID string `json:"id"`
}

func TestMarshalEnvelope(t *testing.T) {
	payload, err := marshalEnvelope(contracts.EventEnvelope{
		Category: contracts.DomainEvent,
		Type:     "PatientCreated",
		Value:    patientCreated{ID: "patient-1"},
	})
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	if !strings.Contains(payload, `"id":"`) ||
		!strings.Contains(payload, `"category":"domain"`) ||
		!strings.Contains(payload, `"type":"PatientCreated"`) ||
		!strings.Contains(payload, `"id":"patient-1"`) {
		t.Fatalf("unexpected payload: %s", payload)
	}
}

func TestDecodeMessageWithRegisteredDecoder(t *testing.T) {
	value, err := json.Marshal(patientCreated{ID: "patient-1"})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(contracts.StoredEventEnvelope{
		ID:       "event-1",
		Category: contracts.DomainEvent,
		Type:     "PatientCreated",
		Value:    value,
	})
	if err != nil {
		t.Fatal(err)
	}
	store := New(nil, "events", "workers", "worker-1", WithJSONDecoder[patientCreated]("PatientCreated"))

	event, err := store.decodeStored("1-0", string(payload))
	if err != nil {
		t.Fatalf("decode stored event: %v", err)
	}
	if event.Category != contracts.DomainEvent || event.Type != "PatientCreated" {
		t.Fatalf("unexpected event metadata: %#v", event)
	}
	if event.ID != "event-1" {
		t.Fatalf("event.ID = %q, want event-1", event.ID)
	}
	if decoded, ok := event.Value.(patientCreated); !ok || decoded.ID != "patient-1" {
		t.Fatalf("event.Value = %#v, want patientCreated patient-1", event.Value)
	}
}

func TestValidateRequiresClientAndNames(t *testing.T) {
	if err := New(nil, "", "", "").validate(); err == nil || !strings.Contains(err.Error(), "redis stream client is required") {
		t.Fatalf("validate error = %v, want client required", err)
	}
}

func TestNewStartsByDrainingPendingEntries(t *testing.T) {
	store := New(nil, "events", "workers", "worker-1")
	if !store.pendingFirst() {
		t.Fatal("expected a new store to drain pending entries before new messages")
	}
	store.setPendingFirst(false)
	if store.pendingFirst() {
		t.Fatal("expected pending drain to be cleared")
	}
	store.setPendingFirst(true)
	if !store.pendingFirst() {
		t.Fatal("expected Nack rewind to re-enable the pending drain")
	}
}

type fakeSeenClient struct {
	values map[string]bool
	key    string
	ttl    time.Duration
}

func (client *fakeSeenClient) Exists(ctx context.Context, keys ...string) *redis.IntCmd {
	var count int64
	for _, key := range keys {
		if client.values[key] {
			count++
		}
	}
	return redis.NewIntResult(count, nil)
}

func (client *fakeSeenClient) Set(ctx context.Context, key string, value any, expiration time.Duration) *redis.StatusCmd {
	client.key = key
	client.ttl = expiration
	if client.values == nil {
		client.values = map[string]bool{}
	}
	client.values[key] = true
	return redis.NewStatusResult("OK", nil)
}

func (client *fakeSeenClient) SetNX(ctx context.Context, key string, value any, expiration time.Duration) *redis.BoolCmd {
	client.key = key
	client.ttl = expiration
	if client.values == nil {
		client.values = map[string]bool{}
	}
	if client.values[key] {
		return redis.NewBoolResult(false, nil)
	}
	client.values[key] = true
	return redis.NewBoolResult(true, nil)
}

func TestSeenStoreUsesExistsAndSetWithTTL(t *testing.T) {
	client := &fakeSeenClient{}
	store := newSeenStore(client, "seen:", time.Minute)

	alreadySeen, err := store.Seen(context.Background(), "event-1")
	if err != nil || alreadySeen {
		t.Fatalf("initial seen seen=%v err=%v, want false nil", alreadySeen, err)
	}
	if err := store.MarkSeen(context.Background(), "event-1"); err != nil {
		t.Fatalf("mark seen: %v", err)
	}
	if client.key != "seen:event-1" || client.ttl != time.Minute {
		t.Fatalf("unexpected SET key/ttl: key=%q ttl=%s", client.key, client.ttl)
	}
	alreadySeen, err = store.Seen(context.Background(), "event-1")
	if err != nil || !alreadySeen {
		t.Fatalf("seen after mark seen=%v err=%v, want true nil", alreadySeen, err)
	}
}

func TestSeenStoreMarkIfNewUsesSetNX(t *testing.T) {
	client := &fakeSeenClient{}
	store := newSeenStore(client, "seen:", time.Minute)

	fresh, err := store.MarkIfNew(context.Background(), "event-1")
	if err != nil || !fresh {
		t.Fatalf("first mark fresh=%v err=%v, want true nil", fresh, err)
	}
	fresh, err = store.MarkIfNew(context.Background(), "event-1")
	if err != nil || fresh {
		t.Fatalf("second mark fresh=%v err=%v, want false nil", fresh, err)
	}
}
