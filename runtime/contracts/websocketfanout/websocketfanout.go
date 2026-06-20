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
	clients    map[*websocket.Conn]*wsClient
	accept     *websocket.AcceptOptions
}

// wsClient is one connected browser. disconnect is closed (once) to ask the
// streaming goroutine to drop the connection so the browser reconnects and
// resynchronizes.
type wsClient struct {
	queue      chan []byte
	disconnect chan struct{}
	once       sync.Once
}

func (client *wsClient) drop() {
	client.once.Do(func() { close(client.disconnect) })
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
		clients:    map[*websocket.Conn]*wsClient{},
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
	client := &wsClient{
		queue:      make(chan []byte, hub.bufferSize),
		disconnect: make(chan struct{}),
	}
	hub.add(conn, client)
	defer hub.remove(conn)

	for {
		select {
		case <-ctx.Done():
			return
		case <-client.disconnect:
			return
		case payload := <-client.queue:
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
	var slow []*wsClient
	for _, client := range hub.clients {
	queueLoop:
		for _, payload := range payloads {
			select {
			case client.queue <- payload:
			default:
				// The client cannot keep up. Drop it so the browser reconnects
				// and resynchronizes, rather than silently missing events (for
				// example a query invalidation) and leaving the UI stale.
				slow = append(slow, client)
				break queueLoop
			}
		}
	}
	hub.mu.Unlock()
	for _, client := range slow {
		client.drop()
	}
	return nil
}

// ClientCount returns the number of currently connected clients.
func (hub *Hub) ClientCount() int {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	return len(hub.clients)
}

func (hub *Hub) add(conn *websocket.Conn, client *wsClient) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	hub.clients[conn] = client
}

func (hub *Hub) remove(conn *websocket.Conn) {
	hub.mu.Lock()
	client, ok := hub.clients[conn]
	delete(hub.clients, conn)
	hub.mu.Unlock()
	// The queue channel is left for the garbage collector: the sender only
	// holds it under the lock, so no send can reach a removed client.
	if ok {
		client.drop()
	}
}
