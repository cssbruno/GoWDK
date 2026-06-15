package trace

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
)

// Collector combines an in-memory ring sink with a small HTTP JSON/SSE
// handler for local inspection.
type Collector struct {
	ring        *RingSink
	mu          sync.Mutex
	subscribers map[chan Snapshot]struct{}
}

// NewCollector creates a collector backed by a RingSink.
func NewCollector(limit int) *Collector {
	return &Collector{
		ring:        NewRingSink(limit),
		subscribers: map[chan Snapshot]struct{}{},
	}
}

// RecordSpan implements Sink.
func (collector *Collector) RecordSpan(ctx context.Context, span Snapshot) error {
	if collector == nil {
		return nil
	}
	if err := collector.ring.RecordSpan(ctx, span); err != nil {
		return err
	}
	collector.mu.Lock()
	for subscriber := range collector.subscribers {
		select {
		case subscriber <- span:
		default:
		}
	}
	collector.mu.Unlock()
	return nil
}

// Spans returns completed spans from oldest to newest.
func (collector *Collector) Spans() []Snapshot {
	if collector == nil {
		return nil
	}
	return collector.ring.Spans()
}

// Dropped returns the ring overflow count.
func (collector *Collector) Dropped() uint64 {
	if collector == nil {
		return 0
	}
	return collector.ring.Dropped()
}

// Handler serves recent spans as JSON. Requests to /events or requests with an
// Accept header containing text/event-stream receive an SSE stream of existing
// and future spans.
func (collector *Collector) Handler() http.Handler {
	if collector == nil {
		collector = NewCollector(defaultRingLimit)
	}
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			response.Header().Set("Allow", http.MethodGet)
			http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if request.URL.Path == "/events" || strings.Contains(request.Header.Get("Accept"), "text/event-stream") {
			collector.serveSSE(response, request)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(struct {
			Spans   []Snapshot `json:"spans"`
			Dropped uint64     `json:"dropped"`
		}{Spans: collector.Spans(), Dropped: collector.Dropped()})
	})
}

func (collector *Collector) serveSSE(response http.ResponseWriter, request *http.Request) {
	flusher, ok := response.(http.Flusher)
	if !ok {
		http.Error(response, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	response.Header().Set("Content-Type", "text/event-stream")
	response.Header().Set("Cache-Control", "no-cache")
	response.Header().Set("Connection", "keep-alive")
	events := make(chan Snapshot, 16)
	collector.mu.Lock()
	collector.subscribers[events] = struct{}{}
	collector.mu.Unlock()
	defer func() {
		collector.mu.Lock()
		delete(collector.subscribers, events)
		collector.mu.Unlock()
		close(events)
	}()
	for _, span := range collector.Spans() {
		if !writeSSE(response, flusher, span) {
			return
		}
	}
	for {
		select {
		case <-request.Context().Done():
			return
		case span := <-events:
			if !writeSSE(response, flusher, span) {
				return
			}
		}
	}
}

func writeSSE(response http.ResponseWriter, flusher http.Flusher, span Snapshot) bool {
	payload, err := json.Marshal(span)
	if err != nil {
		return true
	}
	if _, err := fmt.Fprintf(response, "event: gowdk-trace\ndata: %s\n\n", payload); err != nil {
		return false
	}
	flusher.Flush()
	return true
}
