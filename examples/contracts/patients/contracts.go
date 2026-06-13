package patients

import (
	"context"

	"github.com/cssbruno/gowdk/runtime/contracts"
)

type GetPatientPage struct {
	Filter string
}

type PatientPageData struct {
	Filter string `json:"filter"`
	Source string `json:"source"`
}

type CreatePatient struct {
	Name string
}

type CreatePatientResult struct {
	ID string `json:"id"`
}

type PatientCreated struct {
	ID string
}

func Register(registry *contracts.Registry) {
	contracts.RegisterQuery[GetPatientPage, PatientPageData](registry, LoadPatientPage, contracts.RoleWeb)
	contracts.RegisterCommand[CreatePatient, CreatePatientResult](registry, HandleCreatePatient, contracts.RoleWeb)
	contracts.RegisterDomainEvent[PatientCreated](registry, SendWelcomeEmail, contracts.RoleWorker)
}

func LoadPatientPage(ctx context.Context, query GetPatientPage) (PatientPageData, error) {
	return PatientPageData{Filter: query.Filter, Source: "contracts-example"}, nil
}

func HandleCreatePatient(ctx context.Context, command CreatePatient) (CreatePatientResult, error) {
	if err := contracts.EmitDomain(ctx, PatientCreated{ID: "patient-1"}); err != nil {
		return CreatePatientResult{}, err
	}
	return CreatePatientResult{ID: "patient-1"}, nil
}

func SendWelcomeEmail(ctx context.Context, event PatientCreated) error {
	return nil
}
