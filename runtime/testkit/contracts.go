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
func CaptureCommandEvents[C, R any](tb testing.TB, registry *contracts.Registry, command C) (R, []contracts.EventEnvelope) {
	tb.Helper()
	result, events, err := contracts.CaptureCommandEvents[C, R](context.Background(), registry, command)
	if err != nil {
		tb.Fatalf("capture command events: %v", err)
	}
	return result, events
}

// AssertEmitted finds one captured event with the requested category and type.
func AssertEmitted[E any](tb testing.TB, events []contracts.EventEnvelope, category contracts.EventCategory, check func(E)) {
	tb.Helper()
	wantType := contracts.ContractName[E]()
	for _, event := range events {
		if event.Category != category || event.Type != wantType {
			continue
		}
		value, ok := event.Value.(E)
		if !ok {
			var zero E
			tb.Fatalf("captured %s event %s has value type %T, want %T", category, event.Type, event.Value, zero)
		}
		if check != nil {
			check(value)
		}
		return
	}
	tb.Fatalf("missing captured %s event %s in %#v", category, wantType, events)
}

// AssertNoEvents fails when a command unexpectedly emitted events.
func AssertNoEvents(tb testing.TB, events []contracts.EventEnvelope) {
	tb.Helper()
	if len(events) != 0 {
		tb.Fatalf("expected no captured events, got %#v", events)
	}
}
