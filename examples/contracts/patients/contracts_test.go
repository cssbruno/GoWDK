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
	testkit.AssertEmitted[PatientNotice](t, events, contracts.PresentationEvent, func(event PatientNotice) {
		if event.Patch.Op != "replaceHTML" || event.Patch.Swap != "innerHTML" {
			t.Fatalf("unexpected presentation patch metadata: %#v", event.Patch)
		}
		if event.Patch.HTML != `<p role="status">Patient Ada was queued.</p>` {
			t.Fatalf("presentation patch HTML = %q", event.Patch.HTML)
		}
	})
}
