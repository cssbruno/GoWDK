// Package realtime provides request-time presentation-event fanout helpers for
// generated apps.
package realtime

import (
	"net/http"

	"github.com/cssbruno/gowdk/runtime/contracts"
	"github.com/cssbruno/gowdk/runtime/contracts/sse"
)

// PresentationFanout sends browser-facing presentation events.
type PresentationFanout = contracts.PresentationFanout

// SSEHub fans presentation events out to connected server-sent events clients.
type SSEHub = sse.Hub

// SSEOption configures a dependency-free SSE hub.
type SSEOption = sse.Option

// NewSSE creates a dependency-free server-sent events presentation fanout hub.
func NewSSE(options ...SSEOption) *SSEHub {
	return sse.New(options...)
}

// WithSSEBufferSize sets each SSE client's queued message buffer.
func WithSSEBufferSize(size int) SSEOption {
	return sse.WithBufferSize(size)
}

// WithSSEAudienceFromRequest assigns server-owned audience labels to one SSE
// client. Scoped presentation events are delivered only when every event label
// is present in the client audience set.
func WithSSEAudienceFromRequest(fn func(*http.Request) []string) SSEOption {
	return sse.WithAudienceFromRequest(fn)
}
