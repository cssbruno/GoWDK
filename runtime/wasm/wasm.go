// Package wasm provides helpers for browser WASM island exports.
package wasm

import "errors"

// ErrUnavailable reports that the helper was called outside a js/wasm build.
var ErrUnavailable = errors.New("gowdk wasm helpers are only available in js/wasm")

// Payload is the JSON payload made available to a WASM island export.
type Payload struct {
	ABIVersion  string            `json:"abiVersion,omitempty"`
	Component   string            `json:"component,omitempty"`
	Event       string            `json:"event,omitempty"`
	Binding     string            `json:"binding,omitempty"`
	Traceparent string            `json:"traceparent,omitempty"`
	Detail      map[string]any    `json:"detail,omitempty"`
	State       map[string]any    `json:"state,omitempty"`
	Stores      []string          `json:"stores,omitempty"`
	Props       map[string]any    `json:"props,omitempty"`
	Emits       map[string]any    `json:"emits,omitempty"`
	Refs        map[string]string `json:"refs,omitempty"`
	Bindings    map[string]any    `json:"bindings,omitempty"`
}

// Result is the JSON result shape consumed by the WASM island loader.
type Result struct {
	Patches any            `json:"patches,omitempty"`
	Stores  map[string]any `json:"stores,omitempty"`
}
