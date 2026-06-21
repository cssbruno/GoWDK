package contracts

import (
	"context"
	"fmt"

	gowdktrace "github.com/cssbruno/gowdk/runtime/trace"
)

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
	events = eventsWithTraceparent(ctx, events)
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
	presentation = eventsWithTraceparent(ctx, presentation)
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
//
// Dispatch is sequential and not transactional: if an earlier sink commits
// (for example storing to an outbox or publishing to a broker) and a later
// sink fails, the earlier side effects remain while the command handler
// returns the error and the client may retry the mutation. Order sinks so the
// durable, replayable sink (typically the outbox) runs first, and make
// downstream sinks idempotent — keyed by EventEnvelope.ID — so a retry does
// not duplicate effects.
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
	ctx = contextWithEventTraceparent(ctx, event)
	ctx, span := startContractSpan(ctx, string(ObservationPublishEvent),
		gowdktrace.LaneContract,
		map[string]any{"gowdk.contract.kind": string(Event), "gowdk.contract.type": event.Type, "gowdk.event.category": string(event.Category), "gowdk.event.id": event.ID, "gowdk.contract.role": string(role)},
	)
	var spanErr error
	defer func() { finishContractSpan(span, spanErr) }()
	if registry == nil {
		spanErr = nilRegistryError(Event, event.Type)
		return spanErr
	}
	key := eventKey{category: event.Category, event: event.Type}
	entries := registry.eventEntries(key)
	for index, entry := range entries {
		if !rolesAllow(entry.roles, role) {
			continue
		}
		if entry.dispatch == nil {
			spanErr = unsupportedHandlerError(Event, key.event)
			return spanErr
		}
		if err := entry.dispatch(ctx, event.Value); err != nil {
			if Is(err, ErrUnsupportedHandler) {
				spanErr = err
				return err
			}
			spanErr = Error{
				Kind:     ErrSubscriberFailed,
				Contract: key.event,
				Message:  fmt.Sprintf("%s event subscriber %d for %s failed: %v", key.category, index, key.event, err),
				Cause:    err,
			}
			return spanErr
		}
	}
	return nil
}

func registerEvent[E any](registry *Registry, category EventCategory, handler EventHandler[E], roles []Role) error {
	if handler == nil {
		return nilHandlerError(Event, typeName[E]())
	}
	if registry == nil {
		return nilRegistryError(Event, typeName[E]())
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.ensureMapsLocked()
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
		TraceParent: traceparentFromContext(ctx),
		Category:    category,
		Type:        typeName[E](),
		Value:       event,
	}, role)
}

func eventsWithTraceparent(ctx context.Context, events []EventEnvelope) []EventEnvelope {
	traceparent := traceparentFromContext(ctx)
	if traceparent == "" {
		return events
	}
	out := make([]EventEnvelope, len(events))
	copy(out, events)
	for index := range out {
		if out[index].TraceParent == "" {
			out[index].TraceParent = traceparent
		}
	}
	return out
}

func (registry *Registry) eventEntries(key eventKey) []eventEntry {
	if registry == nil {
		return nil
	}
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	entries := registry.events[key]
	copied := make([]eventEntry, len(entries))
	copy(copied, entries)
	return copied
}
