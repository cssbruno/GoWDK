package natsbroker

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
		Category: contracts.IntegrationEvent,
		Type:     "PatientCreated",
		Value:    patientCreated{ID: "patient-1"},
	})
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	source := string(payload)
	if !strings.Contains(source, `"category":"integration"`) ||
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
	payload, err := json.Marshal(storedEnvelope{
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
	if decoded, ok := event.Value.(patientCreated); !ok || decoded.ID != "patient-1" {
		t.Fatalf("event.Value = %#v, want patientCreated patient-1", event.Value)
	}
}

func TestValidateRequiresConnectionAndSubject(t *testing.T) {
	if err := New(nil, "").validate(); err == nil || !strings.Contains(err.Error(), "nats connection is required") {
		t.Fatalf("validate error = %v, want connection required", err)
	}
}
