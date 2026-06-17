package contracts

import (
	"context"
	"encoding/json"
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

	// RoleAny marks a contract as executable by every caller role, including the
	// untrusted web surface. It is the explicit, audited opt-in that a contract
	// must declare to be reachable without naming concrete roles; an empty role
	// set is treated as "no role may execute" rather than "any role may execute".
	RoleAny Role = "any"
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
	ID          string
	TraceParent string
	Category    EventCategory
	Type        string
	Value       any
}

// QueryInvalidationPresentationEventType is the browser-facing presentation
// event type generated when backend events invalidate query-owned regions.
const QueryInvalidationPresentationEventType = "gowdk.query.invalidate"

// QueryInvalidation records that a backend event invalidates a query type.
type QueryInvalidation struct {
	EventCategory EventCategory
	EventType     string
	QueryType     string
}

// QueryInvalidationNotice is the browser payload sent for invalidated queries.
type QueryInvalidationNotice struct {
	Queries  []string `json:"queries"`
	Events   []string `json:"events,omitempty"`
	EventIDs []string `json:"eventIDs,omitempty"`
}

// EventDecoder converts a stored JSON event value back into the typed Go value
// expected by subscribers.
type EventDecoder func(json.RawMessage) (any, error)

// StoredEventEnvelope is the JSON transport shape shared by contract outbox
// and broker adapters.
type StoredEventEnvelope struct {
	ID          string          `json:"id,omitempty"`
	TraceParent string          `json:"traceparent,omitempty"`
	Category    EventCategory   `json:"category"`
	Type        string          `json:"type"`
	Value       json.RawMessage `json:"value"`
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
	return json.Marshal(StoredEventEnvelope{ID: event.ID, TraceParent: event.TraceParent, Category: event.Category, Type: event.Type, Value: value})
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
	return EventEnvelope{ID: stored.ID, TraceParent: stored.TraceParent, Category: stored.Category, Type: stored.Type, Value: value}, nil
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

// SeenStore records durable event IDs that have already been successfully
// dispatched and acknowledged within an adapter-defined deduplication window.
type SeenStore interface {
	Seen(context.Context, string) (bool, error)
	MarkSeen(context.Context, string) error
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

// Metadata describes one registered contract.
type Metadata struct {
	Kind          Kind
	EventCategory EventCategory
	Type          string
	Result        string
	Handlers      int
	Roles         []Role
}
