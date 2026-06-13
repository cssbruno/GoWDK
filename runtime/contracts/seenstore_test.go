package contracts

import (
	"context"
	"testing"
)

func TestEnsureEventIDsAssignsAndPreservesIDs(t *testing.T) {
	events := EnsureEventIDs([]EventEnvelope{
		{Category: DomainEvent, Type: "PatientCreated", Value: patientCreated{ID: "patient-1"}},
		{ID: "custom-id", Category: DomainEvent, Type: "PatientCreated", Value: patientCreated{ID: "patient-2"}},
	})

	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(events))
	}
	if events[0].ID == "" {
		t.Fatalf("expected first event ID to be assigned: %#v", events[0])
	}
	if events[1].ID != "custom-id" {
		t.Fatalf("expected existing ID to be preserved, got %q", events[1].ID)
	}
	if events[0].ID == events[1].ID {
		t.Fatalf("expected unique event IDs, got %#v", events)
	}
}

func TestMemorySeenStoreMarksNewOnceAndEvictsOldest(t *testing.T) {
	store := NewMemorySeenStore(1)

	fresh, err := store.MarkIfNew(context.Background(), "event-1")
	if err != nil || !fresh {
		t.Fatalf("first mark event-1 fresh=%v err=%v, want true nil", fresh, err)
	}
	fresh, err = store.MarkIfNew(context.Background(), "event-1")
	if err != nil || fresh {
		t.Fatalf("second mark event-1 fresh=%v err=%v, want false nil", fresh, err)
	}
	fresh, err = store.MarkIfNew(context.Background(), "event-2")
	if err != nil || !fresh {
		t.Fatalf("mark event-2 fresh=%v err=%v, want true nil", fresh, err)
	}
	fresh, err = store.MarkIfNew(context.Background(), "event-1")
	if err != nil || !fresh {
		t.Fatalf("event-1 should be new again after eviction, fresh=%v err=%v", fresh, err)
	}
}
