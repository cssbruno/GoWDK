package compiler

import (
	"strings"
	"testing"

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
