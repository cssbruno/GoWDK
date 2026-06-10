package contracts

import (
	"context"
	"errors"
	"log"
)

// ErrEventSourceClosed tells RunEventWorker that the source drained cleanly.
var ErrEventSourceClosed = errors.New("event source closed")

// WorkerLogger receives dispatch failures that RunEventWorker recovered from
// by nacking the batch. Set it to nil to silence recovered-dispatch logging.
// It defaults to the standard log package.
var WorkerLogger func(message string) = func(message string) {
	log.Print(message)
}

func logWorkerDispatchFailure(err error) {
	logger := WorkerLogger
	if logger == nil {
		return
	}
	logger("gowdk: event worker nacked batch after dispatch failure: " + err.Error())
}

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
// subscribers available to role. Dispatch failures that the source accepts
// through Nack are logged via WorkerLogger and the worker keeps consuming;
// it only stops when ctx ends, the source closes, or Ack/Nack fail.
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
		if batch.Nack == nil {
			return err
		}
		if nackErr := batch.Nack(ctx, err); nackErr != nil {
			return errors.Join(err, nackErr)
		}
		// The source accepted the Nack and owns redelivery, so the worker
		// records the failure and keeps consuming instead of exiting.
		logWorkerDispatchFailure(err)
		return nil
	}
	return ackEventBatch(ctx, batch)
}

func ackEventBatch(ctx context.Context, batch EventBatch) error {
	if batch.Ack == nil {
		return nil
	}
	return batch.Ack(ctx)
}
