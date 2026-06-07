// Package natsbroker provides a NATS adapter for runtime/contracts.
package natsbroker

import (
	"context"
	"encoding/json"
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
type Decoder func(json.RawMessage) (any, error)

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
	return WithDecoder(eventType, func(raw json.RawMessage) (any, error) {
		var value T
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, err
		}
		return value, nil
	})
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
	events := []contracts.EventEnvelope{first}
	for len(events) < broker.batchSize {
		pollCtx, cancel := context.WithTimeout(ctx, time.Millisecond)
		event, err := broker.nextMessage(pollCtx)
		cancel()
		if err != nil {
			break
		}
		events = append(events, event)
	}
	return contracts.EventBatch{Events: events}, nil
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
	waitCtx := ctx
	cancel := func() {}
	if broker.timeout > 0 {
		waitCtx, cancel = context.WithTimeout(ctx, broker.timeout)
	}
	defer cancel()
	message, err := broker.sub.NextMsgWithContext(waitCtx)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			if ctx.Err() != nil {
				return contracts.EventEnvelope{}, ctx.Err()
			}
			return contracts.EventEnvelope{}, contracts.ErrEventSourceClosed
		}
		return contracts.EventEnvelope{}, err
	}
	return broker.decodeMessage(message)
}

func (broker *Broker) decodeMessage(message *nats.Msg) (contracts.EventEnvelope, error) {
	return broker.decodePayload(message.Data)
}

func (broker *Broker) decodePayload(payload []byte) (contracts.EventEnvelope, error) {
	var stored storedEnvelope
	if err := json.Unmarshal(payload, &stored); err != nil {
		return contracts.EventEnvelope{}, err
	}
	value := any(stored.Value)
	if decoder := broker.decoders[stored.Type]; decoder != nil {
		decoded, err := decoder(stored.Value)
		if err != nil {
			return contracts.EventEnvelope{}, err
		}
		value = decoded
	}
	return contracts.EventEnvelope{Category: stored.Category, Type: stored.Type, Value: value}, nil
}

type storedEnvelope struct {
	Category contracts.EventCategory `json:"category"`
	Type     string                  `json:"type"`
	Value    json.RawMessage         `json:"value"`
}

func marshalEnvelope(event contracts.EventEnvelope) ([]byte, error) {
	value, err := json.Marshal(event.Value)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(storedEnvelope{Category: event.Category, Type: event.Type, Value: value})
	if err != nil {
		return nil, err
	}
	return payload, nil
}
