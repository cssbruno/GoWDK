package contracts

import "context"

type recorderKey struct{}

type eventRecorder struct {
	events []recordedEvent
}

type recordedEvent struct {
	category        EventCategory
	value           any
	dispatch        func(context.Context, *Registry) error
	dispatchForRole func(context.Context, *Registry, Role) error
}

func withRecorder(ctx context.Context) (context.Context, *eventRecorder) {
	recorder := &eventRecorder{}
	return context.WithValue(ctx, recorderKey{}, recorder), recorder
}

func emit[E any](ctx context.Context, category EventCategory, event E) error {
	recorder, ok := ctx.Value(recorderKey{}).(*eventRecorder)
	if !ok || recorder == nil {
		return Error{
			Kind:     ErrNoEventRecorder,
			Contract: typeName[E](),
			Message:  "events can only be emitted while a command is executing",
		}
	}
	recorder.events = append(recorder.events, recordedEvent{
		category: category,
		value:    event,
		dispatch: func(dispatchCtx context.Context, registry *Registry) error {
			return dispatchEvent(dispatchCtx, registry, category, event)
		},
		dispatchForRole: func(dispatchCtx context.Context, registry *Registry, role Role) error {
			return dispatchEventForRole(dispatchCtx, registry, category, event, role)
		},
	})
	return nil
}

func (recorder *eventRecorder) dispatch(ctx context.Context, registry *Registry) error {
	for _, event := range recorder.events {
		if err := event.dispatch(ctx, registry); err != nil {
			return err
		}
	}
	return nil
}

func (recorder *eventRecorder) dispatchForRole(ctx context.Context, registry *Registry, role Role) error {
	for _, event := range recorder.events {
		if err := event.dispatchForRole(ctx, registry, role); err != nil {
			return err
		}
	}
	return nil
}
