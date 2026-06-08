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

func TestScanLinksContractsAcrossPackageFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "register.go"), `package patients

import contracts "github.com/cssbruno/gowdk/runtime/contracts"

func Register(r *contracts.Registry) {
	contracts.RegisterQuery[GetPatient, PatientPage](r, LoadPatient, contracts.RoleWeb)
	contracts.RegisterCommand[CreatePatient, CreatePatientResult](r, HandleCreatePatient, contracts.RoleWeb)
}
`)
	writeFile(t, filepath.Join(root, "types.go"), `package patients

type GetPatient struct {
	ID string
}

type PatientPage struct{}

type CreatePatient struct {
	Name string
	Tags []string
}

type CreatePatientResult struct{}
`)
	writeFile(t, filepath.Join(root, "handlers.go"), `package patients

import "context"

func LoadPatient(ctx context.Context, query GetPatient) (PatientPage, error) {
	return PatientPage{}, nil
}

func HandleCreatePatient(ctx context.Context, command CreatePatient) (CreatePatientResult, error) {
	return CreatePatientResult{}, nil
}
`)

	report, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %#v", report.Diagnostics)
	}
	command := findContract(t, report.Contracts, runtimecontracts.Command, "CreatePatient")
	if command.Register != "Register" || command.Handler != "HandleCreatePatient" {
		t.Fatalf("unexpected cross-file command metadata: %#v", command)
	}
	if got := inputFieldsString(command.InputFields); got != "Name:Name:string,Tags:Tags:[]string" {
		t.Fatalf("unexpected cross-file command input fields: %s", got)
	}
	query := findContract(t, report.Contracts, runtimecontracts.Query, "GetPatient")
	if got := inputFieldsString(query.InputFields); got != "ID:ID:string" {
		t.Fatalf("unexpected cross-file query input fields: %s", got)
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

func TestScanReportsInvalidImportedHandlerSignatures(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/app\n\ngo 1.26.4\n\nrequire github.com/cssbruno/gowdk v0.0.0\nreplace github.com/cssbruno/gowdk => "+gowdkRepoRoot(t)+"\n")
	writeFile(t, filepath.Join(root, "contractdefs", "types.go"), `package contractdefs

type CreatePatient struct{}
type CreatePatientResult struct{}
type PatientCreated struct{}
type SyncPatients struct{}
`)
	writeFile(t, filepath.Join(root, "patienthandlers", "handlers.go"), `package patienthandlers

import (
	"context"

	"example.com/app/contractdefs"
)

func HandleCreatePatient(ctx context.Context, command string) (contractdefs.CreatePatientResult, error) {
	return contractdefs.CreatePatientResult{}, nil
}

func SendWelcomeEmail(ctx context.Context, event contractdefs.PatientCreated) (int, error) {
	return 0, nil
}

func SyncPatients(ctx context.Context, job contractdefs.SyncPatients) {
}
`)
	writeFile(t, filepath.Join(root, "patients", "register.go"), `package patients

import (
	"example.com/app/contractdefs"
	"example.com/app/patienthandlers"
	contracts "github.com/cssbruno/gowdk/runtime/contracts"
)

func Register(r *contracts.Registry) {
	contracts.RegisterCommand[contractdefs.CreatePatient, contractdefs.CreatePatientResult](r, patienthandlers.HandleCreatePatient)
	contracts.RegisterDomainEvent[contractdefs.PatientCreated](r, patienthandlers.SendWelcomeEmail)
	contracts.RegisterJob[contractdefs.SyncPatients](r, patienthandlers.SyncPatients)
}
`)

	report, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	var command, event, job Diagnostic
	for _, diagnostic := range report.Diagnostics {
		if diagnostic.Code != "contract_handler_invalid" {
			continue
		}
		switch diagnostic.Kind {
		case runtimecontracts.Command:
			command = diagnostic
		case runtimecontracts.Event:
			event = diagnostic
		case runtimecontracts.Job:
			job = diagnostic
		}
	}
	if command.Handler != "patienthandlers.HandleCreatePatient" || !strings.Contains(command.Message, "second parameter must be contractdefs.CreatePatient") {
		t.Fatalf("unexpected imported command diagnostic: %#v in %#v", command, report.Diagnostics)
	}
	if command.TypeImportPath != "example.com/app/contractdefs" {
		t.Fatalf("command TypeImportPath = %q, want example.com/app/contractdefs", command.TypeImportPath)
	}
	if event.Handler != "patienthandlers.SendWelcomeEmail" || !strings.Contains(event.Message, "must return error") {
		t.Fatalf("unexpected imported event diagnostic: %#v in %#v", event, report.Diagnostics)
	}
	if job.Handler != "patienthandlers.SyncPatients" || !strings.Contains(job.Message, "must return error") {
		t.Fatalf("unexpected imported job diagnostic: %#v in %#v", job, report.Diagnostics)
	}
}

func TestScanReportsInvalidImportedContractTypes(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/app\n\ngo 1.26.4\n\nrequire github.com/cssbruno/gowdk v0.0.0\nreplace github.com/cssbruno/gowdk => "+gowdkRepoRoot(t)+"\n")
	writeFile(t, filepath.Join(root, "contractdefs", "types.go"), `package contractdefs

type CreatePatient string
type CreatePatientResult string
type PatientCreated string
`)
	writeFile(t, filepath.Join(root, "patients", "register.go"), `package patients

import (
	"context"

	defs "example.com/app/contractdefs"
	contracts "github.com/cssbruno/gowdk/runtime/contracts"
)

func Register(r *contracts.Registry) {
	contracts.RegisterCommand[defs.CreatePatient, defs.CreatePatientResult](r, HandleCreatePatient)
	contracts.RegisterDomainEvent[defs.PatientCreated](r, SendWelcomeEmail)
}

func HandleCreatePatient(ctx context.Context, command defs.CreatePatient) (defs.CreatePatientResult, error) {
	return "", nil
}

func SendWelcomeEmail(ctx context.Context, event defs.PatientCreated) error {
	return nil
}
`)

	report, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Diagnostics) < 3 {
		t.Fatalf("expected imported type diagnostics, got %#v", report.Diagnostics)
	}
	input := findDiagnostic(t, report.Diagnostics, "contract_type_invalid")
	if input.Type != "defs.CreatePatient" || input.TypeImportPath != "example.com/app/contractdefs" || !strings.Contains(input.Message, "command contract type defs.CreatePatient must be a struct") {
		t.Fatalf("unexpected imported input diagnostic: %#v", input)
	}
	result := findDiagnostic(t, report.Diagnostics, "contract_result_invalid")
	if result.Type != "defs.CreatePatient" || result.TypeImportPath != "example.com/app/contractdefs" || !strings.Contains(result.Message, "command result type defs.CreatePatientResult must be a struct") {
		t.Fatalf("unexpected imported result diagnostic: %#v", result)
	}
	var event Diagnostic
	for _, diagnostic := range report.Diagnostics {
		if diagnostic.Code == "contract_type_invalid" && diagnostic.Kind == runtimecontracts.Event {
			event = diagnostic
			break
		}
	}
	if event.Type != "defs.PatientCreated" || event.TypeImportPath != "example.com/app/contractdefs" || !strings.Contains(event.Message, "event contract type defs.PatientCreated must be a struct") {
		t.Fatalf("unexpected imported event diagnostic: %#v in %#v", event, report.Diagnostics)
	}
}

func TestScanReportsUnexportedLocalHandler(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "patients.go"), `package patients

import (
	"context"
	contracts "github.com/cssbruno/gowdk/runtime/contracts"
)

type CreatePatient struct{}
type CreatePatientResult struct{}

func Register(r *contracts.Registry) {
	contracts.RegisterCommand[CreatePatient, CreatePatientResult](r, handleCreatePatient)
}

func handleCreatePatient(ctx context.Context, command CreatePatient) (CreatePatientResult, error) {
	return CreatePatientResult{}, nil
}
`)

	report, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	diagnostic := findDiagnostic(t, report.Diagnostics, "contract_handler_invalid")
	if diagnostic.Handler != "handleCreatePatient" {
		t.Fatalf("handler = %q, want handleCreatePatient", diagnostic.Handler)
	}
	if want := "handler handleCreatePatient must be exported"; !strings.Contains(diagnostic.Message, want) {
		t.Fatalf("expected %q in diagnostic: %#v", want, diagnostic)
	}
}

func TestScanReportsInvalidLocalContractTypes(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "patients.go"), `package patients

import (
	"context"
	contracts "github.com/cssbruno/gowdk/runtime/contracts"
)

type createPatient struct{}
type CreatePatientResult string
type PatientCreated string

func Register(r *contracts.Registry) {
	contracts.RegisterCommand[createPatient, CreatePatientResult](r, HandleCreatePatient)
	contracts.RegisterDomainEvent[PatientCreated](r, SendWelcomeEmail)
}

func HandleCreatePatient(ctx context.Context, command createPatient) (CreatePatientResult, error) {
	return "", nil
}

func SendWelcomeEmail(ctx context.Context, event PatientCreated) error {
	return nil
}
`)

	report, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Diagnostics) < 3 {
		t.Fatalf("expected at least three diagnostics, got %#v", report.Diagnostics)
	}
	input := findDiagnostic(t, report.Diagnostics, "contract_type_invalid")
	if input.Type != "createPatient" || !strings.Contains(input.Message, "command contract type createPatient must be exported") {
		t.Fatalf("unexpected input diagnostic: %#v", input)
	}
	result := findDiagnostic(t, report.Diagnostics, "contract_result_invalid")
	if result.Type != "createPatient" || !strings.Contains(result.Message, "command result type CreatePatientResult must be a struct") {
		t.Fatalf("unexpected result diagnostic: %#v", result)
	}
	var event Diagnostic
	for _, diagnostic := range report.Diagnostics {
		if diagnostic.Code == "contract_type_invalid" && diagnostic.Kind == runtimecontracts.Event {
			event = diagnostic
			break
		}
	}
	if event.Message == "" || !strings.Contains(event.Message, "event contract type PatientCreated must be a struct") {
		t.Fatalf("unexpected event diagnostic: %#v in %#v", event, report.Diagnostics)
	}
}

func TestScanReportsNoisyEventNames(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "patients.go"), `package patients

import (
	"context"
	contracts "github.com/cssbruno/gowdk/runtime/contracts"
)

type ButtonClicked struct{}
type PatientChanged struct{}

func Register(r *contracts.Registry) {
	contracts.RegisterDomainEvent[ButtonClicked](r, HandleButtonClicked)
	contracts.RegisterIntegrationEvent[PatientChanged](r, HandlePatientChanged)
}

func HandleButtonClicked(ctx context.Context, event ButtonClicked) error {
	return nil
}

func HandlePatientChanged(ctx context.Context, event PatientChanged) error {
	return nil
}
`)

	report, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	var button, changed Diagnostic
	for _, diagnostic := range report.Diagnostics {
		if diagnostic.Code != "contract_event_name_invalid" {
			continue
		}
		switch diagnostic.Type {
		case "ButtonClicked":
			button = diagnostic
		case "PatientChanged":
			changed = diagnostic
		}
	}
	if button.Message == "" || !strings.Contains(button.Message, "looks like a browser UI event") {
		t.Fatalf("expected UI event diagnostic, got %#v in %#v", button, report.Diagnostics)
	}
	if changed.Message == "" || !strings.Contains(changed.Message, "too vague") {
		t.Fatalf("expected vague event diagnostic, got %#v in %#v", changed, report.Diagnostics)
	}
}

func TestScanReportsEmittedEventCategoryMismatch(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "patients.go"), `package patients

import (
	"context"
	contracts "github.com/cssbruno/gowdk/runtime/contracts"
)

type CreatePatient struct{}
type CreatePatientResult struct{}
type PatientCreated struct{}

func Register(r *contracts.Registry) {
	contracts.RegisterCommand[CreatePatient, CreatePatientResult](r, HandleCreatePatient)
	contracts.RegisterDomainEvent[PatientCreated](r, SendWelcomeEmail)
}

func HandleCreatePatient(ctx context.Context, command CreatePatient) (CreatePatientResult, error) {
	if err := contracts.EmitPresentation(ctx, PatientCreated{}); err != nil {
		return CreatePatientResult{}, err
	}
	return CreatePatientResult{}, nil
}

func SendWelcomeEmail(ctx context.Context, event PatientCreated) error {
	return nil
}
`)

	report, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	diagnostic := findDiagnostic(t, report.Diagnostics, "contract_event_category_invalid")
	if diagnostic.Kind != runtimecontracts.Command || diagnostic.Type != "CreatePatient" {
		t.Fatalf("unexpected event category diagnostic metadata: %#v", diagnostic)
	}
	if want := "emits presentation event PatientCreated but scanned registrations use event categories domain"; !strings.Contains(diagnostic.Message, want) {
		t.Fatalf("expected %q in diagnostic: %#v", want, diagnostic)
	}
}

func TestScanReportsGeneratedAppImportCycles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "patients.go"), `package patients

import gowdkapp "gowdk-generated-app/gowdkapp"

var _ = gowdkapp.Handler
`)

	report, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	diagnostic := findDiagnostic(t, report.Diagnostics, "generated_app_import_cycle")
	if diagnostic.Package != "patients" || diagnostic.Source != "patients.go" || diagnostic.Line != 3 {
		t.Fatalf("unexpected generated app import diagnostic metadata: %#v", diagnostic)
	}
	if want := `must not import generated app output "gowdk-generated-app/gowdkapp"`; !strings.Contains(diagnostic.Message, want) {
		t.Fatalf("expected %q in diagnostic: %#v", want, diagnostic)
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
			{Kind: runtimecontracts.Command, Package: "patients", Type: "CreatePatient", Result: "CreatePatientResult", Handler: "HandleCreatePatient", Register: "Register", Roles: []string{"web"}},
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
	if len(linked[0].Roles) != 1 || linked[0].Roles[0] != "web" {
		t.Fatalf("expected bound command role metadata, got %#v", linked[0].Roles)
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
			{Kind: runtimecontracts.Command, Package: "handlers", Type: "patientdefs.CreatePatient", TypeImportPath: "example.com/app/contracts/patients", Result: "patientdefs.CreatePatientResult", ResultImportPath: "example.com/app/contracts/patients", Handler: "HandleCreatePatient"},
			{Kind: runtimecontracts.Command, Package: "handlers", Type: "billingdefs.CreatePatient", TypeImportPath: "example.com/app/contracts/billing", Result: "billingdefs.CreatePatientResult", ResultImportPath: "example.com/app/contracts/billing", Handler: "HandleBillingCreatePatient"},
			{Kind: runtimecontracts.Query, Package: "handlers", Type: "patientdefs.GetPatientPage", TypeImportPath: "example.com/app/contracts/patients", Result: "patientdefs.PatientPageData", ResultImportPath: "example.com/app/contracts/patients", Handler: "LoadPatientPage"},
			{Kind: runtimecontracts.Query, Package: "handlers", Type: "billingdefs.GetPatientPage", TypeImportPath: "example.com/app/contracts/billing", Result: "billingdefs.BillingPatientPageData", ResultImportPath: "example.com/app/contracts/billing", Handler: "LoadBillingPatientPage"},
		},
	}
	linked := LinkReferences([]gwdkir.ContractReference{
		{Kind: gwdkir.ContractCommand, Name: "p.CreatePatient", ImportAlias: "p", ImportPath: "example.com/app/contracts/patients", Type: "CreatePatient"},
		{Kind: gwdkir.ContractQuery, Name: "p.GetPatientPage", ImportAlias: "p", ImportPath: "example.com/app/contracts/patients", Type: "GetPatientPage"},
	}, report)

	if len(linked) != 2 {
		t.Fatalf("expected two linked refs, got %#v", linked)
	}
	if linked[0].Status != gwdkir.ContractBindingBound || linked[0].Handler != "HandleCreatePatient" || linked[0].Result != "patientdefs.CreatePatientResult" {
		t.Fatalf("expected alias command to bind by captured type, got %#v", linked[0])
	}
	if linked[1].Status != gwdkir.ContractBindingBound || linked[1].Handler != "LoadPatientPage" || linked[1].Result != "patientdefs.PatientPageData" {
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

func TestPackageInspectionCacheReusesExportFiles(t *testing.T) {
	calls := 0
	cache := &packageInspectionCache{
		exports: map[string]map[string]string{},
		loadExportFiles: func(packageDir string, importPaths []string) (map[string]string, error) {
			calls++
			return map[string]string{"example.com/app/handlers": "/tmp/handlers.a"}, nil
		},
	}

	first, err := cache.exportFiles("/repo/patients", []string{"example.com/app/handlers", "example.com/app/contracts"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := cache.exportFiles("/repo/patients", []string{"example.com/app/contracts", "example.com/app/handlers"})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("load calls = %d, want 1", calls)
	}
	if first["example.com/app/handlers"] != "/tmp/handlers.a" || second["example.com/app/handlers"] != "/tmp/handlers.a" {
		t.Fatalf("unexpected cached exports: first=%#v second=%#v", first, second)
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

func findDiagnostic(t *testing.T, diagnostics []Diagnostic, code string) Diagnostic {
	t.Helper()
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return diagnostic
		}
	}
	t.Fatalf("missing diagnostic code=%s in %#v", code, diagnostics)
	return Diagnostic{}
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

func gowdkRepoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	return filepath.ToSlash(root)
}
