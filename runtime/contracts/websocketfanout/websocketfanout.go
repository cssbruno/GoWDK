// Package websocketfanout provides a WebSocket presentation fanout adapter for
// runtime/contracts.
package websocketfanout

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/coder/websocket"
	"github.com/cssbruno/gowdk/runtime/contracts"
)

const defaultBufferSize = 16

// Hub fans presentation events out to connected WebSocket clients.
type Hub struct {
	mu         sync.Mutex
	bufferSize int
	clients    map[*websocket.Conn]chan []byte
	accept     *websocket.AcceptOptions
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

// WithAcceptOptions sets the options used for WebSocket upgrades.
func WithAcceptOptions(options websocket.AcceptOptions) Option {
	return func(hub *Hub) {
		hub.accept = &options
	}
}

// New creates a WebSocket fanout hub.
func New(options ...Option) *Hub {
	hub := &Hub{
		bufferSize: defaultBufferSize,
		clients:    map[*websocket.Conn]chan []byte{},
	}
	for _, option := range options {
		if option != nil {
			option(hub)
		}
	}
	return hub
}

// ServeHTTP upgrades one browser connection and streams presentation events.
func (hub *Hub) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	conn, err := websocket.Accept(response, request, hub.accept)
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	ctx := conn.CloseRead(request.Context())
	client := make(chan []byte, hub.bufferSize)
	hub.add(conn, client)
	defer hub.remove(conn)

	for {
		select {
		case <-ctx.Done():
			return
		case payload := <-client:
			if err := conn.Write(ctx, websocket.MessageText, payload); err != nil {
				return
			}
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
	for _, client := range hub.clients {
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

func (hub *Hub) add(conn *websocket.Conn, client chan []byte) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	hub.clients[conn] = client
}

func (hub *Hub) remove(conn *websocket.Conn) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	if client, ok := hub.clients[conn]; ok {
		delete(hub.clients, conn)
		close(client)
	}
}
