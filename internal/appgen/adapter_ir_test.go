package appgen

import (
	"testing"

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
	})

	if len(ir.Registrations) != 2 {
		t.Fatalf("expected action and API registrations, got %#v", ir.Registrations)
	}
	if ir.Registrations[0].Kind != BackendEndpointAction || ir.Registrations[0].Path != "/newsletter" || ir.Registrations[0].Handler != "action" {
		t.Fatalf("unexpected action registration: %#v", ir.Registrations[0])
	}
	if ir.Registrations[1].Kind != BackendEndpointAPI || ir.Registrations[1].Path != "/api/health" || ir.Registrations[1].Handler != "api" {
		t.Fatalf("unexpected API registration: %#v", ir.Registrations[1])
	}
	if len(ir.Decoders) != 1 || ir.Decoders[0].Function == "" || ir.Decoders[0].Input != "SubscribeInput" {
		t.Fatalf("expected action decoder metadata, got %#v", ir.Decoders)
	}
	if len(ir.Calls) != 2 || ir.Calls[0].Alias != "newsletter" || ir.Calls[1].Alias != "status" {
		t.Fatalf("expected bound handler calls, got %#v", ir.Calls)
	}
	if len(ir.Responses) != 2 || !ir.Responses[0].NoStore || ir.Responses[0].Redirect != "/newsletter?ok=1" {
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
