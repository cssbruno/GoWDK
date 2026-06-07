package contractscan

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/manifest"
	runtimecontracts "github.com/cssbruno/gowdk/runtime/contracts"
)

func TestScanDiscoversRuntimeContractRegistrations(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "patients.go"), `package patients

import (
	"context"
	gowdkcontracts "github.com/cssbruno/gowdk/runtime/contracts"
)

type GetPatient struct {
	ID string
}
type PatientPage struct{}
type CreatePatient struct {
	Name string
	Tags []string
	Age int
	Remember bool
}
type CreatePatientResult struct{}
type PatientCreated struct{}
type PatientNotice struct{}
type SyncPatients struct{}

func Register(r *gowdkcontracts.Registry) {
	gowdkcontracts.RegisterQuery[GetPatient, PatientPage](r, LoadPatient, gowdkcontracts.RoleWeb)
	gowdkcontracts.RegisterCommand[CreatePatient, CreatePatientResult](r, HandleCreatePatient, gowdkcontracts.RoleWeb)
	gowdkcontracts.RegisterDomainEvent[PatientCreated](r, SendWelcomeEmail, gowdkcontracts.RoleWorker)
	gowdkcontracts.RegisterPresentationEvent[PatientNotice](r, NotifyBrowser, gowdkcontracts.RoleWeb)
	gowdkcontracts.RegisterJob[SyncPatients](r, Sync, gowdkcontracts.RoleCron)
}

func LoadPatient(ctx context.Context, query GetPatient) (PatientPage, error) {
	return PatientPage{}, nil
}

func HandleCreatePatient(ctx context.Context, command CreatePatient) (CreatePatientResult, error) {
	if err := gowdkcontracts.EmitDomain(ctx, PatientCreated{}); err != nil {
		return CreatePatientResult{}, err
	}
	if err := gowdkcontracts.EmitPresentation[PatientNotice](ctx, PatientNotice{}); err != nil {
		return CreatePatientResult{}, err
	}
	return CreatePatientResult{}, nil
}

func SendWelcomeEmail(ctx context.Context, event PatientCreated) error {
	return nil
}

func NotifyBrowser(ctx context.Context, event PatientNotice) error {
	return nil
}

func Sync(ctx context.Context, job SyncPatients) error {
	return nil
}
`)
	writeFile(t, filepath.Join(root, "ignored_test.go"), `package patients

import c "github.com/cssbruno/gowdk/runtime/contracts"

func RegisterTest(r *c.Registry) {
	c.RegisterCommand[TestCommand, TestResult](r, HandleTest)
}
`)

	report, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Contracts) != 5 {
		t.Fatalf("len(report.Contracts) = %d, want 5: %#v", len(report.Contracts), report.Contracts)
	}
	assertContract(t, report.Contracts, runtimecontracts.Command, "", "CreatePatient", "CreatePatientResult", "HandleCreatePatient")
	assertContract(t, report.Contracts, runtimecontracts.Query, "", "GetPatient", "PatientPage", "LoadPatient")
	assertContract(t, report.Contracts, runtimecontracts.Event, runtimecontracts.DomainEvent, "PatientCreated", "", "SendWelcomeEmail")
	assertContract(t, report.Contracts, runtimecontracts.Event, runtimecontracts.PresentationEvent, "PatientNotice", "", "NotifyBrowser")
	assertContract(t, report.Contracts, runtimecontracts.Job, "", "SyncPatients", "", "Sync")
	command := findContract(t, report.Contracts, runtimecontracts.Command, "CreatePatient")
	if command.Register != "Register" {
		t.Fatalf("unexpected command register function: %#v", command)
	}
	if got := inputFieldsString(command.InputFields); got != "Name:Name:string,Tags:Tags:[]string,Age:Age:int,Remember:Remember:bool" {
		t.Fatalf("unexpected command input fields: %s", got)
	}
	query := findContract(t, report.Contracts, runtimecontracts.Query, "GetPatient")
	if got := inputFieldsString(query.InputFields); got != "ID:ID:string" {
		t.Fatalf("unexpected query input fields: %s", got)
	}
	if len(command.Roles) != 1 || command.Roles[0] != "web" {
		t.Fatalf("unexpected command roles: %#v", command.Roles)
	}
	if len(command.Emits) != 2 {
		t.Fatalf("command emits = %#v, want two events", command.Emits)
	}
	if command.Emits[0] != (EventRef{Category: runtimecontracts.DomainEvent, Type: "PatientCreated"}) ||
		command.Emits[1] != (EventRef{Category: runtimecontracts.PresentationEvent, Type: "PatientNotice"}) {
		t.Fatalf("unexpected command emits: %#v", command.Emits)
	}
	if len(report.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %#v", report.Diagnostics)
	}
}

func TestScanReportsInvalidHandlerSignatures(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "patients.go"), `package patients

import (
	"context"
	contracts "github.com/cssbruno/gowdk/runtime/contracts"
)

type CreatePatient struct{}
type CreatePatientResult struct{}

func Register(r *contracts.Registry) {
	contracts.RegisterCommand[CreatePatient, CreatePatientResult](r, HandleCreatePatient)
}

func HandleCreatePatient(ctx context.Context, command string) (CreatePatientResult, error) {
	return CreatePatientResult{}, nil
}
`)

	report, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Diagnostics) != 1 {
		t.Fatalf("expected one diagnostic, got %#v", report.Diagnostics)
	}
	if report.Diagnostics[0].Severity != "error" || report.Diagnostics[0].Code != "contract_handler_invalid" || report.Diagnostics[0].Source != "patients.go" {
		t.Fatalf("unexpected diagnostic metadata: %#v", report.Diagnostics[0])
	}
	if want := "second parameter must be CreatePatient"; !strings.Contains(report.Diagnostics[0].Message, want) {
		t.Fatalf("expected %q in diagnostic: %#v", want, report.Diagnostics[0])
	}
}

func TestScanReportsDuplicateCommandOwners(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "patients.go"), `package patients

import (
	"context"
	contracts "github.com/cssbruno/gowdk/runtime/contracts"
)

type CreatePatient struct{}
type CreatePatientResult struct{}

func Register(r *contracts.Registry) {
	contracts.RegisterCommand[CreatePatient, CreatePatientResult](r, HandleCreatePatient)
	contracts.RegisterCommand[CreatePatient, CreatePatientResult](r, HandleCreatePatientAgain)
}

func HandleCreatePatient(ctx context.Context, command CreatePatient) (CreatePatientResult, error) {
	return CreatePatientResult{}, nil
}

func HandleCreatePatientAgain(ctx context.Context, command CreatePatient) (CreatePatientResult, error) {
	return CreatePatientResult{}, nil
}
`)

	report, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	var duplicate Diagnostic
	for _, diagnostic := range report.Diagnostics {
		if strings.Contains(diagnostic.Message, "multiple owner registrations") {
			duplicate = diagnostic
			break
		}
	}
	if duplicate.Message == "" {
		t.Fatalf("expected duplicate command diagnostic, got %#v", report.Diagnostics)
	}
	if duplicate.Code != "duplicate_command_owner" || duplicate.Kind != runtimecontracts.Command || duplicate.Package != "patients" || duplicate.Type != "CreatePatient" {
		t.Fatalf("unexpected duplicate diagnostic metadata: %#v", duplicate)
	}
}

func TestLinkReferencesMarksBoundMissingAndInvalidContractRefs(t *testing.T) {
	report := Report{
		Contracts: []Contract{
			{Kind: runtimecontracts.Command, Package: "patients", Type: "CreatePatient", Result: "CreatePatientResult", Handler: "HandleCreatePatient", Register: "Register"},
			{Kind: runtimecontracts.Command, Package: "billing", Type: "PayInvoice", Result: "PayInvoiceResult", Handler: "HandlePayInvoice"},
			{Kind: runtimecontracts.Query, Package: "patients", Type: "GetPatientPage", Result: "PatientPageData", Handler: "LoadPatientPage", Register: "Register"},
			{Kind: runtimecontracts.Query, Package: "billing", Type: "GetInvoicePage", Result: "InvoicePageData", Handler: "LoadInvoicePage"},
		},
		Diagnostics: []Diagnostic{
			{Severity: "error", Kind: runtimecontracts.Command, Package: "billing", Type: "PayInvoice", Handler: "HandlePayInvoice", Message: "bad handler"},
			{Severity: "error", Kind: runtimecontracts.Query, Package: "billing", Type: "GetInvoicePage", Handler: "LoadInvoicePage", Message: "bad query handler"},
		},
	}
	linked := LinkReferences([]gwdkir.ContractReference{
		{Kind: gwdkir.ContractCommand, Name: "patients.CreatePatient"},
		{Kind: gwdkir.ContractCommand, Name: "billing.PayInvoice"},
		{Kind: gwdkir.ContractCommand, Name: "patients.DeletePatient"},
		{Kind: gwdkir.ContractQuery, Name: "patients.GetPatientPage"},
		{Kind: gwdkir.ContractQuery, Name: "billing.GetInvoicePage"},
		{Kind: gwdkir.ContractQuery, Name: "patients.GetMissingPage"},
	}, report)

	if len(linked) != 6 {
		t.Fatalf("expected six linked refs, got %#v", linked)
	}
	if linked[0].Status != gwdkir.ContractBindingBound || linked[0].Handler != "HandleCreatePatient" {
		t.Fatalf("expected bound command, got %#v", linked[0])
	}
	if linked[0].Register != "Register" {
		t.Fatalf("expected bound command register metadata, got %#v", linked[0])
	}
	if linked[0].Type != "CreatePatient" || linked[0].Result != "CreatePatientResult" {
		t.Fatalf("expected bound command type/result metadata, got %#v", linked[0])
	}
	if linked[1].Status != gwdkir.ContractBindingInvalid || linked[1].Handler != "HandlePayInvoice" || linked[1].Message != "bad handler" {
		t.Fatalf("expected invalid command, got %#v", linked[1])
	}
	if linked[2].Status != gwdkir.ContractBindingMissing || !strings.Contains(linked[2].Message, "no scanned Go registration") {
		t.Fatalf("expected missing command, got %#v", linked[2])
	}
	if linked[3].Status != gwdkir.ContractBindingBound || linked[3].Handler != "LoadPatientPage" {
		t.Fatalf("expected bound query, got %#v", linked[3])
	}
	if linked[3].Register != "Register" {
		t.Fatalf("expected bound query register metadata, got %#v", linked[3])
	}
	if linked[3].Type != "GetPatientPage" || linked[3].Result != "PatientPageData" {
		t.Fatalf("expected bound query type/result metadata, got %#v", linked[3])
	}
	if linked[4].Status != gwdkir.ContractBindingInvalid || linked[4].Handler != "LoadInvoicePage" || linked[4].Message != "bad query handler" {
		t.Fatalf("expected invalid query, got %#v", linked[4])
	}
	if linked[5].Status != gwdkir.ContractBindingMissing || !strings.Contains(linked[5].Message, "no scanned Go registration") {
		t.Fatalf("expected missing query, got %#v", linked[5])
	}
}

func TestLinkReferencesUsesCapturedTypeForImportAliases(t *testing.T) {
	report := Report{
		Contracts: []Contract{
			{Kind: runtimecontracts.Command, Package: "patients", Type: "CreatePatient", Result: "CreatePatientResult", Handler: "HandleCreatePatient"},
			{Kind: runtimecontracts.Query, Package: "patients", Type: "GetPatientPage", Result: "PatientPageData", Handler: "LoadPatientPage"},
		},
	}
	linked := LinkReferences([]gwdkir.ContractReference{
		{Kind: gwdkir.ContractCommand, Name: "p.CreatePatient", ImportAlias: "p", ImportPath: "example.com/app/contracts/patients", Type: "CreatePatient"},
		{Kind: gwdkir.ContractQuery, Name: "p.GetPatientPage", ImportAlias: "p", ImportPath: "example.com/app/contracts/patients", Type: "GetPatientPage"},
	}, report)

	if len(linked) != 2 {
		t.Fatalf("expected two linked refs, got %#v", linked)
	}
	if linked[0].Status != gwdkir.ContractBindingBound || linked[0].Handler != "HandleCreatePatient" || linked[0].Result != "CreatePatientResult" {
		t.Fatalf("expected alias command to bind by captured type, got %#v", linked[0])
	}
	if linked[1].Status != gwdkir.ContractBindingBound || linked[1].Handler != "LoadPatientPage" || linked[1].Result != "PatientPageData" {
		t.Fatalf("expected alias query to bind by captured type, got %#v", linked[1])
	}
}

func TestReportJSONCanFilterByKind(t *testing.T) {
	report := Report{
		Version: 1,
		Root:    "/repo",
		Contracts: []Contract{
			{Kind: runtimecontracts.Command, Type: "CreatePatient"},
			{Kind: runtimecontracts.Query, Type: "GetPatient"},
		},
	}
	payload, err := report.JSON(runtimecontracts.Command)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Contracts []Contract `json:"contracts"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Contracts) != 1 || decoded.Contracts[0].Kind != runtimecontracts.Command {
		t.Fatalf("unexpected filtered JSON: %s", payload)
	}
}

func assertContract(t *testing.T, contracts []Contract, kind runtimecontracts.Kind, category runtimecontracts.EventCategory, typ, result, handler string) {
	t.Helper()
	for _, contract := range contracts {
		if contract.Kind == kind &&
			contract.EventCategory == category &&
			contract.Type == typ &&
			contract.Result == result &&
			contract.Handler == handler {
			return
		}
	}
	t.Fatalf("missing contract kind=%s category=%s type=%s result=%s handler=%s in %#v", kind, category, typ, result, handler, contracts)
}

func findContract(t *testing.T, contracts []Contract, kind runtimecontracts.Kind, typ string) Contract {
	t.Helper()
	for _, contract := range contracts {
		if contract.Kind == kind && contract.Type == typ {
			return contract
		}
	}
	t.Fatalf("missing contract kind=%s type=%s in %#v", kind, typ, contracts)
	return Contract{}
}

func inputFieldsString(fields []manifest.BackendInputField) string {
	parts := make([]string, 0, len(fields))
	for _, field := range fields {
		parts = append(parts, field.FieldName+":"+field.FormName+":"+field.Type)
	}
	return strings.Join(parts, ",")
}

func writeFile(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
