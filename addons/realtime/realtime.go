// Package realtime declares the browser presentation-event realtime compiler
// capability and exposes dependency-free SSE helpers.
package realtime

import (
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
