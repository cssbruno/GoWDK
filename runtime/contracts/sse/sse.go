// Package sse provides a dependency-free server-sent events presentation
// fanout adapter for runtime/contracts.
package sse

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/cssbruno/gowdk/runtime/contracts"
)

const (
	defaultBufferSize  = 16
	defaultRetryMillis = 1000
)

// Hub fans presentation events out to connected server-sent events clients.
type Hub struct {
	mu                  sync.Mutex
	bufferSize          int
	retryMillis         int
	replayLimit         int
	replay              []sseMessage
	droppedClients      int64
	revokedClients      int64
	replayedEvents      int64
	replayMisses        int64
	audienceFromRequest func(*http.Request) []string
	clients             map[*sseClient]bool
}

// sseClient is one connected browser. disconnect is closed (once) to ask the
// streaming goroutine to drop the connection so the browser's EventSource
// reconnects and resynchronizes.
type sseClient struct {
	queue      chan sseMessage
	disconnect chan struct{}
	once       sync.Once
	audience   map[string]struct{}
}

func (client *sseClient) drop() {
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

// WithRetryMillis sets the browser EventSource reconnect delay advertised by
// the stream. Non-positive values keep the default one-second delay.
func WithRetryMillis(milliseconds int) Option {
	return func(hub *Hub) {
		if milliseconds > 0 {
			hub.retryMillis = milliseconds
		}
	}
}

// WithReplayLimit keeps the last limit presentation events in memory and
// replays events after the browser's Last-Event-ID on reconnect. Non-positive
// values disable replay.
func WithReplayLimit(limit int) Option {
	return func(hub *Hub) {
		if limit <= 0 {
			hub.replayLimit = 0
			hub.replay = nil
			return
		}
		hub.replayLimit = limit
		if len(hub.replay) > limit {
			hub.replay = append([]sseMessage(nil), hub.replay[len(hub.replay)-limit:]...)
		}
	}
}

// WithAudienceFromRequest assigns one or more server-owned audience labels to a
// connecting client. Empty labels make the client receive broadcast events
// only. Do not read audience labels from client-controlled query parameters
// without authenticating them first.
func WithAudienceFromRequest(fn func(*http.Request) []string) Option {
	return func(hub *Hub) {
		hub.audienceFromRequest = fn
	}
}

// New creates an SSE hub.
func New(options ...Option) *Hub {
	hub := &Hub{
		bufferSize:  defaultBufferSize,
		retryMillis: defaultRetryMillis,
		clients:     map[*sseClient]bool{},
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

	client := &sseClient{
		queue:      make(chan sseMessage, hub.bufferSize),
		disconnect: make(chan struct{}),
		audience:   audienceSet(hub.clientAudience(request)),
	}
	replay := hub.addWithReplay(client, request.Header.Get("Last-Event-ID"))
	defer hub.remove(client)

	if !writeRetry(response, flusher, hub.retryMillis) {
		return
	}
	for _, message := range replay {
		if !writeMessage(response, flusher, message) {
			return
		}
	}

	for {
		select {
		case <-request.Context().Done():
			return
		case <-client.disconnect:
			return
		case message := <-client.queue:
			if !writeMessage(response, flusher, message) {
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
	payloads := make([]payloadWithAudience, 0, len(events))
	for _, event := range events {
		if event.Category != contracts.PresentationEvent {
			continue
		}
		event = contracts.EnsureEventID(event)
		payload, err := json.Marshal(event)
		if err != nil {
			return err
		}
		payloads = append(payloads, payloadWithAudience{id: event.ID, payload: payload, audience: event.AudienceLabels()})
	}
	if len(payloads) == 0 {
		return nil
	}
	hub.mu.Lock()
	for _, payload := range payloads {
		hub.recordReplayLocked(sseMessage(payload))
	}
	var slow []*sseClient
	for client := range hub.clients {
	queueLoop:
		for _, payload := range payloads {
			if !audienceMatches(payload.audience, client.audience) {
				continue
			}
			select {
			case client.queue <- sseMessage(payload):
			default:
				// The client cannot keep up. Drop it so the browser reconnects
				// and resynchronizes, rather than silently missing events (for
				// example a query invalidation) and leaving the UI stale.
				slow = append(slow, client)
				break queueLoop
			}
		}
	}
	hub.droppedClients += int64(len(slow))
	hub.mu.Unlock()
	for _, client := range slow {
		client.drop()
	}
	return nil
}

type sseMessage struct {
	id       string
	payload  []byte
	audience []string
}

type payloadWithAudience sseMessage

// Stats reports process-local SSE hub counters for app-owned metrics export.
type Stats struct {
	ClientCount    int
	DroppedClients int64
	RevokedClients int64
	ReplayedEvents int64
	ReplayMisses   int64
}

// ClientCount returns the number of currently connected clients.
func (hub *Hub) ClientCount() int {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	return len(hub.clients)
}

// Stats returns current process-local hub counters.
func (hub *Hub) Stats() Stats {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	return Stats{
		ClientCount:    len(hub.clients),
		DroppedClients: hub.droppedClients,
		RevokedClients: hub.revokedClients,
		ReplayedEvents: hub.replayedEvents,
		ReplayMisses:   hub.replayMisses,
	}
}

// RevokeAudience disconnects currently connected clients whose server-owned
// audience label set includes every provided label. Include a session-specific
// label in WithAudienceFromRequest to revoke one active browser session.
func (hub *Hub) RevokeAudience(labels ...string) int {
	audience := contracts.EventEnvelope{Audience: labels}.AudienceLabels()
	if len(audience) == 0 {
		return 0
	}
	hub.mu.Lock()
	revoked := make([]*sseClient, 0)
	for client := range hub.clients {
		if audienceMatches(audience, client.audience) {
			revoked = append(revoked, client)
		}
	}
	hub.revokedClients += int64(len(revoked))
	hub.mu.Unlock()
	for _, client := range revoked {
		client.drop()
	}
	return len(revoked)
}

func (hub *Hub) clientAudience(request *http.Request) []string {
	if hub.audienceFromRequest == nil {
		return nil
	}
	return contracts.EventEnvelope{Audience: hub.audienceFromRequest(request)}.AudienceLabels()
}

func (hub *Hub) recordReplayLocked(message sseMessage) {
	if hub.replayLimit <= 0 || message.id == "" {
		return
	}
	hub.replay = append(hub.replay, message)
	if len(hub.replay) > hub.replayLimit {
		hub.replay = append([]sseMessage(nil), hub.replay[len(hub.replay)-hub.replayLimit:]...)
	}
}

func (hub *Hub) replayAfterLocked(lastEventID string, clientAudience map[string]struct{}) []sseMessage {
	lastEventID = strings.TrimSpace(lastEventID)
	if lastEventID == "" || hub.replayLimit <= 0 {
		return nil
	}
	start := -1
	for index, message := range hub.replay {
		if message.id == lastEventID {
			start = index + 1
			break
		}
	}
	if start < 0 || start >= len(hub.replay) {
		hub.replayMisses++
		return nil
	}
	out := make([]sseMessage, 0, len(hub.replay)-start)
	for _, message := range hub.replay[start:] {
		if audienceMatches(message.audience, clientAudience) {
			out = append(out, message)
		}
	}
	hub.replayedEvents += int64(len(out))
	return out
}

func writeRetry(response http.ResponseWriter, flusher http.Flusher, retryMillis int) bool {
	if retryMillis <= 0 {
		retryMillis = defaultRetryMillis
	}
	if _, err := response.Write([]byte("retry: " + strconv.Itoa(retryMillis) + "\n: gowdk presentation stream\n\n")); err != nil {
		return false
	}
	flusher.Flush()
	return true
}

func writeMessage(response http.ResponseWriter, flusher http.Flusher, message sseMessage) bool {
	if id := cleanSSEID(message.id); id != "" {
		if _, err := response.Write([]byte("id: " + id + "\n")); err != nil {
			return false
		}
	}
	if _, err := response.Write([]byte("event: gowdk-presentation\n")); err != nil {
		return false
	}
	if _, err := response.Write([]byte("data: ")); err != nil {
		return false
	}
	if _, err := response.Write(message.payload); err != nil {
		return false
	}
	if _, err := response.Write([]byte("\n\n")); err != nil {
		return false
	}
	flusher.Flush()
	return true
}

func cleanSSEID(id string) string {
	id = strings.TrimSpace(id)
	id = strings.ReplaceAll(id, "\r", "")
	return strings.ReplaceAll(id, "\n", "")
}

func audienceSet(audience []string) map[string]struct{} {
	if len(audience) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(audience))
	for _, label := range audience {
		out[label] = struct{}{}
	}
	return out
}

func audienceMatches(eventAudience []string, clientAudience map[string]struct{}) bool {
	if len(eventAudience) == 0 {
		return true
	}
	if len(clientAudience) == 0 {
		return false
	}
	for _, label := range eventAudience {
		if _, ok := clientAudience[label]; !ok {
			return false
		}
	}
	return true
}

func (hub *Hub) add(client *sseClient) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	hub.clients[client] = true
}

func (hub *Hub) addWithReplay(client *sseClient, lastEventID string) []sseMessage {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	replay := hub.replayAfterLocked(lastEventID, client.audience)
	hub.clients[client] = true
	return replay
}

func (hub *Hub) remove(client *sseClient) {
	hub.mu.Lock()
	delete(hub.clients, client)
	hub.mu.Unlock()
	// The queue channel is intentionally left for the garbage collector: the
	// sender only ever holds it under the lock, so once the client is removed
	// no send can reach it. Closing it here could race a concurrent send.
	client.drop()
}

// ErrNoHub is returned by helpers that require a hub but receive nil.
var ErrNoHub = errors.New("sse hub cannot be nil")
