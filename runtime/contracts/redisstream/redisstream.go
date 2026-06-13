// Package redisstream provides a Redis Streams adapter for runtime/contracts.
package redisstream

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cssbruno/gowdk/runtime/contracts"
	redis "github.com/redis/go-redis/v9"
)

const (
	defaultBatchSize = 100
	eventField       = "event"
)

// Store publishes events to a Redis stream and reads them through a consumer
// group. It implements contracts.Broker and contracts.EventSource.
type Store struct {
	client    *redis.Client
	stream    string
	group     string
	consumer  string
	batchSize int64
	block     time.Duration
	decoders  map[string]Decoder

	mu sync.Mutex
	// readPending makes the next receive re-read this consumer's pending
	// entries (read but never acked) before consuming new messages. It starts
	// true so entries stranded by a crash or restart are redelivered, and it
	// is set again by Nack so failed batches are retried.
	readPending bool
}

// Decoder converts one JSON event value into the typed value expected by
// runtime/contracts subscribers.
type Decoder = contracts.EventDecoder

// Option configures a Store.
type Option func(*Store)

// WithBatchSize sets the max stream messages returned per receive call.
func WithBatchSize(size int64) Option {
	return func(store *Store) {
		if size > 0 {
			store.batchSize = size
		}
	}
}

// WithBlock sets the Redis XREADGROUP block duration.
func WithBlock(block time.Duration) Option {
	return func(store *Store) {
		if block >= 0 {
			store.block = block
		}
	}
}

// WithDecoder registers a decoder for one event type.
func WithDecoder(eventType string, decoder Decoder) Option {
	return func(store *Store) {
		if eventType != "" && decoder != nil {
			store.decoders[eventType] = decoder
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

// New creates a Redis Streams store.
func New(client *redis.Client, stream, group, consumer string, options ...Option) *Store {
	store := &Store{
		client:      client,
		stream:      stream,
		group:       group,
		consumer:    consumer,
		batchSize:   defaultBatchSize,
		decoders:    map[string]Decoder{},
		readPending: true,
	}
	for _, option := range options {
		if option != nil {
			option(store)
		}
	}
	return store
}

// EnsureGroup creates the consumer group if it does not already exist.
func (store *Store) EnsureGroup(ctx context.Context) error {
	if err := store.validate(); err != nil {
		return err
	}
	err := store.client.XGroupCreateMkStream(ctx, store.stream, store.group, "0").Err()
	if err != nil && !stringsContainsBUSYGROUP(err.Error()) {
		return err
	}
	return nil
}

// PublishEvents appends event envelopes to the Redis stream.
func (store *Store) PublishEvents(ctx context.Context, events []contracts.EventEnvelope) error {
	if err := store.validate(); err != nil {
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
		if err := store.client.XAdd(ctx, &redis.XAddArgs{
			Stream: store.stream,
			Values: map[string]any{eventField: payload},
		}).Err(); err != nil {
			return err
		}
	}
	return nil
}

// ReceiveEventBatch reads the next stream batch for this consumer. It first
// drains this consumer's pending entries (read earlier but never acked, for
// example before a crash or after a Nack) and only then consumes new messages.
func (store *Store) ReceiveEventBatch(ctx context.Context) (contracts.EventBatch, error) {
	if err := store.validate(); err != nil {
		return contracts.EventBatch{}, err
	}
	if err := ctx.Err(); err != nil {
		return contracts.EventBatch{}, err
	}
	if store.pendingFirst() {
		batch, ok, err := store.readBatch(ctx, "0")
		if err != nil {
			return contracts.EventBatch{}, err
		}
		if ok {
			return batch, nil
		}
		store.setPendingFirst(false)
	}
	for {
		batch, ok, err := store.readBatch(ctx, ">")
		if err != nil {
			return contracts.EventBatch{}, err
		}
		if ok {
			return batch, nil
		}
		if err := ctx.Err(); err != nil {
			return contracts.EventBatch{}, err
		}
	}
}

func (store *Store) pendingFirst() bool {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.readPending
}

func (store *Store) setPendingFirst(pending bool) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.readPending = pending
}

func (store *Store) readBatch(ctx context.Context, start string) (contracts.EventBatch, bool, error) {
	streams, err := store.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    store.group,
		Consumer: store.consumer,
		Streams:  []string{store.stream, start},
		Count:    store.batchSize,
		Block:    store.block,
	}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return contracts.EventBatch{}, false, nil
		}
		return contracts.EventBatch{}, false, err
	}
	var ids []string
	var events []contracts.EventEnvelope
	for _, stream := range streams {
		for _, message := range stream.Messages {
			event, err := store.decodeMessage(message)
			if err != nil {
				return contracts.EventBatch{}, false, err
			}
			ids = append(ids, message.ID)
			events = append(events, event)
		}
	}
	if len(events) == 0 {
		return contracts.EventBatch{}, false, nil
	}
	return contracts.EventBatch{
		Events: events,
		Ack: func(ctx context.Context) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := store.client.XAck(ctx, store.stream, store.group, ids...).Err(); err != nil {
				return err
			}
			return store.client.XDel(ctx, store.stream, ids...).Err()
		},
		Nack: func(ctx context.Context, cause error) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			// The messages stay in this consumer's pending entries list;
			// rewind the next read so they are redelivered. Redis tracks the
			// delivery count for observability via XPENDING.
			_ = cause
			store.setPendingFirst(true)
			return nil
		},
	}, true, nil
}

func (store *Store) validate() error {
	switch {
	case store.client == nil:
		return errors.New("redis stream client is required")
	case store.stream == "":
		return errors.New("redis stream name is required")
	case store.group == "":
		return errors.New("redis stream group is required")
	case store.consumer == "":
		return errors.New("redis stream consumer is required")
	default:
		return nil
	}
}

func (store *Store) decodeMessage(message redis.XMessage) (contracts.EventEnvelope, error) {
	raw, ok := message.Values[eventField]
	if !ok {
		return contracts.EventEnvelope{}, fmt.Errorf("redis stream message %s missing %q field", message.ID, eventField)
	}
	source, ok := raw.(string)
	if !ok {
		return contracts.EventEnvelope{}, fmt.Errorf("redis stream message %s %q field has type %T", message.ID, eventField, raw)
	}
	return store.decodeStored(message.ID, source)
}

func (store *Store) decodeStored(_ string, source string) (contracts.EventEnvelope, error) {
	return contracts.DecodeEventEnvelopeJSON([]byte(source), store.decoders)
}

func marshalEnvelope(event contracts.EventEnvelope) (string, error) {
	payload, err := contracts.MarshalEventEnvelopeJSON(event)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func stringsContainsBUSYGROUP(value string) bool {
	return strings.HasPrefix(value, "BUSYGROUP")
}
