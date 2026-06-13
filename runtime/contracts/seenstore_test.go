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

func TestMemorySeenStoreSeenMarkSeenAndEvictsOldest(t *testing.T) {
	store := NewMemorySeenStore(1)

	alreadySeen, err := store.Seen(context.Background(), "event-1")
	if err != nil || alreadySeen {
		t.Fatalf("initial seen event-1 seen=%v err=%v, want false nil", alreadySeen, err)
	}
	if err := store.MarkSeen(context.Background(), "event-1"); err != nil {
		t.Fatalf("mark event-1: %v", err)
	}
	alreadySeen, err = store.Seen(context.Background(), "event-1")
	if err != nil || !alreadySeen {
		t.Fatalf("seen event-1 after mark seen=%v err=%v, want true nil", alreadySeen, err)
	}
	if err := store.MarkSeen(context.Background(), "event-2"); err != nil {
		t.Fatalf("mark event-2: %v", err)
	}
	alreadySeen, err = store.Seen(context.Background(), "event-1")
	if err != nil || alreadySeen {
		t.Fatalf("event-1 should be evicted, seen=%v err=%v", alreadySeen, err)
	}
}

func TestMemorySeenStoreMarkIfNew(t *testing.T) {
	store := NewMemorySeenStore(10)

	fresh, err := store.MarkIfNew(context.Background(), "event-1")
	if err != nil || !fresh {
		t.Fatalf("first mark event-1 fresh=%v err=%v, want true nil", fresh, err)
	}
	fresh, err = store.MarkIfNew(context.Background(), "event-1")
	if err != nil || fresh {
		t.Fatalf("second mark event-1 fresh=%v err=%v, want false nil", fresh, err)
	}
}
