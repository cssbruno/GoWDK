package testkit

import (
	"context"
	"testing"

	"github.com/cssbruno/gowdk/runtime/contracts"
)

// ContractRegistry creates an in-memory registry for contract tests.
func ContractRegistry(register ...func(*contracts.Registry)) *contracts.Registry {
	registry := contracts.NewRegistry()
	for _, fn := range register {
		if fn != nil {
			fn(registry)
		}
	}
	return registry
}

// CaptureCommandEvents runs a command and fails the test on command errors.
func CaptureCommandEvents[C, R any](t testing.TB, registry *contracts.Registry, command C) (R, []contracts.EventEnvelope) {
	t.Helper()
	result, events, err := contracts.CaptureCommandEvents[C, R](context.Background(), registry, command)
	if err != nil {
		t.Fatalf("capture command events: %v", err)
	}
	return result, events
}

// AssertEmitted finds one captured event with the requested category and type.
func AssertEmitted[E any](t testing.TB, events []contracts.EventEnvelope, category contracts.EventCategory, check func(E)) {
	t.Helper()
	wantType := contracts.ContractName[E]()
	for _, event := range events {
		if event.Category != category || event.Type != wantType {
			continue
		}
		value, ok := event.Value.(E)
		if !ok {
			var zero E
			t.Fatalf("captured %s event %s has value type %T, want %T", category, event.Type, event.Value, zero)
		}
		if check != nil {
			check(value)
		}
		return
	}
	t.Fatalf("missing captured %s event %s in %#v", category, wantType, events)
}

// AssertNoEvents fails when a command unexpectedly emitted events.
func AssertNoEvents(t testing.TB, events []contracts.EventEnvelope) {
	t.Helper()
	if len(events) != 0 {
		t.Fatalf("expected no captured events, got %#v", events)
	}
}
