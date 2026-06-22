package contracts

import "context"

type recorderKey struct{}

type eventRecorder struct {
	events []recordedEvent
}

type recordedEvent struct {
	category  EventCategory
	eventType string
	audience  []string
	value     any
}

func withRecorder(ctx context.Context) (context.Context, *eventRecorder) {
	recorder := &eventRecorder{}
	return context.WithValue(ctx, recorderKey{}, recorder), recorder
}

func emit[E any](ctx context.Context, category EventCategory, event E) error {
	return emitWithAudience(ctx, category, event, nil)
}

func emitWithAudience[E any](ctx context.Context, category EventCategory, event E, audience []string) error {
	recorder, ok := ctx.Value(recorderKey{}).(*eventRecorder)
	if !ok || recorder == nil {
		return Error{
			Kind:     ErrNoEventRecorder,
			Contract: typeName[E](),
			Message:  "events can only be emitted while a command is executing",
		}
	}
	recorder.events = append(recorder.events, recordedEvent{
		category:  category,
		eventType: typeName[E](),
		audience:  normalizeAudience(audience),
		value:     event,
	})
	return nil
}

func (recorder *eventRecorder) envelopes(ctx context.Context) []EventEnvelope {
	if len(recorder.events) == 0 {
		return nil
	}
	envelopes := make([]EventEnvelope, 0, len(recorder.events))
	traceparent := traceparentFromContext(ctx)
	for _, event := range recorder.events {
		envelopes = append(envelopes, EnsureEventID(EventEnvelope{
			TraceParent: traceparent,
			Audience:    event.audience,
			Category:    event.category,
			Type:        event.eventType,
			Value:       event.value,
		}))
	}
	return envelopes
}
