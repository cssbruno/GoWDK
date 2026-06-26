// Package realtime declares the browser presentation-event realtime compiler
// capability and exposes dependency-free SSE helpers.
package realtime

import (
	"net/http"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/runtime/contracts"
	runtimerealtime "github.com/cssbruno/gowdk/runtime/realtime"
)

// ImportPath is the canonical Go import path for the realtime addon.
const ImportPath = "github.com/cssbruno/gowdk/addons/realtime"

// PresentationFanout sends browser-facing presentation events.
type PresentationFanout = contracts.PresentationFanout

// SSEHub fans presentation events out to connected server-sent events clients.
type SSEHub = runtimerealtime.SSEHub

// SSEOption configures a dependency-free SSE hub.
type SSEOption = runtimerealtime.SSEOption

// SSEStats reports process-local SSE hub counters for app-owned metrics export.
type SSEStats = runtimerealtime.SSEStats

// Addon enables realtime presentation-event fanout support.
func Addon() gowdk.Addon {
	return gowdk.NewAddon("realtime", gowdk.FeatureRealtime)
}

// NewSSE creates a dependency-free server-sent events presentation fanout hub.
func NewSSE(options ...SSEOption) *SSEHub {
	return runtimerealtime.NewSSE(options...)
}

// WithSSEBufferSize sets each SSE client's queued message buffer.
func WithSSEBufferSize(size int) SSEOption {
	return runtimerealtime.WithSSEBufferSize(size)
}

// WithSSERetryMillis sets the browser EventSource reconnect delay advertised by
// the generated SSE stream.
func WithSSERetryMillis(milliseconds int) SSEOption {
	return runtimerealtime.WithSSERetryMillis(milliseconds)
}

// WithSSEReplayLimit keeps a bounded in-memory replay buffer for browser
// reconnects that send Last-Event-ID.
func WithSSEReplayLimit(limit int) SSEOption {
	return runtimerealtime.WithSSEReplayLimit(limit)
}

// WithSSEAudienceFromRequest assigns server-owned audience labels to one SSE
// client. Scoped presentation events are delivered only when every event label
// is present in the client audience set.
func WithSSEAudienceFromRequest(fn func(*http.Request) []string) SSEOption {
	return runtimerealtime.WithSSEAudienceFromRequest(fn)
}
