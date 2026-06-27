package contracts

import (
	"context"
	"strings"

	gowdkcontracts "github.com/cssbruno/gowdk/runtime/contracts"
)

type GetDashboardSnapshot struct{}

type DashboardSnapshot struct {
	OpenWorkflows int    `json:"openWorkflows"`
	Source        string `json:"source"`
}

type StartWorkflow struct {
	Name string
}

type StartWorkflowResult struct {
	ID string `json:"id"`
}

type WorkflowStarted struct {
	ID   string
	Name string
}

func Register(registry *gowdkcontracts.Registry) {
	mustRegister(gowdkcontracts.RegisterQuery[GetDashboardSnapshot, DashboardSnapshot](registry, LoadDashboardSnapshot, gowdkcontracts.RoleWeb))
	mustRegister(gowdkcontracts.RegisterCommand[StartWorkflow, StartWorkflowResult](registry, HandleStartWorkflow, gowdkcontracts.RoleWeb))
	mustRegister(gowdkcontracts.RegisterDomainEvent[WorkflowStarted](registry, RecordWorkflowStarted, gowdkcontracts.RoleWorker))
}

func mustRegister(err error) {
	if err != nil {
		panic(err)
	}
}

func LoadDashboardSnapshot(context.Context, GetDashboardSnapshot) (DashboardSnapshot, error) {
	return DashboardSnapshot{OpenWorkflows: 2, Source: "flagship-example"}, nil
}

func HandleStartWorkflow(ctx context.Context, command StartWorkflow) (StartWorkflowResult, error) {
	name := strings.TrimSpace(command.Name)
	if name == "" {
		name = "untitled"
	}
	result := StartWorkflowResult{ID: "workflow-" + strings.ToLower(strings.ReplaceAll(name, " ", "-"))}
	if err := gowdkcontracts.EmitDomain(ctx, WorkflowStarted{ID: result.ID, Name: name}); err != nil {
		return StartWorkflowResult{}, err
	}
	return result, nil
}

func RecordWorkflowStarted(context.Context, WorkflowStarted) error {
	return nil
}
