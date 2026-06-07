package contracts

import (
	"context"
	"errors"
)

// ErrEventSourceClosed tells RunEventWorker that the source drained cleanly.
var ErrEventSourceClosed = errors.New("event source closed")

// EventBatch is one ordered delivery batch from an outbox, queue, or broker
// adapter. Ack and Nack are optional adapter hooks.
type EventBatch struct {
	Events []EventEnvelope
	Ack    func(context.Context) error
	Nack   func(context.Context, error) error
}

// EventSource receives event batches for a worker role.
type EventSource interface {
	ReceiveEventBatch(context.Context) (EventBatch, error)
}

// RunEventWorker reads batches from source and dispatches them to worker-role
// subscribers until ctx is canceled or source returns ErrEventSourceClosed.
func RunEventWorker(ctx context.Context, registry *Registry, source EventSource) error {
	return RunEventWorkerForRole(ctx, registry, RoleWorker, source)
}

// RunEventWorkerForRole reads batches from source and dispatches them to
// subscribers available to role.
func RunEventWorkerForRole(ctx context.Context, registry *Registry, role Role, source EventSource) error {
	if source == nil {
		return Error{Kind: ErrNilHandler, Message: "event source cannot be nil"}
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		batch, err := source.ReceiveEventBatch(ctx)
		if err != nil {
			if errors.Is(err, ErrEventSourceClosed) {
				return nil
			}
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			return err
		}
		if err := dispatchEventBatch(ctx, registry, role, batch); err != nil {
			return err
		}
	}
}

func dispatchEventBatch(ctx context.Context, registry *Registry, role Role, batch EventBatch) error {
	if len(batch.Events) == 0 {
		return ackEventBatch(ctx, batch)
	}
	if err := PublishEnvelopesForRole(ctx, registry, role, batch.Events); err != nil {
		if batch.Nack != nil {
			if nackErr := batch.Nack(ctx, err); nackErr != nil {
				return errors.Join(err, nackErr)
			}
		}
		return err
	}
	return ackEventBatch(ctx, batch)
}

func ackEventBatch(ctx context.Context, batch EventBatch) error {
	if batch.Ack == nil {
		return nil
	}
	return batch.Ack(ctx)
}
