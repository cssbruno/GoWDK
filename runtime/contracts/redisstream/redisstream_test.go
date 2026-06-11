package redisstream

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk/runtime/contracts"
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
	if !strings.Contains(payload, `"category":"domain"`) ||
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
