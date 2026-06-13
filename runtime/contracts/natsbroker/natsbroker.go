// Package natsbroker provides a NATS adapter for runtime/contracts.
package natsbroker

import (
	"context"
	"errors"
	"time"

	"github.com/cssbruno/gowdk/runtime/contracts"
	nats "github.com/nats-io/nats.go"
)

const defaultBatchSize = 100

// Broker publishes contract events to a NATS subject and can receive them from
// a synchronous subscription. It implements contracts.Broker and
// contracts.EventSource.
type Broker struct {
	conn      *nats.Conn
	subject   string
	queue     string
	batchSize int
	timeout   time.Duration
	decoders  map[string]Decoder
	sub       *nats.Subscription
}

// Decoder converts one JSON event value into the typed value expected by
// runtime/contracts subscribers.
type Decoder = contracts.EventDecoder

// Option configures a Broker.
type Option func(*Broker)

// WithQueue sets an optional queue group for subscribers.
func WithQueue(queue string) Option {
	return func(broker *Broker) {
		broker.queue = queue
	}
}

// WithBatchSize sets the max messages returned per receive call.
func WithBatchSize(size int) Option {
	return func(broker *Broker) {
		if size > 0 {
			broker.batchSize = size
		}
	}
}

// WithTimeout sets the max wait for one message.
func WithTimeout(timeout time.Duration) Option {
	return func(broker *Broker) {
		if timeout >= 0 {
			broker.timeout = timeout
		}
	}
}

// WithDecoder registers a decoder for one event type.
func WithDecoder(eventType string, decoder Decoder) Option {
	return func(broker *Broker) {
		if eventType != "" && decoder != nil {
			broker.decoders[eventType] = decoder
		}
	}
}

// WithJSONDecoder registers a JSON decoder for one event type.
func WithJSONDecoder[T any](eventType string) Option {
	return WithDecoder(eventType, contracts.JSONEventDecoder[T]())
}

// WithJSONTypeDecoder registers a JSON decoder using the same Go type name
// stored by runtime/contracts when T is emitted.
func WithJSONTypeDecoder[T any]() Option {
	return WithJSONDecoder[T](contracts.ContractName[T]())
}

// New creates a NATS broker adapter.
func New(conn *nats.Conn, subject string, options ...Option) *Broker {
	broker := &Broker{
		conn:      conn,
		subject:   subject,
		batchSize: defaultBatchSize,
		timeout:   time.Second,
		decoders:  map[string]Decoder{},
	}
	for _, option := range options {
		if option != nil {
			option(broker)
		}
	}
	return broker
}

// PublishEvents publishes event envelopes to the configured subject.
func (broker *Broker) PublishEvents(ctx context.Context, events []contracts.EventEnvelope) error {
	if err := broker.validate(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, event := range events {
		event = contracts.EnsureEventID(event)
		payload, err := marshalEnvelope(event)
		if err != nil {
			return err
		}
		if err := broker.conn.Publish(broker.subject, payload); err != nil {
			return err
		}
	}
	return broker.conn.FlushWithContext(ctx)
}

// ReceiveEventBatch receives up to the configured batch size. Core NATS
// acknowledgements are implicit, so returned batches do not include ack hooks.
func (broker *Broker) ReceiveEventBatch(ctx context.Context) (contracts.EventBatch, error) {
	if err := broker.validate(); err != nil {
		return contracts.EventBatch{}, err
	}
	if err := broker.ensureSubscription(); err != nil {
		return contracts.EventBatch{}, err
	}
	first, err := broker.nextMessage(ctx)
	if err != nil {
		return contracts.EventBatch{}, err
	}
	events := drainAvailableEvents(ctx, first, broker.batchSize, broker.tryNextMessage)
	return contracts.EventBatch{Events: events}, nil
}

func drainAvailableEvents(ctx context.Context, first contracts.EventEnvelope, batchSize int, next func(context.Context) (contracts.EventEnvelope, bool, error)) []contracts.EventEnvelope {
	events := []contracts.EventEnvelope{first}
	for len(events) < batchSize {
		pollCtx, cancel := context.WithTimeout(ctx, time.Millisecond)
		event, ok, err := next(pollCtx)
		cancel()
		if err != nil {
			return events
		}
		if !ok {
			break
		}
		events = append(events, event)
	}
	return events
}

// Close unsubscribes the receive subscription.
func (broker *Broker) Close() error {
	if broker.sub == nil {
		return nil
	}
	err := broker.sub.Unsubscribe()
	broker.sub = nil
	return err
}

func (broker *Broker) validate() error {
	switch {
	case broker.conn == nil:
		return errors.New("nats connection is required")
	case broker.subject == "":
		return errors.New("nats subject is required")
	default:
		return nil
	}
}

func (broker *Broker) ensureSubscription() error {
	if broker.sub != nil {
		return nil
	}
	var err error
	if broker.queue != "" {
		broker.sub, err = broker.conn.QueueSubscribeSync(broker.subject, broker.queue)
	} else {
		broker.sub, err = broker.conn.SubscribeSync(broker.subject)
	}
	return err
}

func (broker *Broker) nextMessage(ctx context.Context) (contracts.EventEnvelope, error) {
	for {
		waitCtx := ctx
		cancel := func() {}
		if broker.timeout > 0 {
			waitCtx, cancel = context.WithTimeout(ctx, broker.timeout)
		}
		event, ok, err := broker.tryNextMessage(waitCtx)
		cancel()
		if err != nil {
			return contracts.EventEnvelope{}, err
		}
		if ok {
			return event, nil
		}
		if err := ctx.Err(); err != nil {
			return contracts.EventEnvelope{}, err
		}
	}
}

func (broker *Broker) tryNextMessage(ctx context.Context) (contracts.EventEnvelope, bool, error) {
	message, err := broker.sub.NextMsgWithContext(ctx)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return contracts.EventEnvelope{}, false, nil
		}
		return contracts.EventEnvelope{}, false, err
	}
	event, err := broker.decodeMessage(message)
	if err != nil {
		return contracts.EventEnvelope{}, false, err
	}
	return event, true, nil
}

func (broker *Broker) decodeMessage(message *nats.Msg) (contracts.EventEnvelope, error) {
	return broker.decodePayload(message.Data)
}

func (broker *Broker) decodePayload(payload []byte) (contracts.EventEnvelope, error) {
	return contracts.DecodeEventEnvelopeJSON(payload, broker.decoders)
}

func marshalEnvelope(event contracts.EventEnvelope) ([]byte, error) {
	return contracts.MarshalEventEnvelopeJSON(event)
}
