package appgen

import (
	"testing"

	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/manifest"
)

func TestBackendAdapterIRCapturesRouteAndHandlerMetadata(t *testing.T) {
	ir := backendAdapterIR(Options{
		Actions: []ActionEndpoint{{
			PageID:      "newsletter",
			ActionName:  "Subscribe",
			Method:      "POST",
			Route:       "/newsletter",
			InputType:   "SubscribeInput",
			InputFields: []string{"email"},
			Redirect:    "/newsletter?ok=1",
			Binding: manifest.BackendBinding{
				Status:       manifest.BackendBindingBound,
				FunctionName: "Subscribe",
				Signature:    manifest.BackendSignatureActionForm,
				InputType:    "SubscribeInput",
			},
			BackendAlias: "newsletter",
		}},
		APIs: []APIEndpoint{{
			PageID:  "status",
			APIName: "Health",
			Method:  "GET",
			Route:   "/api/health",
			Binding: manifest.BackendBinding{
				Status:       manifest.BackendBindingBound,
				FunctionName: "Health",
				Signature:    manifest.BackendSignatureAPI,
			},
			BackendAlias: "status",
		}},
		Fragments: []FragmentEndpoint{{
			PageID:       "patients",
			FragmentName: "List",
			Method:       "GET",
			Route:        "/patients/list",
			Target:       "#patients",
			HTML:         "<section>Patients</section>",
		}},
	})

	if len(ir.Registrations) != 3 {
		t.Fatalf("expected action, API, and fragment registrations, got %#v", ir.Registrations)
	}
	if ir.Registrations[0].Kind != BackendEndpointAction || ir.Registrations[0].Path != "/newsletter" || ir.Registrations[0].Handler != "action" {
		t.Fatalf("unexpected action registration: %#v", ir.Registrations[0])
	}
	if ir.Registrations[1].Kind != BackendEndpointAPI || ir.Registrations[1].Path != "/api/health" || ir.Registrations[1].Handler != "api" {
		t.Fatalf("unexpected API registration: %#v", ir.Registrations[1])
	}
	if ir.Registrations[2].Kind != BackendEndpointFragment || ir.Registrations[2].Path != "/patients/list" || ir.Registrations[2].Handler != "fragment" {
		t.Fatalf("unexpected fragment registration: %#v", ir.Registrations[2])
	}
	if len(ir.Decoders) != 1 || ir.Decoders[0].Function == "" || ir.Decoders[0].Input != "SubscribeInput" {
		t.Fatalf("expected action decoder metadata, got %#v", ir.Decoders)
	}
	if len(ir.Calls) != 2 || ir.Calls[0].Alias != "newsletter" || ir.Calls[1].Alias != "status" {
		t.Fatalf("expected bound handler calls, got %#v", ir.Calls)
	}
	if len(ir.Responses) != 3 || !ir.Responses[0].NoStore || ir.Responses[0].Redirect != "/newsletter?ok=1" || !ir.Responses[2].Partial {
		t.Fatalf("expected no-store response metadata, got %#v", ir.Responses)
	}
}

func TestBackendAdapterIRCapturesFallbackMetadata(t *testing.T) {
	ir := backendAdapterIR(Options{Actions: []ActionEndpoint{{
		PageID:     "newsletter",
		ActionName: "Subscribe",
		Method:     "POST",
		Route:      "/newsletter",
		Binding: manifest.BackendBinding{
			Status:  manifest.BackendBindingMissing,
			Message: "missing Subscribe",
		},
	}}})

	if len(ir.Fallbacks) != 1 {
		t.Fatalf("expected one fallback, got %#v", ir.Fallbacks)
	}
	if ir.Fallbacks[0].Status != manifest.BackendBindingMissing || ir.Fallbacks[0].Endpoint.Path != "/newsletter" {
		t.Fatalf("unexpected fallback metadata: %#v", ir.Fallbacks[0])
	}
}

func TestBackendAdapterIRCapturesContractExposureMetadata(t *testing.T) {
	program := &gwdkir.Program{ContractRefs: []gwdkir.ContractReference{
		{
			Kind:        gwdkir.ContractQuery,
			Name:        "patients.GetPatientPage",
			ImportAlias: "patients",
			ImportPath:  "example.com/app/contracts/patients",
			Type:        "GetPatientPage",
			Result:      "PatientPageData",
			InputFields: []manifest.BackendInputField{{FieldName: "Filter", FormName: "filter", Type: "string"}},
			Method:      "GET",
			Path:        "/patients",
			Status:      gwdkir.ContractBindingMissing,
			Message:     "query missing",
			Register:    "Register",
			OwnerKind:   gwdkir.SourcePage,
			OwnerID:     "patients",
			Package:     "patients",
			Source:      "patients.page.gwdk",
		},
		{
			Kind:        gwdkir.ContractCommand,
			Name:        "patients.CreatePatient",
			ImportAlias: "patients",
			ImportPath:  "example.com/app/contracts/patients",
			Type:        "CreatePatient",
			Result:      "CreatePatientResult",
			Roles:       []string{"web"},
			Guards:      []string{"auth.required"},
			InputFields: []manifest.BackendInputField{{FieldName: "Name", FormName: "name", Type: "string"}},
			Method:      "POST",
			Path:        "/patients",
			Status:      gwdkir.ContractBindingBound,
			Handler:     "HandleCreatePatient",
			Register:    "Register",
			OwnerKind:   gwdkir.SourcePage,
			OwnerID:     "patients",
			Package:     "patients",
			Source:      "patients.page.gwdk",
		},
	}}

	ir := backendAdapterIR(Options{IR: program})
	if len(ir.ContractExposures) != 2 {
		t.Fatalf("expected two contract exposures, got %#v", ir.ContractExposures)
	}
	command := ir.ContractExposures[0]
	if command.Endpoint.Kind != BackendEndpointCommand || command.Endpoint.Handler != "command" {
		t.Fatalf("unexpected command exposure endpoint: %#v", command.Endpoint)
	}
	if command.Endpoint.Method != "POST" || command.Endpoint.Path != "/patients" {
		t.Fatalf("unexpected command exposure method/path: %#v", command.Endpoint)
	}
	if command.Contract != "patients.CreatePatient" || command.Status != gwdkir.ContractBindingBound || command.Handler != "HandleCreatePatient" || command.Register != "Register" {
		t.Fatalf("unexpected command exposure: %#v", command)
	}
	if command.ImportAlias != "patients" || command.ImportPath != "example.com/app/contracts/patients" || command.Type != "CreatePatient" || command.Result != "CreatePatientResult" {
		t.Fatalf("unexpected command contract metadata: %#v", command)
	}
	if len(command.InputFields) != 1 || command.InputFields[0].FormName != "name" {
		t.Fatalf("unexpected command input fields: %#v", command.InputFields)
	}
	if len(command.Roles) != 1 || command.Roles[0] != "web" {
		t.Fatalf("unexpected command roles: %#v", command.Roles)
	}
	if len(command.Guards) != 1 || command.Guards[0] != "auth.required" {
		t.Fatalf("unexpected command guards: %#v", command.Guards)
	}
	query := ir.ContractExposures[1]
	if query.Endpoint.Kind != BackendEndpointQuery || query.Endpoint.Handler != "query" {
		t.Fatalf("unexpected query exposure endpoint: %#v", query.Endpoint)
	}
	if query.Endpoint.Method != "GET" || query.Endpoint.Path != "/patients" {
		t.Fatalf("unexpected query exposure method/path: %#v", query.Endpoint)
	}
	if query.Contract != "patients.GetPatientPage" || query.Status != gwdkir.ContractBindingMissing || query.Message != "query missing" {
		t.Fatalf("unexpected query exposure: %#v", query)
	}
	if query.ImportAlias != "patients" || query.ImportPath != "example.com/app/contracts/patients" || query.Type != "GetPatientPage" || query.Result != "PatientPageData" {
		t.Fatalf("unexpected query contract metadata: %#v", query)
	}
	if len(query.InputFields) != 1 || query.InputFields[0].FormName != "filter" {
		t.Fatalf("unexpected query input fields: %#v", query.InputFields)
	}
}
