package contracts

import (
	"context"
	"fmt"
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
