package appgen

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/contractscan"
)

func TestGenerateContractWorkerAppBuildsAndRuns(t *testing.T) {
	root := writeContractRoleFixture(t)
	t.Chdir(root)
	report, err := contractscan.Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	appDir := filepath.Join(root, "generated-worker")
	result, err := GenerateContractWorker(appDir, report, gowdk.ContractWorkerConfig{
		EventSource: gowdk.ServiceRef{ImportPath: "example.com/site/providers", Function: "EventSource"},
	})
	if err != nil {
		t.Fatal(err)
	}
	source := readTestFile(t, result.MainPath)
	for _, expected := range []string{
		`source, err := providers.EventSource()`,
		`gowdkapp.RunContractEventWorkerWithOptions(ctx, source, workerOptions...)`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected worker source to contain %q:\n%s", expected, source)
		}
	}
	binary := filepath.Join(root, "bin", "worker")
	if _, err := BuildWorkerBinary(appDir, binary); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(binary)
	command.Env = os.Environ()
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generated worker binary failed: %v\n%s", err, output)
	}
}

func TestGenerateContractCronAppBuildsAndRunsOnceJob(t *testing.T) {
	root := writeContractRoleFixture(t)
	t.Chdir(root)
	report, err := contractscan.Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	appDir := filepath.Join(root, "generated-cron")
	result, err := GenerateContractCron(appDir, report, gowdk.ContractCronConfig{Jobs: []gowdk.ContractCronJobConfig{{
		Type:            "patients.SyncPatients",
		Schedule:        "@once",
		OverlapPolicy:   "skip",
		MissedRunPolicy: "skip",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(result.MainPath); err != nil {
		t.Fatal(err)
	}
	source := readTestFile(t, filepath.Join(appDir, cronFileName))
	for _, expected := range []string{
		`Schedule: "@once"`,
		`gowdkcontracts.ExecuteJobForRole[patients.SyncPatients]`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected cron source to contain %q:\n%s", expected, source)
		}
	}
	binary := filepath.Join(root, "bin", "cron")
	if _, err := BuildCronBinary(appDir, binary); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(root, "cron-ran.txt")
	command := exec.Command(binary)
	command.Env = append(os.Environ(), "GOWDK_TEST_CRON_MARKER="+marker)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generated cron binary failed: %v\n%s", err, output)
	}
	if payload := readTestFile(t, marker); payload != "ran" {
		t.Fatalf("unexpected cron marker: %q", payload)
	}
}

func TestGenerateContractCronRejectsUnknownJob(t *testing.T) {
	root := writeContractRoleFixture(t)
	t.Chdir(root)
	report, err := contractscan.Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	_, err = GenerateContractCron(filepath.Join(root, "generated-cron"), report, gowdk.ContractCronConfig{Jobs: []gowdk.ContractCronJobConfig{{
		Type:     "patients.Missing",
		Schedule: "@once",
	}}})
	if err == nil || !strings.Contains(err.Error(), `contract cron job "patients.Missing" was not found`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func writeContractRoleFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	repoRoot, ok := gowdkRuntimeModuleRoot()
	if !ok {
		t.Fatal("could not locate GOWDK module root")
	}
	writeTestFile(t, filepath.Join(root, "go.mod"), `module example.com/site

go 1.22

require github.com/cssbruno/gowdk v0.0.0

replace github.com/cssbruno/gowdk => `+filepath.ToSlash(repoRoot)+`
`)
	writeTestFile(t, filepath.Join(root, "patients", "contracts.go"), `package patients

import (
	"context"
	"os"

	"github.com/cssbruno/gowdk/runtime/contracts"
)

type PatientCreated struct{ ID string }
type SyncPatients struct{}

func Register(registry *contracts.Registry) {
	contracts.RegisterDomainEvent[PatientCreated](registry, SendWelcomeEmail, contracts.RoleWorker)
	contracts.RegisterJob[SyncPatients](registry, RunSyncPatients, contracts.RoleCron)
}

func SendWelcomeEmail(context.Context, PatientCreated) error { return nil }

func RunSyncPatients(context.Context, SyncPatients) error {
	if path := os.Getenv("GOWDK_TEST_CRON_MARKER"); path != "" {
		return os.WriteFile(path, []byte("ran"), 0o644)
	}
	return nil
}
`)
	writeTestFile(t, filepath.Join(root, "providers", "providers.go"), `package providers

import (
	"context"

	"github.com/cssbruno/gowdk/runtime/contracts"
)

type drainedSource struct{}

func EventSource() (contracts.EventSource, error) {
	return drainedSource{}, nil
}

func (drainedSource) ReceiveEventBatch(context.Context) (contracts.EventBatch, error) {
	return contracts.EventBatch{}, contracts.ErrEventSourceClosed
}
`)
	return root
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(payload)
}
