package gwdkanalysis

import (
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkir"
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

func contractRefsByName(refs []gwdkir.ContractReference) map[string]gwdkir.ContractReference {
	out := make(map[string]gwdkir.ContractReference, len(refs))
	for _, ref := range refs {
		out[ref.Name] = ref
	}
	return out
}
