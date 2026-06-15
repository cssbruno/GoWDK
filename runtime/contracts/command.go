package contracts

import (
	"context"

	gowdktrace "github.com/cssbruno/gowdk/runtime/trace"
)

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
	result, recorder, commandCtx, err := runCommand[C, R](ctx, registry, command, role)
	if err != nil {
		var zero R
		return zero, err
	}
	if err := recorder.dispatchForRole(commandCtx, registry, role); err != nil {
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
	result, recorder, commandCtx, err := runCommand[C, R](ctx, registry, command, role)
	if err != nil {
		var zero R
		return zero, nil, err
	}
	return result, recorder.envelopes(commandCtx), nil
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

func runCommand[C, R any](ctx context.Context, registry *Registry, command C, role Role) (R, *eventRecorder, context.Context, error) {
	var zero R
	contract := typeName[C]()
	ctx, span := startContractSpan(ctx, string(ObservationExecuteCommand),
		gowdktrace.LaneContract,
		map[string]any{"gowdk.contract.kind": string(Command), "gowdk.contract.type": contract, "gowdk.contract.role": string(role)},
	)
	var spanErr error
	defer func() { finishContractSpan(span, spanErr) }()
	entry, ok := registry.command(typeName[C]())
	if !ok {
		spanErr = missingHandlerError(Command, contract)
		return zero, nil, ctx, missingHandlerError(Command, typeName[C]())
	}
	if !roleMayExecute(entry.roles, role) {
		spanErr = roleNotAllowedError(Command, contract, role)
		return zero, nil, ctx, spanErr
	}
	handler, ok := entry.handler.(CommandHandler[C, R])
	if !ok {
		spanErr = unsupportedHandlerError(Command, contract)
		return zero, nil, ctx, spanErr
	}
	commandCtx, recorder := withRecorder(ctx)
	result, err := handler(commandCtx, command)
	if err != nil {
		spanErr = err
		return zero, nil, commandCtx, err
	}
	return result, recorder, commandCtx, nil
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
