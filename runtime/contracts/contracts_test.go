package contracts

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"
)

type createPatient struct {
	Name string
}

type createPatientResult struct {
	ID string
}

type patientCreated struct {
	ID string
}

type patientCreatedNotice struct {
	ID string
}

type patientPageQuery struct {
	ID string
}

type patientPage struct {
	Name string
}

type syncPatientsJob struct {
	Limit int
}

type recordingOutbox struct {
	events []EventEnvelope
	err    error
}

func (outbox *recordingOutbox) StoreEvents(ctx context.Context, events []EventEnvelope) error {
	if outbox.err != nil {
		return outbox.err
	}
	outbox.events = append(outbox.events, events...)
	return nil
}

func TestCommandDispatchesDomainEventsAfterSuccess(t *testing.T) {
	registry := NewRegistry()
	var handled []string
	if err := RegisterDomainEvent[patientCreated](registry, func(ctx context.Context, event patientCreated) error {
		handled = append(handled, event.ID)
		return nil
	}, RoleWorker); err != nil {
		t.Fatalf("register event: %v", err)
	}
	if err := RegisterCommand[createPatient, createPatientResult](registry, func(ctx context.Context, command createPatient) (createPatientResult, error) {
		if err := EmitDomain(ctx, patientCreated{ID: "patient-1"}); err != nil {
			return createPatientResult{}, err
		}
		if len(handled) != 0 {
			t.Fatalf("event dispatched before command returned")
		}
		return createPatientResult{ID: "patient-1"}, nil
	}, RoleWeb); err != nil {
		t.Fatalf("register command: %v", err)
	}

	result, err := ExecuteCommand[createPatient, createPatientResult](context.Background(), registry, createPatient{Name: "Ada"})
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}
	if result.ID != "patient-1" {
		t.Fatalf("result.ID = %q, want patient-1", result.ID)
	}
	if !reflect.DeepEqual(handled, []string{"patient-1"}) {
		t.Fatalf("handled = %#v, want patient-1", handled)
	}
}

func TestCommandDoesNotDispatchEventsAfterFailure(t *testing.T) {
	registry := NewRegistry()
	var handled int
	if err := RegisterDomainEvent[patientCreated](registry, func(ctx context.Context, event patientCreated) error {
		handled++
		return nil
	}); err != nil {
		t.Fatalf("register event: %v", err)
	}
	if err := RegisterCommand[createPatient, createPatientResult](registry, func(ctx context.Context, command createPatient) (createPatientResult, error) {
		if err := EmitDomain(ctx, patientCreated{ID: "patient-1"}); err != nil {
			return createPatientResult{}, err
		}
		return createPatientResult{}, errors.New("insert failed")
	}); err != nil {
		t.Fatalf("register command: %v", err)
	}

	_, err := ExecuteCommand[createPatient, createPatientResult](context.Background(), registry, createPatient{Name: "Ada"})
	if err == nil {
		t.Fatalf("execute command returned nil error")
	}
	if handled != 0 {
		t.Fatalf("handled = %d, want 0", handled)
	}
}

func TestCaptureCommandEventsDoesNotDispatchSubscribers(t *testing.T) {
	registry := NewRegistry()
	var handled int
	must(t, RegisterDomainEvent[patientCreated](registry, func(ctx context.Context, event patientCreated) error {
		handled++
		return nil
	}))
	must(t, RegisterPresentationEvent[patientCreatedNotice](registry, func(ctx context.Context, event patientCreatedNotice) error {
		handled++
		return nil
	}))
	must(t, RegisterCommand[createPatient, createPatientResult](registry, func(ctx context.Context, command createPatient) (createPatientResult, error) {
		if err := EmitDomain(ctx, patientCreated{ID: "patient-1"}); err != nil {
			return createPatientResult{}, err
		}
		if err := EmitPresentation(ctx, patientCreatedNotice{ID: "patient-1"}); err != nil {
			return createPatientResult{}, err
		}
		return createPatientResult{ID: "patient-1"}, nil
	}))

	result, events, err := CaptureCommandEvents[createPatient, createPatientResult](context.Background(), registry, createPatient{Name: "Ada"})
	if err != nil {
		t.Fatalf("capture command events: %v", err)
	}
	if result.ID != "patient-1" {
		t.Fatalf("result.ID = %q, want patient-1", result.ID)
	}
	if handled != 0 {
		t.Fatalf("handled = %d, want 0", handled)
	}
	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2: %#v", len(events), events)
	}
	if events[0].Category != DomainEvent || events[0].Type != typeName[patientCreated]() {
		t.Fatalf("first event = %#v, want domain patientCreated", events[0])
	}
	if value, ok := events[0].Value.(patientCreated); !ok || value.ID != "patient-1" {
		t.Fatalf("first event value = %#v, want patientCreated patient-1", events[0].Value)
	}
	if events[1].Category != PresentationEvent || events[1].Type != typeName[patientCreatedNotice]() {
		t.Fatalf("second event = %#v, want presentation patientCreatedNotice", events[1])
	}
}

func TestCaptureCommandEventsDropsEventsAfterFailure(t *testing.T) {
	registry := NewRegistry()
	var handled int
	must(t, RegisterDomainEvent[patientCreated](registry, func(ctx context.Context, event patientCreated) error {
		handled++
		return nil
	}))
	must(t, RegisterCommand[createPatient, createPatientResult](registry, func(ctx context.Context, command createPatient) (createPatientResult, error) {
		if err := EmitDomain(ctx, patientCreated{ID: "patient-1"}); err != nil {
			return createPatientResult{}, err
		}
		return createPatientResult{}, errors.New("insert failed")
	}))

	_, events, err := CaptureCommandEvents[createPatient, createPatientResult](context.Background(), registry, createPatient{Name: "Ada"})
	if err == nil {
		t.Fatal("capture command events returned nil error")
	}
	if events != nil {
		t.Fatalf("events = %#v, want nil", events)
	}
	if handled != 0 {
		t.Fatalf("handled = %d, want 0", handled)
	}
}

func TestExecuteCommandToOutboxStoresEventsAfterSuccess(t *testing.T) {
	registry := NewRegistry()
	var handled int
	must(t, RegisterDomainEvent[patientCreated](registry, func(ctx context.Context, event patientCreated) error {
		handled++
		return nil
	}))
	must(t, RegisterCommand[createPatient, createPatientResult](registry, func(ctx context.Context, command createPatient) (createPatientResult, error) {
		return createPatientResult{ID: "patient-1"}, EmitDomain(ctx, patientCreated{ID: "patient-1"})
	}))
	outbox := &recordingOutbox{}

	result, err := ExecuteCommandToOutbox[createPatient, createPatientResult](context.Background(), registry, outbox, createPatient{Name: "Ada"})
	if err != nil {
		t.Fatalf("execute command to outbox: %v", err)
	}
	if result.ID != "patient-1" {
		t.Fatalf("result.ID = %q, want patient-1", result.ID)
	}
	if handled != 0 {
		t.Fatalf("handled = %d, want 0", handled)
	}
	if len(outbox.events) != 1 {
		t.Fatalf("len(outbox.events) = %d, want 1: %#v", len(outbox.events), outbox.events)
	}
	if outbox.events[0].Category != DomainEvent || outbox.events[0].Type != typeName[patientCreated]() {
		t.Fatalf("outbox event = %#v, want domain patientCreated", outbox.events[0])
	}
}

func TestExecuteCommandToOutboxReturnsStoreError(t *testing.T) {
	registry := NewRegistry()
	must(t, RegisterCommand[createPatient, createPatientResult](registry, func(ctx context.Context, command createPatient) (createPatientResult, error) {
		return createPatientResult{ID: "patient-1"}, EmitDomain(ctx, patientCreated{ID: "patient-1"})
	}))
	storeErr := errors.New("outbox unavailable")
	outbox := &recordingOutbox{err: storeErr}

	_, err := ExecuteCommandToOutbox[createPatient, createPatientResult](context.Background(), registry, outbox, createPatient{Name: "Ada"})
	if !errors.Is(err, storeErr) {
		t.Fatalf("execute command to outbox error = %v, want %v", err, storeErr)
	}
	if len(outbox.events) != 0 {
		t.Fatalf("outbox.events = %#v, want none after store error", outbox.events)
	}
}

func TestPublishEnvelopeDispatchesCapturedEvent(t *testing.T) {
	registry := NewRegistry()
	var handled []string
	must(t, RegisterDomainEvent[patientCreated](registry, func(ctx context.Context, event patientCreated) error {
		handled = append(handled, event.ID)
		return nil
	}))

	err := PublishEnvelope(context.Background(), registry, EventEnvelope{
		Category: DomainEvent,
		Type:     typeName[patientCreated](),
		Value:    patientCreated{ID: "patient-1"},
	})
	if err != nil {
		t.Fatalf("publish envelope: %v", err)
	}
	if !reflect.DeepEqual(handled, []string{"patient-1"}) {
		t.Fatalf("handled = %#v, want patient-1", handled)
	}
}

func TestPublishEnvelopesForRoleFiltersSubscribers(t *testing.T) {
	registry := NewRegistry()
	var webHandled, workerHandled, rolelessHandled int
	must(t, RegisterDomainEvent[patientCreated](registry, func(ctx context.Context, event patientCreated) error {
		webHandled++
		return nil
	}, RoleWeb))
	must(t, RegisterDomainEvent[patientCreated](registry, func(ctx context.Context, event patientCreated) error {
		workerHandled++
		return nil
	}, RoleWorker))
	must(t, RegisterDomainEvent[patientCreated](registry, func(ctx context.Context, event patientCreated) error {
		rolelessHandled++
		return nil
	}))

	events := []EventEnvelope{{
		Category: DomainEvent,
		Type:     typeName[patientCreated](),
		Value:    patientCreated{ID: "patient-1"},
	}}
	if err := PublishEnvelopesForRole(context.Background(), registry, RoleWorker, events); err != nil {
		t.Fatalf("publish envelopes for role: %v", err)
	}
	if webHandled != 0 || workerHandled != 1 || rolelessHandled != 1 {
		t.Fatalf("unexpected role dispatch counts: web=%d worker=%d roleless=%d", webHandled, workerHandled, rolelessHandled)
	}
}

func TestPublishEnvelopeRejectsWrongValueType(t *testing.T) {
	registry := NewRegistry()
	var handled int
	must(t, RegisterDomainEvent[patientCreated](registry, func(ctx context.Context, event patientCreated) error {
		handled++
		return nil
	}))

	err := PublishEnvelope(context.Background(), registry, EventEnvelope{
		Category: DomainEvent,
		Type:     typeName[patientCreated](),
		Value:    patientCreatedNotice{ID: "patient-1"},
	})
	if !Is(err, ErrUnsupportedHandler) {
		t.Fatalf("publish envelope error = %v, want %s", err, ErrUnsupportedHandler)
	}
	if handled != 0 {
		t.Fatalf("handled = %d, want 0", handled)
	}
}

func TestCommandCanHaveOnlyOneOwner(t *testing.T) {
	registry := NewRegistry()
	handler := func(ctx context.Context, command createPatient) (createPatientResult, error) {
		return createPatientResult{}, nil
	}
	if err := RegisterCommand[createPatient, createPatientResult](registry, handler); err != nil {
		t.Fatalf("register command: %v", err)
	}
	err := RegisterCommand[createPatient, createPatientResult](registry, handler)
	if !Is(err, ErrDuplicateHandler) {
		t.Fatalf("duplicate register error = %v, want %s", err, ErrDuplicateHandler)
	}
}

func TestEventCategoriesAreSeparate(t *testing.T) {
	registry := NewRegistry()
	var domain, presentation int
	if err := RegisterDomainEvent[patientCreated](registry, func(ctx context.Context, event patientCreated) error {
		domain++
		return nil
	}); err != nil {
		t.Fatalf("register domain event: %v", err)
	}
	if err := RegisterPresentationEvent[patientCreated](registry, func(ctx context.Context, event patientCreated) error {
		presentation++
		return nil
	}); err != nil {
		t.Fatalf("register presentation event: %v", err)
	}

	if err := PublishDomain(context.Background(), registry, patientCreated{ID: "patient-1"}); err != nil {
		t.Fatalf("publish domain: %v", err)
	}
	if domain != 1 || presentation != 0 {
		t.Fatalf("after domain publish: domain=%d presentation=%d", domain, presentation)
	}
	if err := PublishPresentation(context.Background(), registry, patientCreated{ID: "patient-1"}); err != nil {
		t.Fatalf("publish presentation: %v", err)
	}
	if domain != 1 || presentation != 1 {
		t.Fatalf("after presentation publish: domain=%d presentation=%d", domain, presentation)
	}
}

func TestEmitRequiresCommandContext(t *testing.T) {
	err := EmitDomain(context.Background(), patientCreated{ID: "patient-1"})
	if !Is(err, ErrNoEventRecorder) {
		t.Fatalf("emit error = %v, want %s", err, ErrNoEventRecorder)
	}
}

func TestQueryAndJobDispatch(t *testing.T) {
	registry := NewRegistry()
	if err := RegisterQuery[patientPageQuery, patientPage](registry, func(ctx context.Context, query patientPageQuery) (patientPage, error) {
		return patientPage{Name: "Ada"}, nil
	}, RoleWeb); err != nil {
		t.Fatalf("register query: %v", err)
	}
	var jobLimit int
	if err := RegisterJob[syncPatientsJob](registry, func(ctx context.Context, job syncPatientsJob) error {
		jobLimit = job.Limit
		return nil
	}, RoleCron); err != nil {
		t.Fatalf("register job: %v", err)
	}

	page, err := ExecuteQuery[patientPageQuery, patientPage](context.Background(), registry, patientPageQuery{ID: "patient-1"})
	if err != nil {
		t.Fatalf("execute query: %v", err)
	}
	if page.Name != "Ada" {
		t.Fatalf("page.Name = %q, want Ada", page.Name)
	}
	if err := ExecuteJob(context.Background(), registry, syncPatientsJob{Limit: 10}); err != nil {
		t.Fatalf("execute job: %v", err)
	}
	if jobLimit != 10 {
		t.Fatalf("jobLimit = %d, want 10", jobLimit)
	}
}

func TestRoleSpecificCommandDispatchSkipsOtherRoleSubscribers(t *testing.T) {
	registry := NewRegistry()
	var webHandled, workerHandled, rolelessHandled int
	must(t, RegisterDomainEvent[patientCreated](registry, func(ctx context.Context, event patientCreated) error {
		webHandled++
		return nil
	}, RoleWeb))
	must(t, RegisterDomainEvent[patientCreated](registry, func(ctx context.Context, event patientCreated) error {
		workerHandled++
		return nil
	}, RoleWorker))
	must(t, RegisterDomainEvent[patientCreated](registry, func(ctx context.Context, event patientCreated) error {
		rolelessHandled++
		return nil
	}))
	must(t, RegisterCommand[createPatient, createPatientResult](registry, func(ctx context.Context, command createPatient) (createPatientResult, error) {
		return createPatientResult{}, EmitDomain(ctx, patientCreated{ID: "patient-1"})
	}, RoleWeb))

	if _, err := ExecuteCommandForRole[createPatient, createPatientResult](context.Background(), registry, RoleWeb, createPatient{}); err != nil {
		t.Fatalf("execute command for role: %v", err)
	}
	if webHandled != 1 || workerHandled != 0 || rolelessHandled != 1 {
		t.Fatalf("unexpected role dispatch counts: web=%d worker=%d roleless=%d", webHandled, workerHandled, rolelessHandled)
	}

	_, err := ExecuteCommandForRole[createPatient, createPatientResult](context.Background(), registry, RoleWorker, createPatient{})
	if !Is(err, ErrRoleNotAllowed) {
		t.Fatalf("wrong-role command error = %v, want %s", err, ErrRoleNotAllowed)
	}
}

func TestRoleSpecificPublishAndJobExecution(t *testing.T) {
	registry := NewRegistry()
	var webHandled, workerHandled int
	must(t, RegisterPresentationEvent[patientCreatedNotice](registry, func(ctx context.Context, event patientCreatedNotice) error {
		webHandled++
		return nil
	}, RoleWeb))
	must(t, RegisterPresentationEvent[patientCreatedNotice](registry, func(ctx context.Context, event patientCreatedNotice) error {
		workerHandled++
		return nil
	}, RoleWorker))
	var jobRuns int
	must(t, RegisterJob[syncPatientsJob](registry, func(ctx context.Context, job syncPatientsJob) error {
		jobRuns++
		return nil
	}, RoleCron))

	if err := PublishPresentationForRole(context.Background(), registry, RoleWeb, patientCreatedNotice{}); err != nil {
		t.Fatalf("publish presentation for web: %v", err)
	}
	if webHandled != 1 || workerHandled != 0 {
		t.Fatalf("unexpected presentation handlers: web=%d worker=%d", webHandled, workerHandled)
	}
	if err := ExecuteJobForRole(context.Background(), registry, RoleWorker, syncPatientsJob{}); !Is(err, ErrRoleNotAllowed) {
		t.Fatalf("wrong-role job error = %v, want %s", err, ErrRoleNotAllowed)
	}
	if err := ExecuteJobForRole(context.Background(), registry, RoleCron, syncPatientsJob{}); err != nil {
		t.Fatalf("execute cron job: %v", err)
	}
	if jobRuns != 1 {
		t.Fatalf("jobRuns = %d, want 1", jobRuns)
	}
}

func TestContractsForRoleFiltersMetadata(t *testing.T) {
	registry := NewRegistry()
	must(t, RegisterCommand[createPatient, createPatientResult](registry, func(ctx context.Context, command createPatient) (createPatientResult, error) {
		return createPatientResult{}, nil
	}, RoleWeb))
	must(t, RegisterQuery[patientPageQuery, patientPage](registry, func(ctx context.Context, query patientPageQuery) (patientPage, error) {
		return patientPage{}, nil
	}))
	must(t, RegisterDomainEvent[patientCreated](registry, func(ctx context.Context, event patientCreated) error {
		return nil
	}, RoleWorker))
	must(t, RegisterPresentationEvent[patientCreatedNotice](registry, func(ctx context.Context, event patientCreatedNotice) error {
		return nil
	}, RoleWeb))
	must(t, RegisterJob[syncPatientsJob](registry, func(ctx context.Context, job syncPatientsJob) error {
		return nil
	}, RoleCron))

	metadata := registry.ContractsForRole(RoleWeb)
	var kinds []Kind
	for _, item := range metadata {
		kinds = append(kinds, item.Kind)
		if item.Kind == Event && item.Type == typeName[patientCreated]() {
			t.Fatalf("worker-only domain event leaked into web metadata: %#v", metadata)
		}
		if item.Kind == Job {
			t.Fatalf("cron job leaked into web metadata: %#v", metadata)
		}
	}
	if !slices.Equal(kinds, []Kind{Command, Event, Query}) {
		t.Fatalf("web metadata kinds = %#v, want command, event, query", kinds)
	}
}

func TestMetadataIsDeterministic(t *testing.T) {
	registry := NewRegistry()
	must(t, RegisterCommand[createPatient, createPatientResult](registry, func(ctx context.Context, command createPatient) (createPatientResult, error) {
		return createPatientResult{}, nil
	}, RoleWeb))
	must(t, RegisterQuery[patientPageQuery, patientPage](registry, func(ctx context.Context, query patientPageQuery) (patientPage, error) {
		return patientPage{}, nil
	}, RoleWeb))
	must(t, RegisterDomainEvent[patientCreated](registry, func(ctx context.Context, event patientCreated) error {
		return nil
	}, RoleWorker))
	must(t, RegisterPresentationEvent[patientCreatedNotice](registry, func(ctx context.Context, event patientCreatedNotice) error {
		return nil
	}, RoleWeb))
	must(t, RegisterJob[syncPatientsJob](registry, func(ctx context.Context, job syncPatientsJob) error {
		return nil
	}, RoleCron))

	metadata := registry.Contracts()
	if len(metadata) != 5 {
		t.Fatalf("len(metadata) = %d, want 5", len(metadata))
	}
	kinds := []Kind{metadata[0].Kind, metadata[1].Kind, metadata[2].Kind, metadata[3].Kind, metadata[4].Kind}
	if !slices.Equal(kinds, []Kind{Command, Event, Event, Job, Query}) {
		t.Fatalf("kinds = %#v", kinds)
	}
	if metadata[1].EventCategory != DomainEvent {
		t.Fatalf("first event category = %q, want domain", metadata[1].EventCategory)
	}
	if metadata[2].EventCategory != PresentationEvent {
		t.Fatalf("second event category = %q, want presentation", metadata[2].EventCategory)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
