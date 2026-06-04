package actions

import (
	"context"

	"github.com/cssbruno/gowdk/runtime/form"
	"github.com/cssbruno/gowdk/runtime/response"
)

// Handler is a generated typed action endpoint.
type Handler func(context.Context, form.Values) (response.Response, error)

// Registry maps generated action names to handlers.
type Registry map[string]Handler

// Register stores one action handler.
func (registry Registry) Register(name string, handler Handler) {
	registry[name] = handler
}
