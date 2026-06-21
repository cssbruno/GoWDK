package contracts

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/cssbruno/gowdk/runtime/security"
	gowdktrace "github.com/cssbruno/gowdk/runtime/trace"
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
	logger("gowdk: event worker nacked batch after dispatch failure: " + security.RedactSecrets(err.Error()))
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

// EventWorkerRetry describes one nacked worker delivery attempt. Attempt is a
// one-based count of consecutive nacked batches for the running worker.
type EventWorkerRetry struct {
	Role       Role
	Attempt    int
	EventCount int
	Cause      error
}

// EventWorkerBackoff returns the delay before a worker consumes the next batch
// after EventSource accepts a Nack. Non-positive durations mean no delay.
type EventWorkerBackoff func(EventWorkerRetry) time.Duration

// EventWorkerOption configures RunEventWorker option variants.
type EventWorkerOption func(*eventWorkerOptions)

type eventWorkerOptions struct {
	backoff EventWorkerBackoff
	seen    SeenStore
}

// WithEventWorkerBackoff sets the retry delay policy used after EventSource
// accepts a nacked batch. Nil preserves the default immediate retry behavior.
func WithEventWorkerBackoff(backoff EventWorkerBackoff) EventWorkerOption {
	return func(options *eventWorkerOptions) {
		options.backoff = backoff
	}
}

// WithEventWorkerSeenStore skips duplicate event IDs already present in seen.
// Duplicate-only batches are acknowledged without invoking subscribers. A nil
// seen store preserves the default at-least-once worker behavior.
func WithEventWorkerSeenStore(seen SeenStore) EventWorkerOption {
	return func(options *eventWorkerOptions) {
		options.seen = seen
	}
}

// ConstantEventWorkerBackoff returns a backoff policy with the same delay for
// every nacked batch. Non-positive delays result in immediate retry.
func ConstantEventWorkerBackoff(delay time.Duration) EventWorkerBackoff {
	return func(EventWorkerRetry) time.Duration {
		if delay <= 0 {
			return 0
		}
		return delay
	}
}

// RunEventWorker reads batches from source and dispatches them to worker-role
// subscribers until ctx is canceled or source returns ErrEventSourceClosed.
func RunEventWorker(ctx context.Context, registry *Registry, source EventSource) error {
	return RunEventWorkerForRole(ctx, registry, RoleWorker, source)
}

// RunEventWorkerWithOptions reads worker-role batches with explicit worker
// options such as retry backoff.
func RunEventWorkerWithOptions(ctx context.Context, registry *Registry, source EventSource, options ...EventWorkerOption) error {
	return RunEventWorkerForRoleWithOptions(ctx, registry, RoleWorker, source, options...)
}

// RunEventWorkerForRole reads batches from source and dispatches them to
// subscribers available to role. Dispatch failures that the source accepts
// through Nack are logged via WorkerLogger and the worker keeps consuming;
// it only stops when ctx ends, the source closes, or Ack/Nack fail.
func RunEventWorkerForRole(ctx context.Context, registry *Registry, role Role, source EventSource) error {
	return RunEventWorkerForRoleWithOptions(ctx, registry, role, source)
}

// RunEventWorkerForRoleWithOptions reads batches for role with explicit worker
// options such as retry backoff.
func RunEventWorkerForRoleWithOptions(ctx context.Context, registry *Registry, role Role, source EventSource, options ...EventWorkerOption) error {
	return runEventWorkerForRole(ctx, registry, role, source, options...)
}

// RunEventWorkerWithSeenStore reads worker-role batches and skips duplicate
// event IDs already present in seen. Duplicate-only batches are acknowledged
// without invoking subscribers.
func RunEventWorkerWithSeenStore(ctx context.Context, registry *Registry, source EventSource, seen SeenStore) error {
	return RunEventWorkerForRoleWithSeenStore(ctx, registry, RoleWorker, source, seen)
}

// RunEventWorkerWithSeenStoreAndOptions reads worker-role batches with a seen
// store and explicit worker options such as retry backoff.
func RunEventWorkerWithSeenStoreAndOptions(ctx context.Context, registry *Registry, source EventSource, seen SeenStore, options ...EventWorkerOption) error {
	return RunEventWorkerForRoleWithSeenStoreAndOptions(ctx, registry, RoleWorker, source, seen, options...)
}

// RunEventWorkerForRoleWithSeenStore reads batches for role and skips duplicate
// event IDs already present in seen. A nil seen store preserves the default
// at-least-once worker behavior.
func RunEventWorkerForRoleWithSeenStore(ctx context.Context, registry *Registry, role Role, source EventSource, seen SeenStore) error {
	return RunEventWorkerForRoleWithOptions(ctx, registry, role, source, WithEventWorkerSeenStore(seen))
}

// RunEventWorkerForRoleWithSeenStoreAndOptions reads batches for role with a
// seen store and explicit worker options such as retry backoff.
func RunEventWorkerForRoleWithSeenStoreAndOptions(ctx context.Context, registry *Registry, role Role, source EventSource, seen SeenStore, options ...EventWorkerOption) error {
	return RunEventWorkerForRoleWithOptions(ctx, registry, role, source, append([]EventWorkerOption{WithEventWorkerSeenStore(seen)}, options...)...)
}

func runEventWorkerForRole(ctx context.Context, registry *Registry, role Role, source EventSource, optionFns ...EventWorkerOption) error {
	if source == nil {
		return Error{Kind: ErrNilHandler, Message: "event source cannot be nil"}
	}
	options := newEventWorkerOptions(optionFns)
	nackedAttempts := 0
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
		_, receiveSpan := startContractSpan(ctx, string(ObservationWorkerReceiveEventBatch),
			gowdktrace.LaneJob,
			map[string]any{"gowdk.contract.role": string(role), "gowdk.event.count": len(batch.Events)},
		)
		finishContractSpan(receiveSpan, nil)
		result, err := dispatchEventBatch(ctx, registry, role, batch, options.seen)
		if err != nil {
			return err
		}
		if result.nacked {
			nackedAttempts++
			delay := options.retryDelay(EventWorkerRetry{
				Role:       role,
				Attempt:    nackedAttempts,
				EventCount: result.eventCount,
				Cause:      result.cause,
			})
			if err := waitEventWorkerBackoff(ctx, delay); err != nil {
				return err
			}
			continue
		}
		nackedAttempts = 0
	}
}

type eventWorkerDispatchResult struct {
	nacked     bool
	eventCount int
	cause      error
}

func dispatchEventBatch(ctx context.Context, registry *Registry, role Role, batch EventBatch, seen SeenStore) (eventWorkerDispatchResult, error) {
	if len(batch.Events) == 0 {
		return eventWorkerDispatchResult{}, ackEventBatch(ctx, batch)
	}
	events, deliveredIDs, err := unseenEvents(ctx, role, batch.Events, seen)
	if err != nil {
		return eventWorkerDispatchResult{}, err
	}
	if len(events) == 0 {
		return eventWorkerDispatchResult{}, ackEventBatch(ctx, batch)
	}
	if err := PublishEnvelopesForRole(ctx, registry, role, events); err != nil {
		if batch.Nack == nil {
			return eventWorkerDispatchResult{}, err
		}
		if nackErr := batch.Nack(ctx, err); nackErr != nil {
			return eventWorkerDispatchResult{}, errors.Join(err, nackErr)
		}
		// The source accepted the Nack and owns redelivery, so the worker
		// records the failure and keeps consuming instead of exiting.
		logWorkerDispatchFailure(err)
		return eventWorkerDispatchResult{nacked: true, eventCount: len(events), cause: err}, nil
	}
	if err := ackEventBatch(ctx, batch); err != nil {
		return eventWorkerDispatchResult{}, err
	}
	return eventWorkerDispatchResult{}, markSeenEvents(ctx, seen, deliveredIDs)
}

func unseenEvents(ctx context.Context, role Role, events []EventEnvelope, seen SeenStore) ([]EventEnvelope, []string, error) {
	if seen == nil {
		return events, nil, nil
	}
	out := make([]EventEnvelope, 0, len(events))
	deliveredIDs := make([]string, 0, len(events))
	pending := map[string]bool{}
	for _, event := range events {
		if event.ID == "" {
			out = append(out, event)
			continue
		}
		if pending[event.ID] {
			logWorkerDedupSkip(event, role)
			continue
		}
		alreadySeen, err := seen.Seen(ctx, event.ID)
		if err != nil {
			return nil, nil, err
		}
		if alreadySeen {
			logWorkerDedupSkip(event, role)
			continue
		}
		pending[event.ID] = true
		deliveredIDs = append(deliveredIDs, event.ID)
		out = append(out, event)
	}
	return out, deliveredIDs, nil
}

func markSeenEvents(ctx context.Context, seen SeenStore, ids []string) error {
	if seen == nil {
		return nil
	}
	for _, id := range ids {
		if err := seen.MarkSeen(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

func logWorkerDedupSkip(event EventEnvelope, role Role) {
	logger := WorkerLogger
	if logger == nil {
		return
	}
	observation := event.ObservationForRole(ObservationWorkerDedupSkip, role)
	logger("gowdk: event worker skipped duplicate event " + event.ID + " (" + string(observation.Labels.EventCategory) + " " + observation.Labels.Contract + ")")
}

func ackEventBatch(ctx context.Context, batch EventBatch) error {
	if batch.Ack == nil {
		return nil
	}
	return batch.Ack(ctx)
}

func newEventWorkerOptions(optionFns []EventWorkerOption) eventWorkerOptions {
	var options eventWorkerOptions
	for _, option := range optionFns {
		if option != nil {
			option(&options)
		}
	}
	return options
}

func (options eventWorkerOptions) retryDelay(retry EventWorkerRetry) time.Duration {
	if options.backoff == nil {
		return 0
	}
	delay := options.backoff(retry)
	if delay <= 0 {
		return 0
	}
	return delay
}

func waitEventWorkerBackoff(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
