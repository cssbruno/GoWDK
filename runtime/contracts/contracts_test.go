package contracts

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"

	gowdktrace "github.com/cssbruno/gowdk/runtime/trace"
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

type commandDispatchContextKey struct{}

type recordingOutbox struct {
	events []EventEnvelope
	err    error
}

type recordingBroker struct {
	events []EventEnvelope
	calls  int
	err    error
}

type recordingFanout struct {
	events []EventEnvelope
	calls  int
	err    error
}

func (outbox *recordingOutbox) StoreEvents(ctx context.Context, events []EventEnvelope) error {
	if outbox.err != nil {
		return outbox.err
	}
	outbox.events = append(outbox.events, events...)
	return nil
}

func (broker *recordingBroker) PublishEvents(ctx context.Context, events []EventEnvelope) error {
	broker.calls++
	if broker.err != nil {
		return broker.err
	}
	broker.events = append(broker.events, events...)
	return nil
}

func (fanout *recordingFanout) SendPresentationEvents(ctx context.Context, events []EventEnvelope) error {
	fanout.calls++
	if fanout.err != nil {
		return fanout.err
	}
	fanout.events = append(fanout.events, events...)
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

func TestCommandDispatchUsesTraceContextWithoutRecorder(t *testing.T) {
	registry := NewRegistry()
	parentTrace := gowdktrace.TraceContext{TraceID: "4bf92f3577b34da6a3ce929d0e0e4736", SpanID: "00f067aa0ba902b7", Sampled: true}
	type requestContextKey struct{}
	deadline := time.Now().Add(time.Minute)
	ctx := context.WithValue(context.Background(), requestContextKey{}, "request-1")
	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	ctx = gowdktrace.ContextWithTraceContext(ctx, parentTrace)
	var subscriberTrace gowdktrace.TraceContext
	var subscriberEmitErr error
	var subscriberValue any
	var subscriberDeadline time.Time
	var subscriberDeadlineOK bool

	must(t, RegisterDomainEvent[patientCreated](registry, func(ctx context.Context, event patientCreated) error {
		var ok bool
		subscriberTrace, ok = gowdktrace.TraceContextFromContext(ctx)
		if !ok {
			return errors.New("subscriber context lost trace context")
		}
		subscriberValue = ctx.Value(requestContextKey{})
		subscriberDeadline, subscriberDeadlineOK = ctx.Deadline()
		subscriberEmitErr = EmitDomain(ctx, patientCreated{ID: "subscriber-event"})
		return subscriberEmitErr
	}))
	must(t, RegisterCommand[createPatient, createPatientResult](registry, func(ctx context.Context, command createPatient) (createPatientResult, error) {
		return createPatientResult{ID: "patient-1"}, EmitDomain(ctx, patientCreated{ID: "patient-1"})
	}))

	_, err := ExecuteCommand[createPatient, createPatientResult](ctx, registry, createPatient{Name: "Ada"})
	if err == nil {
		t.Fatal("execute command returned nil error")
	}
	if !Is(err, ErrSubscriberFailed) {
		t.Fatalf("execute command error = %v, want %s", err, ErrSubscriberFailed)
	}
	if !Is(subscriberEmitErr, ErrNoEventRecorder) {
		t.Fatalf("subscriber emit error = %v, want %s", subscriberEmitErr, ErrNoEventRecorder)
	}
	if subscriberTrace.TraceID != parentTrace.TraceID || subscriberTrace.SpanID != parentTrace.SpanID {
		t.Fatalf("subscriber trace = %#v, want %#v", subscriberTrace, parentTrace)
	}
	if subscriberValue != "request-1" {
		t.Fatalf("subscriber context value = %#v, want request-1", subscriberValue)
	}
	if !subscriberDeadlineOK || !subscriberDeadline.Equal(deadline) {
		t.Fatalf("subscriber deadline = %v, ok=%v; want %v", subscriberDeadline, subscriberDeadlineOK, deadline)
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

func TestExecuteCommandToBrokerPublishesEventsAfterSuccess(t *testing.T) {
	registry := NewRegistry()
	var handled int
	must(t, RegisterDomainEvent[patientCreated](registry, func(ctx context.Context, event patientCreated) error {
		handled++
		return nil
	}))
	must(t, RegisterCommand[createPatient, createPatientResult](registry, func(ctx context.Context, command createPatient) (createPatientResult, error) {
		return createPatientResult{ID: "patient-1"}, EmitIntegration(ctx, patientCreated{ID: "patient-1"})
	}))
	broker := &recordingBroker{}

	result, err := ExecuteCommandToBroker[createPatient, createPatientResult](context.Background(), registry, broker, createPatient{Name: "Ada"})
	if err != nil {
		t.Fatalf("execute command to broker: %v", err)
	}
	if result.ID != "patient-1" {
		t.Fatalf("result.ID = %q, want patient-1", result.ID)
	}
	if handled != 0 {
		t.Fatalf("handled = %d, want 0", handled)
	}
	if broker.calls != 1 {
		t.Fatalf("broker.calls = %d, want 1", broker.calls)
	}
	if len(broker.events) != 1 {
		t.Fatalf("len(broker.events) = %d, want 1: %#v", len(broker.events), broker.events)
	}
	if broker.events[0].Category != IntegrationEvent || broker.events[0].Type != typeName[patientCreated]() {
		t.Fatalf("broker event = %#v, want integration patientCreated", broker.events[0])
	}
}

func TestExecuteCommandToBrokerReturnsPublishError(t *testing.T) {
	registry := NewRegistry()
	must(t, RegisterCommand[createPatient, createPatientResult](registry, func(ctx context.Context, command createPatient) (createPatientResult, error) {
		return createPatientResult{ID: "patient-1"}, EmitIntegration(ctx, patientCreated{ID: "patient-1"})
	}))
	publishErr := errors.New("broker unavailable")
	broker := &recordingBroker{err: publishErr}

	_, err := ExecuteCommandToBroker[createPatient, createPatientResult](context.Background(), registry, broker, createPatient{Name: "Ada"})
	if !errors.Is(err, publishErr) {
		t.Fatalf("execute command to broker error = %v, want %v", err, publishErr)
	}
	if broker.calls != 1 {
		t.Fatalf("broker.calls = %d, want 1", broker.calls)
	}
	if len(broker.events) != 0 {
		t.Fatalf("broker.events = %#v, want none after publish error", broker.events)
	}
}

func TestPublishEventsToBrokerSkipsEmptyBatches(t *testing.T) {
	broker := &recordingBroker{}

	if err := PublishEventsToBroker(context.Background(), broker, nil); err != nil {
		t.Fatalf("publish empty batch to broker: %v", err)
	}
	if broker.calls != 0 {
		t.Fatalf("broker.calls = %d, want 0", broker.calls)
	}
}

func TestExecuteCommandToPresentationFanoutSendsOnlyPresentationEvents(t *testing.T) {
	registry := NewRegistry()
	var handled int
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
	fanout := &recordingFanout{}

	result, err := ExecuteCommandToPresentationFanout[createPatient, createPatientResult](context.Background(), registry, fanout, createPatient{Name: "Ada"})
	if err != nil {
		t.Fatalf("execute command to presentation fanout: %v", err)
	}
	if result.ID != "patient-1" {
		t.Fatalf("result.ID = %q, want patient-1", result.ID)
	}
	if handled != 0 {
		t.Fatalf("handled = %d, want 0", handled)
	}
	if fanout.calls != 1 {
		t.Fatalf("fanout.calls = %d, want 1", fanout.calls)
	}
	if len(fanout.events) != 1 {
		t.Fatalf("len(fanout.events) = %d, want 1: %#v", len(fanout.events), fanout.events)
	}
	if fanout.events[0].Category != PresentationEvent || fanout.events[0].Type != typeName[patientCreatedNotice]() {
		t.Fatalf("fanout event = %#v, want presentation patientCreatedNotice", fanout.events[0])
	}
}

func TestExecuteCommandToPresentationFanoutReturnsSendError(t *testing.T) {
	registry := NewRegistry()
	must(t, RegisterCommand[createPatient, createPatientResult](registry, func(ctx context.Context, command createPatient) (createPatientResult, error) {
		return createPatientResult{ID: "patient-1"}, EmitPresentation(ctx, patientCreatedNotice{ID: "patient-1"})
	}))
	sendErr := errors.New("fanout unavailable")
	fanout := &recordingFanout{err: sendErr}

	_, err := ExecuteCommandToPresentationFanout[createPatient, createPatientResult](context.Background(), registry, fanout, createPatient{Name: "Ada"})
	if !errors.Is(err, sendErr) {
		t.Fatalf("execute command to presentation fanout error = %v, want %v", err, sendErr)
	}
	if fanout.calls != 1 {
		t.Fatalf("fanout.calls = %d, want 1", fanout.calls)
	}
	if len(fanout.events) != 0 {
		t.Fatalf("fanout.events = %#v, want none after send error", fanout.events)
	}
}

func TestSendPresentationEventsToFanoutSkipsNonPresentationBatches(t *testing.T) {
	fanout := &recordingFanout{}

	err := SendPresentationEventsToFanout(context.Background(), fanout, []EventEnvelope{{
		Category: DomainEvent,
		Type:     typeName[patientCreated](),
		Value:    patientCreated{ID: "patient-1"},
	}})
	if err != nil {
		t.Fatalf("send presentation events to fanout: %v", err)
	}
	if fanout.calls != 0 {
		t.Fatalf("fanout.calls = %d, want 0", fanout.calls)
	}
}

func TestDispatchCommandEventsUsesDefaultInProcessSink(t *testing.T) {
	registry := NewRegistry()
	var handled []string
	must(t, RegisterDomainEvent[patientCreated](registry, func(ctx context.Context, event patientCreated) error {
		handled = append(handled, event.ID)
		return nil
	}, RoleWeb))

	err := DispatchCommandEvents(context.Background(), nil, registry, RoleWeb, []EventEnvelope{{
		Category: DomainEvent,
		Type:     typeName[patientCreated](),
		Value:    patientCreated{ID: "patient-1"},
	}})
	if err != nil {
		t.Fatalf("dispatch command events: %v", err)
	}
	if !reflect.DeepEqual(handled, []string{"patient-1"}) {
		t.Fatalf("handled = %#v, want patient-1", handled)
	}
}

func TestDispatchCommandEventsSkipsEmptyBatch(t *testing.T) {
	called := false
	sink := commandEventSinkFunc(func(ctx context.Context, registry *Registry, role Role, events []EventEnvelope) error {
		called = true
		return nil
	})

	if err := DispatchCommandEvents(context.Background(), sink, NewRegistry(), RoleWeb, nil); err != nil {
		t.Fatalf("dispatch empty command events: %v", err)
	}
	if called {
		t.Fatalf("sink was called for empty event batch")
	}
}

func TestInProcessCommandEventSinkReturnsSubscriberError(t *testing.T) {
	registry := NewRegistry()
	subscriberErr := errors.New("audit failed")
	must(t, RegisterDomainEvent[patientCreated](registry, func(ctx context.Context, event patientCreated) error {
		return subscriberErr
	}, RoleWeb))

	err := DispatchCommandEvents(context.Background(), InProcessCommandEventSink(), registry, RoleWeb, []EventEnvelope{{
		Category: DomainEvent,
		Type:     typeName[patientCreated](),
		Value:    patientCreated{ID: "patient-1"},
	}})
	if !errors.Is(err, subscriberErr) || !Is(err, ErrSubscriberFailed) {
		t.Fatalf("dispatch command events error = %v, want subscriber_failed wrapping %v", err, subscriberErr)
	}
}

func TestOutboxCommandEventSinkStoresEvents(t *testing.T) {
	outbox := &recordingOutbox{}
	events := []EventEnvelope{{
		Category: DomainEvent,
		Type:     typeName[patientCreated](),
		Value:    patientCreated{ID: "patient-1"},
	}}

	if err := DispatchCommandEvents(context.Background(), OutboxCommandEventSink(outbox), NewRegistry(), RoleWeb, events); err != nil {
		t.Fatalf("outbox command event sink: %v", err)
	}
	if !reflect.DeepEqual(outbox.events, events) {
		t.Fatalf("outbox.events = %#v, want %#v", outbox.events, events)
	}
}

func TestBrokerCommandEventSinkPublishesEvents(t *testing.T) {
	broker := &recordingBroker{}
	events := []EventEnvelope{{
		Category: IntegrationEvent,
		Type:     typeName[patientCreated](),
		Value:    patientCreated{ID: "patient-1"},
	}}

	if err := DispatchCommandEvents(context.Background(), BrokerCommandEventSink(broker), NewRegistry(), RoleWeb, events); err != nil {
		t.Fatalf("broker command event sink: %v", err)
	}
	if broker.calls != 1 {
		t.Fatalf("broker.calls = %d, want 1", broker.calls)
	}
	if !reflect.DeepEqual(broker.events, events) {
		t.Fatalf("broker.events = %#v, want %#v", broker.events, events)
	}
}

func TestCommandEventSinkAdaptersSkipEmptyBatches(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry()
	for name, sink := range map[string]CommandEventSink{
		"in-process":   InProcessCommandEventSink(),
		"outbox":       OutboxCommandEventSink(nil),
		"broker":       BrokerCommandEventSink(nil),
		"presentation": PresentationFanoutCommandEventSink(nil),
	} {
		if err := sink.HandleCommandEvents(ctx, registry, RoleWeb, nil); err != nil {
			t.Fatalf("%s sink empty batch error = %v, want nil", name, err)
		}
	}
}

func TestCompositeCommandEventSinkRunsSinksInOrder(t *testing.T) {
	var calls []string
	first := commandEventSinkFunc(func(ctx context.Context, registry *Registry, role Role, events []EventEnvelope) error {
		calls = append(calls, "first")
		return nil
	})
	second := commandEventSinkFunc(func(ctx context.Context, registry *Registry, role Role, events []EventEnvelope) error {
		calls = append(calls, "second")
		return nil
	})
	events := []EventEnvelope{{Category: DomainEvent, Type: typeName[patientCreated](), Value: patientCreated{ID: "patient-1"}}}

	err := DispatchCommandEvents(context.Background(), CompositeCommandEventSink(first, nil, second), NewRegistry(), RoleWeb, events)
	if err != nil {
		t.Fatalf("composite command event sink: %v", err)
	}
	if !slices.Equal(calls, []string{"first", "second"}) {
		t.Fatalf("calls = %#v, want first then second", calls)
	}
}

func TestCompositeCommandEventSinkStopsOnError(t *testing.T) {
	sinkErr := errors.New("sink failed")
	var secondCalled bool
	first := commandEventSinkFunc(func(ctx context.Context, registry *Registry, role Role, events []EventEnvelope) error {
		return sinkErr
	})
	second := commandEventSinkFunc(func(ctx context.Context, registry *Registry, role Role, events []EventEnvelope) error {
		secondCalled = true
		return nil
	})
	events := []EventEnvelope{{Category: DomainEvent, Type: typeName[patientCreated](), Value: patientCreated{ID: "patient-1"}}}

	err := DispatchCommandEvents(context.Background(), CompositeCommandEventSink(first, second), NewRegistry(), RoleWeb, events)
	if !errors.Is(err, sinkErr) {
		t.Fatalf("composite command event sink error = %v, want %v", err, sinkErr)
	}
	if secondCalled {
		t.Fatalf("second sink was called after first sink error")
	}
}

func TestPresentationFanoutCommandEventSinkSendsOnlyPresentationEvents(t *testing.T) {
	fanout := &recordingFanout{}

	err := DispatchCommandEvents(context.Background(), PresentationFanoutCommandEventSink(fanout), NewRegistry(), RoleWeb, []EventEnvelope{
		{Category: DomainEvent, Type: typeName[patientCreated](), Value: patientCreated{ID: "patient-1"}},
		{Category: PresentationEvent, Type: typeName[patientCreatedNotice](), Value: patientCreatedNotice{ID: "patient-1"}},
	})
	if err != nil {
		t.Fatalf("presentation fanout command event sink: %v", err)
	}
	if fanout.calls != 1 {
		t.Fatalf("fanout.calls = %d, want 1", fanout.calls)
	}
	if len(fanout.events) != 1 || fanout.events[0].Category != PresentationEvent {
		t.Fatalf("fanout.events = %#v, want one presentation event", fanout.events)
	}
}

func TestRegisterInvalidationRecordsDomainEventToQueryEdge(t *testing.T) {
	registry := NewRegistry()
	must(t, RegisterInvalidation[patientCreated, patientPageQuery](registry))
	must(t, RegisterInvalidation[patientCreated, patientPageQuery](registry))

	invalidations := registry.Invalidations()
	if len(invalidations) != 1 {
		t.Fatalf("invalidations = %#v, want one deduplicated edge", invalidations)
	}
	if invalidations[0] != (QueryInvalidation{EventCategory: DomainEvent, EventType: typeName[patientCreated](), QueryType: typeName[patientPageQuery]()}) {
		t.Fatalf("unexpected invalidation edge: %#v", invalidations[0])
	}
}

func TestQueryInvalidationCommandEventSinkSendsGeneratedPresentationEvent(t *testing.T) {
	fanout := &recordingFanout{}
	sink := QueryInvalidationCommandEventSink(fanout, []QueryInvalidation{{
		EventCategory: DomainEvent,
		EventType:     typeName[patientCreated](),
		QueryType:     typeName[patientPageQuery](),
	}})

	err := DispatchCommandEvents(context.Background(), sink, NewRegistry(), RoleWeb, []EventEnvelope{{
		Category: DomainEvent,
		ID:       "event-1",
		Type:     typeName[patientCreated](),
		Value:    patientCreated{ID: "patient-1"},
	}})
	if err != nil {
		t.Fatalf("query invalidation sink: %v", err)
	}
	if fanout.calls != 1 || len(fanout.events) != 1 {
		t.Fatalf("fanout calls/events = %d/%#v, want one event", fanout.calls, fanout.events)
	}
	event := fanout.events[0]
	if event.Category != PresentationEvent || event.Type != QueryInvalidationPresentationEventType {
		t.Fatalf("unexpected invalidation presentation event: %#v", event)
	}
	notice, ok := event.Value.(QueryInvalidationNotice)
	if !ok {
		t.Fatalf("event value type = %T, want QueryInvalidationNotice", event.Value)
	}
	if !reflect.DeepEqual(notice.Queries, []string{typeName[patientPageQuery]()}) {
		t.Fatalf("notice queries = %#v", notice.Queries)
	}
	if !reflect.DeepEqual(notice.EventIDs, []string{"event-1"}) {
		t.Fatalf("notice event IDs = %#v", notice.EventIDs)
	}
}

func TestInvalidatedQueryTypesReturnsMatchedQueries(t *testing.T) {
	invalidations := []QueryInvalidation{{
		EventCategory: DomainEvent,
		EventType:     typeName[patientCreated](),
		QueryType:     typeName[patientPageQuery](),
	}}
	queries := InvalidatedQueryTypes(invalidations, []EventEnvelope{{
		Category: DomainEvent,
		ID:       "event-1",
		Type:     typeName[patientCreated](),
		Value:    patientCreated{ID: "patient-1"},
	}})
	if !reflect.DeepEqual(queries, []string{typeName[patientPageQuery]()}) {
		t.Fatalf("InvalidatedQueryTypes = %#v, want the matched query type", queries)
	}
	eventIDs := InvalidatedEventIDs(invalidations, []EventEnvelope{{
		Category: DomainEvent,
		ID:       "event-1",
		Type:     typeName[patientCreated](),
		Value:    patientCreated{ID: "patient-1"},
	}})
	if !reflect.DeepEqual(eventIDs, []string{"event-1"}) {
		t.Fatalf("InvalidatedEventIDs = %#v, want the matched event ID", eventIDs)
	}
}

func TestInvalidatedQueryTypesReturnsNilWhenNoEdgeMatches(t *testing.T) {
	invalidations := []QueryInvalidation{{
		EventCategory: DomainEvent,
		EventType:     typeName[patientCreated](),
		QueryType:     typeName[patientPageQuery](),
	}}
	queries := InvalidatedQueryTypes(invalidations, []EventEnvelope{{
		Category: DomainEvent,
		Type:     "example.com/app/contracts/other.Unrelated",
	}})
	if len(queries) != 0 {
		t.Fatalf("InvalidatedQueryTypes = %#v, want nil for an unrelated event", queries)
	}
}

func TestQueryInvalidationCommandEventSinkIgnoresFanoutErrors(t *testing.T) {
	fanout := &recordingFanout{err: errors.New("offline")}
	sink := QueryInvalidationCommandEventSink(fanout, []QueryInvalidation{{
		EventCategory: DomainEvent,
		EventType:     typeName[patientCreated](),
		QueryType:     typeName[patientPageQuery](),
	}})

	err := DispatchCommandEvents(context.Background(), sink, NewRegistry(), RoleWeb, []EventEnvelope{{
		Category: DomainEvent,
		Type:     typeName[patientCreated](),
		Value:    patientCreated{ID: "patient-1"},
	}})
	if err != nil {
		t.Fatalf("query invalidation sink error = %v, want nil", err)
	}
	if fanout.calls != 1 {
		t.Fatalf("fanout.calls = %d, want 1", fanout.calls)
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

func TestContractTracingRecordsCommandQueryEventAndJobSpans(t *testing.T) {
	ring := gowdktrace.NewRingSink(8)
	tracer := gowdktrace.NewTracer(gowdktrace.WithSink(ring))
	ctx := gowdktrace.ContextWithTracer(context.Background(), tracer)
	registry := NewRegistry()
	must(t, RegisterDomainEvent[patientCreated](registry, func(ctx context.Context, event patientCreated) error {
		return nil
	}, RoleWorker))
	must(t, RegisterCommand[createPatient, createPatientResult](registry, func(ctx context.Context, command createPatient) (createPatientResult, error) {
		if err := EmitDomain(ctx, patientCreated{ID: "patient-1"}); err != nil {
			return createPatientResult{}, err
		}
		return createPatientResult{ID: "patient-1"}, nil
	}, RoleWeb))
	must(t, RegisterQuery[patientPageQuery, patientPage](registry, func(ctx context.Context, query patientPageQuery) (patientPage, error) {
		return patientPage{Name: "Ada"}, nil
	}, RoleWeb))
	must(t, RegisterJob[syncPatientsJob](registry, func(ctx context.Context, job syncPatientsJob) error {
		return nil
	}, RoleCron))

	if _, err := ExecuteCommandForRole[createPatient, createPatientResult](ctx, registry, RoleWeb, createPatient{Name: "Ada"}); err != nil {
		t.Fatalf("execute command: %v", err)
	}
	if _, err := ExecuteQueryForRole[patientPageQuery, patientPage](ctx, registry, RoleWeb, patientPageQuery{ID: "patient-1"}); err != nil {
		t.Fatalf("execute query: %v", err)
	}
	if err := ExecuteJobForRole(ctx, registry, RoleCron, syncPatientsJob{Limit: 10}); err != nil {
		t.Fatalf("execute job: %v", err)
	}

	spans := ring.Spans()
	assertContractSpan(t, spans, string(ObservationExecuteCommand), gowdktrace.LaneContract, string(Command))
	assertContractSpan(t, spans, string(ObservationPublishEvent), gowdktrace.LaneContract, string(Event))
	assertContractSpan(t, spans, string(ObservationExecuteQuery), gowdktrace.LaneContract, string(Query))
	assertContractSpan(t, spans, string(ObservationExecuteJob), gowdktrace.LaneJob, string(Job))
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

func TestContractsForRoleExcludesRolelessCommandsAndQueries(t *testing.T) {
	registry := NewRegistry()
	must(t, RegisterCommand[createPatient, createPatientResult](registry, func(ctx context.Context, command createPatient) (createPatientResult, error) {
		return createPatientResult{}, nil
	}))
	must(t, RegisterQuery[patientPageQuery, patientPage](registry, func(ctx context.Context, query patientPageQuery) (patientPage, error) {
		return patientPage{}, nil
	}))

	// A concrete role must not see roleless command/query metadata, mirroring the
	// fail-closed Execute*ForRole gate: advertising them would contradict
	// execution, which denies the same call.
	if metadata := registry.ContractsForRole(RoleWeb); len(metadata) != 0 {
		t.Fatalf("roleless command/query leaked into web metadata: %#v", metadata)
	}
	// Trusted in-process enumeration (no role) still sees everything.
	if metadata := registry.Contracts(); len(metadata) != 2 {
		t.Fatalf("roleless enumeration = %d, want 2: %#v", len(metadata), metadata)
	}
}

func TestMetadataObservationUsesStableNameAndCopiedLabels(t *testing.T) {
	registry := NewRegistry()
	must(t, RegisterCommand[createPatient, createPatientResult](registry, func(ctx context.Context, command createPatient) (createPatientResult, error) {
		return createPatientResult{}, nil
	}, RoleWeb))

	metadata := registry.ContractsForRole(RoleWeb)
	if len(metadata) != 1 {
		t.Fatalf("len(metadata) = %d, want 1: %#v", len(metadata), metadata)
	}
	observation := metadata[0].ObservationForRole(ObservationExecuteCommand, RoleWeb)
	if observation.Name != "gowdk.contract.execute.command" {
		t.Fatalf("observation name = %q", observation.Name)
	}
	if observation.Labels.Role != RoleWeb {
		t.Fatalf("role = %q, want web", observation.Labels.Role)
	}
	if observation.Labels.Kind != Command {
		t.Fatalf("kind = %q, want command", observation.Labels.Kind)
	}
	if observation.Labels.Contract != ContractName[createPatient]() {
		t.Fatalf("contract = %q, want createPatient", observation.Labels.Contract)
	}
	if observation.Labels.Result != ContractName[createPatientResult]() {
		t.Fatalf("result = %q, want createPatientResult", observation.Labels.Result)
	}
	if observation.Labels.Handlers != 1 {
		t.Fatalf("handlers = %d, want 1", observation.Labels.Handlers)
	}
	if !slices.Equal(observation.Labels.Roles, []Role{RoleWeb}) {
		t.Fatalf("roles = %#v, want web", observation.Labels.Roles)
	}

	observation.Labels.Roles[0] = RoleAdmin
	if metadata[0].Roles[0] != RoleWeb {
		t.Fatalf("observation roles alias metadata roles: %#v", metadata[0].Roles)
	}
}

func TestNewObservationCopiesRoleLabels(t *testing.T) {
	roles := []Role{RoleWorker}
	observation := NewObservation(ObservationWorkerReceiveEventBatch, ObservationLabels{
		Kind:     Event,
		Role:     RoleWorker,
		Roles:    roles,
		Handlers: 2,
	})

	roles[0] = RoleAdmin
	if !slices.Equal(observation.Labels.Roles, []Role{RoleWorker}) {
		t.Fatalf("observation roles = %#v, want copied worker role", observation.Labels.Roles)
	}
	if observation.Labels.Role != RoleWorker {
		t.Fatalf("role = %q, want worker", observation.Labels.Role)
	}
}

func TestEventEnvelopeObservationUsesStableLabels(t *testing.T) {
	envelope := EventEnvelope{
		ID:       "event-1",
		Category: DomainEvent,
		Type:     ContractName[patientCreated](),
		Value:    patientCreated{ID: "patient-1"},
	}

	observation := envelope.ObservationForRole(ObservationPublishEvent, RoleWorker)
	if observation.Name != "gowdk.contract.publish.event" {
		t.Fatalf("observation name = %q", observation.Name)
	}
	if observation.Labels.Role != RoleWorker {
		t.Fatalf("role = %q, want worker", observation.Labels.Role)
	}
	if observation.Labels.Kind != Event {
		t.Fatalf("kind = %q, want event", observation.Labels.Kind)
	}
	if observation.Labels.EventCategory != DomainEvent {
		t.Fatalf("event category = %q, want domain", observation.Labels.EventCategory)
	}
	if observation.Labels.EventID != "event-1" {
		t.Fatalf("event id = %q, want event-1", observation.Labels.EventID)
	}
	if observation.Labels.Contract != ContractName[patientCreated]() {
		t.Fatalf("contract = %q, want patientCreated", observation.Labels.Contract)
	}
	if observation.Labels.Handlers != 0 {
		t.Fatalf("handlers = %d, want 0 for envelope labels", observation.Labels.Handlers)
	}
}

func TestEventEnvelopeJSONPreservesID(t *testing.T) {
	payload, err := MarshalEventEnvelopeJSON(EventEnvelope{
		ID:       "event-1",
		Category: DomainEvent,
		Type:     ContractName[patientCreated](),
		Value:    patientCreated{ID: "patient-1"},
	})
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	decoded, err := DecodeEventEnvelopeJSON(payload, map[string]EventDecoder{
		ContractName[patientCreated](): JSONEventDecoder[patientCreated](),
	})
	if err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if decoded.ID != "event-1" {
		t.Fatalf("decoded ID = %q, want event-1", decoded.ID)
	}
	if value, ok := decoded.Value.(patientCreated); !ok || value.ID != "patient-1" {
		t.Fatalf("decoded value = %#v, want patientCreated patient-1", decoded.Value)
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

func TestExecuteRolelessContractFailsClosedForConcreteRole(t *testing.T) {
	registry := NewRegistry()
	must(t, RegisterCommand[createPatient, createPatientResult](registry, func(ctx context.Context, command createPatient) (createPatientResult, error) {
		return createPatientResult{}, nil
	}))
	must(t, RegisterQuery[patientPageQuery, patientPage](registry, func(ctx context.Context, query patientPageQuery) (patientPage, error) {
		return patientPage{}, nil
	}))

	// A contract that declares no roles must not be executable by the web
	// surface (or any other concrete role): the data-layer gate fails closed.
	if _, err := ExecuteCommandForRole[createPatient, createPatientResult](context.Background(), registry, RoleWeb, createPatient{}); !Is(err, ErrRoleNotAllowed) {
		t.Fatalf("roleless command for web = %v, want %s", err, ErrRoleNotAllowed)
	}
	if _, err := ExecuteQueryForRole[patientPageQuery, patientPage](context.Background(), registry, RoleWeb, patientPageQuery{}); !Is(err, ErrRoleNotAllowed) {
		t.Fatalf("roleless query for web = %v, want %s", err, ErrRoleNotAllowed)
	}

	// Trusted in-process callers (no role context) still execute.
	if _, err := ExecuteCommand[createPatient, createPatientResult](context.Background(), registry, createPatient{}); err != nil {
		t.Fatalf("roleless command in-process: %v", err)
	}
	if _, err := ExecuteQuery[patientPageQuery, patientPage](context.Background(), registry, patientPageQuery{}); err != nil {
		t.Fatalf("roleless query in-process: %v", err)
	}
}

func TestExecuteRoleAnyContractAllowsConcreteRole(t *testing.T) {
	registry := NewRegistry()
	must(t, RegisterCommand[createPatient, createPatientResult](registry, func(ctx context.Context, command createPatient) (createPatientResult, error) {
		return createPatientResult{}, nil
	}, RoleAny))
	must(t, RegisterQuery[patientPageQuery, patientPage](registry, func(ctx context.Context, query patientPageQuery) (patientPage, error) {
		return patientPage{}, nil
	}, RoleAny))

	// RoleAny is the explicit opt-in that makes a contract executable by any
	// caller role, including the untrusted web surface.
	if _, err := ExecuteCommandForRole[createPatient, createPatientResult](context.Background(), registry, RoleWeb, createPatient{}); err != nil {
		t.Fatalf("RoleAny command for web: %v", err)
	}
	if _, err := ExecuteQueryForRole[patientPageQuery, patientPage](context.Background(), registry, RoleWeb, patientPageQuery{}); err != nil {
		t.Fatalf("RoleAny query for web: %v", err)
	}
}

func assertContractSpan(t *testing.T, spans []gowdktrace.Snapshot, name string, lane gowdktrace.Lane, kind string) {
	t.Helper()
	for _, span := range spans {
		if span.Name != name {
			continue
		}
		if span.Surface != gowdktrace.SurfaceBackend || span.Lane != lane || span.Status.Code != gowdktrace.StatusOK {
			t.Fatalf("span %q = %#v", name, span)
		}
		if got := contractSpanAttr(span, "gowdk.contract.kind"); got != kind {
			t.Fatalf("span %q kind attr = %#v, want %q", name, got, kind)
		}
		return
	}
	t.Fatalf("missing span %q in %#v", name, spans)
}

func contractSpanAttr(span gowdktrace.Snapshot, key string) any {
	for _, attr := range span.Attributes {
		if attr.Key == key {
			return attr.Value
		}
	}
	return nil
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
