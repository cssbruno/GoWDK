package appgen

import (
	"testing"

	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

func TestBackendAdapterIRCapturesRouteAndHandlerMetadata(t *testing.T) {
	ir := backendAdapterIR(Options{
		Actions: []ActionEndpoint{{
			PageID:      "newsletter",
			ActionName:  "Subscribe",
			Method:      "POST",
			Route:       "/newsletter",
			Guards:      []string{"auth.required"},
			InputType:   "SubscribeInput",
			InputFields: []string{"email"},
			RequiredMessages: map[string]string{
				"email": "Email is required",
			},
			Redirect: "/newsletter?ok=1",
			Binding: source.BackendBinding{
				Status:       source.BackendBindingBound,
				ImportPath:   "example.com/app/newsletter",
				FunctionName: "Subscribe",
				Signature:    source.BackendSignatureActionForm,
				InputType:    "SubscribeInput",
			},
			BackendAlias: "newsletter",
		}},
		APIs: []APIEndpoint{{
			PageID:  "status",
			APIName: "Health",
			Method:  "GET",
			Route:   "/api/health",
			Binding: source.BackendBinding{
				Status:       source.BackendBindingBound,
				ImportPath:   "example.com/app/status",
				FunctionName: "Health",
				Signature:    source.BackendSignatureAPI,
			},
			BackendAlias: "status",
		}},
		Fragments: []FragmentEndpoint{{
			PageID:       "patients",
			FragmentName: "List",
			Method:       "GET",
			Route:        "/patients/{id}/list",
			Target:       "#patients",
			HTML:         "<section>Patients</section>",
			Binding: source.BackendBinding{
				Status:       source.BackendBindingBound,
				ImportPath:   "example.com/app/patients",
				FunctionName: "List",
				Signature:    source.BackendSignatureFragment,
			},
			BackendAlias: "patients",
		}},
	})

	if len(ir.Registrations) != 3 {
		t.Fatalf("expected action, API, and fragment registrations, got %#v", ir.Registrations)
	}
	if len(ir.Actions) != 1 || ir.Actions[0].Endpoint.Path != ir.Registrations[0].Path || ir.Actions[0].Endpoint.Kind != BackendEndpointAction || ir.Actions[0].RequiredMessages["email"] != "Email is required" {
		t.Fatalf("expected action adapter metadata, got %#v", ir.Actions)
	}
	if len(ir.APIs) != 1 || ir.APIs[0].Endpoint.Path != ir.Registrations[1].Path || ir.APIs[0].Endpoint.Kind != BackendEndpointAPI || ir.APIs[0].APIName != "Health" {
		t.Fatalf("expected API adapter metadata, got %#v", ir.APIs)
	}
	if len(ir.Fragments) != 1 || ir.Fragments[0].Endpoint.Path != ir.Registrations[2].Path || ir.Fragments[0].Endpoint.Kind != BackendEndpointFragment || ir.Fragments[0].Target != "#patients" {
		t.Fatalf("expected fragment adapter metadata, got %#v", ir.Fragments)
	}
	if ir.Registrations[0].Kind != BackendEndpointAction || ir.Registrations[0].Path != "/newsletter" || ir.Registrations[0].Handler != "action" {
		t.Fatalf("unexpected action registration: %#v", ir.Registrations[0])
	}
	if ir.Registrations[1].Kind != BackendEndpointAPI || ir.Registrations[1].Path != "/api/health" || ir.Registrations[1].Handler != "api" {
		t.Fatalf("unexpected API registration: %#v", ir.Registrations[1])
	}
	if ir.Registrations[2].Kind != BackendEndpointFragment || ir.Registrations[2].Path != "/patients/{id}/list" || ir.Registrations[2].Handler != "fragment" || !ir.Registrations[2].Dynamic {
		t.Fatalf("unexpected fragment registration: %#v", ir.Registrations[2])
	}
	if len(ir.Decoders) != 1 || ir.Decoders[0].Function == "" || ir.Decoders[0].Input != "SubscribeInput" {
		t.Fatalf("expected action decoder metadata, got %#v", ir.Decoders)
	}
	if len(ir.Calls) != 3 || ir.Calls[0].Alias != "newsletter" || ir.Calls[0].ImportPath != "example.com/app/newsletter" || ir.Calls[1].Alias != "status" || ir.Calls[2].Alias != "patients" {
		t.Fatalf("expected bound handler calls, got %#v", ir.Calls)
	}
	if len(ir.Responses) != 3 || !ir.Responses[0].NoStore || ir.Responses[0].Redirect != "/newsletter?ok=1" || !ir.Responses[2].Partial {
		t.Fatalf("expected no-store response metadata, got %#v", ir.Responses)
	}
	if !ir.HasEndpointKind(BackendEndpointAction) || !ir.HasEndpointKind(BackendEndpointAPI) || !ir.HasEndpointKind(BackendEndpointFragment) {
		t.Fatalf("expected adapter endpoint kind lookup to include action, API, and fragment: %#v", ir.Registrations)
	}
	if !ir.HasDynamicRoutes() {
		t.Fatalf("expected adapter IR to report dynamic fragment route")
	}
	guards := ir.GuardNames()
	if len(guards) != 1 || guards[0] != "auth.required" {
		t.Fatalf("expected adapter guard metadata, got %#v", guards)
	}
	imports := ir.BackendImports()
	if imports["example.com/app/newsletter"] != "newsletter" || imports["example.com/app/status"] != "status" || imports["example.com/app/patients"] != "patients" {
		t.Fatalf("expected adapter backend imports, got %#v", imports)
	}
}

func TestBackendAdapterIRCapturesFallbackMetadata(t *testing.T) {
	ir := backendAdapterIR(Options{Actions: []ActionEndpoint{{
		PageID:     "newsletter",
		ActionName: "Subscribe",
		Method:     "POST",
		Route:      "/newsletter",
		Binding: source.BackendBinding{
			Status:  source.BackendBindingMissing,
			Message: "missing Subscribe",
		},
	}}})

	if len(ir.Fallbacks) != 1 {
		t.Fatalf("expected one fallback, got %#v", ir.Fallbacks)
	}
	if ir.Fallbacks[0].Status != source.BackendBindingMissing || ir.Fallbacks[0].Endpoint.Path != "/newsletter" {
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
			InputFields: []source.BackendInputField{{FieldName: "Filter", FormName: "filter", Type: "string"}},
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
			InputFields: []source.BackendInputField{{FieldName: "Name", FormName: "name", Type: "string"}},
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
