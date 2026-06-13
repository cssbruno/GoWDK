package membroker

import (
	"context"
	"errors"
	"testing"

	"github.com/cssbruno/gowdk/runtime/contracts"
)

type patientCreated struct {
	ID string
}

func TestBrokerPublishesReceivesAndAcksEvents(t *testing.T) {
	broker := New(WithBatchSize(1))
	events := []contracts.EventEnvelope{
		{Category: contracts.DomainEvent, Type: "PatientCreated", Value: patientCreated{ID: "patient-1"}},
		{Category: contracts.DomainEvent, Type: "PatientCreated", Value: patientCreated{ID: "patient-2"}},
	}

	if err := broker.PublishEvents(context.Background(), events); err != nil {
		t.Fatalf("publish events: %v", err)
	}
	if broker.Len() != 2 {
		t.Fatalf("broker.Len() = %d, want 2", broker.Len())
	}
	batch, err := broker.ReceiveEventBatch(context.Background())
	if err != nil {
		t.Fatalf("receive event batch: %v", err)
	}
	if len(batch.Events) != 1 {
		t.Fatalf("len(batch.Events) = %d, want 1", len(batch.Events))
	}
	if batch.Events[0].ID == "" {
		t.Fatalf("expected broker to assign event ID: %#v", batch.Events[0])
	}
	if err := batch.Ack(context.Background()); err != nil {
		t.Fatalf("ack batch: %v", err)
	}
	if broker.Len() != 1 {
		t.Fatalf("broker.Len() = %d, want 1 after ack", broker.Len())
	}
}

func TestBrokerNackLeavesEventsQueued(t *testing.T) {
	broker := New()
	if err := broker.PublishEvents(context.Background(), []contracts.EventEnvelope{{
		Category: contracts.DomainEvent,
		Type:     "PatientCreated",
		Value:    patientCreated{ID: "patient-1"},
	}}); err != nil {
		t.Fatalf("publish events: %v", err)
	}
	batch, err := broker.ReceiveEventBatch(context.Background())
	if err != nil {
		t.Fatalf("receive event batch: %v", err)
	}
	if batch.Nack != nil {
		t.Fatalf("expected nil nack for in-memory broker")
	}
	if broker.Len() != 1 {
		t.Fatalf("broker.Len() = %d, want queued event before ack", broker.Len())
	}
}

func TestBrokerReturnsClosedWhenEmpty(t *testing.T) {
	broker := New()
	_, err := broker.ReceiveEventBatch(context.Background())
	if !errors.Is(err, contracts.ErrEventSourceClosed) {
		t.Fatalf("ReceiveEventBatch error = %v, want ErrEventSourceClosed", err)
	}
}
