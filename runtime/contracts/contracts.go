// Package contracts provides the local typed contract registry used by GOWDK
// runtime roles.
package contracts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"sync"
)

// Kind identifies a registered contract type.
type Kind string

const (
	Query   Kind = "query"
	Command Kind = "command"
	Event   Kind = "event"
	Job     Kind = "job"
)

// EventCategory identifies the trust boundary for an event.
type EventCategory string

const (
	DomainEvent       EventCategory = "domain"
	IntegrationEvent  EventCategory = "integration"
	PresentationEvent EventCategory = "presentation"
)

// Role identifies a runtime role that can execute a contract.
type Role string

const (
	RoleWeb    Role = "web"
	RoleWorker Role = "worker"
	RoleCron   Role = "cron"
	RoleAPI    Role = "api"
	RoleAdmin  Role = "admin"
)

// Handler types accepted by the registry.
type (
	QueryHandler[Q, R any]   func(context.Context, Q) (R, error)
	CommandHandler[C, R any] func(context.Context, C) (R, error)
	EventHandler[E any]      func(context.Context, E) error
	JobHandler[J any]        func(context.Context, J) error
)

// EventEnvelope is a backend-owned event captured from a successful command.
type EventEnvelope struct {
	ID       string
	Category EventCategory
	Type     string
	Value    any
}

// EventDecoder converts a stored JSON event value back into the typed Go value
// expected by subscribers.
type EventDecoder func(json.RawMessage) (any, error)

// StoredEventEnvelope is the JSON transport shape shared by contract outbox
// and broker adapters.
type StoredEventEnvelope struct {
	ID       string          `json:"id,omitempty"`
	Category EventCategory   `json:"category"`
	Type     string          `json:"type"`
	Value    json.RawMessage `json:"value"`
}

// JSONEventDecoder registers a generic JSON decoder for a contract event type.
func JSONEventDecoder[T any]() EventDecoder {
	return func(raw json.RawMessage) (any, error) {
		var value T
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, err
		}
		return value, nil
	}
}

// MarshalEventEnvelopeJSON encodes an event envelope into the shared JSON
// transport shape.
func MarshalEventEnvelopeJSON(event EventEnvelope) ([]byte, error) {
	event = EnsureEventID(event)
	value, err := json.Marshal(event.Value)
	if err != nil {
		return nil, err
	}
	return json.Marshal(StoredEventEnvelope{ID: event.ID, Category: event.Category, Type: event.Type, Value: value})
}

// DecodeEventEnvelopeJSON decodes the shared JSON transport shape and uses a
// registered decoder when one exists for the event type. Without a decoder the
// event value remains json.RawMessage.
func DecodeEventEnvelopeJSON(payload []byte, decoders map[string]EventDecoder) (EventEnvelope, error) {
	var stored StoredEventEnvelope
	if err := json.Unmarshal(payload, &stored); err != nil {
		return EventEnvelope{}, err
	}
	value := any(stored.Value)
	if decoder := decoders[stored.Type]; decoder != nil {
		decoded, err := decoder(stored.Value)
		if err != nil {
			return EventEnvelope{}, err
		}
		value = decoded
	}
	return EventEnvelope{ID: stored.ID, Category: stored.Category, Type: stored.Type, Value: value}, nil
}

// Outbox stores command-emitted events for durable delivery. Implementations
// decide persistence, transactions, retries, and broker publication.
type Outbox interface {
	StoreEvents(context.Context, []EventEnvelope) error
}

// Broker publishes command-emitted events to an external delivery system.
// Implementations decide serialization, acknowledgements, and delivery policy.
type Broker interface {
	PublishEvents(context.Context, []EventEnvelope) error
}

// PresentationFanout sends browser-facing presentation events to a realtime
// transport such as SSE or WebSocket.
type PresentationFanout interface {
	SendPresentationEvents(context.Context, []EventEnvelope) error
}

// SeenStore records durable event IDs that have already been accepted for
// worker dispatch within an adapter-defined deduplication window.
type SeenStore interface {
	MarkIfNew(context.Context, string) (bool, error)
}

// CommandEventSink receives events captured from a successful command. The
// registry and role let sinks choose between in-process subscriber dispatch,
// durable storage, broker publication, or browser-facing presentation delivery.
type CommandEventSink interface {
	HandleCommandEvents(context.Context, *Registry, Role, []EventEnvelope) error
}

type commandEventSinkFunc func(context.Context, *Registry, Role, []EventEnvelope) error

func (sink commandEventSinkFunc) HandleCommandEvents(ctx context.Context, registry *Registry, role Role, events []EventEnvelope) error {
	return sink(ctx, registry, role, events)
}

// ErrorKind identifies contract registry or dispatch failures.
type ErrorKind string

const (
	ErrDuplicateHandler   ErrorKind = "duplicate_handler"
	ErrMissingHandler     ErrorKind = "missing_handler"
	ErrUnsupportedHandler ErrorKind = "unsupported_handler"
	ErrNilHandler         ErrorKind = "nil_handler"
	ErrNoEventRecorder    ErrorKind = "no_event_recorder"
	ErrSubscriberFailed   ErrorKind = "subscriber_failed"
	ErrRoleNotAllowed     ErrorKind = "role_not_allowed"
)

// Error is returned for contract registry and dispatch failures.
type Error struct {
	Kind     ErrorKind
	Contract string
	Message  string
	Cause    error
}

func (err Error) Error() string {
	if err.Message != "" {
		return err.Message
	}
	if err.Contract != "" {
		return fmt.Sprintf("%s: %s", err.Kind, err.Contract)
	}
	return string(err.Kind)
}

func (err Error) Unwrap() error {
	return err.Cause
}

// Is reports whether err or one of its causes is a contract Error with kind.
func Is(err error, kind ErrorKind) bool {
	var contractErr Error
	return errors.As(err, &contractErr) && contractErr.Kind == kind
}

// Metadata describes one registered contract.
type Metadata struct {
	Kind          Kind
	EventCategory EventCategory
	Type          string
	Result        string
	Handlers      int
	Roles         []Role
}

// ObservationName is a stable operation name for contract logs, metrics, and
// traces. These names are intentionally independent from CLI display text.
type ObservationName string

const (
	ObservationRegisterQuery           ObservationName = "gowdk.contract.register.query"
	ObservationRegisterCommand         ObservationName = "gowdk.contract.register.command"
	ObservationRegisterEvent           ObservationName = "gowdk.contract.register.event"
	ObservationRegisterJob             ObservationName = "gowdk.contract.register.job"
	ObservationExecuteQuery            ObservationName = "gowdk.contract.execute.query"
	ObservationExecuteCommand          ObservationName = "gowdk.contract.execute.command"
	ObservationCaptureCommand          ObservationName = "gowdk.contract.capture.command"
	ObservationExecuteJob              ObservationName = "gowdk.contract.execute.job"
	ObservationPublishEvent            ObservationName = "gowdk.contract.publish.event"
	ObservationStoreCommandEvents      ObservationName = "gowdk.contract.outbox.store"
	ObservationPublishBrokerEvents     ObservationName = "gowdk.contract.broker.publish"
	ObservationSendPresentationEvents  ObservationName = "gowdk.contract.presentation.send"
	ObservationWorkerReceiveEventBatch ObservationName = "gowdk.contract.worker.receive"
	ObservationWorkerAckEventBatch     ObservationName = "gowdk.contract.worker.ack"
	ObservationWorkerNackEventBatch    ObservationName = "gowdk.contract.worker.nack"
	ObservationWorkerDedupSkip         ObservationName = "gowdk.contract.worker.dedup_skip"
)

// ObservationLabels are stable contract attributes for logs, metrics, and
// traces. Empty fields are intentionally omitted by callers that do not need
// them.
type ObservationLabels struct {
	Kind          Kind
	EventCategory EventCategory
	EventID       string
	Contract      string
	Result        string
	Role          Role
	Roles         []Role
	Handlers      int
}

// Observation combines a stable operation name with stable contract labels.
type Observation struct {
	Name   ObservationName
	Labels ObservationLabels
}

// NewObservation creates an observation and copies slice labels so callers can
// safely reuse or mutate their input values.
func NewObservation(name ObservationName, labels ObservationLabels) Observation {
	labels.Roles = copyRoles(labels.Roles)
	return Observation{Name: name, Labels: labels}
}

// ObservationLabels returns stable labels for this registered contract.
func (metadata Metadata) ObservationLabels() ObservationLabels {
	return ObservationLabels{
		Kind:          metadata.Kind,
		EventCategory: metadata.EventCategory,
		Contract:      metadata.Type,
		Result:        metadata.Result,
		Roles:         copyRoles(metadata.Roles),
		Handlers:      metadata.Handlers,
	}
}

// Observation returns a named observation for this registered contract.
func (metadata Metadata) Observation(name ObservationName) Observation {
	return NewObservation(name, metadata.ObservationLabels())
}

// ObservationForRole returns a named observation for this registered contract
// and records the runtime role performing the operation.
func (metadata Metadata) ObservationForRole(name ObservationName, role Role) Observation {
	labels := metadata.ObservationLabels()
	labels.Role = role
	return NewObservation(name, labels)
}

// ObservationLabels returns stable labels for this captured event envelope.
func (event EventEnvelope) ObservationLabels() ObservationLabels {
	return ObservationLabels{
		Kind:          Event,
		EventCategory: event.Category,
		EventID:       event.ID,
		Contract:      event.Type,
	}
}

// Observation returns a named observation for this captured event envelope.
func (event EventEnvelope) Observation(name ObservationName) Observation {
	return NewObservation(name, event.ObservationLabels())
}

// ObservationForRole returns a named observation for this captured event
// envelope and records the runtime role performing the operation.
func (event EventEnvelope) ObservationForRole(name ObservationName, role Role) Observation {
	labels := event.ObservationLabels()
	labels.Role = role
	return NewObservation(name, labels)
}

// ContractName returns the stable contract name used by registry metadata and
// event envelopes for T.
func ContractName[T any]() string {
	return typeName[T]()
}

// Registry stores typed contract handlers for one runtime.
type Registry struct {
	mu       sync.RWMutex
	queries  map[string]queryEntry
	commands map[string]commandEntry
	events   map[eventKey][]eventEntry
	jobs     map[string]jobEntry
}

// NewRegistry creates an empty contract registry.
func NewRegistry() *Registry {
	return &Registry{
		queries:  map[string]queryEntry{},
		commands: map[string]commandEntry{},
		events:   map[eventKey][]eventEntry{},
		jobs:     map[string]jobEntry{},
	}
}

// RegisterQuery registers one readonly query handler.
func RegisterQuery[Q, R any](registry *Registry, handler QueryHandler[Q, R], roles ...Role) error {
	if handler == nil {
		return nilHandlerError(Query, typeName[Q]())
	}
	return registry.registerQuery(typeName[Q](), typeName[R](), handler, roles)
}

// ExecuteQuery runs a registered query handler.
func ExecuteQuery[Q, R any](ctx context.Context, registry *Registry, query Q) (R, error) {
	return executeQuery[Q, R](ctx, registry, query, "")
}

// ExecuteQueryForRole runs a query handler only when it is available to role.
func ExecuteQueryForRole[Q, R any](ctx context.Context, registry *Registry, role Role, query Q) (R, error) {
	return executeQuery[Q, R](ctx, registry, query, role)
}

func executeQuery[Q, R any](ctx context.Context, registry *Registry, query Q, role Role) (R, error) {
	var zero R
	entry, ok := registry.query(typeName[Q]())
	if !ok {
		return zero, missingHandlerError(Query, typeName[Q]())
	}
	if !rolesAllow(entry.roles, role) {
		return zero, roleNotAllowedError(Query, typeName[Q](), role)
	}
	handler, ok := entry.handler.(QueryHandler[Q, R])
	if !ok {
		return zero, unsupportedHandlerError(Query, typeName[Q]())
	}
	return handler(ctx, query)
}

// RegisterCommand registers one command owner. A command can have exactly one
// owner handler.
func RegisterCommand[C, R any](registry *Registry, handler CommandHandler[C, R], roles ...Role) error {
	if handler == nil {
		return nilHandlerError(Command, typeName[C]())
	}
	return registry.registerCommand(typeName[C](), typeName[R](), handler, roles)
}

// ExecuteCommand runs a command and dispatches events recorded with Emit* only
// after the command handler succeeds.
func ExecuteCommand[C, R any](ctx context.Context, registry *Registry, command C) (R, error) {
	return executeCommand[C, R](ctx, registry, command, "")
}

// ExecuteCommandForRole runs a command owner for role and dispatches only
// matching event subscribers after the command succeeds.
func ExecuteCommandForRole[C, R any](ctx context.Context, registry *Registry, role Role, command C) (R, error) {
	return executeCommand[C, R](ctx, registry, command, role)
}

func executeCommand[C, R any](ctx context.Context, registry *Registry, command C, role Role) (R, error) {
	result, recorder, err := runCommand[C, R](ctx, registry, command, role)
	if err != nil {
		var zero R
		return zero, err
	}
	if err := recorder.dispatchForRole(ctx, registry, role); err != nil {
		var zero R
		return zero, err
	}
	return result, nil
}

// CaptureCommandEvents runs a command and returns events recorded with Emit*
// after the command handler succeeds. Subscribers are not dispatched.
func CaptureCommandEvents[C, R any](ctx context.Context, registry *Registry, command C) (R, []EventEnvelope, error) {
	return captureCommandEvents[C, R](ctx, registry, command, "")
}

// CaptureCommandEventsForRole runs a command for role and captures emitted
// events without dispatching subscribers.
func CaptureCommandEventsForRole[C, R any](ctx context.Context, registry *Registry, role Role, command C) (R, []EventEnvelope, error) {
	return captureCommandEvents[C, R](ctx, registry, command, role)
}

func captureCommandEvents[C, R any](ctx context.Context, registry *Registry, command C, role Role) (R, []EventEnvelope, error) {
	result, recorder, err := runCommand[C, R](ctx, registry, command, role)
	if err != nil {
		var zero R
		return zero, nil, err
	}
	return result, recorder.envelopes(), nil
}

// ExecuteCommandToOutbox runs a command and stores emitted events in outbox
// after the command handler succeeds. Subscribers are not dispatched.
func ExecuteCommandToOutbox[C, R any](ctx context.Context, registry *Registry, outbox Outbox, command C) (R, error) {
	return executeCommandToOutbox[C, R](ctx, registry, outbox, command, "")
}

// ExecuteCommandToOutboxForRole runs a command for role and stores emitted
// events in outbox after the command handler succeeds.
func ExecuteCommandToOutboxForRole[C, R any](ctx context.Context, registry *Registry, outbox Outbox, role Role, command C) (R, error) {
	return executeCommandToOutbox[C, R](ctx, registry, outbox, command, role)
}

func executeCommandToOutbox[C, R any](ctx context.Context, registry *Registry, outbox Outbox, command C, role Role) (R, error) {
	var zero R
	if outbox == nil {
		return zero, Error{Kind: ErrNilHandler, Contract: typeName[C](), Message: "command outbox cannot be nil"}
	}
	result, events, err := captureCommandEvents[C, R](ctx, registry, command, role)
	if err != nil {
		return zero, err
	}
	if len(events) > 0 {
		if err := outbox.StoreEvents(ctx, events); err != nil {
			return zero, err
		}
	}
	return result, nil
}

// ExecuteCommandToBroker runs a command and publishes emitted events to broker
// after the command handler succeeds. Subscribers are not dispatched.
func ExecuteCommandToBroker[C, R any](ctx context.Context, registry *Registry, broker Broker, command C) (R, error) {
	return executeCommandToBroker[C, R](ctx, registry, broker, command, "")
}

// ExecuteCommandToBrokerForRole runs a command for role and publishes emitted
// events to broker after the command handler succeeds.
func ExecuteCommandToBrokerForRole[C, R any](ctx context.Context, registry *Registry, broker Broker, role Role, command C) (R, error) {
	return executeCommandToBroker[C, R](ctx, registry, broker, command, role)
}

func executeCommandToBroker[C, R any](ctx context.Context, registry *Registry, broker Broker, command C, role Role) (R, error) {
	var zero R
	if broker == nil {
		return zero, Error{Kind: ErrNilHandler, Contract: typeName[C](), Message: "command event broker cannot be nil"}
	}
	result, events, err := captureCommandEvents[C, R](ctx, registry, command, role)
	if err != nil {
		return zero, err
	}
	if err := PublishEventsToBroker(ctx, broker, events); err != nil {
		return zero, err
	}
	return result, nil
}

// ExecuteCommandToPresentationFanout runs a command and sends presentation
// events to fanout after the command handler succeeds. Subscribers are not
// dispatched and non-presentation events are not sent to fanout.
func ExecuteCommandToPresentationFanout[C, R any](ctx context.Context, registry *Registry, fanout PresentationFanout, command C) (R, error) {
	return executeCommandToPresentationFanout[C, R](ctx, registry, fanout, command, "")
}

// ExecuteCommandToPresentationFanoutForRole runs a command for role and sends
// presentation events to fanout after the command handler succeeds.
func ExecuteCommandToPresentationFanoutForRole[C, R any](ctx context.Context, registry *Registry, fanout PresentationFanout, role Role, command C) (R, error) {
	return executeCommandToPresentationFanout[C, R](ctx, registry, fanout, command, role)
}

func executeCommandToPresentationFanout[C, R any](ctx context.Context, registry *Registry, fanout PresentationFanout, command C, role Role) (R, error) {
	var zero R
	if fanout == nil {
		return zero, Error{Kind: ErrNilHandler, Contract: typeName[C](), Message: "command presentation fanout cannot be nil"}
	}
	result, events, err := captureCommandEvents[C, R](ctx, registry, command, role)
	if err != nil {
		return zero, err
	}
	if err := SendPresentationEventsToFanout(ctx, fanout, events); err != nil {
		return zero, err
	}
	return result, nil
}

func runCommand[C, R any](ctx context.Context, registry *Registry, command C, role Role) (R, *eventRecorder, error) {
	var zero R
	entry, ok := registry.command(typeName[C]())
	if !ok {
		return zero, nil, missingHandlerError(Command, typeName[C]())
	}
	if !rolesAllow(entry.roles, role) {
		return zero, nil, roleNotAllowedError(Command, typeName[C](), role)
	}
	handler, ok := entry.handler.(CommandHandler[C, R])
	if !ok {
		return zero, nil, unsupportedHandlerError(Command, typeName[C]())
	}
	commandCtx, recorder := withRecorder(ctx)
	result, err := handler(commandCtx, command)
	if err != nil {
		return zero, nil, err
	}
	return result, recorder, nil
}

// RegisterDomainEvent registers a subscriber for a backend-owned domain event.
func RegisterDomainEvent[E any](registry *Registry, handler EventHandler[E], roles ...Role) error {
	return registerEvent(registry, DomainEvent, handler, roles)
}

// RegisterIntegrationEvent registers a subscriber for a durable integration event.
func RegisterIntegrationEvent[E any](registry *Registry, handler EventHandler[E], roles ...Role) error {
	return registerEvent(registry, IntegrationEvent, handler, roles)
}

// RegisterPresentationEvent registers a subscriber or fanout hook for a
// browser-facing presentation event. Presentation events are output only; they
// must not be treated as trusted domain input.
func RegisterPresentationEvent[E any](registry *Registry, handler EventHandler[E], roles ...Role) error {
	return registerEvent(registry, PresentationEvent, handler, roles)
}

// EmitDomain records a backend-owned domain event for dispatch after the
// current command succeeds.
func EmitDomain[E any](ctx context.Context, event E) error {
	return emit(ctx, DomainEvent, event)
}

// EmitIntegration records a durable integration event for dispatch after the
// current command succeeds.
func EmitIntegration[E any](ctx context.Context, event E) error {
	return emit(ctx, IntegrationEvent, event)
}

// EmitPresentation records a browser-facing presentation event for dispatch
// after the current command succeeds.
func EmitPresentation[E any](ctx context.Context, event E) error {
	return emit(ctx, PresentationEvent, event)
}

// PublishDomain dispatches a domain event immediately.
func PublishDomain[E any](ctx context.Context, registry *Registry, event E) error {
	return dispatchEvent(ctx, registry, DomainEvent, event)
}

// PublishDomainForRole dispatches a domain event to subscribers available to role.
func PublishDomainForRole[E any](ctx context.Context, registry *Registry, role Role, event E) error {
	return dispatchEventForRole(ctx, registry, DomainEvent, event, role)
}

// PublishIntegration dispatches an integration event immediately.
func PublishIntegration[E any](ctx context.Context, registry *Registry, event E) error {
	return dispatchEvent(ctx, registry, IntegrationEvent, event)
}

// PublishIntegrationForRole dispatches an integration event to subscribers available to role.
func PublishIntegrationForRole[E any](ctx context.Context, registry *Registry, role Role, event E) error {
	return dispatchEventForRole(ctx, registry, IntegrationEvent, event, role)
}

// PublishPresentation dispatches a presentation event immediately.
func PublishPresentation[E any](ctx context.Context, registry *Registry, event E) error {
	return dispatchEvent(ctx, registry, PresentationEvent, event)
}

// PublishPresentationForRole dispatches a presentation event to subscribers available to role.
func PublishPresentationForRole[E any](ctx context.Context, registry *Registry, role Role, event E) error {
	return dispatchEventForRole(ctx, registry, PresentationEvent, event, role)
}

// PublishEnvelope dispatches one captured event envelope immediately.
func PublishEnvelope(ctx context.Context, registry *Registry, event EventEnvelope) error {
	return publishEnvelopeForRole(ctx, registry, event, "")
}

// PublishEnvelopeForRole dispatches one captured event envelope to subscribers
// available to role.
func PublishEnvelopeForRole(ctx context.Context, registry *Registry, role Role, event EventEnvelope) error {
	return publishEnvelopeForRole(ctx, registry, event, role)
}

// PublishEnvelopes dispatches captured event envelopes in order.
func PublishEnvelopes(ctx context.Context, registry *Registry, events []EventEnvelope) error {
	return publishEnvelopesForRole(ctx, registry, events, "")
}

// PublishEnvelopesForRole dispatches captured event envelopes in order to
// subscribers available to role.
func PublishEnvelopesForRole(ctx context.Context, registry *Registry, role Role, events []EventEnvelope) error {
	return publishEnvelopesForRole(ctx, registry, events, role)
}

// PublishEventsToBroker sends captured events to broker in one ordered batch.
func PublishEventsToBroker(ctx context.Context, broker Broker, events []EventEnvelope) error {
	if broker == nil {
		return Error{Kind: ErrNilHandler, Message: "event broker cannot be nil"}
	}
	if len(events) == 0 {
		return nil
	}
	return broker.PublishEvents(ctx, events)
}

// SendPresentationEventsToFanout sends only presentation events to fanout.
func SendPresentationEventsToFanout(ctx context.Context, fanout PresentationFanout, events []EventEnvelope) error {
	if fanout == nil {
		return Error{Kind: ErrNilHandler, Message: "presentation fanout cannot be nil"}
	}
	presentation := eventsForCategory(events, PresentationEvent)
	if len(presentation) == 0 {
		return nil
	}
	return fanout.SendPresentationEvents(ctx, presentation)
}

// DispatchCommandEvents sends captured command events to sink. A nil sink uses
// the default in-process subscriber dispatch path.
func DispatchCommandEvents(ctx context.Context, sink CommandEventSink, registry *Registry, role Role, events []EventEnvelope) error {
	if len(events) == 0 {
		return nil
	}
	if sink == nil {
		sink = InProcessCommandEventSink()
	}
	return sink.HandleCommandEvents(ctx, registry, role, events)
}

// InProcessCommandEventSink returns a sink that dispatches captured events
// through the local registry with role filtering.
func InProcessCommandEventSink() CommandEventSink {
	return commandEventSinkFunc(func(ctx context.Context, registry *Registry, role Role, events []EventEnvelope) error {
		if len(events) == 0 {
			return nil
		}
		if registry == nil {
			return Error{Kind: ErrNilHandler, Message: "contract event registry cannot be nil"}
		}
		return PublishEnvelopesForRole(ctx, registry, role, events)
	})
}

// OutboxCommandEventSink returns a sink that stores captured events in outbox
// without dispatching local subscribers.
func OutboxCommandEventSink(outbox Outbox) CommandEventSink {
	return commandEventSinkFunc(func(ctx context.Context, registry *Registry, role Role, events []EventEnvelope) error {
		if len(events) == 0 {
			return nil
		}
		if outbox == nil {
			return Error{Kind: ErrNilHandler, Message: "command outbox cannot be nil"}
		}
		return outbox.StoreEvents(ctx, events)
	})
}

// CompositeCommandEventSink returns a sink that sends the same captured event
// batch to each sink in order. Nil sinks are ignored.
func CompositeCommandEventSink(sinks ...CommandEventSink) CommandEventSink {
	copied := make([]CommandEventSink, 0, len(sinks))
	for _, sink := range sinks {
		if sink != nil {
			copied = append(copied, sink)
		}
	}
	return commandEventSinkFunc(func(ctx context.Context, registry *Registry, role Role, events []EventEnvelope) error {
		if len(events) == 0 {
			return nil
		}
		for _, sink := range copied {
			if err := sink.HandleCommandEvents(ctx, registry, role, events); err != nil {
				return err
			}
		}
		return nil
	})
}

// BrokerCommandEventSink returns a sink that publishes captured events to
// broker without dispatching local subscribers.
func BrokerCommandEventSink(broker Broker) CommandEventSink {
	return commandEventSinkFunc(func(ctx context.Context, registry *Registry, role Role, events []EventEnvelope) error {
		if len(events) == 0 {
			return nil
		}
		return PublishEventsToBroker(ctx, broker, events)
	})
}

// PresentationFanoutCommandEventSink returns a sink that sends only
// presentation events to fanout.
func PresentationFanoutCommandEventSink(fanout PresentationFanout) CommandEventSink {
	return commandEventSinkFunc(func(ctx context.Context, registry *Registry, role Role, events []EventEnvelope) error {
		if len(events) == 0 {
			return nil
		}
		return SendPresentationEventsToFanout(ctx, fanout, events)
	})
}

func publishEnvelopesForRole(ctx context.Context, registry *Registry, events []EventEnvelope, role Role) error {
	for _, event := range events {
		if err := publishEnvelopeForRole(ctx, registry, event, role); err != nil {
			return err
		}
	}
	return nil
}

func publishEnvelopeForRole(ctx context.Context, registry *Registry, event EventEnvelope, role Role) error {
	key := eventKey{category: event.Category, event: event.Type}
	entries := registry.eventEntries(key)
	for index, entry := range entries {
		if !rolesAllow(entry.roles, role) {
			continue
		}
		if entry.dispatch == nil {
			return unsupportedHandlerError(Event, key.event)
		}
		if err := entry.dispatch(ctx, event.Value); err != nil {
			if Is(err, ErrUnsupportedHandler) {
				return err
			}
			return Error{
				Kind:     ErrSubscriberFailed,
				Contract: key.event,
				Message:  fmt.Sprintf("%s event subscriber %d for %s failed: %v", key.category, index, key.event, err),
				Cause:    err,
			}
		}
	}
	return nil
}

// RegisterJob registers one background or scheduled job handler.
func RegisterJob[J any](registry *Registry, handler JobHandler[J], roles ...Role) error {
	if handler == nil {
		return nilHandlerError(Job, typeName[J]())
	}
	return registry.registerJob(typeName[J](), handler, roles)
}

// ExecuteJob runs a registered job handler.
func ExecuteJob[J any](ctx context.Context, registry *Registry, job J) error {
	return executeJob(ctx, registry, job, "")
}

// ExecuteJobForRole runs a job handler only when it is available to role.
func ExecuteJobForRole[J any](ctx context.Context, registry *Registry, role Role, job J) error {
	return executeJob(ctx, registry, job, role)
}

func executeJob[J any](ctx context.Context, registry *Registry, job J, role Role) error {
	entry, ok := registry.job(typeName[J]())
	if !ok {
		return missingHandlerError(Job, typeName[J]())
	}
	if !rolesAllow(entry.roles, role) {
		return roleNotAllowedError(Job, typeName[J](), role)
	}
	handler, ok := entry.handler.(JobHandler[J])
	if !ok {
		return unsupportedHandlerError(Job, typeName[J]())
	}
	return handler(ctx, job)
}

// Contracts returns deterministic metadata for registered contracts.
func (registry *Registry) Contracts() []Metadata {
	return registry.contractsForRole("")
}

// ContractsForRole returns deterministic metadata for contracts available to role.
func (registry *Registry) ContractsForRole(role Role) []Metadata {
	return registry.contractsForRole(role)
}

func (registry *Registry) contractsForRole(role Role) []Metadata {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	metadata := make([]Metadata, 0, len(registry.queries)+len(registry.commands)+len(registry.events)+len(registry.jobs))
	for _, entry := range registry.queries {
		if rolesAllow(entry.roles, role) {
			metadata = append(metadata, Metadata{Kind: Query, Type: entry.query, Result: entry.result, Handlers: 1, Roles: copyRoles(entry.roles)})
		}
	}
	for _, entry := range registry.commands {
		if rolesAllow(entry.roles, role) {
			metadata = append(metadata, Metadata{Kind: Command, Type: entry.command, Result: entry.result, Handlers: 1, Roles: copyRoles(entry.roles)})
		}
	}
	for key, entries := range registry.events {
		allowedEntries := eventEntriesForRole(entries, role)
		if len(allowedEntries) > 0 {
			metadata = append(metadata, Metadata{Kind: Event, EventCategory: key.category, Type: key.event, Handlers: len(allowedEntries), Roles: eventRoles(allowedEntries)})
		}
	}
	for _, entry := range registry.jobs {
		if rolesAllow(entry.roles, role) {
			metadata = append(metadata, Metadata{Kind: Job, Type: entry.job, Handlers: 1, Roles: copyRoles(entry.roles)})
		}
	}
	sort.Slice(metadata, func(i, j int) bool {
		if metadata[i].Kind != metadata[j].Kind {
			return metadata[i].Kind < metadata[j].Kind
		}
		if metadata[i].EventCategory != metadata[j].EventCategory {
			return metadata[i].EventCategory < metadata[j].EventCategory
		}
		return metadata[i].Type < metadata[j].Type
	})
	return metadata
}

type queryEntry struct {
	query   string
	result  string
	handler any
	roles   []Role
}

type commandEntry struct {
	command string
	result  string
	handler any
	roles   []Role
}

type eventKey struct {
	category EventCategory
	event    string
}

type eventEntry struct {
	event    string
	dispatch func(context.Context, any) error
	roles    []Role
}

type jobEntry struct {
	job     string
	handler any
	roles   []Role
}

func (registry *Registry) registerQuery(query, result string, handler any, roles []Role) error {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if _, exists := registry.queries[query]; exists {
		return duplicateHandlerError(Query, query)
	}
	registry.queries[query] = queryEntry{query: query, result: result, handler: handler, roles: copyRoles(roles)}
	return nil
}

func (registry *Registry) query(query string) (queryEntry, bool) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	entry, ok := registry.queries[query]
	return entry, ok
}

func (registry *Registry) registerCommand(command, result string, handler any, roles []Role) error {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if _, exists := registry.commands[command]; exists {
		return duplicateHandlerError(Command, command)
	}
	registry.commands[command] = commandEntry{command: command, result: result, handler: handler, roles: copyRoles(roles)}
	return nil
}

func (registry *Registry) command(command string) (commandEntry, bool) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	entry, ok := registry.commands[command]
	return entry, ok
}

func registerEvent[E any](registry *Registry, category EventCategory, handler EventHandler[E], roles []Role) error {
	if handler == nil {
		return nilHandlerError(Event, typeName[E]())
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	key := eventKey{category: category, event: typeName[E]()}
	registry.events[key] = append(registry.events[key], eventEntry{
		event:    key.event,
		dispatch: eventDispatcher(handler),
		roles:    copyRoles(roles),
	})
	return nil
}

func eventDispatcher[E any](handler EventHandler[E]) func(context.Context, any) error {
	return func(ctx context.Context, value any) error {
		event, ok := value.(E)
		if !ok {
			return Error{
				Kind:     ErrUnsupportedHandler,
				Contract: typeName[E](),
				Message:  fmt.Sprintf("event %s envelope value has type %T", typeName[E](), value),
			}
		}
		return handler(ctx, event)
	}
}

func dispatchEvent[E any](ctx context.Context, registry *Registry, category EventCategory, event E) error {
	return dispatchEventForRole(ctx, registry, category, event, "")
}

func dispatchEventForRole[E any](ctx context.Context, registry *Registry, category EventCategory, event E, role Role) error {
	return publishEnvelopeForRole(ctx, registry, EventEnvelope{
		Category: category,
		Type:     typeName[E](),
		Value:    event,
	}, role)
}

func (registry *Registry) eventEntries(key eventKey) []eventEntry {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	entries := registry.events[key]
	copied := make([]eventEntry, len(entries))
	copy(copied, entries)
	return copied
}

func (registry *Registry) registerJob(job string, handler any, roles []Role) error {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if _, exists := registry.jobs[job]; exists {
		return duplicateHandlerError(Job, job)
	}
	registry.jobs[job] = jobEntry{job: job, handler: handler, roles: copyRoles(roles)}
	return nil
}

func (registry *Registry) job(job string) (jobEntry, bool) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	entry, ok := registry.jobs[job]
	return entry, ok
}

func duplicateHandlerError(kind Kind, contract string) error {
	return Error{Kind: ErrDuplicateHandler, Contract: contract, Message: fmt.Sprintf("%s %s already has a handler", kind, contract)}
}

func missingHandlerError(kind Kind, contract string) error {
	return Error{Kind: ErrMissingHandler, Contract: contract, Message: fmt.Sprintf("%s %s has no registered handler", kind, contract)}
}

func unsupportedHandlerError(kind Kind, contract string) error {
	return Error{Kind: ErrUnsupportedHandler, Contract: contract, Message: fmt.Sprintf("%s %s has an unsupported handler signature", kind, contract)}
}

func roleNotAllowedError(kind Kind, contract string, role Role) error {
	return Error{Kind: ErrRoleNotAllowed, Contract: contract, Message: fmt.Sprintf("%s %s is not available to role %q", kind, contract, role)}
}

func nilHandlerError(kind Kind, contract string) error {
	return Error{Kind: ErrNilHandler, Contract: contract, Message: fmt.Sprintf("%s %s cannot register a nil handler", kind, contract)}
}

func typeName[T any]() string {
	t := reflect.TypeOf((*T)(nil)).Elem()
	if t.PkgPath() == "" || t.Name() == "" {
		return t.String()
	}
	return t.PkgPath() + "." + t.Name()
}

func copyRoles(roles []Role) []Role {
	if len(roles) == 0 {
		return nil
	}
	copied := make([]Role, len(roles))
	copy(copied, roles)
	return copied
}

func rolesAllow(roles []Role, role Role) bool {
	if role == "" || len(roles) == 0 {
		return true
	}
	for _, candidate := range roles {
		if candidate == role {
			return true
		}
	}
	return false
}

func eventEntriesForRole(entries []eventEntry, role Role) []eventEntry {
	if role == "" {
		copied := make([]eventEntry, len(entries))
		copy(copied, entries)
		return copied
	}
	var allowed []eventEntry
	for _, entry := range entries {
		if rolesAllow(entry.roles, role) {
			allowed = append(allowed, entry)
		}
	}
	return allowed
}

func eventsForCategory(events []EventEnvelope, category EventCategory) []EventEnvelope {
	var filtered []EventEnvelope
	for _, event := range events {
		if event.Category == category {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

func eventRoles(entries []eventEntry) []Role {
	seen := map[Role]bool{}
	var roles []Role
	for _, entry := range entries {
		for _, role := range entry.roles {
			if !seen[role] {
				seen[role] = true
				roles = append(roles, role)
			}
		}
	}
	sort.Slice(roles, func(i, j int) bool { return roles[i] < roles[j] })
	return roles
}
