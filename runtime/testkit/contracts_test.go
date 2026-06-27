package testkit

import (
	"context"
	"testing"

	"github.com/cssbruno/gowdk/runtime/contracts"
)

type testCommand struct {
	Name string
}

type testCommandResult struct {
	ID string
}

type testEvent struct {
	ID string
}

func TestContractRegistryCaptureAndAssertEmitted(t *testing.T) {
	registry := ContractRegistry(func(registry *contracts.Registry) {
		if err := contracts.RegisterCommand[testCommand, testCommandResult](registry, func(ctx context.Context, command testCommand) (testCommandResult, error) {
			if err := contracts.EmitDomain(ctx, testEvent{ID: "event-1"}); err != nil {
				return testCommandResult{}, err
			}
			return testCommandResult{ID: command.Name}, nil
		}, contracts.RoleWeb); err != nil {
			t.Fatal(err)
		}
	})

	result, events := CaptureCommandEvents[testCommand, testCommandResult](t, registry, testCommand{Name: "command-1"})
	if result.ID != "command-1" {
		t.Fatalf("result ID = %q, want command-1", result.ID)
	}
	AssertEmitted[testEvent](t, events, contracts.DomainEvent, func(event testEvent) {
		if event.ID != "event-1" {
			t.Fatalf("event ID = %q, want event-1", event.ID)
		}
	})
}

func TestAssertNoEvents(t *testing.T) {
	AssertNoEvents(t, nil)
}
