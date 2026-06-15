package patients

import (
	"testing"

	"github.com/cssbruno/gowdk/runtime/contracts"
	"github.com/cssbruno/gowdk/runtime/testkit"
)

func TestCreatePatientCapturesDomainEvent(t *testing.T) {
	registry := testkit.ContractRegistry(Register)

	result, events := testkit.CaptureCommandEvents[CreatePatient, CreatePatientResult](t, registry, CreatePatient{Name: "Ada"})
	if result.ID != "patient-1" {
		t.Fatalf("result ID = %q, want patient-1", result.ID)
	}

	testkit.AssertEmitted[PatientCreated](t, events, contracts.DomainEvent, func(event PatientCreated) {
		if event.ID != "patient-1" {
			t.Fatalf("event ID = %q, want patient-1", event.ID)
		}
	})
}
