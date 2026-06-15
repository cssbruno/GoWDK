package compiler

import (
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

func TestValidateContractReferencesRejectsBoundNonWebRole(t *testing.T) {
	err := ValidateContractReferences([]gwdkir.ContractReference{{
		Kind:      gwdkir.ContractCommand,
		Name:      "patients.CreatePatient",
		Roles:     []string{"worker", "cron"},
		Status:    gwdkir.ContractBindingBound,
		OwnerKind: gwdkir.SourcePage,
		OwnerID:   "patients",
		Source:    "patients.page.gwdk",
		Span: source.SourceSpan{
			Start: source.SourcePosition{Line: 8, Column: 42},
		},
	}})
	if err == nil {
		t.Fatal("expected non-web role diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if len(diagnostics) != 1 || diagnostics[0].Code != "contract_reference_role_not_allowed" {
		t.Fatalf("unexpected diagnostics: %#v", diagnostics)
	}
	if diagnostics[0].PageID != "patients" || diagnostics[0].Source != "patients.page.gwdk" {
		t.Fatalf("unexpected diagnostic owner/source: %#v", diagnostics[0])
	}
	if !strings.Contains(diagnostics[0].Message, "worker, cron") || !strings.Contains(diagnostics[0].Message, "role web") {
		t.Fatalf("unexpected diagnostic message: %s", diagnostics[0].Message)
	}
}

func TestValidateContractReferencesAcceptsWebAndUnrestrictedRoles(t *testing.T) {
	err := ValidateContractReferences([]gwdkir.ContractReference{
		{Kind: gwdkir.ContractCommand, Name: "patients.CreatePatient", Roles: []string{"web"}, Status: gwdkir.ContractBindingBound},
		{Kind: gwdkir.ContractQuery, Name: "patients.GetPatientPage", Status: gwdkir.ContractBindingBound},
	})
	if err != nil {
		t.Fatalf("expected web/unrestricted roles to validate, got %v", err)
	}
}

func TestValidateRealtimeSubscriptionsRequireAddon(t *testing.T) {
	diagnostics := validateRealtimeSubscriptions(gowdk.Config{}, []gwdkir.RealtimeSubscription{{
		Event:     "patients.PatientNotice",
		OwnerKind: gwdkir.SourcePage,
		OwnerID:   "patients",
		Source:    "patients.page.gwdk",
		Span: source.SourceSpan{
			Start: source.SourcePosition{Line: 9, Column: 65},
		},
	}})
	if len(diagnostics) != 1 || diagnostics[0].Code != "missing_realtime_addon" {
		t.Fatalf("unexpected diagnostics: %#v", diagnostics)
	}
	if diagnostics[0].PageID != "patients" || diagnostics[0].Source != "patients.page.gwdk" {
		t.Fatalf("unexpected diagnostic owner/source: %#v", diagnostics[0])
	}

	diagnostics = validateRealtimeSubscriptions(gowdk.Config{Addons: []gowdk.Addon{gowdk.NewAddon("realtime", gowdk.FeatureRealtime)}}, []gwdkir.RealtimeSubscription{{
		Event: "patients.PatientNotice",
	}})
	if len(diagnostics) != 0 {
		t.Fatalf("expected realtime addon to allow subscriptions, got %#v", diagnostics)
	}
}

func TestValidateRealtimeSubscriptionBindings(t *testing.T) {
	err := ValidateRealtimeSubscriptionBindings([]gwdkir.RealtimeSubscription{
		{
			Event:     "patients.PatientNotice",
			Status:    gwdkir.ContractBindingMissing,
			OwnerKind: gwdkir.SourcePage,
			OwnerID:   "patients",
			Source:    "patients.page.gwdk",
		},
		{
			Event:     "patients.AdminNotice",
			Status:    gwdkir.ContractBindingBound,
			Roles:     []string{"admin"},
			OwnerKind: gwdkir.SourcePage,
			OwnerID:   "patients",
			Source:    "patients.page.gwdk",
		},
		{
			Event:     "patients.BadNotice",
			Status:    gwdkir.ContractBindingInvalid,
			Message:   "bad event handler",
			OwnerKind: gwdkir.SourceComponent,
			OwnerID:   "PatientList",
			Source:    "components/patient_list.cmp.gwdk",
		},
		{
			Event:  "patients.WebNotice",
			Status: gwdkir.ContractBindingBound,
			Roles:  []string{"web"},
		},
	})
	if err == nil {
		t.Fatal("expected subscription diagnostics")
	}
	diagnostics := err.(ValidationErrors)
	if len(diagnostics) != 3 {
		t.Fatalf("expected three diagnostics, got %#v", diagnostics)
	}
	wantCodes := []string{"realtime_subscription_missing", "realtime_subscription_role_not_allowed", "realtime_subscription_invalid"}
	for index, want := range wantCodes {
		if diagnostics[index].Code != want {
			t.Fatalf("diagnostic %d code = %q, want %q in %#v", index, diagnostics[index].Code, want, diagnostics)
		}
	}
	if diagnostics[1].PageID != "patients" || !strings.Contains(diagnostics[1].Message, "admin") {
		t.Fatalf("unexpected role diagnostic: %#v", diagnostics[1])
	}
	if diagnostics[2].ComponentName != "PatientList" || !strings.Contains(diagnostics[2].Message, "bad event handler") {
		t.Fatalf("unexpected invalid diagnostic: %#v", diagnostics[2])
	}
}

func TestValidateQueryInvalidationsRequireRealtimeAddon(t *testing.T) {
	err := ValidateQueryInvalidations(gowdk.Config{}, []gwdkir.QueryInvalidation{{
		Query:     "patients.GetPatientPage",
		Event:     "patients.PatientCreated",
		Status:    gwdkir.ContractBindingBound,
		OwnerKind: gwdkir.SourcePage,
		OwnerID:   "patients",
		Source:    "patients.page.gwdk",
	}})
	if err == nil || !strings.Contains(err.Error(), "requires realtime.Addon()") {
		t.Fatalf("expected missing realtime addon diagnostic, got %v", err)
	}

	err = ValidateQueryInvalidations(gowdk.Config{Addons: []gowdk.Addon{gowdk.NewAddon("realtime", gowdk.FeatureRealtime)}}, []gwdkir.QueryInvalidation{{
		Query:     "patients.GetPatientPage",
		Event:     "patients.PatientCreated",
		Status:    gwdkir.ContractBindingBound,
		OwnerKind: gwdkir.SourcePage,
		OwnerID:   "patients",
		Source:    "patients.page.gwdk",
	}})
	if err != nil {
		t.Fatalf("expected realtime addon to satisfy query invalidation, got %v", err)
	}
}
