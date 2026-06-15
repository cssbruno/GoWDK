// Package sse provides a dependency-free server-sent events presentation
// fanout adapter for runtime/contracts.
package sse

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"

	"github.com/cssbruno/gowdk/runtime/contracts"
)

const (
	defaultBufferSize     = 16
	defaultRetryDirective = "retry: 1000\n"
)

// Hub fans presentation events out to connected server-sent events clients.
type Hub struct {
	mu         sync.Mutex
	bufferSize int
	clients    map[chan []byte]bool
}

// Option configures a Hub.
type Option func(*Hub)

// WithBufferSize sets each client's queued message buffer. Non-positive values
// keep the default.
func WithBufferSize(size int) Option {
	return func(hub *Hub) {
		if size > 0 {
			hub.bufferSize = size
		}
	}
}

// New creates an SSE hub.
func New(options ...Option) *Hub {
	hub := &Hub{
		bufferSize: defaultBufferSize,
		clients:    map[chan []byte]bool{},
	}
	for _, option := range options {
		if option != nil {
			option(hub)
		}
	}
	return hub
}

// ServeHTTP streams presentation events to one browser client.
func (hub *Hub) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	flusher, ok := response.(http.Flusher)
	if !ok {
		http.Error(response, "streaming is not supported", http.StatusInternalServerError)
		return
	}

	response.Header().Set("Content-Type", "text/event-stream")
	response.Header().Set("Cache-Control", "no-store")
	response.Header().Set("Connection", "keep-alive")
	response.Header().Set("X-Accel-Buffering", "no")

	client := make(chan []byte, hub.bufferSize)
	hub.add(client)
	defer hub.remove(client)

	_, _ = response.Write([]byte(defaultRetryDirective + ": gowdk presentation stream\n\n"))
	flusher.Flush()

	for {
		select {
		case <-request.Context().Done():
			return
		case payload := <-client:
			if _, err := response.Write([]byte("event: gowdk-presentation\n")); err != nil {
				return
			}
			if _, err := response.Write([]byte("data: ")); err != nil {
				return
			}
			if _, err := response.Write(payload); err != nil {
				return
			}
			if _, err := response.Write([]byte("\n\n")); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// SendPresentationEvents broadcasts presentation events to connected clients.
// Non-presentation events are ignored.
func (hub *Hub) SendPresentationEvents(ctx context.Context, events []contracts.EventEnvelope) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	payloads := make([][]byte, 0, len(events))
	for _, event := range events {
		if event.Category != contracts.PresentationEvent {
			continue
		}
		payload, err := json.Marshal(event)
		if err != nil {
			return err
		}
		payloads = append(payloads, payload)
	}
	if len(payloads) == 0 {
		return nil
	}
	hub.mu.Lock()
	defer hub.mu.Unlock()
	for client := range hub.clients {
		for _, payload := range payloads {
			select {
			case client <- payload:
			default:
			}
		}
	}
	return nil
}

// ClientCount returns the number of currently connected clients.
func (hub *Hub) ClientCount() int {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	return len(hub.clients)
}

func (hub *Hub) add(client chan []byte) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	hub.clients[client] = true
}

func (hub *Hub) remove(client chan []byte) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	delete(hub.clients, client)
	close(client)
}

// ErrNoHub is returned by helpers that require a hub but receive nil.
var ErrNoHub = errors.New("sse hub cannot be nil")
