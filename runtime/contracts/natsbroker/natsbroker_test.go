package natsbroker

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk/runtime/contracts"
)

type patientCreated struct {
	ID string `json:"id"`
}

func TestMarshalEnvelope(t *testing.T) {
	payload, err := marshalEnvelope(contracts.EventEnvelope{
		Category: contracts.IntegrationEvent,
		Type:     "PatientCreated",
		Value:    patientCreated{ID: "patient-1"},
	})
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	source := string(payload)
	if !strings.Contains(source, `"id":"`) ||
		!strings.Contains(source, `"category":"integration"`) ||
		!strings.Contains(source, `"type":"PatientCreated"`) ||
		!strings.Contains(source, `"id":"patient-1"`) {
		t.Fatalf("unexpected payload: %s", source)
	}
}

func TestDecodePayloadWithRegisteredDecoder(t *testing.T) {
	value, err := json.Marshal(patientCreated{ID: "patient-1"})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(contracts.StoredEventEnvelope{
		ID:       "event-1",
		Category: contracts.IntegrationEvent,
		Type:     "PatientCreated",
		Value:    value,
	})
	if err != nil {
		t.Fatal(err)
	}
	broker := New(nil, "events", WithJSONDecoder[patientCreated]("PatientCreated"))

	event, err := broker.decodePayload(payload)
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if event.Category != contracts.IntegrationEvent || event.Type != "PatientCreated" {
		t.Fatalf("unexpected event metadata: %#v", event)
	}
	if event.ID != "event-1" {
		t.Fatalf("event.ID = %q, want event-1", event.ID)
	}
	if decoded, ok := event.Value.(patientCreated); !ok || decoded.ID != "patient-1" {
		t.Fatalf("event.Value = %#v, want patientCreated patient-1", event.Value)
	}
}

func TestDrainAvailableEventsReturnsAccumulatedEventsOnLaterError(t *testing.T) {
	decodeErr := errors.New("decode failed")
	first := contracts.EventEnvelope{ID: "event-1", Category: contracts.IntegrationEvent, Type: "PatientCreated", Value: patientCreated{ID: "patient-1"}}
	second := contracts.EventEnvelope{ID: "event-2", Category: contracts.IntegrationEvent, Type: "PatientCreated", Value: patientCreated{ID: "patient-2"}}
	calls := 0

	events := drainAvailableEvents(context.Background(), first, 3, func(ctx context.Context) (contracts.EventEnvelope, bool, error) {
		calls++
		if calls == 1 {
			return second, true, nil
		}
		return contracts.EventEnvelope{}, false, decodeErr
	})
	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2: %#v", len(events), events)
	}
	if events[0].ID != "event-1" || events[1].ID != "event-2" {
		t.Fatalf("unexpected event IDs: %#v", events)
	}
}

func TestValidateRequiresConnectionAndSubject(t *testing.T) {
	if err := New(nil, "").validate(); err == nil || !strings.Contains(err.Error(), "nats connection is required") {
		t.Fatalf("validate error = %v, want connection required", err)
	}
}
