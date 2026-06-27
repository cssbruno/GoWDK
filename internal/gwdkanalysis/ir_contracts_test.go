package gwdkanalysis

import (
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

func TestBuildProgramDerivesPageOwnedContractRoutes(t *testing.T) {
	program := BuildProgram(gowdk.Config{}, Sources{Pages: []gwdkir.Page{{
		Package: "pages",
		ID:      "patients",
		Route:   "/patients",
		Blocks: gwdkir.Blocks{
			View: true,
			ViewBody: `<main>
  <form method="patch" action="/patients/archive" g:command="patients.ArchivePatients"></form>
  <form g:command="patients.CreatePatient"></form>
  <section g:query="patients.GetPatientPage"></section>
</main>`,
		},
	}}})

	refs := contractRefsByName(program.ContractRefs)
	if ref := refs["patients.ArchivePatients"]; ref.Method != "PATCH" || ref.Path != "/patients/archive" {
		t.Fatalf("literal command method/path = %s %s, want PATCH /patients/archive", ref.Method, ref.Path)
	}
	if ref := refs["patients.CreatePatient"]; ref.Method != "POST" || ref.Path != "/patients" {
		t.Fatalf("default command method/path = %s %s, want POST /patients", ref.Method, ref.Path)
	}
	if ref := refs["patients.GetPatientPage"]; ref.Method != "GET" || ref.Path != "/patients" {
		t.Fatalf("page query method/path = %s %s, want GET /patients", ref.Method, ref.Path)
	}
}

func TestBuildProgramLowersServerFields(t *testing.T) {
	program := BuildProgram(gowdk.Config{}, Sources{Pages: []gwdkir.Page{{
		Package: "pages",
		ID:      "dashboard",
		Route:   "/dashboard",
		Blocks: gwdkir.Blocks{
			Server: true,
			ServerBody: `user := session.User()
  => { title: "Dashboard", user.name, account.plan }`,
			View:     true,
			ViewBody: `<main>{title} {user.name} {account.plan}</main>`,
		},
	}}})

	if len(program.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %#v", program.Diagnostics)
	}
	if len(program.Pages) != 1 {
		t.Fatalf("expected one page, got %#v", program.Pages)
	}
	got := program.Pages[0].Blocks.ServerFields
	want := []string{"title", "user.name", "account.plan"}
	if len(got) != len(want) {
		t.Fatalf("server fields = %#v, want %#v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("server fields = %#v, want %#v", got, want)
		}
	}
}

func TestBuildProgramRejectsDynamicPageOwnedDefaultContractRoutes(t *testing.T) {
	program := BuildProgram(gowdk.Config{}, Sources{Pages: []gwdkir.Page{{
		Package: "pages",
		ID:      "blog.show",
		Route:   "/blog/{slug}",
		Blocks: gwdkir.Blocks{
			Paths: true,
			View:  true,
			ViewBody: `<main>
  <form g:command="posts.CreateComment"></form>
  <section g:query="posts.GetComments"></section>
</main>`,
		},
	}}})

	refs := contractRefsByName(program.ContractRefs)
	if ref := refs["posts.CreateComment"]; ref.Method != "POST" || ref.Path != "" {
		t.Fatalf("dynamic default command method/path = %s %s, want POST with empty non-routable path", ref.Method, ref.Path)
	}
	if ref := refs["posts.GetComments"]; ref.Method != "" || ref.Path != "" {
		t.Fatalf("dynamic default query method/path = %s %s, want empty non-routable metadata", ref.Method, ref.Path)
	}
	if len(program.Diagnostics) != 2 {
		t.Fatalf("expected two dynamic default contract route diagnostics, got %#v", program.Diagnostics)
	}
	for _, diagnostic := range program.Diagnostics {
		if diagnostic.Code != "contract_route_invalid" {
			t.Fatalf("expected contract_route_invalid diagnostic, got %#v", diagnostic)
		}
	}
}

func TestBuildProgramKeepsComponentQueryNonRoutable(t *testing.T) {
	program := BuildProgram(gowdk.Config{}, Sources{Components: []gwdkir.Component{{
		Package: "components",
		Name:    "PatientList",
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<section g:query="patients.GetPatientPage"></section>`,
		},
	}}})

	refs := contractRefsByName(program.ContractRefs)
	if ref := refs["patients.GetPatientPage"]; ref.Method != "" || ref.Path != "" {
		t.Fatalf("component query method/path = %s %s, want empty non-routable metadata", ref.Method, ref.Path)
	}
}

func TestBuildProgramLowersRealtimeSubscriptions(t *testing.T) {
	program := BuildProgram(gowdk.Config{}, Sources{Pages: []gwdkir.Page{{
		Source:  "pages/patients.page.gwdk",
		Package: "pages",
		ID:      "patients",
		Route:   "/patients",
		Imports: []gwdkir.Import{{
			Alias: "patientcontracts",
			Path:  "example.com/app/contracts/patients",
		}},
		Blocks: gwdkir.Blocks{
			View: true,
			Spans: gwdkir.BlockSpans{
				ViewBodyStart: sourcePosition(7, 1),
			},
			ViewBody: `<main>
  <section g:query="patientcontracts.GetPatientPage" g:subscribe="patientcontracts.PatientNotice"></section>
</main>`,
		},
	}}})

	if len(program.RealtimeSubscriptions) != 1 {
		t.Fatalf("expected one realtime subscription, got %#v", program.RealtimeSubscriptions)
	}
	subscription := program.RealtimeSubscriptions[0]
	if subscription.Query != "patientcontracts.GetPatientPage" || subscription.QueryType != "GetPatientPage" || subscription.QueryImportAlias != "patientcontracts" {
		t.Fatalf("unexpected query metadata: %#v", subscription)
	}
	if subscription.QueryImportPath != "example.com/app/contracts/patients" {
		t.Fatalf("unexpected query import path: %#v", subscription)
	}
	if subscription.Event != "patientcontracts.PatientNotice" || subscription.EventType != "PatientNotice" || subscription.EventImportAlias != "patientcontracts" {
		t.Fatalf("unexpected event metadata: %#v", subscription)
	}
	if subscription.EventImportPath != "example.com/app/contracts/patients" {
		t.Fatalf("unexpected event import path: %#v", subscription)
	}
	if subscription.OwnerKind != gwdkir.SourcePage || subscription.OwnerID != "patients" || subscription.Source != "pages/patients.page.gwdk" {
		t.Fatalf("unexpected owner metadata: %#v", subscription)
	}
	if subscription.Span.Start.Line != 8 || subscription.QuerySpan.Start.Line != 8 {
		t.Fatalf("unexpected subscription spans: %#v", subscription)
	}
}

func contractRefsByName(refs []gwdkir.ContractReference) map[string]gwdkir.ContractReference {
	out := make(map[string]gwdkir.ContractReference, len(refs))
	for _, ref := range refs {
		out[ref.Name] = ref
	}
	return out
}

func sourcePosition(line, column int) source.SourcePosition {
	return source.SourcePosition{Line: line, Column: column}
}
